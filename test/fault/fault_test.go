package fault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

func TestCorruptModsArchiveLeavesUploadIntact(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"panel-data/uploads",
		"backups/mods",
		".locks",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	upload := filepath.Join(root, "panel-data", "uploads", "upload.tar.gz")
	body := []byte("this is not a gzip stream\n")
	if err := os.WriteFile(upload, body, 0o640); err != nil {
		t.Fatalf("write upload: %v", err)
	}

	res := runRoot(t, root, nil, "--json", "mods", "import", "--file", upload)
	res.requireExit(t, int(ops.ExitArchiveInvalid))
	env := res.decodeEnvelope(t)
	requireFailureCode(t, env, ops.CodeArchiveInvalid)

	got, err := os.ReadFile(upload)
	if err != nil {
		t.Fatalf("upload was removed after a failed import: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("upload changed after a failed import: got %q, want %q", got, body)
	}

	entries, err := os.ReadDir(filepath.Join(root, "backups", "mods"))
	if err != nil {
		t.Fatalf("read backups/mods: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("backups/mods contains %d entries after a failed import, want none",
			len(entries))
	}
}

func TestLockedMutationDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	lockDir := filepath.Join(root, ".locks")
	if err := os.MkdirAll(lockDir, 0o750); err != nil {
		t.Fatalf("create .locks: %v", err)
	}

	held, err := lock.Acquire(filepath.Join(lockDir, "ops.lock"),
		lock.Exclusive, 0)
	if errors.Is(err, lock.ErrUnsupported) {
		t.Skip("file locking is not supported on this platform")
	}
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	res := runRoot(t, root, nil,
		"--json", "--lock-timeout=0", "backups", "create", "--type", "state")
	res.requireExit(t, int(ops.ExitLocked))
	env := res.decodeEnvelope(t)
	requireFailureCode(t, env, ops.CodeLocked)

	if _, err := os.Stat(filepath.Join(root, "backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backups/ exists after locked mutation: %v", err)
	}
}

func TestSystemdUnavailableNeverReportsRunning(t *testing.T) {
	root := t.TempDir()
	emptyBin := filepath.Join(t.TempDir(), "empty-bin")
	if err := os.MkdirAll(emptyBin, 0o750); err != nil {
		t.Fatalf("create empty PATH: %v", err)
	}

	res := runRoot(t, root, map[string]string{"PATH": emptyBin},
		"--json", "status")
	res.requireExit(t, int(ops.ExitServiceUnavailable))
	env := res.decodeEnvelope(t)
	requireFailureCode(t, env, ops.CodeServiceUnavailable)
	if env.Data != nil {
		t.Fatalf("failure envelope has data: %s", env.Data)
	}
	if bytes.Contains(bytes.ToLower([]byte(res.Stdout)), []byte("running")) {
		t.Fatalf("stdout reports running while systemd is unavailable: %s", res.Stdout)
	}
}

// waitForPart polls for an atomicfile .part file until it reaches minSize.
// It returns the part path and true, or false when done closes first.
func waitForPart(dir string, minSize int64, done <-chan error,
) (string, bool, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.part"))
		if err != nil {
			return "", false, err
		}
		for _, path := range matches {
			info, statErr := os.Stat(path)
			if statErr == nil && info.Size() >= minSize {
				return path, true, nil
			}
		}

		select {
		case waitErr := <-done:
			return "", false, waitErr
		case <-time.After(time.Millisecond):
		}
	}
	return "", false, errors.New("timed out waiting for .part")
}
