package panel

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	server, err := New(testConfig(t), nil, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return server
}

func serveTestServer(t *testing.T) (*Server, func() error) {
	t.Helper()
	server := newTestServer(t)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()

	var (
		once sync.Once
		err  error
	)
	stop := func() error {
		once.Do(func() {
			cancel()
			select {
			case err = <-done:
			case <-time.After(5 * time.Second):
				err = context.DeadlineExceeded
			}
		})
		return err
	}
	t.Cleanup(func() {
		if stopErr := stop(); stopErr != nil {
			t.Errorf("Serve shutdown: %v", stopErr)
		}
	})
	return server, stop
}

func testClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // the test intentionally uses self-signed TLS
				MinVersion:         tls.VersionTLS12,
			},
		},
	}
}

func TestServerServesHTTPSHealthz(t *testing.T) {
	server, _ := serveTestServer(t)
	client := testClient()
	defer client.CloseIdleConnections()

	resp, err := client.Get("https://" + server.HTTPSAddr() + "/healthz")
	if err != nil {
		t.Fatalf("GET healthz: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if string(body) != "ok\n" {
		t.Fatalf("body = %q, want ok\\n", body)
	}
	if resp.TLS == nil || resp.TLS.Version < tls.VersionTLS12 {
		t.Fatalf("TLS = %#v, want >= TLS 1.2", resp.TLS)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	assertSecurityHeaders(t, resp.Header)
}

func TestServerRedirectsHTTPToHTTPS(t *testing.T) {
	server, _ := serveTestServer(t)
	client := testClient()
	defer client.CloseIdleConnections()

	resp, err := client.Get("http://" + server.HTTPAddr() + "/admin?x=1")
	if err != nil {
		t.Fatalf("GET http: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want 308", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "https://") ||
		!strings.HasSuffix(location, "/admin?x=1") {
		t.Fatalf("Location = %q", location)
	}
	assertSecurityHeaders(t, resp.Header)
}

func TestServerRejectsPostHealthz(t *testing.T) {
	server, _ := serveTestServer(t)
	client := testClient()
	defer client.CloseIdleConnections()

	resp, err := client.Post("https://"+server.HTTPSAddr()+"/healthz",
		"text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatalf("POST healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "GET") {
		t.Fatalf("Allow = %q, want GET", allow)
	}
}

func TestServeStopsOnContextCancel(t *testing.T) {
	server, stop := serveTestServer(t)
	if server.HTTPAddr() == "" || server.HTTPSAddr() == "" {
		t.Fatal("server is not listening")
	}
	if err := stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	conn, err := net.DialTimeout("tcp", server.HTTPAddr(), 200*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("HTTP listener still accepts connections after shutdown")
	}
}

func TestServeCancelledBeforeStartDoesNotListen(t *testing.T) {
	server := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := server.Serve(ctx); err != nil {
		t.Fatalf("Serve = %v, want nil", err)
	}
	if server.HTTPAddr() != "" || server.HTTPSAddr() != "" {
		t.Fatalf("listeners = %q, %q, want empty", server.HTTPAddr(), server.HTTPSAddr())
	}
}

func TestListenClosesHTTPWhenHTTPSFails(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer occupied.Close()

	cfg := testConfig(t)
	cfg.HTTPSAddr = occupied.Addr().String()
	server, err := New(cfg, nil, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := server.Listen(); err == nil {
		t.Fatal("Listen succeeded with an occupied HTTPS port")
	}
	if server.HTTPAddr() != "" || server.HTTPSAddr() != "" {
		t.Fatalf("listeners = %q, %q, want empty", server.HTTPAddr(), server.HTTPSAddr())
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	cfg := testConfig(t)
	cfg.HTTPAddr = "not-an-address"
	if _, err := New(cfg, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "invalid panel config") {
		t.Fatalf("New error = %v, want invalid config", err)
	}
}

func assertSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	for name, want := range map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "same-origin",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := header.Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Errorf("Content-Security-Policy = %q", got)
	}
}
