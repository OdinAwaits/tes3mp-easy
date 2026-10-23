package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
)

func TestCheckServerUsesPublicAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request,
	) {
		if r.URL.Path != "/api/v1/server" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w,
			`{"name":"Friends","address":"203.0.113.10","port":25565,"status":"running","mods_sha256":null,"panel_version":"0.1.0"}`)
	}))
	defer server.Close()

	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	profile := testServer()
	profile.PanelURL = server.URL
	id, err := service.SaveServer("", profile)
	if err != nil {
		t.Fatal(err)
	}
	info, err := service.CheckServer(t.Context(), id)
	if err != nil {
		t.Fatalf("CheckServer: %v", err)
	}
	if info.Name != "Friends" || info.Status != "running" {
		t.Fatalf("server = %+v", info)
	}
	if _, err := service.CheckServer(t.Context(), "missing"); err == nil {
		t.Fatal("missing profile accepted")
	}
	if _, err := service.CheckServer(t.Context(), ""); err == nil {
		t.Fatal("empty profile accepted")
	}
}

func TestUpdateModsWiring(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := makeMorrowind(t, morrowind.RequiredFiles...)
	workDir := filepath.Join(t.TempDir(), "work")
	if err := service.UpdateSettings(Settings{
		MorrowindPath: root,
		WorkDir:       workDir,
	}); err != nil {
		t.Fatal(err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatal(err)
	}

	var captured mods.Options
	service.installMods = func(_ context.Context, options mods.Options,
	) (mods.Result, error) {
		captured = options
		return mods.Result{Skipped: true}, nil
	}
	called := false
	result, err := service.UpdateMods(t.Context(), id, func(mods.Progress) {
		called = true
	})
	if err != nil || !result.Skipped {
		t.Fatalf("UpdateMods = %+v, %v", result, err)
	}
	if captured.WorkDir != workDir || captured.CacheDir != paths.CacheDir ||
		captured.ServerID != id || captured.Client == nil {
		t.Fatalf("options = %+v", captured)
	}
	if captured.Morrowind.Root != root {
		t.Fatalf("Morrowind root = %q", captured.Morrowind.Root)
	}
	if captured.Progress == nil {
		t.Fatal("progress callback is nil")
	}
	captured.Progress(mods.Progress{Stage: mods.StageInstalling})
	if !called {
		t.Fatal("progress callback was not forwarded")
	}
	if _, err := service.UpdateMods(t.Context(), "missing", nil); err == nil {
		t.Fatal("missing profile accepted")
	}
}

func TestUpdateModsRequiresMorrowind(t *testing.T) {
	service, err := OpenWithEnv(testPaths(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateMods(t.Context(), id, nil); err == nil {
		t.Fatal("update without Morrowind accepted")
	}
}

func TestModsState(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatal(err)
	}
	serverDir := filepath.Join(paths.DataDir, "servers", id)
	dataFiles := filepath.Join(serverDir, "data", "Data Files")
	if err := os.MkdirAll(dataFiles, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataFiles, "mod.esp"),
		[]byte("mod"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "openmw.cfg"),
		[]byte("content=mod.esp\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	if err := os.WriteFile(filepath.Join(serverDir, ".mods-sha256"),
		[]byte(digest+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ModsState(id)
	if err != nil {
		t.Fatalf("ModsState: %v", err)
	}
	if !state.Installed || state.SHA256 != digest || state.Plugins != 1 {
		t.Fatalf("state = %+v", state)
	}
}
