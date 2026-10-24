package panel

import "net/http"

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// withSecurityHeaders applies headers that are safe for both public pages and
// the admin UI. The panel only listens on HTTPS, so HSTS is safe here.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy",
			"default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		header.Set("Strict-Transport-Security", "max-age=31536000")
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
