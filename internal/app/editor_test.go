package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

func TestEditorStateReportsExpectedPath(t *testing.T) {
	service, workDir, _ := installedService(t)
	state, err := service.EditorState()
	if err != nil {
		t.Fatalf("EditorState: %v", err)
	}
	want := filepath.Join(tes3mp.Dir(workDir), launcher.OpenMWCSExecutable)
	if state.Path != want {
		t.Fatalf("path = %q, want %q", state.Path, want)
	}
	if state.Found {
		t.Fatalf("state = %+v, want not found", state)
	}
}

func TestEditorStateFindsExecutable(t *testing.T) {
	service, workDir, _ := installedService(t)
	tes3mpDir := tes3mp.Dir(workDir)
	if err := os.MkdirAll(tes3mpDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tes3mpDir, launcher.OpenMWCSExecutable)
	if err := os.WriteFile(path, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.EditorState()
	if err != nil {
		t.Fatalf("EditorState: %v", err)
	}
	if !state.Found || state.Path != path {
		t.Fatalf("state = %+v, want found at %q", state, path)
	}
}
