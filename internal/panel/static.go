package panel

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// staticFS contains the same-origin CSS and small scripts used by the panel.
// Inline scripts and styles stay forbidden by the CSP.
//
//go:embed static/*
var staticFS embed.FS

// staticETags caches one strong validator per embedded asset. Assets only
// change with a new build, so each hash is computed once per process.
var staticETags sync.Map

// staticFiles serves the embedded assets with a long max-age and an ETag
// derived from the content, so browsers reuse them until a release changes the
// hash.
func (h *panelHandler) staticFiles() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/static/")
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		data, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", staticETag(name, data))
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}

func staticETag(name string, data []byte) string {
	if cached, ok := staticETags.Load(name); ok {
		return cached.(string)
	}
	sum := sha256.Sum256(data)
	etag := strconv.Quote(hex.EncodeToString(sum[:8]))
	staticETags.Store(name, etag)
	return etag
}
