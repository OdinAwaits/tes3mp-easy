package panel

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type restoreZipEntry struct {
	name string
	body []byte
}

func restoreZip(t *testing.T, entries ...restoreZipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		target, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", entry.name, err)
		}
		if _, err := target.Write(entry.body); err != nil {
			t.Fatalf("write zip entry %s: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func restoreUploadRequest(t *testing.T, env *adminTestEnv, csrf, filename string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	if csrf != "" {
		if err := writer.WriteField(csrfFormField, csrf); err != nil {
			t.Fatalf("write csrf: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("write upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		"https://panel.test/admin/state/restore", &payload)
	req.RemoteAddr = "203.0.113.9:4321"
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(env.cookie)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

func restoreBundleFixture(t *testing.T) ([]byte, fixtures.ModsArchive,
	fixtures.StateArchive,
) {
	t.Helper()
	mods := fixtures.ScriptedMods()
	modsBody := mods.Bytes(t)
	manifest := mods.Manifest(modsBody)
	state := fixtures.FullState(manifest)
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)
	stateBody := state.Bytes(t)
	bundle := restoreZip(t,
		restoreZipEntry{name: manifest.Name, body: modsBody},
		restoreZipEntry{name: state.Name, body: stateBody})
	return bundle, mods, state
}

func TestAdminStateRestoreBundleImportsModsThenState(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	bundle, mods, state := restoreBundleFixture(t)

	rec := restoreUploadRequest(t, env, env.session.CSRF, "state-with-mods.zip", bundle)
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/state?ok=state_restored" {
		t.Fatalf("restore = %d %q: %s", rec.Code,
			rec.Header().Get("Location"), rec.Body.String())
	}

	if len(env.ops.importNamedCalls) != 1 {
		t.Fatalf("mods import calls = %+v", env.ops.importNamedCalls)
	}
	modsCall := env.ops.importNamedCalls[0]
	if modsCall.name != mods.ArchiveName() ||
		!bytes.Equal(modsCall.body, mods.Bytes(t)) {
		t.Fatalf("mods import = name %q body %d bytes", modsCall.name,
			len(modsCall.body))
	}
	if len(env.ops.stateImports) != 1 {
		t.Fatalf("state imports = %+v", env.ops.stateImports)
	}
	stateCall := env.ops.stateImports[0]
	if stateCall.name != state.Name || !bytes.Equal(stateCall.body, state.Bytes(t)) {
		t.Fatalf("state import = name %q body %d bytes", stateCall.name,
			len(stateCall.body))
	}
	if len(env.ops.deploys) != 2 ||
		env.ops.deploys[0].kind != "mods" ||
		env.ops.deploys[0].ref != mods.ArchiveName() ||
		env.ops.deploys[1].kind != "state" ||
		env.ops.deploys[1].ref != state.Name {
		t.Fatalf("deploy order = %+v", env.ops.deploys)
	}
	checkUploadsEmpty(t, env.cfg.DataDir)
}

func TestAdminStateRestoreSingleStateUsesActiveMods(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	mods := fixtures.ScriptedMods()
	manifest := mods.Install(t, filepath.Join(env.cfg.Root, "backups", "mods"))
	state := fixtures.FullState(manifest)
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)

	rec := restoreUploadRequest(t, env, env.session.CSRF, state.Name, state.Bytes(t))
	if rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/admin/state?ok=state_restored" {
		t.Fatalf("restore = %d %q: %s", rec.Code,
			rec.Header().Get("Location"), rec.Body.String())
	}
	if len(env.ops.importNamedCalls) != 0 {
		t.Fatalf("single state imported mods: %+v", env.ops.importNamedCalls)
	}
	if len(env.ops.stateImports) != 1 || env.ops.stateImports[0].name != state.Name {
		t.Fatalf("state imports = %+v", env.ops.stateImports)
	}
	if len(env.ops.deploys) != 1 || env.ops.deploys[0].kind != "state" ||
		env.ops.deploys[0].ref != state.Name {
		t.Fatalf("deploys = %+v", env.ops.deploys)
	}
}

func TestAdminStateRestoreRejectsMismatchedBundleBeforeMutation(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	expected := fixtures.ScriptedMods()
	expectedBody := expected.Bytes(t)
	state := fixtures.FullState(expected.Manifest(expectedBody))
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)
	other := fixtures.DefaultMods()
	bundle := restoreZip(t,
		restoreZipEntry{name: other.ArchiveName(), body: other.Bytes(t)},
		restoreZipEntry{name: state.Name, body: state.Bytes(t)})

	rec := restoreUploadRequest(t, env, env.session.CSRF, "bundle.zip", bundle)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatched bundle = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "does not match the snapshot") {
		t.Fatalf("bundle error is not explanatory:\n%s", rec.Body.String())
	}
	if len(env.ops.importNamedCalls) != 0 || len(env.ops.stateImports) != 0 ||
		len(env.ops.deploys) != 0 {
		t.Fatalf("mismatched bundle reached ops: imports=%+v states=%+v deploys=%+v",
			env.ops.importNamedCalls, env.ops.stateImports, env.ops.deploys)
	}
}

func TestAdminStateRestoreRejectsCollisionsBeforeMutation(t *testing.T) {
	t.Run("mods", func(t *testing.T) {
		env := newAdminTestEnv(t, &fakeAdminOps{})
		bundle, mods, _ := restoreBundleFixture(t)
		dir := filepath.Join(env.cfg.Root, "backups", "mods")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, mods.ArchiveName())
		if err := os.WriteFile(target, []byte("different bytes"), 0o640); err != nil {
			t.Fatal(err)
		}
		rec := restoreUploadRequest(t, env, env.session.CSRF, "bundle.zip", bundle)
		if rec.Code != http.StatusConflict {
			t.Fatalf("mods collision = %d, want 409: %s", rec.Code, rec.Body.String())
		}
		if len(env.ops.importNamedCalls) != 0 || len(env.ops.stateImports) != 0 ||
			len(env.ops.deploys) != 0 {
			t.Fatalf("mods collision reached ops: %+v %+v %+v",
				env.ops.importNamedCalls, env.ops.stateImports, env.ops.deploys)
		}
		checkUploadsEmpty(t, env.cfg.DataDir)
	})

	t.Run("state", func(t *testing.T) {
		env := newAdminTestEnv(t, &fakeAdminOps{})
		bundle, _, state := restoreBundleFixture(t)
		dir := filepath.Join(env.cfg.Root, "backups", "state")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, state.Name)
		if err := os.WriteFile(target, []byte("different bytes"), 0o640); err != nil {
			t.Fatal(err)
		}
		rec := restoreUploadRequest(t, env, env.session.CSRF, "bundle.zip", bundle)
		if rec.Code != http.StatusConflict {
			t.Fatalf("state collision = %d, want 409: %s", rec.Code, rec.Body.String())
		}
		if len(env.ops.importNamedCalls) != 0 || len(env.ops.stateImports) != 0 ||
			len(env.ops.deploys) != 0 {
			t.Fatalf("state collision reached ops: %+v %+v %+v",
				env.ops.importNamedCalls, env.ops.stateImports, env.ops.deploys)
		}
		checkUploadsEmpty(t, env.cfg.DataDir)
	})
}

func TestAdminStateRestoreRejectsStateForOtherActiveMods(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	active := fixtures.DefaultMods()
	active.Install(t, filepath.Join(env.cfg.Root, "backups", "mods"))
	other := fixtures.ScriptedMods()
	otherBody := other.Bytes(t)
	state := fixtures.FullState(other.Manifest(otherBody))
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)

	rec := restoreUploadRequest(t, env, env.session.CSRF, state.Name, state.Bytes(t))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("state for other mods = %d, want 422: %s", rec.Code,
			rec.Body.String())
	}
	if len(env.ops.stateImports) != 0 || len(env.ops.deploys) != 0 {
		t.Fatalf("mismatched state reached ops: %+v %+v",
			env.ops.stateImports, env.ops.deploys)
	}
}

func TestAdminStateRestoreRequiresCSRFAndCleansUpload(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	rec := restoreUploadRequest(t, env, "", "bundle.zip", []byte("not a zip"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("restore without CSRF = %d, want 403", rec.Code)
	}
	if len(env.ops.importNamedCalls) != 0 || len(env.ops.stateImports) != 0 {
		t.Fatal("restore without CSRF reached ops")
	}
	checkUploadsEmpty(t, env.cfg.DataDir)
}

func TestRestoreStateNameFallsBackToGeneratedName(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	name, err := env.h.restoreStateName("downloaded.tar.gz")
	if err != nil {
		t.Fatalf("restoreStateName: %v", err)
	}
	if !strings.HasPrefix(name, "state-manual-") ||
		!strings.HasSuffix(name, ".tar.gz") {
		t.Fatalf("generated state name = %q", name)
	}
}

func TestRestoreExtractsBundleWithReadableModes(t *testing.T) {
	env := newAdminTestEnv(t, nil)
	bundle, _, _ := restoreBundleFixture(t)
	path := filepath.Join(t.TempDir(), "bundle.zip")
	if err := os.WriteFile(path, bundle, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, cleanup, err := env.h.extractRestoreBundle(path)
	if err != nil {
		t.Fatalf("extractRestoreBundle: %v", err)
	}
	defer cleanup()
	for _, path := range []string{plan.statePath, plan.modsPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat extracted %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o640 {
			t.Fatalf("%s mode = %04o, want 0640", path, info.Mode().Perm())
		}
	}
	// ops runs as tes3mp and renames the archives into backups/, so the shared
	// group needs write access to the directory, not just traverse.
	dirInfo, err := os.Stat(filepath.Dir(plan.modsPath))
	if err != nil {
		t.Fatalf("stat restore directory: %v", err)
	}
	if dirInfo.Mode().Perm()&0o070 != 0o070 {
		t.Fatalf("restore directory mode = %04o, want group rwx (ops renames files out of it)",
			dirInfo.Mode().Perm())
	}
	if dirInfo.Mode()&os.ModeSetgid == 0 {
		t.Fatalf("restore directory mode = %04o, want the setgid bit so the group stays shared",
			dirInfo.Mode().Perm())
	}
}
