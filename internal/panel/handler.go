package panel

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

const acmeChallengePrefix = "/.well-known/acme-challenge/"

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// withSecurityHeaders applies headers that are safe for both public pages and
// the future admin UI. CSP starts strict; later pages must keep scripts and
// styles same-origin instead of adding inline handlers.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy",
			"default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// newHTTPRedirectHandler serves the HTTP listener: ACME HTTP-01 paths stay on
// HTTP (404 until issuance is implemented) and everything else is redirected to
// HTTPS without changing the path or query.
func newHTTPRedirectHandler(cfg Config) http.Handler {
	return withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, acmeChallengePrefix) {
			http.NotFound(w, r)
			return
		}
		target := "https://" + httpsHost(cfg, r.Host) + requestURI(r)
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	}))
}

func httpsHost(cfg Config, requestHost string) string {
	port := listenPort(cfg.HTTPSAddr)
	if cfg.PublicAddress != "" {
		if _, _, err := net.SplitHostPort(cfg.PublicAddress); err == nil {
			return cfg.PublicAddress
		}
		return joinPublicHost(cfg.PublicAddress, port)
	}

	host := requestHost
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	if host == "" {
		host = "localhost"
	}
	return joinPublicHost(host, port)
}

func listenPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

// joinPublicHost adds a non-default HTTPS port and brackets bare IPv6
// addresses. A port of 0 is an ephemeral test listener and is omitted.
func joinPublicHost(host, port string) string {
	if host == "" {
		return ""
	}
	if parsed := net.ParseIP(host); parsed != nil && strings.Contains(host, ":") {
		if port != "" && port != "443" && port != "0" {
			return net.JoinHostPort(host, port)
		}
		return "[" + host + "]"
	}
	if strings.Contains(host, ":") {
		// An invalid host:port in a Host header cannot be made safe by
		// guessing; fall back to localhost rather than emit a broken URL.
		host = "localhost"
	}
	if port != "" && port != "443" && port != "0" {
		return net.JoinHostPort(host, port)
	}
	return host
}

func requestURI(r *http.Request) string {
	uri := r.URL.RequestURI()
	if uri == "" {
		return "/"
	}
	if !strings.HasPrefix(uri, "/") {
		return fmt.Sprintf("/%s", uri)
	}
	return uri
}
