package panel

import (
	"sync"
	"time"
)

const (
	maxLoginAttempts   = 5
	loginAttemptWindow = 15 * time.Minute
	maxLoginClients    = 4096

	maxDownloadRequests = 60
	downloadRateWindow  = time.Minute
	maxDownloadClients  = 4096
)

// loginLimiter counts failed attempts per opaque key (normally a remote IP).
// It has a hard client cap so arbitrary remote addresses cannot grow the map
// without bound.
type loginLimiter struct {
	mu         sync.Mutex
	entries    map[string][]time.Time
	window     time.Duration
	maxPerKey  int
	maxClients int
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		entries:    make(map[string][]time.Time),
		window:     loginAttemptWindow,
		maxPerKey:  maxLoginAttempts,
		maxClients: maxLoginClients,
	}
}

func (l *loginLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempts := l.pruneLocked(key, now)
	if len(attempts) < l.maxPerKey {
		return true, 0
	}
	retry := attempts[0].Add(l.window).Sub(now)
	if retry < 0 {
		retry = 0
	}
	return false, retry
}

func (l *loginLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempts, exists := l.entries[key]
	attempts = pruneAttempts(attempts, now, l.window)
	if !exists && len(l.entries) >= l.maxClients {
		l.evictOldestLocked()
	}
	l.entries[key] = append(attempts, now)
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

func (l *loginLimiter) pruneLocked(key string, now time.Time) []time.Time {
	attempts, exists := l.entries[key]
	if !exists {
		return nil
	}
	attempts = pruneAttempts(attempts, now, l.window)
	if len(attempts) == 0 {
		delete(l.entries, key)
		return nil
	}
	l.entries[key] = attempts
	return attempts
}

func pruneAttempts(attempts []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	first := 0
	for first < len(attempts) && !attempts[first].After(cutoff) {
		first++
	}
	return attempts[first:]
}

func (l *loginLimiter) evictOldestLocked() {
	var oldestKey string
	var oldestLast time.Time
	for key, attempts := range l.entries {
		if len(attempts) == 0 {
			continue
		}
		last := attempts[len(attempts)-1]
		if oldestKey == "" || last.Before(oldestLast) {
			oldestKey = key
			oldestLast = last
		}
	}
	if oldestKey != "" {
		delete(l.entries, oldestKey)
	}
}

// downloadLimiter is a sliding-window request counter. Unlike loginLimiter it
// records successful requests too: the public backup/mod endpoints allow 60
// downloads per minute per client address.
type downloadLimiter struct {
	mu         sync.Mutex
	entries    map[string][]time.Time
	window     time.Duration
	maxPerKey  int
	maxClients int
}

func newDownloadLimiter() *downloadLimiter {
	return &downloadLimiter{
		entries:    make(map[string][]time.Time),
		window:     downloadRateWindow,
		maxPerKey:  maxDownloadRequests,
		maxClients: maxDownloadClients,
	}
}

func (l *downloadLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempts := pruneAttempts(l.entries[key], now, l.window)
	if len(attempts) >= l.maxPerKey {
		retry := attempts[0].Add(l.window).Sub(now)
		if retry < 0 {
			retry = 0
		}
		l.entries[key] = attempts
		return false, retry
	}

	if _, exists := l.entries[key]; !exists && len(l.entries) >= l.maxClients {
		l.evictOldestLocked()
	}
	l.entries[key] = append(attempts, now)
	return true, 0
}

func (l *downloadLimiter) evictOldestLocked() {
	var oldestKey string
	var oldestLast time.Time
	for key, attempts := range l.entries {
		if len(attempts) == 0 {
			continue
		}
		last := attempts[len(attempts)-1]
		if oldestKey == "" || last.Before(oldestLast) {
			oldestKey = key
			oldestLast = last
		}
	}
	if oldestKey != "" {
		delete(l.entries, oldestKey)
	}
}
