package panel

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestPanelAuthContract(t *testing.T) {
	p := startPanel(t, nil)
	client := p.newClient()
	initial := p.initialPassword(t)

	resp := p.request(t, client, http.MethodGet, "/admin/login", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/login")
	requireContains(t, resp.Body, `action="/admin/login"`)
	loginCSRF := formToken(t, resp.Body)
	loginCookie := requireCookie(t, resp.Cookies, "tes3mp_login_csrf")
	requireCookieFlags(t, loginCookie, "/admin/login")
	if loginCookie.MaxAge <= 0 {
		t.Fatalf("login CSRF MaxAge = %d, want positive", loginCookie.MaxAge)
	}

	// Missing login CSRF is rejected before the password is checked.
	resp = p.request(t, client, http.MethodPost, "/admin/login", url.Values{
		"password": {initial},
	}, nil)
	requireStatus(t, resp, http.StatusForbidden, "POST /admin/login without CSRF")
	requireContains(t, resp.Body, "Форма устарела")

	// A wrong password keeps the session gate closed and is auditable.
	resp = p.request(t, client, http.MethodGet, "/admin/login", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/login again")
	loginCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/login", url.Values{
		"_csrf":    {loginCSRF},
		"password": {"definitely wrong"},
	}, nil)
	requireStatus(t, resp, http.StatusUnauthorized, "POST /admin/login wrong password")
	requireContains(t, resp.Body, "Неверный пароль")

	resp = p.request(t, client, http.MethodGet, "/admin/login", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/login before success")
	loginCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/login", url.Values{
		"_csrf":    {loginCSRF},
		"password": {initial},
	}, nil)
	requireStatus(t, resp, http.StatusSeeOther, "POST /admin/login")
	requireLocation(t, resp, "/admin/password")
	sessionCookie := requireCookie(t, resp.Cookies, "tes3mp_session")
	requireCookieFlags(t, sessionCookie, "/")
	if sessionCookie.MaxAge != 0 {
		t.Fatalf("session MaxAge = %d, want 0 (session cookie)", sessionCookie.MaxAge)
	}
	loginSessionID := sessionCookie.Value
	clearedLoginCookie := requireCookie(t, resp.Cookies, "tes3mp_login_csrf")
	if clearedLoginCookie.MaxAge >= 0 {
		t.Fatalf("login CSRF MaxAge = %d after login, want cleared", clearedLoginCookie.MaxAge)
	}

	// The forced-password-change gate blocks every other admin route.
	resp = p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusSeeOther, "GET /admin while must-change")
	requireLocation(t, resp, "/admin/password")

	const newPassword = "correct horse battery staple"

	resp = p.request(t, client, http.MethodGet, "/admin/password", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/password")
	passwordCSRF := formToken(t, resp.Body)

	resp = p.request(t, client, http.MethodPost, "/admin/password", url.Values{
		"_csrf":            {passwordCSRF},
		"current_password": {initial},
		"new_password":     {newPassword},
		"confirm_password": {"different password"},
	}, nil)
	requireStatus(t, resp, http.StatusBadRequest, "POST /admin/password mismatch")
	requireContains(t, resp.Body, "Пароли не совпадают")

	resp = p.request(t, client, http.MethodGet, "/admin/password", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/password before short")
	passwordCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/password", url.Values{
		"_csrf":            {passwordCSRF},
		"current_password": {initial},
		"new_password":     {"short"},
		"confirm_password": {"short"},
	}, nil)
	requireStatus(t, resp, http.StatusBadRequest, "POST /admin/password short")
	requireContains(t, resp.Body, "не менее 12")

	resp = p.request(t, client, http.MethodGet, "/admin/password", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/password before wrong current")
	passwordCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/password", url.Values{
		"_csrf":            {passwordCSRF},
		"current_password": {"not the current password"},
		"new_password":     {newPassword},
		"confirm_password": {newPassword},
	}, nil)
	requireStatus(t, resp, http.StatusUnauthorized, "POST /admin/password wrong current")
	requireContains(t, resp.Body, "Текущий пароль неверен")

	resp = p.request(t, client, http.MethodGet, "/admin/password", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/password before success")
	passwordCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/password", url.Values{
		"_csrf":            {passwordCSRF},
		"current_password": {initial},
		"new_password":     {newPassword},
		"confirm_password": {newPassword},
	}, nil)
	requireStatus(t, resp, http.StatusSeeOther, "POST /admin/password")
	requireLocation(t, resp, "/admin")
	rotatedCookie := requireCookie(t, resp.Cookies, "tes3mp_session")
	if rotatedCookie.Value == loginSessionID {
		t.Fatal("session ID was not rotated after the password change")
	}

	initialPath := filepath.Join(p.root, "panel-data", "initial-password.txt")
	if _, err := os.Stat(initialPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("initial password file still exists: %v", err)
	}

	resp = p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin after password change")
	dashboardCSRF := formToken(t, resp.Body)

	// Unsafe admin requests require the session CSRF token.
	resp = p.request(t, client, http.MethodPost, "/admin/logout", nil, nil)
	requireStatus(t, resp, http.StatusForbidden, "POST /admin/logout without CSRF")
	resp = p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin after rejected logout")

	resp = p.request(t, client, http.MethodPost, "/admin/logout", nil,
		http.Header{"X-Csrf-Token": {dashboardCSRF}})
	requireStatus(t, resp, http.StatusSeeOther, "POST /admin/logout")
	requireLocation(t, resp, "/admin/login")
	clearedSession := requireCookie(t, resp.Cookies, "tes3mp_session")
	if clearedSession.MaxAge >= 0 {
		t.Fatalf("session MaxAge = %d after logout, want cleared", clearedSession.MaxAge)
	}
	resp = p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusSeeOther, "GET /admin after logout")
	requireLocation(t, resp, "/admin/login")

	// The temporary password is gone; only the new one authenticates.
	resp = p.request(t, client, http.MethodGet, "/admin/login", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/login after logout")
	loginCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/login", url.Values{
		"_csrf":    {loginCSRF},
		"password": {initial},
	}, nil)
	requireStatus(t, resp, http.StatusUnauthorized, "POST /admin/login with old password")

	resp = p.request(t, client, http.MethodGet, "/admin/login", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/login for new password")
	loginCSRF = formToken(t, resp.Body)
	resp = p.request(t, client, http.MethodPost, "/admin/login", url.Values{
		"_csrf":    {loginCSRF},
		"password": {newPassword},
	}, nil)
	requireStatus(t, resp, http.StatusSeeOther, "POST /admin/login with new password")
	requireLocation(t, resp, "/admin")
}

func requireCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %q cookie", name)
	return nil
}

func requireCookieFlags(t *testing.T, cookie *http.Cookie, path string) {
	t.Helper()
	switch {
	case !cookie.HttpOnly:
		t.Fatalf("%s cookie is not HttpOnly", cookie.Name)
	case !cookie.Secure:
		t.Fatalf("%s cookie is not Secure", cookie.Name)
	case cookie.SameSite != http.SameSiteStrictMode:
		t.Fatalf("%s SameSite = %v, want Strict", cookie.Name, cookie.SameSite)
	case cookie.Path != path:
		t.Fatalf("%s Path = %q, want %q", cookie.Name, cookie.Path, path)
	}
}
