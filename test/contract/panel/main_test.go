package panel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

var (
	testPanelBinary string
	testFakeBin     string
)

func TestMain(m *testing.M) {
	code, err := runTestMain(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "panel contract harness: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runTestMain(m *testing.M) (int, error) {
	root, err := moduleRoot()
	if err != nil {
		return 0, err
	}

	tempDir, err := os.MkdirTemp("", "tes3mp-easy-panel-contract-*")
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	binary := filepath.Join(tempDir, "tes3mp-easy-panel"+exeSuffix())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build",
		"-trimpath", "-o", binary, "./cmd/tes3mp-easy-panel")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, buildErr := cmd.CombinedOutput(); buildErr != nil {
		return 0, fmt.Errorf("build panel binary: %w\n%s", buildErr, output)
	}

	fakeBin := filepath.Join(tempDir, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		return 0, fmt.Errorf("create fake bin: %w", err)
	}
	sudoPath := filepath.Join(fakeBin, "sudo")
	if err := os.WriteFile(sudoPath, []byte(fakeSudo), 0o755); err != nil {
		return 0, fmt.Errorf("write fake sudo: %w", err)
	}

	testPanelBinary = binary
	testFakeBin = fakeBin
	return m.Run(), nil
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil &&
			!info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
