package panel

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	sessionCookieName    = "tes3mp_session"
	loginCSRFCookieName  = "tes3mp_login_csrf"
	csrfFormField        = "_csrf"
	csrfHeaderName       = "X-CSRF-Token"
	sessionIdleTimeout   = 12 * time.Hour
	sessionAbsoluteLimit = 7 * 24 * time.Hour
	loginCSRFLifetime    = 10 * time.Minute
	maxLoginCSRFTokens   = 4096
	sessionTokenBytes    = 32
)

type sessionState struct {
	ID         string
	CSRF       string
	Created    time.Time
	LastSeen   time.Time
	MustChange bool
}

type sessionRecord struct {
	csrf       string
	created    time.Time
	lastSeen   time.Time
	mustChange bool
}

// sessionStore is intentionally in memory: a panel restart logs the operator
// out, but no session material is written to disk.
type sessionStore struct {
	mu          sync.Mutex
	sessions    map[string]*sessionRecord
	loginTokens map[string]time.Time
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions:    make(map[string]*sessionRecord),
		loginTokens: make(map[string]time.Time),
	}
}

func (s *sessionStore) create(now time.Time, mustChange bool) (sessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	id, err := uniqueToken(s.sessions, sessionTokenBytes)
	if err != nil {
		return sessionState{}, err
	}
	csrf, err := randomToken(sessionTokenBytes)
	if err != nil {
		return sessionState{}, err
	}
	record := &sessionRecord{
		csrf:       csrf,
		created:    now,
		lastSeen:   now,
		mustChange: mustChange,
	}
	s.sessions[id] = record
	return sessionState{
		ID:         id,
		CSRF:       csrf,
		Created:    now,
		LastSeen:   now,
		MustChange: mustChange,
	}, nil
}

// get validates and touches a session. Expired sessions are deleted eagerly so
// an abandoned browser tab cannot keep memory alive indefinitely.
func (s *sessionStore) get(id string, now time.Time) (sessionState, bool) {
	if id == "" {
		return sessionState{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.sessions[id]
	if !ok {
		return sessionState{}, false
	}
	if sessionExpired(record, now) {
		delete(s.sessions, id)
		return sessionState{}, false
	}
	record.lastSeen = now
	return stateFromRecord(id, record), true
}

// rotate replaces a session ID and CSRF token after a password change. The
// caller sets the forced-password-change flag for the replacement session.
func (s *sessionStore) rotate(id string, now time.Time, mustChange bool) (sessionState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.sessions[id]
	if !ok {
		return sessionState{}, false, nil
	}
	if sessionExpired(record, now) {
		delete(s.sessions, id)
		return sessionState{}, false, nil
	}

	newID, err := uniqueToken(s.sessions, sessionTokenBytes)
	if err != nil {
		return sessionState{}, false, err
	}
	newCSRF, err := randomToken(sessionTokenBytes)
	if err != nil {
		return sessionState{}, false, err
	}
	delete(s.sessions, id)
	next := &sessionRecord{
		csrf:       newCSRF,
		created:    now,
		lastSeen:   now,
		mustChange: mustChange,
	}
	s.sessions[newID] = next
	return stateFromRecord(newID, next), true, nil
}

func (s *sessionStore) destroy(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// issueLoginCSRF returns a one-time token for the login form. The token is both
// stored in an HttpOnly cookie and embedded in the form so a cross-site POST
// cannot obtain or replay it.
func (s *sessionStore) issueLoginCSRF(now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLoginLocked(now)
	if len(s.loginTokens) >= maxLoginCSRFTokens {
		s.evictOldestLoginLocked()
	}

	var token string
	for {
		var err error
		token, err = randomToken(sessionTokenBytes)
		if err != nil {
			return "", err
		}
		if _, exists := s.loginTokens[token]; !exists {
			break
		}
	}
	s.loginTokens[token] = now.Add(loginCSRFLifetime)
	return token, nil
}

func (s *sessionStore) consumeLoginCSRF(cookieValue, formValue string, now time.Time) bool {
	if cookieValue == "" || formValue == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.loginTokens[cookieValue]
	if !ok {
		return false
	}
	if !tokensEqual(cookieValue, formValue) {
		return false
	}
	delete(s.loginTokens, cookieValue)
	if !now.Before(expires) {
		return false
	}
	return true
}

func (s *sessionStore) cleanupLocked(now time.Time) {
	for id, record := range s.sessions {
		if sessionExpired(record, now) {
			delete(s.sessions, id)
		}
	}
	s.cleanupLoginLocked(now)
}

func (s *sessionStore) cleanupLoginLocked(now time.Time) {
	for token, expires := range s.loginTokens {
		if !now.Before(expires) {
			delete(s.loginTokens, token)
		}
	}
}

func (s *sessionStore) evictOldestLoginLocked() {
	var oldestToken string
	var oldestExpiry time.Time
	for token, expires := range s.loginTokens {
		if oldestToken == "" || expires.Before(oldestExpiry) {
			oldestToken = token
			oldestExpiry = expires
		}
	}
	delete(s.loginTokens, oldestToken)
}

func sessionExpired(record *sessionRecord, now time.Time) bool {
	return now.Sub(record.lastSeen) > sessionIdleTimeout ||
		now.Sub(record.created) > sessionAbsoluteLimit
}

func stateFromRecord(id string, record *sessionRecord) sessionState {
	return sessionState{
		ID:         id,
		CSRF:       record.csrf,
		Created:    record.created,
		LastSeen:   record.lastSeen,
		MustChange: record.mustChange,
	}
}

func tokensEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func uniqueToken(existing map[string]*sessionRecord, size int) (string, error) {
	for {
		token, err := randomToken(size)
		if err != nil {
			return "", err
		}
		if _, exists := existing[token]; !exists {
			return token, nil
		}
	}
}

func setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func setLoginCSRFCookie(w http.ResponseWriter, token string, now time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     loginCSRFCookieName,
		Value:    token,
		Path:     "/admin/login",
		MaxAge:   int(loginCSRFLifetime / time.Second),
		Expires:  now.Add(loginCSRFLifetime),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearLoginCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     loginCSRFCookieName,
		Value:    "",
		Path:     "/admin/login",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
