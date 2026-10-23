package panel

import (
	"testing"
	"time"
)

func TestLoginLimiterBlocksAfterFailures(t *testing.T) {
	limiter := newLoginLimiter()
	now := sessionTestNow
	for i := 0; i < maxLoginAttempts; i++ {
		if allowed, _ := limiter.allow("ip", now); !allowed {
			t.Fatalf("attempt %d was blocked early", i+1)
		}
		limiter.fail("ip", now)
	}

	allowed, retry := limiter.allow("ip", now)
	if allowed {
		t.Fatal("attempt after the limit was allowed")
	}
	if retry != loginAttemptWindow {
		t.Fatalf("retry = %v, want %v", retry, loginAttemptWindow)
	}
	if got := retryAfterSeconds(retry); got != "900" {
		t.Fatalf("Retry-After = %s, want 900", got)
	}

	if allowed, _ := limiter.allow("ip", now.Add(loginAttemptWindow+time.Second)); !allowed {
		t.Fatal("attempt after the window was blocked")
	}
}

func TestLoginLimiterReset(t *testing.T) {
	limiter := newLoginLimiter()
	for i := 0; i < maxLoginAttempts; i++ {
		limiter.fail("ip", sessionTestNow)
	}
	limiter.reset("ip")
	if allowed, _ := limiter.allow("ip", sessionTestNow); !allowed {
		t.Fatal("reset did not clear failures")
	}
}

func TestLoginLimiterCapsClients(t *testing.T) {
	limiter := &loginLimiter{
		entries:    make(map[string][]time.Time),
		window:     time.Minute,
		maxPerKey:  maxLoginAttempts,
		maxClients: 2,
	}
	limiter.fail("a", sessionTestNow)
	limiter.fail("b", sessionTestNow.Add(time.Second))
	limiter.fail("c", sessionTestNow.Add(2*time.Second))

	if len(limiter.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(limiter.entries))
	}
	if _, ok := limiter.entries["a"]; ok {
		t.Fatal("oldest client was not evicted")
	}
}

func TestDownloadLimiterCountsAndCapsClients(t *testing.T) {
	limiter := &downloadLimiter{
		entries:    make(map[string][]time.Time),
		window:     time.Minute,
		maxPerKey:  2,
		maxClients: 2,
	}
	now := sessionTestNow
	for i := 0; i < 2; i++ {
		if allowed, _ := limiter.allow("ip", now); !allowed {
			t.Fatalf("request %d was blocked", i+1)
		}
	}
	if allowed, retry := limiter.allow("ip", now); allowed || retry != time.Minute {
		t.Fatalf("request over limit = %v, %v", allowed, retry)
	}
	if allowed, _ := limiter.allow("ip", now.Add(time.Minute+time.Second)); !allowed {
		t.Fatal("request after the window was blocked")
	}

	limiter.entries = make(map[string][]time.Time)
	limiter.allow("a", now)
	limiter.allow("b", now.Add(time.Second))
	limiter.allow("c", now.Add(2*time.Second))
	if len(limiter.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(limiter.entries))
	}
	if _, ok := limiter.entries["a"]; ok {
		t.Fatal("oldest download client was not evicted")
	}
}
