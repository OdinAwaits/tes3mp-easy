package panel

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

var csrfInputPattern = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

type routeTestEnv struct {
	handler  http.Handler
	h        *panelHandler
	auth     *authStore
	password string
	clock    *time.Time
}

func newRouteTestEnv(t *testing.T) *routeTestEnv {
	t.Helper()
	cfg := testConfig(t)
	auth, err := openAuthStore(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("openAuthStore: %v", err)
	}
	password, err := readAuthFile(pathsForAuth(cfg).initial)
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	now := sessionTestNow
	h, err := newPanelHandler(cfg, auth, nil, log.New(io.Discard, "", 0), func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("newPanelHandler: %v", err)
	}
	return &routeTestEnv{
		handler:  withSecurityHeaders(h.routes()),
		h:        h,
		auth:     auth,
		password: strings.TrimSpace(string(password)),
		clock:    &now,
	}
}

func panelRequest(t *testing.T, handler http.Handler, method, path string,
	form url.Values, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, "https://panel.test"+path, body)
	req.RemoteAddr = "203.0.113.9:4321"
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func loginFormToken(t *testing.T, env *routeTestEnv) (string, *http.Cookie) {
	t.Helper()
	rec := panelRequest(t, env.handler, http.MethodGet, "/admin/login", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/login = %d, want 200", rec.Code)
	}
	return csrfInputToken(t, rec.Body.String()),
		responseCookie(t, rec, loginCSRFCookieName)
}

func csrfInputToken(t *testing.T, body string) string {
	t.Helper()
	match := csrfInputPattern.FindStringSubmatch(body)
	if len(match) != 2 || match[1] == "" {
		t.Fatalf("CSRF input not found in body:\n%s", body)
	}
	return match[1]
}

func TestAdminAuthenticationAndPasswordChange(t *testing.T) {
	env := newRouteTestEnv(t)

	rec := panelRequest(t, env.handler, http.MethodGet, "/admin", nil, nil, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("anonymous GET /admin = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}

	token, csrfCookie := loginFormToken(t, env)
	rec = panelRequest(t, env.handler, http.MethodGet, "/admin/login", nil, nil, nil)
	assertSecurityHeaders(t, rec.Header())
	if !strings.Contains(rec.Body.String(), `action="/admin/login"`) {
		t.Fatal("login page has no form")
	}

	loginRec := panelRequest(t, env.handler, http.MethodPost, "/admin/login",
		url.Values{csrfFormField: {token}, "password": {env.password}},
		[]*http.Cookie{csrfCookie}, nil)
	if loginRec.Code != http.StatusSeeOther ||
		loginRec.Header().Get("Location") != "/admin/password" {
		t.Fatalf("login = %d %q", loginRec.Code, loginRec.Header().Get("Location"))
	}
	sessionCookie := responseCookie(t, loginRec, sessionCookieName)
	if sessionCookie.Value == "" {
		t.Fatal("login did not set a session cookie")
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/admin",
		nil, []*http.Cookie{sessionCookie}, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/password" {
		t.Fatalf("GET /admin before password change = %d %q",
			rec.Code, rec.Header().Get("Location"))
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/admin/password",
		nil, []*http.Cookie{sessionCookie}, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "temporary password") {
		t.Fatalf("GET /admin/password = %d", rec.Code)
	}
	passwordCSRF := csrfInputToken(t, rec.Body.String())

	newPassword := "a new long panel password"
	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/password",
		url.Values{
			"current_password": {env.password},
			"new_password":     {newPassword},
			"confirm_password": {newPassword},
		}, []*http.Cookie{sessionCookie}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("password change without CSRF = %d, want 403", rec.Code)
	}

	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/password",
		url.Values{
			csrfFormField:      {passwordCSRF},
			"current_password": {"wrong current password"},
			"new_password":     {newPassword},
			"confirm_password": {newPassword},
		}, []*http.Cookie{sessionCookie}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("password change with wrong current = %d, want 401", rec.Code)
	}

	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/password",
		url.Values{
			csrfFormField:      {passwordCSRF},
			"current_password": {env.password},
			"new_password":     {newPassword},
			"confirm_password": {newPassword},
		}, []*http.Cookie{sessionCookie}, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin" {
		t.Fatalf("password change = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rotatedCookie := responseCookie(t, rec, sessionCookieName)
	if rotatedCookie.Value == sessionCookie.Value {
		t.Fatal("password change did not rotate the session")
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/admin",
		nil, []*http.Cookie{sessionCookie}, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("old session after password change = %d %q",
			rec.Code, rec.Header().Get("Location"))
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/admin",
		nil, []*http.Cookie{rotatedCookie}, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Панель запущена") {
		t.Fatalf("GET /admin = %d", rec.Code)
	}
	dashboardCSRF := csrfInputToken(t, rec.Body.String())
	if dashboardCSRF == passwordCSRF {
		t.Fatal("session rotation reused the CSRF token")
	}

	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/logout",
		nil, []*http.Cookie{rotatedCookie}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF = %d, want 403", rec.Code)
	}
	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/logout",
		url.Values{csrfFormField: {dashboardCSRF}},
		[]*http.Cookie{rotatedCookie}, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("logout = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = panelRequest(t, env.handler, http.MethodGet, "/admin",
		nil, []*http.Cookie{rotatedCookie}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("GET /admin after logout = %d, want 303", rec.Code)
	}
}
