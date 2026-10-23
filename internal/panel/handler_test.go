package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPRedirectTarget(t *testing.T) {
	tests := []struct {
		name         string
		httpsAddr    string
		public       string
		requestHost  string
		path         string
		wantLocation string
	}{
		{
			name:         "public IP default port",
			httpsAddr:    ":443",
			public:       "203.0.113.10",
			requestHost:  "203.0.113.10",
			path:         "/admin?x=1",
			wantLocation: "https://203.0.113.10/admin?x=1",
		},
		{
			name:         "public IP custom port",
			httpsAddr:    ":8443",
			public:       "203.0.113.10",
			requestHost:  "203.0.113.10",
			path:         "/admin",
			wantLocation: "https://203.0.113.10:8443/admin",
		},
		{
			name:         "public address already has port",
			httpsAddr:    ":443",
			public:       "panel.example:9443",
			requestHost:  "panel.example",
			path:         "/backups",
			wantLocation: "https://panel.example:9443/backups",
		},
		{
			name:         "request host replaces HTTP port",
			httpsAddr:    ":443",
			requestHost:  "panel.example:80",
			path:         "/",
			wantLocation: "https://panel.example/",
		},
		{
			name:         "empty host falls back to localhost",
			httpsAddr:    ":8443",
			requestHost:  "",
			path:         "/healthz",
			wantLocation: "https://localhost:8443/healthz",
		},
		{
			name:         "public ipv6 custom port",
			httpsAddr:    ":8443",
			public:       "2001:db8::1",
			requestHost:  "[2001:db8::1]",
			path:         "/admin",
			wantLocation: "https://[2001:db8::1]:8443/admin",
		},
		{
			name:         "public ipv6 with port",
			httpsAddr:    ":443",
			public:       "[2001:db8::1]:9443",
			requestHost:  "[2001:db8::1]",
			path:         "/admin",
			wantLocation: "https://[2001:db8::1]:9443/admin",
		},
		{
			name:         "request ipv6 custom port",
			httpsAddr:    ":8443",
			requestHost:  "[::1]:80",
			path:         "/admin",
			wantLocation: "https://[::1]:8443/admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				HTTPAddr:      ":80",
				HTTPSAddr:     tt.httpsAddr,
				PublicAddress: tt.public,
			}
			handler := newHTTPRedirectHandler(cfg)
			req := httptest.NewRequest(http.MethodGet,
				"http://example.test"+tt.path, nil)
			req.Host = tt.requestHost
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusPermanentRedirect {
				t.Fatalf("status = %d, want 308", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("Location = %q, want %q", got, tt.wantLocation)
			}
			assertSecurityHeaders(t, rec.Header())
		})
	}
}

func TestHTTPRedirectReservesACMEChallenge(t *testing.T) {
	handler := newHTTPRedirectHandler(Config{
		HTTPAddr:  ":80",
		HTTPSAddr: ":443",
	})
	req := httptest.NewRequest(http.MethodGet,
		"http://panel.example/.well-known/acme-challenge/token", nil)
	req.Host = "panel.example"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("Location = %q, want empty", location)
	}
}
