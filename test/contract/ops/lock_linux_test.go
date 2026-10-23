//go:build linux

package ops

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// holdOpsLock takes the production ops lock exclusively and keeps it until the
// test finishes. Read commands must still succeed with a tiny --lock-timeout.
func holdOpsLock(t *testing.T, root string) {
	t.Helper()

	path := filepath.Join(root, ".locks", "ops.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create .locks: %v", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open ops lock: %v", err)
	}
	if err := syscall.Flock(int(file.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		t.Fatalf("lock %s: %v", path, err)
	}

	t.Cleanup(func() {
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Errorf("close %s: %v", path, err)
		}
	})
}
