package panel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

const maxAuthFormBytes = 16 << 10

type panelHandler struct {
	cfg       Config
	auth      *authStore
	ops       Ops
	sessions  *sessionStore
	limiter   *loginLimiter
	downloads *downloadLimiter
	templates pageTemplates
	logger    *log.Logger
	now       func() time.Time
	metrics   *metricsStore
	language  *languageSetting

	doctorMu     sync.Mutex
	doctorReport Doctor
	doctorErr    error
	doctorAt     time.Time
	doctorValid  bool
}

type loginPageData struct {
	pageI18n
	CSRF  string
	Error string
	Nav   string
}

type passwordPageData struct {
	pageI18n
	CSRF       string
	Path       string
	Error      string
	MustChange bool
	Nav        string
}

type authenticatedHandler func(http.ResponseWriter, *http.Request, sessionState)

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
	normalized := cfg.Normalize()
	language, err := openLanguageSetting(normalized.DataDir,
		i18n.Language(normalized.Language), logger)
	if err != nil {
		return nil, err
	}
	return &panelHandler{
		cfg:       normalized,
		auth:      auth,
		ops:       opsClient,
		sessions:  newSessionStore(),
		limiter:   newLoginLimiter(),
		downloads: newDownloadLimiter(),
		templates: templates,
		logger:    logger,
		now:       now,
		metrics:   newMetricsStore(cfg.Root),
		language:  language,
	}, nil
}

func (h *panelHandler) routes() http.Handler {
	mux := http.NewServeMux()
	for _, route := range allPanelRoutes() {
		handler := route.handle(h)
		if route.csrf == csrfStream {
			handler = withStreamCSRF(handler)
		}
		mux.Handle(route.pattern, handler)
	}
	return mux
}

// routeCSRF describes how the CSRF token is validated for a route.
type routeCSRF int

const (
	csrfForm routeCSRF = iota
	csrfStream
)

// panelRoute is the single source of truth for panel URLs. Registration,
// active navigation and the CSRF strategy live in one table, so a new page
// cannot drift away from the navigation or from the middleware.
type panelRoute struct {
	pattern string
	nav     string
	csrf    routeCSRF
	handle  func(*panelHandler) http.Handler
}

var (
	panelRoutesOnce sync.Once
	panelRouteTable []panelRoute
)

func allPanelRoutes() []panelRoute {
	panelRoutesOnce.Do(func() {
		panelRouteTable = []panelRoute{
			{pattern: "GET /{$}", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleHome) }},
			{pattern: "GET /healthz", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(handleHealthz) }},
			{pattern: "GET /backups", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleBackups) }},
			{pattern: "GET /backups/mods/{name}", handle: func(h *panelHandler) http.Handler { return h.handleBackupDownload("mods") }},
			{pattern: "GET /backups/state/{name}", handle: func(h *panelHandler) http.Handler { return h.handleBackupDownload("state") }},
			{pattern: "GET /backups/state/{name}/bundle", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleStateBundle) }},
			{pattern: "GET /api/v1/server", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleServerInfo) }},
			{pattern: "GET /api/v1/mods/current", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleModsCurrent) }},
			{pattern: "GET /api/v1/mods/current/archive", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleModsArchive) }},
			{pattern: "GET /admin/login", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleLoginGet) }},
			{pattern: "POST /admin/login", handle: func(h *panelHandler) http.Handler { return http.HandlerFunc(h.handleLoginPost) }},
			{pattern: "POST /admin/logout", handle: func(h *panelHandler) http.Handler { return h.authenticated(true, h.handleLogout) }},
			{pattern: "POST /admin/language", handle: func(h *panelHandler) http.Handler { return h.authenticated(true, h.handleLanguage) }},
			{pattern: "GET /admin/password", nav: "password", handle: func(h *panelHandler) http.Handler { return h.authenticated(true, h.handlePasswordGet) }},
			{pattern: "POST /admin/password", nav: "password", handle: func(h *panelHandler) http.Handler { return h.authenticated(true, h.handlePasswordPost) }},
			{pattern: "GET /admin", nav: "dashboard", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleDashboard) }},
			{pattern: "POST /admin/server/{action}", nav: "dashboard", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleServerAction) }},
			{pattern: "GET /admin/logs", nav: "logs", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleLogs) }},
			{pattern: "GET /admin/mods", nav: "mods", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleMods) }},
			{pattern: "POST /admin/mods/upload", nav: "mods", csrf: csrfStream, handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleModsUpload) }},
			{pattern: "POST /admin/mods/deploy", nav: "mods", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleModsDeploy) }},
			{pattern: "POST /admin/mods/rollback", nav: "mods", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleModsRollback) }},
			{pattern: "GET /admin/mods/download/{name}", nav: "mods", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleModsDownload) }},
			{pattern: "GET /admin/state", nav: "state", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleState) }},
			{pattern: "POST /admin/state/create", nav: "state", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleStateCreate) }},
			{pattern: "POST /admin/state/restore", nav: "state", csrf: csrfStream, handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleStateRestore) }},
			{pattern: "POST /admin/state/deploy", nav: "state", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleStateDeploy) }},
			{pattern: "GET /admin/state/download/{name}", nav: "state", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleStateDownload) }},
			{pattern: "GET /admin/config", nav: "config", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleConfig) }},
			{pattern: "POST /admin/config/{file}", nav: "config", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleConfigPost) }},
			{pattern: "GET /admin/staff", nav: "staff", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleStaff) }},
			{pattern: "POST /admin/staff/{player}", nav: "staff", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleStaffSet) }},
			{pattern: "GET /api/v1/admin/logs", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleLogsAPI) }},
			{pattern: "GET /api/v1/admin/status", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleAdminStatus) }},
			{pattern: "GET /api/v1/admin/metrics", handle: func(h *panelHandler) http.Handler { return h.authenticated(false, h.handleAdminMetrics) }},
			{pattern: "GET /static/", handle: func(h *panelHandler) http.Handler { return h.staticFiles() }},
		}
	})
	return panelRouteTable
}

// adminActiveNav resolves the navigation item for an admin URL from the route
// table, so routes and navigation cannot get out of sync.
func adminActiveNav(path string) string {
	for _, route := range allPanelRoutes() {
		if route.nav == "" {
			continue
		}
		if patternMatchesPath(route.pattern, path) {
			return route.nav
		}
	}
	return ""
}

func patternMatchesPath(pattern, path string) bool {
	if _, rest, ok := strings.Cut(pattern, " "); ok {
		pattern = rest
	}
	if pattern == "/" || pattern == "/{$}" {
		return path == "" || path == "/"
	}
	pattern = strings.TrimSuffix(pattern, "/")
	path = strings.TrimSuffix(path, "/")
	if !strings.HasPrefix(pattern, "/") || !strings.HasPrefix(path, "/") {
		return false
	}
	patternSegments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	pathSegments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(patternSegments) != len(pathSegments) {
		return false
	}
	for index, segment := range patternSegments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			if pathSegments[index] == "" {
				return false
			}
			continue
		}
		if segment != pathSegments[index] {
			return false
		}
	}
	return true
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
			h.tr().T(i18n.KeyPanelAuthRateLimited))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, r, http.StatusBadRequest, h.tr().T(i18n.KeyPanelAuthBadForm))
		return
	}

	var cookieValue string
	if cookie, err := r.Cookie(loginCSRFCookieName); err == nil {
		cookieValue = cookie.Value
	}
	if !h.sessions.consumeLoginCSRF(cookieValue, r.PostForm.Get(csrfFormField), now) {
		clearLoginCSRFCookie(w)
		h.renderLogin(w, r, http.StatusForbidden,
			h.tr().T(i18n.KeyPanelAuthStaleForm))
		return
	}

	password := r.PostForm.Get("password")
	if password == "" {
		h.limiter.fail(limitKey, now)
		h.logger.Printf("auth: empty password from %s", ip)
		h.renderLogin(w, r, http.StatusUnauthorized, h.tr().T(i18n.KeyPanelAuthPasswordRequired))
		return
	}
	if !h.auth.verify(password) {
		h.limiter.fail(limitKey, now)
		h.logger.Printf("auth: failed login from %s", ip)
		h.renderLogin(w, r, http.StatusUnauthorized, h.tr().T(i18n.KeyPanelAuthPasswordInvalid))
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
	h.render(w, status, "login.html", loginPageData{
		pageI18n: h.pageI18n(),
		CSRF:     token,
		Error:    message,
	})
}

func (h *panelHandler) render(w http.ResponseWriter, status int, name string, data any) {
	page, ok := h.templates[name]
	if !ok {
		h.logger.Printf("panel: render %s: unknown template", name)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var body bytes.Buffer
	if err := page.ExecuteTemplate(&body, "layout", data); err != nil {
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
			// Streaming multipart uploads declare csrfStream in panelRoutes:
			// their handler validates the field while reading the body.
			if streamingCSRF(r) {
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

type streamCSRFKey struct{}

// withStreamCSRF marks routes that read the CSRF field directly from a
// multipart stream. The middleware then only rejects a supplied but invalid
// header token instead of buffering the upload body; the handler validates the
// field while it reads the stream.
func withStreamCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), streamCSRFKey{}, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func streamingCSRF(r *http.Request) bool {
	value, _ := r.Context().Value(streamCSRFKey{}).(bool)
	return value
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
			h.tr().T(i18n.KeyPanelAuthRateLimitedChange))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderPassword(w, session, http.StatusBadRequest, h.tr().T(i18n.KeyPanelAuthBadFormChange))
		return
	}

	current := r.PostForm.Get("current_password")
	next := r.PostForm.Get("new_password")
	confirm := r.PostForm.Get("confirm_password")
	switch {
	case next != confirm:
		h.renderPassword(w, session, http.StatusBadRequest, h.tr().T(i18n.KeyPanelAuthPasswordMismatch))
		return
	case next == current:
		h.renderPassword(w, session, http.StatusBadRequest,
			h.tr().T(i18n.KeyPanelAuthPasswordSame))
		return
	case validateNewPassword(next) != nil:
		message := h.tr().T(i18n.KeyPanelAuthPasswordShort)
		if len(next) > maxPasswordBytes {
			message = h.tr().T(i18n.KeyPanelAuthPasswordLong)
		}
		h.renderPassword(w, session, http.StatusBadRequest, message)
		return
	}

	if err := h.auth.changePassword(current, next); err != nil {
		if errors.Is(err, errCurrentPassword) {
			h.limiter.fail(limitKey, now)
			h.logger.Printf("auth: failed password change from %s", ip)
			h.renderPassword(w, session, http.StatusUnauthorized, h.tr().T(i18n.KeyPanelAuthPasswordWrong))
			return
		}
		h.logger.Printf("auth: change password from %s: %v", ip, err)
		h.renderPassword(w, session, http.StatusInternalServerError,
			h.tr().T(i18n.KeyPanelAuthPasswordSaveFail))
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
		pageI18n:   h.pageI18n(),
		CSRF:       session.CSRF,
		Path:       "/admin/password",
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

// handleLanguage switches the interface language of the whole panel. The
// choice is stored in panel-data/settings.json, so it survives a restart and
// every browser sees the same panel.
func (h *panelHandler) handleLanguage(w http.ResponseWriter, r *http.Request, session sessionState) {
	h.private(w)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	language, ok := i18n.Parse(r.PostForm.Get("language"))
	if !ok {
		http.Error(w, "unsupported language", http.StatusBadRequest)
		return
	}
	if err := h.language.Set(language); err != nil {
		h.logger.Printf("panel: set language: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("panel: language switched to %s from %s", language, clientIP(r))
	http.Redirect(w, r, languageReturnPath(r), http.StatusSeeOther)
}

// languageReturnPath keeps the operator on the page the selector was used on,
// but never leaves the panel: anything else falls back to the dashboard.
func languageReturnPath(r *http.Request) string {
	next := r.PostForm.Get("next")
	if !strings.HasPrefix(next, "/admin") || strings.Contains(next, "//") {
		return "/admin"
	}
	return next
}
