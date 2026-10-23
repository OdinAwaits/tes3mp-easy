package resource

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

var opsBinary string

func TestMain(m *testing.M) {
	code, err := runTestMain(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource harness: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runTestMain(m *testing.M) (int, error) {
	root, err := moduleRoot()
	if err != nil {
		return 0, err
	}

	tempDir, err := os.MkdirTemp("", "tes3mp-easy-ops-resource-*")
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	opsBinary = filepath.Join(tempDir, "tes3mp-easy-ops"+exeSuffix())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build",
		"-trimpath", "-o", opsBinary, "./cmd/tes3mp-easy-ops")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, buildErr := cmd.CombinedOutput(); buildErr != nil {
		return 0, fmt.Errorf("build ops binary: %w\n%s", buildErr, output)
	}

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
