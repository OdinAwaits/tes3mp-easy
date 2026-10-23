package panel

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPanelHTTPContract(t *testing.T) {
	p := startPanel(t, nil)

	client := p.newClient()

	// The HTTP listener only redirects to HTTPS; paths that ACME uses are
	// reserved and must never inherit the catch-all redirect.
	resp := doHTTP(t, client, http.MethodGet, p.httpURL("/healthz?probe=1"), "panel.example")
	if resp.Status != http.StatusPermanentRedirect {
		t.Fatalf("HTTP /healthz = %d, want %d", resp.Status, http.StatusPermanentRedirect)
	}
	wantLocation := "https://127.0.0.1" + httpsPort(t, p) + "/healthz?probe=1"
	if got := resp.Header.Get("Location"); got != wantLocation {
		t.Fatalf("HTTP /healthz Location = %q, want %q", got, wantLocation)
	}

	resp = doHTTP(t, client, http.MethodGet, p.httpURL("/"), "panel.example")
	if resp.Status != http.StatusPermanentRedirect {
		t.Fatalf("HTTP / = %d, want %d", resp.Status, http.StatusPermanentRedirect)
	}
	if got := resp.Header.Get("Location"); got !=
		"https://127.0.0.1"+httpsPort(t, p)+"/" {
		t.Fatalf("HTTP / Location = %q", got)
	}

	resp = doHTTP(t, client, http.MethodGet,
		p.httpURL("/.well-known/acme-challenge/token"), "panel.example")
	if resp.Status != http.StatusNotFound {
		t.Fatalf("ACME path = %d, want %d", resp.Status, http.StatusNotFound)
	}
	if location := resp.Header.Get("Location"); location != "" {
		t.Fatalf("ACME path Location = %q, want empty", location)
	}

	// HTTPS serves health, public pages, and the read-only public API.
	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/healthz"), "127.0.0.1")
	requireStatus(t, resp, http.StatusOK, "HTTPS /healthz")
	requireBody(t, resp, "ok\n")
	requireSecurityHeaders(t, resp)

	resp = doHTTP(t, client, http.MethodPost, p.httpsURL("/healthz"), "127.0.0.1")
	if resp.Status != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz = %d, want %d", resp.Status, http.StatusMethodNotAllowed)
	}

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/"), "127.0.0.1")
	requireStatus(t, resp, http.StatusOK, "HTTPS /")
	requireContains(t, resp.Body, "TES3MP")
	requireSecurityHeaders(t, resp)

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/backups"), "127.0.0.1")
	requireStatus(t, resp, http.StatusOK, "HTTPS /backups")
	requireContains(t, resp.Body, "Публичные бэкапы")
	requireContains(t, resp.Body, "Архивов пока нет")
	requireSecurityHeaders(t, resp)

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/backups?type=mods"), "127.0.0.1")
	requireStatus(t, resp, http.StatusOK, "HTTPS /backups?type=mods")
	requireContains(t, resp.Body, "Публичные бэкапы")

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/backups?type=bogus"), "127.0.0.1")
	requireStatus(t, resp, http.StatusBadRequest, "HTTPS /backups?type=bogus")
	requireContains(t, resp.Body, "Неизвестный тип бэкапа")

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/api/v1/server"), "127.0.0.1")
	if resp.Status != http.StatusOK {
		t.Fatalf("GET /api/v1/server = %d: %s\npanel log:\n%s\nops log:\n%s",
			resp.Status, resp.Body, p.logContents(), readLogFile(p.opsLogPath()))
	}
	expectJSON(t, resp, http.StatusOK)
	requireContains(t, resp.Body, `"status":"running"`)
	requireContains(t, resp.Body, `"port":25565`)
	requireContains(t, resp.Body, `"address":"127.0.0.1"`)
	requireSecurityHeaders(t, resp)

	writeFakeMode(t, p.modePath, "status-fail")
	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/api/v1/server"), "127.0.0.1")
	expectJSON(t, resp, http.StatusServiceUnavailable)
	requireContains(t, resp.Body, `"code":"service_unavailable"`)
	requireNotContains(t, resp.Body, "systemd is not running")
	writeFakeMode(t, p.modePath, "ok")

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/api/v1/mods/current"), "127.0.0.1")
	expectJSON(t, resp, http.StatusNotFound)
	requireContains(t, resp.Body, `"code":"no_mods"`)
	requireSecurityHeaders(t, resp)

	resp = doHTTP(t, client, http.MethodGet,
		p.httpsURL("/api/v1/mods/current/archive"), "127.0.0.1")
	expectJSON(t, resp, http.StatusNotFound)
	requireContains(t, resp.Body, `"code":"no_mods"`)

	resp = doHTTP(t, client, http.MethodGet, p.httpsURL("/admin"), "127.0.0.1")
	requireStatus(t, resp, http.StatusSeeOther, "HTTPS /admin")
	requireLocation(t, resp, "/admin/login")
	requireSecurityHeaders(t, resp)
}

func doHTTP(t *testing.T, client *http.Client, method, url, host string) panelResponse {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, url, err)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		t.Fatalf("read %s %s: %v", method, url, err)
	}
	return panelResponse{
		Status:  resp.StatusCode,
		Header:  resp.Header.Clone(),
		Body:    string(data),
		Cookies: resp.Cookies(),
	}
}

func httpsPort(t *testing.T, p *panelProc) string {
	t.Helper()
	_, port, ok := strings.Cut(p.httpsAddr, ":")
	if !ok {
		t.Fatalf("HTTPS address %q has no port", p.httpsAddr)
	}
	return ":" + port
}

func requireSecurityHeaders(t *testing.T, resp panelResponse) {
	t.Helper()
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	}
	for name, value := range want {
		if got := resp.Header.Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("Content-Security-Policy = %q", csp)
	}
}
