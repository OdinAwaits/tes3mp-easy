package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var sessionTestNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func TestSessionStoreCreateGetRotateDestroy(t *testing.T) {
	store := newSessionStore()
	session, err := store.create(sessionTestNow, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if session.ID == "" || session.CSRF == "" || session.ID == session.CSRF {
		t.Fatalf("session tokens = %+v", session)
	}
	if !session.MustChange {
		t.Fatal("MustChange = false, want true")
	}

	later := sessionTestNow.Add(time.Minute)
	got, ok := store.get(session.ID, later)
	if !ok {
		t.Fatal("get returned false")
	}
	if got.CSRF != session.CSRF || got.Created != sessionTestNow ||
		got.LastSeen != later || !got.MustChange {
		t.Fatalf("session = %+v", got)
	}

	rotated, ok, err := store.rotate(session.ID, later.Add(time.Minute), false)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if !ok {
		t.Fatal("rotate returned false")
	}
	if rotated.ID == session.ID || rotated.CSRF == session.CSRF {
		t.Fatal("rotate reused a session ID or CSRF token")
	}
	if rotated.MustChange {
		t.Fatal("rotated MustChange = true")
	}
	if _, ok := store.get(session.ID, later); ok {
		t.Fatal("old session is still valid after rotate")
	}

	store.destroy(rotated.ID)
	if _, ok := store.get(rotated.ID, later); ok {
		t.Fatal("destroyed session is still valid")
	}
}

func TestSessionStoreExpiry(t *testing.T) {
	t.Run("idle timeout", func(t *testing.T) {
		store := newSessionStore()
		session, err := store.create(sessionTestNow, false)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, ok := store.get(session.ID,
			sessionTestNow.Add(sessionIdleTimeout)); !ok {
			t.Fatal("session expired exactly at the idle timeout")
		}
		if _, ok := store.get(session.ID,
			sessionTestNow.Add(2*sessionIdleTimeout+time.Second)); ok {
			t.Fatal("session survived the idle timeout")
		}
	})

	t.Run("absolute limit", func(t *testing.T) {
		store := newSessionStore()
		session, err := store.create(sessionTestNow, false)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		// Keep the idle timestamp recent so only the absolute limit can expire
		// the session.
		lastSeen := sessionTestNow.Add(sessionAbsoluteLimit - time.Hour)
		store.mu.Lock()
		store.sessions[session.ID].lastSeen = lastSeen
		store.mu.Unlock()

		if _, ok := store.get(session.ID, lastSeen.Add(30*time.Minute)); !ok {
			t.Fatal("session expired before the absolute limit")
		}
		if _, ok := store.get(session.ID,
			sessionTestNow.Add(sessionAbsoluteLimit+time.Second)); ok {
			t.Fatal("session survived the absolute limit")
		}
	})
}

func TestSessionStoreDoesNotSurviveRestart(t *testing.T) {
	store := newSessionStore()
	session, err := store.create(sessionTestNow, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	restarted := newSessionStore()
	if _, ok := restarted.get(session.ID, sessionTestNow); ok {
		t.Fatal("new session store accepted a session from the previous process")
	}
}

func TestLoginCSRFIsOneTimeAndExpires(t *testing.T) {
	store := newSessionStore()
	token, err := store.issueLoginCSRF(sessionTestNow)
	if err != nil {
		t.Fatalf("issueLoginCSRF: %v", err)
	}
	if token == "" {
		t.Fatal("empty CSRF token")
	}
	if store.consumeLoginCSRF(token, "wrong", sessionTestNow) {
		t.Fatal("wrong form token was accepted")
	}
	if !store.consumeLoginCSRF(token, token, sessionTestNow.Add(time.Minute)) {
		t.Fatal("valid CSRF token was rejected")
	}
	if store.consumeLoginCSRF(token, token, sessionTestNow.Add(time.Minute)) {
		t.Fatal("CSRF token was accepted twice")
	}

	expired, err := store.issueLoginCSRF(sessionTestNow)
	if err != nil {
		t.Fatalf("issueLoginCSRF: %v", err)
	}
	if store.consumeLoginCSRF(expired, expired, sessionTestNow.Add(loginCSRFLifetime)) {
		t.Fatal("expired CSRF token was accepted")
	}
}

func TestSessionCookiesAreSecure(t *testing.T) {
	t.Run("session", func(t *testing.T) {
		rec := httptest.NewRecorder()
		setSessionCookie(rec, "session-value")
		cookie := responseCookie(t, rec, sessionCookieName)
		if cookie.Value != "session-value" || cookie.Path != "/" {
			t.Fatalf("cookie = %+v", cookie)
		}
		if !cookie.HttpOnly || !cookie.Secure ||
			cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie flags = %+v", cookie)
		}
	})

	t.Run("login csrf", func(t *testing.T) {
		rec := httptest.NewRecorder()
		setLoginCSRFCookie(rec, "csrf-value", sessionTestNow)
		cookie := responseCookie(t, rec, loginCSRFCookieName)
		if cookie.Value != "csrf-value" || cookie.Path != "/admin/login" {
			t.Fatalf("cookie = %+v", cookie)
		}
		if !cookie.HttpOnly || !cookie.Secure ||
			cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie flags = %+v", cookie)
		}
		if cookie.MaxAge != int(loginCSRFLifetime/time.Second) ||
			!cookie.Expires.Equal(sessionTestNow.Add(loginCSRFLifetime)) {
			t.Fatalf("cookie lifetime = %+v", cookie)
		}
	})

	t.Run("clear", func(t *testing.T) {
		rec := httptest.NewRecorder()
		clearSessionCookie(rec)
		cookie := responseCookie(t, rec, sessionCookieName)
		if cookie.Value != "" || cookie.MaxAge >= 0 {
			t.Fatalf("clear cookie = %+v", cookie)
		}
	})
}

func responseCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("Set-Cookie %s not found in %v", name, rec.Header().Values("Set-Cookie"))
	return nil
}

func TestTokensEqual(t *testing.T) {
	if !tokensEqual("same", "same") {
		t.Fatal("equal tokens rejected")
	}
	if tokensEqual("same", "different") || tokensEqual("different", "same") {
		t.Fatal("different tokens accepted")
	}
}
