package panel

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func directSession(t *testing.T, env *routeTestEnv, mustChange bool) (sessionState, *http.Cookie) {
	t.Helper()
	session, err := env.h.sessions.create(sessionTestNow, mustChange)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session, &http.Cookie{Name: sessionCookieName, Value: session.ID}
}

func TestLoginRejectsWrongPasswordAndMissingCSRF(t *testing.T) {
	env := newRouteTestEnv(t)

	_, csrfCookie := loginFormToken(t, env)
	rec := panelRequest(t, env.handler, http.MethodPost, "/admin/login",
		url.Values{"password": {env.password}},
		[]*http.Cookie{csrfCookie}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("login without CSRF = %d, want 403", rec.Code)
	}
	if hasResponseCookie(rec, sessionCookieName) {
		t.Fatal("failed CSRF check created a session")
	}

	token, csrfCookie := loginFormToken(t, env)
	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/login",
		url.Values{csrfFormField: {token}, "password": {"definitely wrong"}},
		[]*http.Cookie{csrfCookie}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Неверный пароль") {
		t.Fatal("wrong-password page has no error message")
	}
	if hasResponseCookie(rec, sessionCookieName) {
		t.Fatal("wrong password created a session")
	}

	token, csrfCookie = loginFormToken(t, env)
	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/login",
		url.Values{csrfFormField: {token}, "password": {env.password}},
		[]*http.Cookie{csrfCookie}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login after one failure = %d, want 303", rec.Code)
	}
	if !hasResponseCookie(rec, sessionCookieName) {
		t.Fatal("successful login did not set a session cookie")
	}
}

func TestLoginRateLimit(t *testing.T) {
	env := newRouteTestEnv(t)
	for i := 0; i < maxLoginAttempts; i++ {
		token, csrfCookie := loginFormToken(t, env)
		rec := panelRequest(t, env.handler, http.MethodPost, "/admin/login",
			url.Values{csrfFormField: {token}, "password": {"wrong password"}},
			[]*http.Cookie{csrfCookie}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d = %d, want 401", i+1, rec.Code)
		}
	}

	token, csrfCookie := loginFormToken(t, env)
	rec := panelRequest(t, env.handler, http.MethodPost, "/admin/login",
		url.Values{csrfFormField: {token}, "password": {env.password}},
		[]*http.Cookie{csrfCookie}, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("login after failures = %d, want 429", rec.Code)
	}
	retry, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retry < 1 {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "Слишком много") {
		t.Fatal("rate-limit page has no message")
	}
}

func TestAdminAcceptsCSRFHeader(t *testing.T) {
	env := newRouteTestEnv(t)
	session, cookie := directSession(t, env, false)

	rec := panelRequest(t, env.handler, http.MethodPost, "/admin/logout",
		nil, []*http.Cookie{cookie}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logout without token = %d, want 403", rec.Code)
	}

	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/logout",
		nil, []*http.Cookie{cookie},
		map[string]string{csrfHeaderName: session.CSRF})
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("logout with header = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if _, ok := env.h.sessions.get(session.ID, sessionTestNow); ok {
		t.Fatal("header-based logout did not destroy the session")
	}
}

func TestAdminSessionExpires(t *testing.T) {
	env := newRouteTestEnv(t)
	_, cookie := directSession(t, env, false)

	rec := panelRequest(t, env.handler, http.MethodGet, "/admin",
		nil, []*http.Cookie{cookie}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin = %d, want 200", rec.Code)
	}

	*env.clock = sessionTestNow.Add(sessionIdleTimeout + time.Minute)
	rec = panelRequest(t, env.handler, http.MethodGet, "/admin",
		nil, []*http.Cookie{cookie}, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("expired session = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if cleared := responseCookie(t, rec, sessionCookieName); cleared.MaxAge >= 0 {
		t.Fatalf("expired session cookie was not cleared: %+v", cleared)
	}
}

func TestLoginPageRedirectsAuthenticatedSession(t *testing.T) {
	env := newRouteTestEnv(t)
	tests := []struct {
		name       string
		mustChange bool
		want       string
	}{
		{name: "ready", want: "/admin"},
		{name: "must change", mustChange: true, want: "/admin/password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cookie := directSession(t, env, tt.mustChange)
			rec := panelRequest(t, env.handler, http.MethodGet, "/admin/login",
				nil, []*http.Cookie{cookie}, nil)
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.want {
				t.Fatalf("GET /admin/login = %d %q", rec.Code, rec.Header().Get("Location"))
			}
		})
	}
}

func TestPasswordPageValidatesInputBeforeHashing(t *testing.T) {
	env := newRouteTestEnv(t)
	_, cookie := directSession(t, env, false)
	rec := panelRequest(t, env.handler, http.MethodGet, "/admin/password",
		nil, []*http.Cookie{cookie}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/password = %d", rec.Code)
	}
	csrf := csrfInputToken(t, rec.Body.String())

	tests := []struct {
		name   string
		form   url.Values
		status int
	}{
		{
			name: "mismatch",
			form: url.Values{
				csrfFormField:      {csrf},
				"current_password": {"current password"},
				"new_password":     {"new long password"},
				"confirm_password": {"different long password"},
			},
			status: http.StatusBadRequest,
		},
		{
			name: "short",
			form: url.Values{
				csrfFormField:      {csrf},
				"current_password": {"current password"},
				"new_password":     {"short"},
				"confirm_password": {"short"},
			},
			status: http.StatusBadRequest,
		},
		{
			name: "unchanged",
			form: url.Values{
				csrfFormField:      {csrf},
				"current_password": {"the same password"},
				"new_password":     {"the same password"},
				"confirm_password": {"the same password"},
			},
			status: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := panelRequest(t, env.handler, http.MethodPost, "/admin/password",
				tt.form, []*http.Cookie{cookie}, nil)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}

func hasResponseCookie(rec *httptest.ResponseRecorder, name string) bool {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return true
		}
	}
	return false
}
