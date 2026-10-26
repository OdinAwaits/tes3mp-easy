package panel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

type backupsCall struct {
	typ    string
	limit  int
	before string
}

type fakePublicOps struct {
	status     Status
	statusErr  error
	backups    Backups
	backupsErr error

	statusCalls  []string
	backupsCalls []backupsCall
}

func (f *fakePublicOps) Status(_ context.Context, unit string) (Status, error) {
	f.statusCalls = append(f.statusCalls, unit)
	return f.status, f.statusErr
}

func (f *fakePublicOps) BackupsList(_ context.Context, typ string, limit int,
	before string,
) (Backups, error) {
	f.backupsCalls = append(f.backupsCalls, backupsCall{typ: typ, limit: limit, before: before})
	return f.backups, f.backupsErr
}

func (f *fakePublicOps) Start(context.Context) error                 { return nil }
func (f *fakePublicOps) Stop(context.Context) error                  { return nil }
func (f *fakePublicOps) Restart(context.Context) error               { return nil }
func (f *fakePublicOps) Logs(context.Context, int) ([]string, error) { return nil, nil }
func (f *fakePublicOps) BackupCreate(context.Context, string, bool) (Backup, error) {
	return Backup{}, nil
}
func (f *fakePublicOps) ModsImport(context.Context, string) (ImportResult, error) {
	return ImportResult{}, nil
}
func (f *fakePublicOps) ModsDeploy(context.Context, string) (DeployResult, error) {
	return DeployResult{}, nil
}
func (f *fakePublicOps) StateDeploy(context.Context, string) (DeployResult, error) {
	return DeployResult{}, nil
}
func (f *fakePublicOps) StaffSet(context.Context, string, int) error { return nil }
func (f *fakePublicOps) ConfigGet(context.Context, string) (map[string]any, error) {
	return nil, nil
}
func (f *fakePublicOps) ConfigSet(context.Context, string, string, any) error { return nil }

type publicTestEnv struct {
	handler http.Handler
	h       *panelHandler
	ops     *fakePublicOps
	cfg     Config
}

func newPublicTestEnv(t *testing.T) *publicTestEnv {
	t.Helper()
	auth, _, cfg := newTestAuthStore(t)
	opsClient := &fakePublicOps{
		status: Status{Unit: "tes3mp.service", State: "running", Health: "healthy"},
	}
	h, err := newPanelHandler(cfg, auth, opsClient, log.New(io.Discard, "", 0),
		func() time.Time { return sessionTestNow })
	if err != nil {
		t.Fatalf("newPanelHandler: %v", err)
	}
	return &publicTestEnv{
		handler: withSecurityHeaders(h.routes()),
		h:       h,
		ops:     opsClient,
		cfg:     cfg,
	}
}

func writeTestServerConfig(t *testing.T, root, hostname string, port int) {
	t.Helper()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	body := fmt.Sprintf("[General]\nhostname = %s\nport = %d\n", hostname, port)
	if err := os.WriteFile(filepath.Join(dir, "tes3mp-server-default.cfg"),
		[]byte(body), 0o600); err != nil {
		t.Fatalf("write server config: %v", err)
	}
}

func writeTestCurrentMods(t *testing.T, root string, body []byte) (name, sum string) {
	t.Helper()
	name = "mods-2026-10-23_12-00-00.tar.gz"
	dir := filepath.Join(root, "backups", "mods")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir backups/mods: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
		t.Fatalf("write mods archive: %v", err)
	}
	hash := sha256.Sum256(body)
	sum = hex.EncodeToString(hash[:])
	if err := os.WriteFile(filepath.Join(dir, "current.txt"),
		[]byte(sum+" "+name+"\n"), 0o600); err != nil {
		t.Fatalf("write current.txt: %v", err)
	}
	return name, sum
}

func writeTestBackup(t *testing.T, root, typ, name string, body []byte) string {
	t.Helper()
	dir := filepath.Join(root, "backups", typ)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir backups/%s: %v", typ, err)
	}
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	return target
}

func decodePublicJSON(t *testing.T, rec *httptest.ResponseRecorder, value any) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), value); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
}

func TestPublicServerInfo(t *testing.T) {
	env := newPublicTestEnv(t)
	writeTestServerConfig(t, env.cfg.Root, "Morrowind with friends", 25565)
	_, sum := writeTestCurrentMods(t, env.cfg.Root, []byte("mods data"))
	env.h.cfg.PublicAddress = "203.0.113.10:8443"

	rec := panelRequest(t, env.handler, http.MethodGet, "/api/v1/server", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/server = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	assertSecurityHeaders(t, rec.Header())
	var response serverInfoResponse
	decodePublicJSON(t, rec, &response)
	if response.Name != "Morrowind with friends" ||
		response.Address != "203.0.113.10" || response.Port != 25565 ||
		response.Status != "running" || response.PanelVersion != Version {
		t.Fatalf("server response = %+v", response)
	}
	if response.ModsSHA256 == nil || *response.ModsSHA256 != sum {
		t.Fatalf("mods_sha256 = %v, want %s", response.ModsSHA256, sum)
	}
	if len(env.ops.statusCalls) != 1 || env.ops.statusCalls[0] != "tes3mp.service" {
		t.Fatalf("status calls = %v", env.ops.statusCalls)
	}
}

func TestPublicServerInfoWithoutModsFallsBackToRequestHost(t *testing.T) {
	env := newPublicTestEnv(t)
	rec := panelRequest(t, env.handler, http.MethodGet, "/api/v1/server", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/server = %d: %s", rec.Code, rec.Body.String())
	}
	var response serverInfoResponse
	decodePublicJSON(t, rec, &response)
	if response.ModsSHA256 != nil {
		t.Fatalf("mods_sha256 = %v, want null", *response.ModsSHA256)
	}
	if response.Address != "panel.test" {
		t.Fatalf("address = %q, want request host", response.Address)
	}
	if response.Name != defaultServerName || response.Port != defaultGamePort {
		t.Fatalf("name/port = %q/%d", response.Name, response.Port)
	}
}

func TestPublicAddressUsesConfigThenRequestHost(t *testing.T) {
	tests := []struct {
		name   string
		public string
		host   string
		want   string
	}{
		{name: "configured ip and port", public: "203.0.113.10:8443", host: "ignored", want: "203.0.113.10"},
		{name: "configured ipv6", public: "2001:db8::1", host: "ignored", want: "2001:db8::1"},
		{name: "configured ipv6 and port", public: "[2001:db8::1]:8443", host: "ignored", want: "2001:db8::1"},
		{name: "request host and port", host: "panel.example:8443", want: "panel.example"},
		{name: "request ipv6", host: "[::1]:8443", want: "::1"},
		{name: "empty request host", want: "localhost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "https://"+tt.host+"/", nil)
			if tt.host == "" {
				request.Host = ""
			}
			if got := publicAddress(Config{PublicAddress: tt.public}, request); got != tt.want {
				t.Fatalf("publicAddress = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPublicServerInfoWithoutOpsIsUnavailable(t *testing.T) {
	auth, _, cfg := newTestAuthStore(t)
	h, err := newPanelHandler(cfg, auth, nil, log.New(io.Discard, "", 0),
		func() time.Time { return sessionTestNow })
	if err != nil {
		t.Fatalf("newPanelHandler: %v", err)
	}
	handler := withSecurityHeaders(h.routes())
	rec := panelRequest(t, handler, http.MethodGet, "/api/v1/server", nil, nil, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if response := decodePublicError(t, rec); response.Error.Code != "service_unavailable" {
		t.Fatalf("error = %+v", response)
	}
}

func TestPublicServerInfoOpsFailure(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name: "service unavailable",
			err: &OpsError{
				Exit: ops.ExitServiceUnavailable, Code: ops.CodeServiceUnavailable,
				Message: "systemd is down",
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "service_unavailable",
		},
		{
			name:       "internal",
			err:        &OpsError{Exit: ops.ExitInternal, Code: ops.CodeInternal},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newPublicTestEnv(t)
			env.ops.statusErr = tt.err
			rec := panelRequest(t, env.handler, http.MethodGet, "/api/v1/server", nil, nil, nil)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q", got)
			}
			if response := decodePublicError(t, rec); response.Error.Code != tt.wantCode {
				t.Fatalf("error = %+v, want code %q", response, tt.wantCode)
			}
			if strings.Contains(rec.Body.String(), "systemd is down") {
				t.Fatal("public API leaked the ops error message")
			}
		})
	}
}

func decodePublicError(t *testing.T, rec *httptest.ResponseRecorder) publicAPIErrorResponse {
	t.Helper()
	var response publicAPIErrorResponse
	decodePublicJSON(t, rec, &response)
	return response
}

func TestPublicModsCurrentAndArchive(t *testing.T) {
	env := newPublicTestEnv(t)
	body := []byte("synthetic mods archive")
	name, sum := writeTestCurrentMods(t, env.cfg.Root, body)

	rec := panelRequest(t, env.handler, http.MethodGet, "/api/v1/mods/current", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET current = %d: %s", rec.Code, rec.Body.String())
	}
	var current modsCurrentResponse
	decodePublicJSON(t, rec, &current)
	if current.Name != name || current.SHA256 != sum ||
		current.Size != int64(len(body)) || current.URL != modsCurrentURL {
		t.Fatalf("current = %+v", current)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("current Cache-Control = %q", got)
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/api/v1/mods/current/archive", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET archive = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/gzip" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="`+name+`"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := rec.Header().Get("ETag"); got != `"`+sum+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=0, must-revalidate" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Body.String() != string(body) {
		t.Fatalf("archive body = %q", rec.Body.String())
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/api/v1/mods/current/archive",
		nil, nil, map[string]string{"If-None-Match": `"` + sum + `"`})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match = %d, want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", rec.Body.String())
	}
}

func TestPublicModsNoMods(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{name: "missing current", setup: func(*testing.T, string) {}},
		{
			name: "invalid current",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "backups", "mods")
				if err := os.MkdirAll(dir, 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "current.txt"),
					[]byte("not a current file\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing archive",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "backups", "mods")
				if err := os.MkdirAll(dir, 0o750); err != nil {
					t.Fatal(err)
				}
				name := "mods-2026-10-23_12-00-00.tar.gz"
				sum := strings.Repeat("a", 64)
				if err := os.WriteFile(filepath.Join(dir, "current.txt"),
					[]byte(sum+" "+name+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newPublicTestEnv(t)
			tt.setup(t, env.cfg.Root)
			for _, requestPath := range []string{
				"/api/v1/mods/current",
				"/api/v1/mods/current/archive",
			} {
				rec := panelRequest(t, env.handler, http.MethodGet, requestPath, nil, nil, nil)
				if rec.Code != http.StatusNotFound {
					t.Fatalf("%s = %d, want 404", requestPath, rec.Code)
				}
				if response := decodePublicError(t, rec); response.Error.Code != "no_mods" {
					t.Fatalf("%s error = %+v", requestPath, response)
				}
			}
		})
	}
}

func TestPublicBackupsPage(t *testing.T) {
	env := newPublicTestEnv(t)
	current := "mods-current.tar.gz"
	next := "mods-next.tar.gz"
	env.ops.backups = Backups{
		Type:    "mods",
		Current: &current,
		Items: []BackupItem{
			{
				Name: "mods-2026-10-23_11-00-00.tar.gz", Size: 2048,
				Mtime: "2026-10-23T11:00:00Z", Current: true,
			},
			{
				Name: "mods-2026-10-01_11-00-00.tar.gz", Size: 1024,
				Mtime: "2026-10-01T11:00:00Z",
			},
			{
				Name: "mods-2025-01-15_11-00-00.tar.gz", Size: 10,
				Mtime: "2025-01-15T11:00:00Z",
			},
		},
		NextBefore: &next,
	}

	rec := panelRequest(t, env.handler, http.MethodGet,
		"/backups?type=mods&limit=2&before=mods-before.tar.gz", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /backups = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if len(env.ops.backupsCalls) != 1 {
		t.Fatalf("backups calls = %+v", env.ops.backupsCalls)
	}
	if call := env.ops.backupsCalls[0]; call.typ != "mods" || call.limit != 2 ||
		call.before != "mods-before.tar.gz" {
		t.Fatalf("BackupsList call = %+v", call)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"mods-2026-10-23_11-00-00.tar.gz",
		"mods-2026-10-01_11-00-00.tar.gz",
		"mods-2025-01-15_11-00-00.tar.gz",
		"2026-10-23", "2026-W40", "2025-01",
		"mods-current.tar.gz", "(текущий)",
		"/backups/mods/mods-2026-10-23_11-00-00.tar.gz",
		"type=mods", "before=mods-next.tar.gz", "limit=2",
		"Показать более старые",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("backups page does not contain %q:\n%s", want, body)
		}
	}
}

func TestPublicBackupsDefaultsAndValidation(t *testing.T) {
	env := newPublicTestEnv(t)
	env.ops.backups = Backups{Type: "state", Items: []BackupItem{}}

	rec := panelRequest(t, env.handler, http.MethodGet, "/backups", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("default backups = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Архивов пока нет") {
		t.Fatal("empty backups page has no empty-state message")
	}
	if len(env.ops.backupsCalls) != 1 ||
		env.ops.backupsCalls[0].typ != "state" ||
		env.ops.backupsCalls[0].limit != defaultBackupLimit ||
		env.ops.backupsCalls[0].before != "" {
		t.Fatalf("default BackupsList call = %+v", env.ops.backupsCalls)
	}

	tests := []struct {
		path string
	}{
		{path: "/backups?type=unknown"},
		{path: "/backups?type=state&limit=0"},
		{path: "/backups?type=state&limit=501"},
		{path: "/backups?type=state&limit=nope"},
		{path: "/backups?type=state&before=../secret.tar.gz"},
		{path: "/backups?type=state&before=notes.txt"},
	}
	for _, tt := range tests {
		rec := panelRequest(t, env.handler, http.MethodGet, tt.path, nil, nil, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", tt.path, rec.Code)
		}
	}
	if len(env.ops.backupsCalls) != 1 {
		t.Fatalf("invalid queries called ops: %+v", env.ops.backupsCalls)
	}
}

func TestPublicBackupsOpsFailure(t *testing.T) {
	env := newPublicTestEnv(t)
	env.ops.backupsErr = &OpsError{
		Exit: ops.ExitServiceUnavailable, Code: ops.CodeServiceUnavailable,
		Message: "systemd is unavailable",
	}
	rec := panelRequest(t, env.handler, http.MethodGet, "/backups?type=state", nil, nil, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("backups ops failure = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Сервер временно недоступен") {
		t.Fatalf("page does not show a service-unavailable message:\n%s", body)
	}
	if strings.Contains(body, "systemd is unavailable") {
		t.Fatal("page leaked the ops error message")
	}
	if strings.Contains(body, "stack trace") || strings.Contains(body, "goroutine") {
		t.Fatal("page leaks internal diagnostics")
	}
}

func TestPublicBackupDownload(t *testing.T) {
	env := newPublicTestEnv(t)
	name := "state-2026-10-23_11-00-00.tar.gz"
	body := []byte("state archive body")
	target := writeTestBackup(t, env.cfg.Root, "state", name, body)
	if err := os.Mkdir(filepath.Join(filepath.Dir(target), "directory.tar.gz"), 0o750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.tar.gz")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(filepath.Dir(target), "link.tar.gz")); err != nil {
		t.Fatal(err)
	}

	rec := panelRequest(t, env.handler, http.MethodGet, "/backups/state/"+name, nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET backup = %d: %s", rec.Code, rec.Body.String())
	}
	assertSecurityHeaders(t, rec.Header())
	if got := rec.Header().Get("Content-Type"); got != "application/gzip" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="`+name+`"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if rec.Body.String() != string(body) {
		t.Fatalf("body = %q", rec.Body.String())
	}

	for _, path := range []string{
		"/backups/state/missing.tar.gz",
		"/backups/state/notes.txt",
		"/backups/state/directory.tar.gz",
		"/backups/state/link.tar.gz",
		"/backups/mods/" + name,
		"/backups/state/..%2Fsecret.tar.gz",
	} {
		rec := panelRequest(t, env.handler, http.MethodGet, path, nil, nil, nil)
		if rec.Code == http.StatusOK {
			t.Fatalf("%s = 200, want an error", path)
		}
		if strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("%s leaked an outside file", path)
		}
	}
}

func TestPublicDownloadRateLimit(t *testing.T) {
	env := newPublicTestEnv(t)
	name := "state-2026-10-23_11-00-00.tar.gz"
	writeTestBackup(t, env.cfg.Root, "state", name, []byte("state"))

	for i := 0; i < maxDownloadRequests; i++ {
		rec := panelRequest(t, env.handler, http.MethodGet, "/backups/state/"+name, nil, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("download %d = %d, want 200", i+1, rec.Code)
		}
	}
	rec := panelRequest(t, env.handler, http.MethodGet, "/backups/state/"+name, nil, nil, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("download after limit = %d, want 429", rec.Code)
	}
	if retry := rec.Header().Get("Retry-After"); retry == "" || retry == "0" {
		t.Fatalf("Retry-After = %q", retry)
	}

	rec = panelRequest(t, env.handler, http.MethodGet,
		"/api/v1/mods/current/archive", nil, nil, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("API download after limit = %d, want 429", rec.Code)
	}
	if response := decodePublicError(t, rec); response.Error.Code != "rate_limited" {
		t.Fatalf("API rate limit error = %+v", response)
	}
}

func TestPublicHomePage(t *testing.T) {
	env := newPublicTestEnv(t)
	writeTestServerConfig(t, env.cfg.Root, "Morrowind with friends", 25565)
	name, sum := writeTestCurrentMods(t, env.cfg.Root, []byte("mods"))

	rec := panelRequest(t, env.handler, http.MethodGet, "/", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Morrowind with friends", "running", "работает", "25565",
		name, sum, "/backups", "/admin/login",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("home page does not contain %q:\n%s", want, body)
		}
	}

	rec = panelRequest(t, env.handler, http.MethodGet, "/does-not-exist", nil, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path = %d, want 404", rec.Code)
	}
	rec = panelRequest(t, env.handler, http.MethodPost, "/", nil, nil, nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST / = %d, want 405", rec.Code)
	}
}

func TestPublicHomeShowsServiceUnavailable(t *testing.T) {
	env := newPublicTestEnv(t)
	env.ops.statusErr = &OpsError{
		Exit: ops.ExitServiceUnavailable, Code: ops.CodeServiceUnavailable,
		Message: "systemd is unavailable",
	}
	rec := panelRequest(t, env.handler, http.MethodGet, "/", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "service_unavailable") ||
		!strings.Contains(body, "Сервер временно недоступен") {
		t.Fatalf("home page does not show service_unavailable:\n%s", body)
	}
	if strings.Contains(body, "running") {
		t.Fatalf("home page pretends the server is running:\n%s", body)
	}
	if strings.Contains(body, "systemd is unavailable") {
		t.Fatal("home page leaked the ops error message")
	}
}
