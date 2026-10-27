package panel

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxAuthFormBytes = 16 << 10

type panelHandler struct {
	cfg       Config
	auth      *authStore
	ops       Ops
	sessions  *sessionStore
	limiter   *loginLimiter
	downloads *downloadLimiter
	templates *template.Template
	logger    *log.Logger
	now       func() time.Time
	metrics   *metricsStore

	doctorMu     sync.Mutex
	doctorReport Doctor
	doctorErr    error
	doctorAt     time.Time
	doctorValid  bool
}

type loginPageData struct {
	CSRF  string
	Error string
}

type passwordPageData struct {
	CSRF       string
	Error      string
	MustChange bool
}

type authenticatedHandler func(http.ResponseWriter, *http.Request, sessionState)

// newHandler builds all public and admin routes. A nil ops client is allowed for
// tests that only exercise authentication; public routes then report
// service_unavailable instead of pretending the server is running.
func newHandler(cfg Config, auth *authStore, opsClient Ops, logger *log.Logger,
	now func() time.Time,
) (http.Handler, error) {
	h, err := newPanelHandler(cfg, auth, opsClient, logger, now)
	if err != nil {
		return nil, err
	}
	return h.routes(), nil
}

func newPanelHandler(cfg Config, auth *authStore, opsClient Ops, logger *log.Logger,
	now func() time.Time,
) (*panelHandler, error) {
	if auth == nil {
		return nil, errors.New("panel auth store is nil")
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if now == nil {
		now = time.Now
	}
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	return &panelHandler{
		cfg:       cfg.Normalize(),
		auth:      auth,
		ops:       opsClient,
		sessions:  newSessionStore(),
		limiter:   newLoginLimiter(),
		downloads: newDownloadLimiter(),
		templates: templates,
		logger:    logger,
		now:       now,
		metrics:   newMetricsStore(cfg.Root),
	}, nil
}

func (h *panelHandler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.handleHome)
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /backups", h.handleBackups)
	mux.HandleFunc("GET /backups/mods/{name}", h.handleBackupDownload("mods"))
	mux.HandleFunc("GET /backups/state/{name}", h.handleBackupDownload("state"))
	mux.HandleFunc("GET /backups/checkpoints/{name}", h.handleBackupDownload("checkpoints"))
	mux.HandleFunc("GET /api/v1/server", h.handleServerInfo)
	mux.HandleFunc("GET /api/v1/mods/current", h.handleModsCurrent)
	mux.HandleFunc("GET /api/v1/mods/current/archive", h.handleModsArchive)
	mux.HandleFunc("GET /admin/login", h.handleLoginGet)
	mux.HandleFunc("POST /admin/login", h.handleLoginPost)
	mux.HandleFunc("POST /admin/logout", h.authenticated(true, h.handleLogout))
	mux.HandleFunc("GET /admin/password", h.authenticated(true, h.handlePasswordGet))
	mux.HandleFunc("POST /admin/password", h.authenticated(true, h.handlePasswordPost))
	mux.HandleFunc("GET /admin", h.authenticated(false, h.handleDashboard))
	mux.HandleFunc("POST /admin/server/{action}", h.authenticated(false, h.handleServerAction))
	mux.HandleFunc("GET /admin/logs", h.authenticated(false, h.handleLogs))
	mux.HandleFunc("GET /admin/mods", h.authenticated(false, h.handleMods))
	mux.HandleFunc("POST /admin/mods/upload", h.authenticated(false, h.handleModsUpload))
	mux.HandleFunc("POST /admin/mods/deploy", h.authenticated(false, h.handleModsDeploy))
	mux.HandleFunc("POST /admin/mods/rollback", h.authenticated(false, h.handleModsRollback))
	mux.HandleFunc("GET /admin/restore-points", h.authenticated(false, h.handleState))
	mux.HandleFunc("GET /admin/state", h.redirectToRestorePoints)
	mux.HandleFunc("POST /admin/restore-points/checkpoint/create", h.authenticated(false, h.handleStateCreate))
	mux.HandleFunc("POST /admin/restore-points/snapshot/create", h.authenticated(false, h.handleStateCreate))
	mux.HandleFunc("POST /admin/restore-points/restore", h.authenticated(false, h.handleStateDeploy))
	// Old POST addresses remain as aliases for bookmarks and tests; GET /admin/state
	// is a permanent redirect to the new page.
	mux.HandleFunc("POST /admin/state/create", h.authenticated(false, h.handleStateCreate))
	mux.HandleFunc("POST /admin/state/deploy", h.authenticated(false, h.handleStateDeploy))
	mux.HandleFunc("GET /admin/config", h.authenticated(false, h.handleConfig))
	mux.HandleFunc("POST /admin/config/{file}", h.authenticated(false, h.handleConfigPost))
	mux.HandleFunc("GET /admin/staff", h.authenticated(false, h.handleStaff))
	mux.HandleFunc("POST /admin/staff/{player}", h.authenticated(false, h.handleStaffSet))
	mux.HandleFunc("GET /api/v1/admin/logs", h.authenticated(false, h.handleLogsAPI))
	mux.HandleFunc("GET /api/v1/admin/status", h.authenticated(false, h.handleAdminStatus))
	mux.HandleFunc("GET /api/v1/admin/metrics", h.authenticated(false, h.handleAdminMetrics))
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	return mux
}

func (h *panelHandler) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	h.private(w)
	if session, ok := h.currentSession(w, r); ok {
		h.redirectAfterLogin(w, r, session)
		return
	}
	h.renderLogin(w, r, http.StatusOK, "")
}

func (h *panelHandler) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	h.private(w)
	now := h.now()
	ip := clientIP(r)
	limitKey := "login:" + ip
	if allowed, retry := h.limiter.allow(limitKey, now); !allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(retry))
		h.renderLogin(w, r, http.StatusTooManyRequests,
			"Слишком много попыток входа. Подождите и попробуйте снова.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, r, http.StatusBadRequest, "Некорректная форма входа.")
		return
	}

	var cookieValue string
	if cookie, err := r.Cookie(loginCSRFCookieName); err == nil {
		cookieValue = cookie.Value
	}
	if !h.sessions.consumeLoginCSRF(cookieValue, r.PostForm.Get(csrfFormField), now) {
		clearLoginCSRFCookie(w)
		h.renderLogin(w, r, http.StatusForbidden,
			"Форма устарела. Обновите страницу и попробуйте снова.")
		return
	}

	password := r.PostForm.Get("password")
	if password == "" {
		h.limiter.fail(limitKey, now)
		h.logger.Printf("auth: empty password from %s", ip)
		h.renderLogin(w, r, http.StatusUnauthorized, "Введите пароль.")
		return
	}
	if !h.auth.verify(password) {
		h.limiter.fail(limitKey, now)
		h.logger.Printf("auth: failed login from %s", ip)
		h.renderLogin(w, r, http.StatusUnauthorized, "Неверный пароль.")
		return
	}

	h.limiter.reset(limitKey)
	session, err := h.sessions.create(now, h.auth.mustChangePassword())
	if err != nil {
		h.logger.Printf("auth: create session: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, session.ID)
	clearLoginCSRFCookie(w)
	h.logger.Printf("auth: successful login from %s", ip)
	h.redirectAfterLogin(w, r, session)
}

func (h *panelHandler) redirectAfterLogin(w http.ResponseWriter, r *http.Request, session sessionState) {
	target := "/admin"
	if session.MustChange {
		target = "/admin/password"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (h *panelHandler) currentSession(w http.ResponseWriter, r *http.Request) (sessionState, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return sessionState{}, false
	}
	session, ok := h.sessions.get(cookie.Value, h.now())
	if !ok {
		clearSessionCookie(w)
		return sessionState{}, false
	}
	return session, true
}

func (h *panelHandler) renderLogin(w http.ResponseWriter, r *http.Request,
	status int, message string) {
	token, err := h.sessions.issueLoginCSRF(h.now())
	if err != nil {
		h.logger.Printf("auth: issue login CSRF token: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	setLoginCSRFCookie(w, token, h.now())
	h.render(w, status, "login.html", loginPageData{CSRF: token, Error: message})
}

func (h *panelHandler) render(w http.ResponseWriter, status int, name string, data any) {
	var body bytes.Buffer
	if err := h.templates.ExecuteTemplate(&body, name, data); err != nil {
		h.logger.Printf("panel: render %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func (h *panelHandler) private(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func retryAfterSeconds(retry time.Duration) string {
	seconds := int((retry + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if value := strings.TrimSpace(r.RemoteAddr); value != "" {
		return value
	}
	return "unknown"
}

// authenticated enforces a valid session, the forced-password-change gate and a
// CSRF token on unsafe methods. GET pages are protected by SameSite=Strict and
// the session cookie; every state-changing request also needs a token.
func (h *panelHandler) authenticated(allowMustChange bool, next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.private(w)
		cookie, err := r.Cookie(sessionCookieName)
		var session sessionState
		var ok bool
		if err == nil {
			session, ok = h.sessions.get(cookie.Value, h.now())
		}
		if !ok {
			clearSessionCookie(w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if !allowMustChange && session.MustChange {
			http.Redirect(w, r, "/admin/password", http.StatusSeeOther)
			return
		}
		if isUnsafeMethod(r.Method) {
			// Streaming multipart uploads cannot be parsed by the generic
			// CSRF middleware without buffering the whole body. The upload
			// handler validates the field while reading the stream.
			if isMultipartForm(r) && r.URL.Path == "/admin/mods/upload" {
				if token := r.Header.Get(csrfHeaderName); token != "" &&
					!tokensEqual(token, session.CSRF) {
					http.Error(w, "CSRF token is missing or invalid", http.StatusForbidden)
					return
				}
			} else if !h.validCSRF(w, r, session.CSRF) {
				http.Error(w, "CSRF token is missing or invalid", http.StatusForbidden)
				return
			}
		}
		next(w, r, session)
	}
}

func (h *panelHandler) validCSRF(w http.ResponseWriter, r *http.Request, expected string) bool {
	if token := r.Header.Get(csrfHeaderName); token != "" {
		return tokensEqual(token, expected)
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		// Treat a missing Content-Type as a form. This keeps simple clients
		// working while still requiring the secret token in the body.
		contentType = "application/x-www-form-urlencoded"
		r.Header.Set("Content-Type", contentType)
	}
	if !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		return false
	}
	limit := int64(maxAuthFormBytes)
	if strings.HasPrefix(r.URL.Path, "/admin/config/") {
		limit = maxConfigFormBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseForm(); err != nil {
		return false
	}
	return tokensEqual(r.PostForm.Get(csrfFormField), expected)
}

func isMultipartForm(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "multipart/form-data"
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func (h *panelHandler) handlePasswordGet(w http.ResponseWriter, _ *http.Request, session sessionState) {
	h.renderPassword(w, session, http.StatusOK, "")
}

func (h *panelHandler) handlePasswordPost(w http.ResponseWriter, r *http.Request, session sessionState) {
	now := h.now()
	ip := clientIP(r)
	limitKey := "password:" + ip
	if allowed, retry := h.limiter.allow(limitKey, now); !allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(retry))
		h.renderPassword(w, session, http.StatusTooManyRequests,
			"Слишком много попыток. Подождите и попробуйте снова.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderPassword(w, session, http.StatusBadRequest, "Некорректная форма.")
		return
	}

	current := r.PostForm.Get("current_password")
	next := r.PostForm.Get("new_password")
	confirm := r.PostForm.Get("confirm_password")
	switch {
	case next != confirm:
		h.renderPassword(w, session, http.StatusBadRequest, "Пароли не совпадают.")
		return
	case next == current:
		h.renderPassword(w, session, http.StatusBadRequest,
			"Новый пароль должен отличаться от текущего.")
		return
	case validateNewPassword(next) != nil:
		message := "Новый пароль должен содержать не менее 12 символов."
		if len(next) > maxPasswordBytes {
			message = "Новый пароль слишком длинный."
		}
		h.renderPassword(w, session, http.StatusBadRequest, message)
		return
	}

	if err := h.auth.changePassword(current, next); err != nil {
		if errors.Is(err, errCurrentPassword) {
			h.limiter.fail(limitKey, now)
			h.logger.Printf("auth: failed password change from %s", ip)
			h.renderPassword(w, session, http.StatusUnauthorized, "Текущий пароль неверен.")
			return
		}
		h.logger.Printf("auth: change password from %s: %v", ip, err)
		h.renderPassword(w, session, http.StatusInternalServerError,
			"Не удалось сохранить пароль.")
		return
	}
	h.limiter.reset(limitKey)

	newSession, rotated, err := h.sessions.rotate(session.ID, now, false)
	if err != nil || !rotated {
		h.logger.Printf("auth: rotate session after password change: rotated=%v err=%v", rotated, err)
		h.sessions.destroy(session.ID)
		newSession, err = h.sessions.create(now, false)
		if err != nil {
			h.logger.Printf("auth: create session after password change: %v", err)
			http.Error(w, "password changed; sign in again", http.StatusInternalServerError)
			return
		}
	}
	setSessionCookie(w, newSession.ID)
	h.logger.Printf("auth: password changed from %s", ip)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *panelHandler) renderPassword(w http.ResponseWriter, session sessionState,
	status int, message string) {
	h.render(w, status, "password.html", passwordPageData{
		CSRF:       session.CSRF,
		Error:      message,
		MustChange: session.MustChange,
	})
}

func (h *panelHandler) handleLogout(w http.ResponseWriter, r *http.Request, session sessionState) {
	h.sessions.destroy(session.ID)
	clearSessionCookie(w)
	h.logger.Printf("auth: logout from %s", clientIP(r))
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}
