package panel

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"
)

func TestPanelStylesheetIsServed(t *testing.T) {
	env := newPublicTestEnv(t)
	rec := panelRequest(t, env.handler, http.MethodGet, "/static/panel.css", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/panel.css = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"--accent:", "prefers-color-scheme: light", ".table-wrap", ".auth-card",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("panel.css does not contain %q", want)
		}
	}
	assertSecurityHeaders(t, rec.Header())
}

func TestStaticAssetsAreCacheable(t *testing.T) {
	env := newPublicTestEnv(t)
	rec := panelRequest(t, env.handler, http.MethodGet, "/static/panel.css", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/panel.css = %d", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("static asset has no ETag")
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q, want immutable", got)
	}
	rec = panelRequest(t, env.handler, http.MethodGet, "/static/panel.css",
		nil, nil, map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("conditional GET /static/panel.css = %d, want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("304 response carries %d bytes", rec.Body.Len())
	}
}

func TestPanelTemplatesUseEmbeddedStylesheet(t *testing.T) {
	entries, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no templates found")
	}

	head, err := templateFS.ReadFile("templates/head.html")
	if err != nil {
		t.Fatalf("read head.html: %v", err)
	}
	if !strings.Contains(string(head), `href="/static/panel.css"`) {
		t.Fatal("head partial does not link /static/panel.css")
	}
	for _, want := range []string{`name="color-scheme"`, `name="theme-color"`, `name="viewport"`} {
		if !strings.Contains(string(head), want) {
			t.Fatalf("head partial does not contain %q", want)
		}
	}

	for _, name := range entries {
		body, err := templateFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(body)
		if strings.Contains(source, "<body") && !strings.Contains(source, `{{template "head"}}`) {
			t.Errorf("%s has <body> but does not include the head partial", name)
		}
		if strings.Contains(source, "style=") {
			t.Errorf("%s uses an inline style attribute; CSP forbids inline styles", name)
		}
	}
}

func TestAdminActiveNav(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/admin", want: "dashboard"},
		{path: "/admin/server/start", want: "dashboard"},
		{path: "/admin/logs", want: "logs"},
		{path: "/admin/mods", want: "mods"},
		{path: "/admin/mods/deploy", want: "mods"},
		{path: "/admin/state/deploy", want: "state"},
		{path: "/admin/config/server", want: "config"},
		{path: "/admin/staff/Frodo", want: "staff"},
		{path: "/admin/password", want: "password"},
		{path: "/admin/unknown", want: ""},
	}
	for _, tt := range tests {
		if got := adminActiveNav(tt.path); got != tt.want {
			t.Errorf("adminActiveNav(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestAdminDashboardMarksActiveNav(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	rec := env.request(t, http.MethodGet, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<a href="/admin" aria-current="page">Обзор</a>`) {
		t.Fatal("dashboard does not mark the overview link as current")
	}
	if strings.Contains(body, `<a href="/admin/logs" aria-current="page">Логи</a>`) {
		t.Fatal("dashboard marks more than one nav item as current")
	}
}
