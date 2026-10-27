package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

type adminDeployCall struct {
	kind string
	ref  string
}

type adminConfigSetCall struct {
	file  string
	key   string
	value any
}

type fakeAdminOps struct {
	statuses map[string]Status
	statusFn func(unit string) (Status, error)

	logs    []string
	logsErr error
	tail    int

	backups    map[string]Backups
	backupsErr error
	lastBefore string

	backup    Backup
	backupErr error
	created   bool
	manual    bool

	importResult ImportResult
	importErr    error
	importPath   string
	importData   []byte
	importMode   os.FileMode

	deploys   []adminDeployCall
	deployErr error

	stateDeployErr error

	staffCalls []struct {
		player string
		rank   int
	}
	staffErr error

	configSnapshots map[string]ConfigSnapshot
	configErr       error
	configSets      []adminConfigSetCall
	configSetErr    error

	doctor    Doctor
	doctorErr error

	started    int
	stopped    int
	restarted  int
	startErr   error
	stopErr    error
	restartErr error
}

func (f *fakeAdminOps) Status(_ context.Context, unit string) (Status, error) {
	if f.statusFn != nil {
		return f.statusFn(unit)
	}
	if status, ok := f.statuses[unit]; ok {
		return status, nil
	}
	return Status{Unit: unit, State: "unknown", Health: "unknown"}, nil
}

func (f *fakeAdminOps) Start(context.Context) error {
	f.started++
	return f.startErr
}

func (f *fakeAdminOps) Stop(context.Context) error {
	f.stopped++
	return f.stopErr
}

func (f *fakeAdminOps) Restart(context.Context) error {
	f.restarted++
	return f.restartErr
}

func (f *fakeAdminOps) Logs(_ context.Context, tail int) ([]string, error) {
	f.tail = tail
	return f.logs, f.logsErr
}

func (f *fakeAdminOps) BackupsList(_ context.Context, typ string, _ int,
	before string,
) (Backups, error) {
	f.lastBefore = before
	return f.backups[typ], f.backupsErr
}

func (f *fakeAdminOps) BackupCreate(_ context.Context, _ string, manual bool) (Backup, error) {
	f.created = true
	f.manual = manual
	return f.backup, f.backupErr
}

func (f *fakeAdminOps) ModsImport(_ context.Context, path string) (ImportResult, error) {
	f.importPath = path
	f.importData, _ = os.ReadFile(path)
	if info, err := os.Stat(path); err == nil {
		f.importMode = info.Mode().Perm()
	}
	return f.importResult, f.importErr
}

func (f *fakeAdminOps) ModsDeploy(_ context.Context, ref string) (DeployResult, error) {
	f.deploys = append(f.deploys, adminDeployCall{kind: "mods", ref: ref})
	return DeployResult{Archive: ref}, f.deployErr
}

func (f *fakeAdminOps) StateDeploy(_ context.Context, name string) (DeployResult, error) {
	f.deploys = append(f.deploys, adminDeployCall{kind: "state", ref: name})
	return DeployResult{Archive: name}, f.stateDeployErr
}

func (f *fakeAdminOps) StaffSet(_ context.Context, player string, rank int) error {
	f.staffCalls = append(f.staffCalls, struct {
		player string
		rank   int
	}{player: player, rank: rank})
	return f.staffErr
}

func (f *fakeAdminOps) ConfigGet(_ context.Context, file string) (map[string]any, error) {
	snapshot, ok := f.configSnapshots[file]
	if !ok {
		return map[string]any{}, f.configErr
	}
	return snapshot.Values, f.configErr
}

func (f *fakeAdminOps) ConfigSnapshot(_ context.Context, file string) (ConfigSnapshot, error) {
	snapshot, ok := f.configSnapshots[file]
	if !ok {
		return ConfigSnapshot{File: file, Values: map[string]any{}}, f.configErr
	}
	return snapshot, f.configErr
}

func (f *fakeAdminOps) ConfigSet(_ context.Context, file, key string, value any) error {
	f.configSets = append(f.configSets, adminConfigSetCall{file: file, key: key, value: value})
	return f.configSetErr
}

func (f *fakeAdminOps) Doctor(context.Context) (Doctor, error) {
	return f.doctor, f.doctorErr
}

type adminTestEnv struct {
	handler http.Handler
	h       *panelHandler
	cfg     Config
	ops     *fakeAdminOps
	session sessionState
	cookie  *http.Cookie
}

func newAdminTestEnv(t *testing.T, opsClient *fakeAdminOps) *adminTestEnv {
	t.Helper()
	if opsClient == nil {
		opsClient = &fakeAdminOps{}
	}
	auth, _, cfg := newTestAuthStore(t)
	h, err := newPanelHandler(cfg, auth, opsClient, log.New(io.Discard, "", 0),
		func() time.Time { return sessionTestNow })
	if err != nil {
		t.Fatalf("newPanelHandler: %v", err)
	}
	session, err := h.sessions.create(sessionTestNow, false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &adminTestEnv{
		handler: withSecurityHeaders(h.routes()),
		h:       h,
		cfg:     cfg,
		ops:     opsClient,
		session: session,
		cookie:  &http.Cookie{Name: sessionCookieName, Value: session.ID},
	}
}

func (env *adminTestEnv) request(t *testing.T, method, path string,
	form url.Values,
) *httptest.ResponseRecorder {
	t.Helper()
	if form != nil {
		if _, ok := form[csrfFormField]; !ok {
			form.Set(csrfFormField, env.session.CSRF)
		}
	}
	return panelRequest(t, env.handler, method, path, form,
		[]*http.Cookie{env.cookie}, nil)
}

func adminError(code string, exit ops.ExitCode, message string,
	details map[string]any,
) error {
	return &OpsError{Exit: exit, Code: code, Message: message, Details: details}
}

func TestAdminDashboardShowsStatusDoctorAndTimer(t *testing.T) {
	opsClient := &fakeAdminOps{
		statuses: map[string]Status{
			"tes3mp.service": {
				Unit: "tes3mp.service", State: "running", Health: "healthy",
			},
			"tes3mp-easy-backup.timer": {
				Unit: "tes3mp-easy-backup.timer", State: "stopped", Health: "unhealthy",
			},
		},
		doctor: Doctor{Status: "warn", Checks: []DoctorCheck{
			{Name: "disk", Status: "warn", Message: "свободно меньше 1 ГиБ"},
		}},
	}
	env := newAdminTestEnv(t, opsClient)
	writeTestServerConfig(t, env.cfg.Root, "Friends", 25565)
	rec := env.request(t, http.MethodGet, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Панель запущена", "работает", "Friends",
		"State backup", "Автобэкапы не идут",
		"свободно меньше 1 ГиБ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard does not contain %q:\n%s", want, body)
		}
	}
}

func TestAdminArchiveDownloadRejectsSymlink(t *testing.T) {
	tests := []struct {
		typ  string
		name string
	}{
		{typ: "state", name: "state-manual-2026-10-27_12-00-00.tar.gz"},
		{typ: "mods", name: "mods-2026-10-27_12-00-00.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			env := newAdminTestEnv(t, nil)
			dir := filepath.Join(env.cfg.Root, "backups", tt.typ)
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "secret.tar.gz")
			if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, tt.name)); err != nil {
				t.Fatal(err)
			}
			rec := env.request(t, http.MethodGet,
				"/admin/"+tt.typ+"/download/"+tt.name, nil)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("download symlink = %d, want 404: %s",
					rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminServerActionsCallOps(t *testing.T) {
	tests := []struct {
		action string
		check  func(*fakeAdminOps) int
		ok     string
	}{
		{action: "start", check: func(f *fakeAdminOps) int { return f.started }, ok: "server_started"},
		{action: "stop", check: func(f *fakeAdminOps) int { return f.stopped }, ok: "server_stopped"},
		{action: "restart", check: func(f *fakeAdminOps) int { return f.restarted }, ok: "server_restarted"},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			env := newAdminTestEnv(t, nil)
			rec := env.request(t, http.MethodPost, "/admin/server/"+tt.action,
				url.Values{})
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Location"); got != "/admin?ok="+tt.ok {
				t.Fatalf("Location = %q", got)
			}
			if got := tt.check(env.ops); got != 1 {
				t.Fatalf("action calls = %d, want 1", got)
			}
		})
	}
}

func TestAdminServerActionRequiresCSRF(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	rec := panelRequest(t, env.handler, http.MethodPost, "/admin/server/start", nil,
		[]*http.Cookie{env.cookie}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF = %d, want 403", rec.Code)
	}
	if env.ops.started != 0 {
		t.Fatal("start was called without CSRF")
	}
}

func TestAdminServerActionFailureKeepsStatus(t *testing.T) {
	opsClient := &fakeAdminOps{
		startErr: adminError(ops.CodeServiceUnavailable,
			ops.ExitServiceUnavailable, "systemd is not running", nil),
	}
	env := newAdminTestEnv(t, opsClient)
	rec := env.request(t, http.MethodPost, "/admin/server/start", url.Values{})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed action = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "systemd is not running") {
		t.Fatalf("failure page lost the ops message:\n%s", rec.Body.String())
	}
}

func TestAdminDoctorErrorBlocksMutations(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		doctor: Doctor{Status: "error", Checks: []DoctorCheck{
			{Name: "disk", Status: "error", Message: "меньше 512 MiB"},
		}},
	})
	rec := env.request(t, http.MethodPost, "/admin/server/start", url.Values{})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("doctor-blocked mutation = %d, want 503", rec.Code)
	}
	if env.ops.started != 0 {
		t.Fatal("doctor-blocked mutation reached ops")
	}
	if !strings.Contains(rec.Body.String(), "disk") ||
		!strings.Contains(rec.Body.String(), "меньше 512 MiB") {
		t.Fatalf("doctor error was not shown:\n%s", rec.Body.String())
	}
}

func TestAdminLogsPageAndAPI(t *testing.T) {
	opsClient := &fakeAdminOps{logs: []string{"first", "second"}}
	env := newAdminTestEnv(t, opsClient)

	rec := env.request(t, http.MethodGet, "/admin/logs?tail=25", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/logs = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "second") || opsClient.tail != 25 {
		t.Fatalf("logs page/body tail = %d, body=%s", opsClient.tail, rec.Body.String())
	}

	rec = env.request(t, http.MethodGet, "/api/v1/admin/logs?tail=7", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET admin logs API = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	var response struct {
		Lines []string `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if strings.Join(response.Lines, ",") != "first,second" || opsClient.tail != 7 {
		t.Fatalf("logs response = %+v, tail=%d", response, opsClient.tail)
	}

	rec = env.request(t, http.MethodGet, "/api/v1/admin/logs?tail=0", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid tail = %d, want 400", rec.Code)
	}
}

func TestAdminStatusAPI(t *testing.T) {
	started := "2026-10-23T10:00:00Z"
	env := newAdminTestEnv(t, &fakeAdminOps{statuses: map[string]Status{
		"tes3mp.service": {
			State: "running", Health: "healthy", StartedAt: &started,
		},
		"tes3mp-easy-backup.timer": {State: "stopped"},
	}})
	rec := env.request(t, http.MethodGet, "/api/v1/admin/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Status      string  `json:"status"`
		Health      string  `json:"health"`
		StartedAt   *string `json:"started_at"`
		BackupTimer string  `json:"backup_timer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if response.Status != "running" || response.Health != "healthy" ||
		response.StartedAt == nil || *response.StartedAt != started ||
		response.BackupTimer != "stopped" {
		t.Fatalf("status response = %+v", response)
	}

	anon := panelRequest(t, env.handler, http.MethodGet, "/api/v1/admin/status",
		nil, nil, nil)
	if anon.Code != http.StatusSeeOther {
		t.Fatalf("anonymous status API = %d, want login redirect", anon.Code)
	}
}

func TestAdminStaticScriptIsServed(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	rec := panelRequest(t, env.handler, http.MethodGet, "/static/admin.js", nil, nil, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "admin/status") {
		t.Fatalf("GET /static/admin.js = %d: %s", rec.Code, rec.Body.String())
	}
}

func modsBackupsFixture() Backups {
	current := "mods-2026-10-23_12-00-00.tar.gz"
	previous := "mods-2026-10-22_12-00-00.tar.gz"
	return Backups{
		Type:    "mods",
		Current: &current,
		Items: []BackupItem{
			{Name: current, Size: 10, Mtime: "2026-10-23T12:00:00Z", Current: true},
			{Name: previous, Size: 20, Mtime: "2026-10-22T12:00:00Z"},
		},
	}
}

func TestAdminModsPageDeployAndRollback(t *testing.T) {
	opsClient := &fakeAdminOps{
		backups: map[string]Backups{"mods": modsBackupsFixture()},
	}
	env := newAdminTestEnv(t, opsClient)
	writeTestCurrentMods(t, env.cfg.Root, []byte("current"))

	rec := env.request(t, http.MethodGet, "/admin/mods", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/mods = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"mods-2026-10-23_12-00-00.tar.gz", "mods-2026-10-22_12-00-00.tar.gz",
		"Rollback", "multipart/form-data", "Формат архива",
		"requiredDataFiles.json", "Morrowind.esm", "scripts/",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("mods page does not contain %q:\n%s", want, body)
		}
	}

	rec = env.request(t, http.MethodPost, "/admin/mods/deploy",
		url.Values{"archive": {"mods-2026-10-22_12-00-00.tar.gz"}})
	if rec.Code != http.StatusSeeOther || len(opsClient.deploys) != 1 ||
		opsClient.deploys[0].ref != "mods-2026-10-22_12-00-00.tar.gz" {
		t.Fatalf("deploy = %d calls=%+v", rec.Code, opsClient.deploys)
	}

	opsClient.deploys = nil
	rec = env.request(t, http.MethodPost, "/admin/mods/rollback", url.Values{})
	if rec.Code != http.StatusSeeOther || len(opsClient.deploys) != 1 ||
		opsClient.deploys[0].ref != "mods-2026-10-22_12-00-00.tar.gz" {
		t.Fatalf("rollback = %d calls=%+v", rec.Code, opsClient.deploys)
	}
}

func TestAdminModsDeployRejectsUnsafeOrForeignName(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	for _, name := range []string{
		"../mods-2026-10-23_12-00-00.tar.gz",
		"state-2026-10-23_12-00-00.tar.gz",
		"mods-current.tar.gz",
	} {
		rec := env.request(t, http.MethodPost, "/admin/mods/deploy",
			url.Values{"archive": {name}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("deploy %q = %d, want 400", name, rec.Code)
		}
	}
	if len(env.ops.deploys) != 0 {
		t.Fatalf("unsafe archives reached ops: %+v", env.ops.deploys)
	}
}

func uploadRequest(t *testing.T, env *adminTestEnv, csrf string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	if csrf != "" {
		if err := writer.WriteField(csrfFormField, csrf); err != nil {
			t.Fatalf("write csrf field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", "ignored-original.tar.gz")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("write upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "https://panel.test/admin/mods/upload",
		&payload)
	req.RemoteAddr = "203.0.113.9:4321"
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(env.cookie)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

func TestAdminModsUploadStreamsToUploadsAndCleansUp(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		importResult: ImportResult{Archive: "mods-2026-10-23_13-00-00.tar.gz"},
	})
	body := bytes.Repeat([]byte("archive-data"), 1024)
	rec := uploadRequest(t, env, env.session.CSRF, body)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/mods?ok=mods_uploaded" {
		t.Fatalf("upload = %d %q: %s", rec.Code,
			rec.Header().Get("Location"), rec.Body.String())
	}
	if !strings.HasPrefix(env.ops.importPath, filepath.Join(env.cfg.DataDir, "uploads")+string(os.PathSeparator)) ||
		!strings.HasSuffix(env.ops.importPath, ".tar.gz") {
		t.Fatalf("ops import path = %q", env.ops.importPath)
	}
	if bytes.Equal(env.ops.importData, nil) || !bytes.Equal(env.ops.importData, body) {
		t.Fatal("ops did not receive the uploaded bytes")
	}
	if env.ops.importMode != 0o640 {
		t.Fatalf("upload mode = %04o, want 0640 (ops runs as tes3mp)", env.ops.importMode)
	}
	if _, err := os.Stat(env.ops.importPath); !os.IsNotExist(err) {
		t.Fatalf("upload was not removed after import: %v", err)
	}
}

func TestAdminModsUploadRejectsMissingCSRFAndCleansUp(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	rec := uploadRequest(t, env, "", []byte("data"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("upload without CSRF = %d, want 403", rec.Code)
	}
	if env.ops.importPath != "" {
		t.Fatal("upload without CSRF reached ops")
	}
	checkUploadsEmpty(t, env.cfg.DataDir)
}

func TestAdminModsUploadFailureRemovesUpload(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		importErr: adminError(ops.CodeArchiveInvalid, ops.ExitArchiveInvalid,
			"requiredDataFiles.json: CRC mismatch", map[string]any{"file": "test.esp"}),
	})
	rec := uploadRequest(t, env, env.session.CSRF, []byte("bad archive"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("failed upload = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CRC mismatch") {
		t.Fatalf("upload failure lost the ops reason:\n%s", rec.Body.String())
	}
	if _, err := os.Stat(env.ops.importPath); !os.IsNotExist(err) {
		t.Fatalf("failed upload was not removed: %v", err)
	}
}

func checkUploadsEmpty(t *testing.T, dataDir string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, "uploads"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read uploads: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("uploads directory is not empty: %v", entries)
	}
}

func stateBackupsFixture() Backups {
	next := "state-2026-10-20_12-00-00.tar.gz"
	modsName := "mods-2026-10-23_12-00-00.tar.gz"
	modsAvailable := true
	missing := false
	return Backups{
		Type: "state",
		Items: []BackupItem{
			{Name: "state-manual-2026-10-23_12-00-00.tar.gz", Size: 100,
				Mtime: "2026-10-23T12:00:00Z", ModsArchive: &modsName,
				ModsAvailable: &modsAvailable},
			{Name: "state-predeploy-2026-10-22_12-00-00.tar.gz", Size: 200,
				Mtime: "2026-10-22T12:00:00Z", ModsArchive: &modsName,
				ModsAvailable: &missing},
			{Name: next, Size: 300, Mtime: "2026-10-20T12:00:00Z",
				ModsArchive: &modsName, ModsAvailable: &modsAvailable},
		},
		NextBefore: &next,
	}
}

func TestAdminStatePageGroupsAndValidatesInput(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		backups: map[string]Backups{
			"state": stateBackupsFixture(),
			"mods":  modsBackupsFixture(),
		},
	})
	rec := env.request(t, http.MethodGet, "/admin/state", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/state = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"state-manual", "state-predeploy", "Восстановить",
		"Скачать состояние", "mods-2026-10-23_12-00-00.tar.gz",
		"Показать более старые", "state-2026-10-20_12-00-00.tar.gz",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("state page does not contain %q:\n%s", want, body)
		}
	}

	rec = env.request(t, http.MethodGet, "/admin/restore-points", nil)
	if rec.Code != http.StatusMovedPermanently ||
		rec.Header().Get("Location") != "/admin/state" {
		t.Fatalf("GET /admin/restore-points = %d %q", rec.Code,
			rec.Header().Get("Location"))
	}

	rec = env.request(t, http.MethodGet, "/admin/state?before=../bad.tar.gz", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsafe before = %d, want 400", rec.Code)
	}
	rec = env.request(t, http.MethodGet, "/admin/state?limit=999", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit = %d, want 400", rec.Code)
	}

	rec = env.request(t, http.MethodGet,
		"/admin/state?before=state-2026-10-20_12-00-00.tar.gz", nil)
	if rec.Code != http.StatusOK ||
		env.ops.lastBefore != "state-2026-10-20_12-00-00.tar.gz" {
		t.Fatalf("before was not forwarded: %d %q", rec.Code, env.ops.lastBefore)
	}
}

func TestAdminStateCreateAndDeploy(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		backups: map[string]Backups{
			"state": stateBackupsFixture(),
			"mods":  modsBackupsFixture(),
		},
		backup: Backup{Archive: "state-manual-2026-10-23_13-00-00.tar.gz"},
	})
	rec := env.request(t, http.MethodPost, "/admin/state/create", url.Values{})
	if rec.Code != http.StatusSeeOther || !env.ops.created || !env.ops.manual {
		t.Fatalf("create = %d created=%v manual=%v", rec.Code,
			env.ops.created, env.ops.manual)
	}

	rec = env.request(t, http.MethodPost, "/admin/state/deploy",
		url.Values{"archive": {"state-manual-2026-10-23_12-00-00.tar.gz"}})
	if rec.Code != http.StatusSeeOther || len(env.ops.deploys) != 1 ||
		env.ops.deploys[0].kind != "state" {
		t.Fatalf("deploy = %d calls=%+v", rec.Code, env.ops.deploys)
	}

	env.ops.deploys = nil
	rec = env.request(t, http.MethodPost, "/admin/state/deploy",
		url.Values{"archive": {"mods-2026-10-23_12-00-00.tar.gz"}})
	if rec.Code != http.StatusBadRequest || len(env.ops.deploys) != 0 {
		t.Fatalf("foreign archive = %d calls=%+v", rec.Code, env.ops.deploys)
	}
}

func TestAdminStateCreateFailureRendersReason(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		backups: map[string]Backups{"state": stateBackupsFixture()},
		backupErr: adminError(ops.CodeNoSpace, ops.ExitNoSpace,
			"not enough space for state backup", nil),
	})
	rec := env.request(t, http.MethodPost, "/admin/state/create", url.Values{})
	if rec.Code != http.StatusInsufficientStorage ||
		!strings.Contains(rec.Body.String(), "not enough space") {
		t.Fatalf("create failure = %d: %s", rec.Code, rec.Body.String())
	}
}

func adminConfigSnapshots() map[string]ConfigSnapshot {
	return map[string]ConfigSnapshot{
		"server": {
			File: "server",
			Values: map[string]any{
				"hostname": "Morrowind with friends", "port": 25565,
				"maximumPlayers": 8, "password": "", "logLevel": 1,
				"masterServerEnabled": false,
			},
		},
		"config": {
			File: "config",
			Values: map[string]any{
				"gameMode": "Default", "loginTime": 60, "passTimeWhenEmpty": false,
				"allowConsole": false, "shareBounty": false,
			},
			ReadOnly: []string{"gameSettings", "defaultTimeTable"},
		},
		"banlist": {
			File: "banlist",
			Values: map[string]any{
				"playerNames": []string{"Trouble Maker"},
				"ipAddresses": []string{"203.0.113.44"},
			},
		},
	}
}

func TestAdminConfigPageAndSave(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		configSnapshots: adminConfigSnapshots(),
	})
	rec := env.request(t, http.MethodGet, "/admin/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/config = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Morrowind with friends", "loginTime", "playerNames", "Trouble Maker",
		`action="/admin/config/config"`, `action="/admin/config/banlist"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("config page does not contain %q:\n%s", want, body)
		}
	}

	rec = env.request(t, http.MethodPost, "/admin/config/config",
		url.Values{"key": {"loginTime"}, "value": {"120"}})
	if rec.Code != http.StatusOK || len(env.ops.configSets) != 1 {
		t.Fatalf("config save = %d calls=%+v", rec.Code, env.ops.configSets)
	}
	call := env.ops.configSets[0]
	if call.file != "config" || call.key != "loginTime" || call.value != "120" {
		t.Fatalf("config set call = %+v", call)
	}
	if !strings.Contains(rec.Body.String(), "Настройка сохранена") ||
		!strings.Contains(rec.Body.String(), "Нужен перезапуск сервера") {
		t.Fatalf("config save page has no confirmation:\n%s", rec.Body.String())
	}

	rec = env.request(t, http.MethodPost, "/admin/config/server",
		url.Values{"key": {"masterServerEnabled"}})
	if rec.Code != http.StatusOK || len(env.ops.configSets) != 2 ||
		env.ops.configSets[1].value != "false" {
		t.Fatalf("unchecked bool save = %d calls=%+v", rec.Code, env.ops.configSets)
	}
}

func TestAdminConfigSavesWholeFormDiff(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		configSnapshots: adminConfigSnapshots(),
	})
	form := url.Values{
		"hostname":            {"Morrowind with friends"},
		"port":                {"25566"},
		"maximumPlayers":      {"8"},
		"password":            {""},
		"logLevel":            {"1"},
		"masterServerEnabled": {"false"},
	}
	rec := env.request(t, http.MethodPost, "/admin/config/server", form)
	if rec.Code != http.StatusOK || len(env.ops.configSets) != 1 {
		t.Fatalf("full form save = %d calls=%+v", rec.Code, env.ops.configSets)
	}
	call := env.ops.configSets[0]
	if call.key != "port" || call.value != 25566 {
		t.Fatalf("full form changed %+v, want only port", call)
	}
	if !strings.Contains(rec.Body.String(), "Сохранено полей: 1") {
		t.Fatalf("full form page has no diff count:\n%s", rec.Body.String())
	}

	env.ops.configSnapshots["server"].Values["port"] = 25566
	rec = env.request(t, http.MethodPost, "/admin/config/server", form)
	if rec.Code != http.StatusOK || len(env.ops.configSets) != 1 {
		t.Fatalf("second unchanged form = %d calls=%+v", rec.Code, env.ops.configSets)
	}
	if !strings.Contains(rec.Body.String(), "Изменений нет") {
		t.Fatalf("unchanged form page has no notice:\n%s", rec.Body.String())
	}
}

func TestAdminConfigSaveFailureHighlightsField(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		configSnapshots: adminConfigSnapshots(),
		configSetErr: adminError(ops.CodeConfigInvalid, ops.ExitConfigInvalid,
			"loginTime must be in 10..600", map[string]any{"key": "loginTime"}),
	})
	rec := env.request(t, http.MethodPost, "/admin/config/config",
		url.Values{"key": {"loginTime"}, "value": {"1"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid config = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "loginTime must be in 10..600") ||
		!strings.Contains(body, "Проверьте поле: loginTime") {
		t.Fatalf("config failure does not highlight the field:\n%s", body)
	}
}

func TestAdminConfigRejectsUnknownFile(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	rec := env.request(t, http.MethodPost, "/admin/config/other",
		url.Values{"key": {"x"}, "value": {"y"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown config file = %d, want 404", rec.Code)
	}
}

func writePlayerFile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "players")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir players: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o640); err != nil {
		t.Fatalf("write player %s: %v", name, err)
	}
}

func TestAdminStaffListsPlayersAndSetsRank(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	writePlayerFile(t, env.cfg.Root, "John Doe",
		`{"name":"John Doe","settings":{"staffRank":0,"other":"keep"}}`)
	writePlayerFile(t, env.cfg.Root, "Jane",
		`{"name":"Jane","settings":{"staffRank":2}}`)
	writePlayerFile(t, env.cfg.Root, "Broken", `{not json`)

	rec := env.request(t, http.MethodGet, "/admin/staff", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/staff = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"John Doe", "Jane", "2 — админ", "0 — игрок", "повреждённый JSON",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("staff page does not contain %q:\n%s", want, body)
		}
	}

	rec = env.request(t, http.MethodPost, "/admin/staff/John%20Doe",
		url.Values{"rank": {"3"}})
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/staff?ok=staff_saved&restart=1" {
		t.Fatalf("staff set = %d %q: %s", rec.Code,
			rec.Header().Get("Location"), rec.Body.String())
	}
	if len(env.ops.staffCalls) != 1 || env.ops.staffCalls[0].player != "John Doe" ||
		env.ops.staffCalls[0].rank != 3 {
		t.Fatalf("staff calls = %+v", env.ops.staffCalls)
	}

	rec = env.request(t, http.MethodPost, "/admin/staff/Jane",
		url.Values{"rank": {"9"}})
	if rec.Code != http.StatusBadRequest || len(env.ops.staffCalls) != 1 {
		t.Fatalf("invalid rank = %d calls=%+v", rec.Code, env.ops.staffCalls)
	}

	rec = env.request(t, http.MethodGet,
		"/admin/staff?ok=staff_saved&restart=1", nil)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "Нужен перезапуск сервера") {
		t.Fatalf("staff restart banner = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidPlayerName(t *testing.T) {
	for _, name := range []string{"John Doe", "Jane", "player_1", "Игрок"} {
		if !validPlayerName(name) {
			t.Errorf("validPlayerName(%q) = false", name)
		}
	}
	for _, name := range []string{"", ".", "..", "../x", "a/b", `a\b`,
		"bad\x00name", "a..b"} {
		if validPlayerName(name) {
			t.Errorf("validPlayerName(%q) = true", name)
		}
	}
}

func TestAdminMutatingPageRequiresLogin(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	rec := panelRequest(t, env.handler, http.MethodPost, "/admin/state/create",
		url.Values{csrfFormField: {"anything"}}, nil, nil)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("anonymous mutation = %d %q", rec.Code,
			rec.Header().Get("Location"))
	}
	if env.ops.created {
		t.Fatal("anonymous mutation reached ops")
	}
}
