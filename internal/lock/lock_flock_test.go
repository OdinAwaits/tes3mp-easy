//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package lock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func lockPath(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), ".locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create .locks: %v", err)
	}
	return filepath.Join(dir, "ops.lock")
}

func acquire(t *testing.T, path string, mode Mode, timeout time.Duration) *Lock {
	t.Helper()

	held, err := Acquire(path, mode, timeout)
	if err != nil {
		t.Fatalf("Acquire(%s, %s, %s): %v", path, mode, timeout, err)
	}
	return held
}

func TestExclusiveLockContract(t *testing.T) {
	path := lockPath(t)
	first := acquire(t, path, Exclusive, 0)
	if first.Path() != path {
		t.Fatalf("Path() = %q, want %q", first.Path(), path)
	}

	if _, err := Acquire(path, Exclusive, 0); !errors.Is(err, ErrLocked) {
		t.Fatalf("second exclusive = %v, want ErrLocked", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	second := acquire(t, path, Exclusive, 0)
	if err := second.Close(); err != nil {
		t.Fatalf("Close second: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("lock mode = %v, want regular file", info.Mode())
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("lock permissions = %o, want 600", perm)
	}
}

func TestSharedLockContract(t *testing.T) {
	path := lockPath(t)
	first := acquire(t, path, Shared, 0)
	second := acquire(t, path, Shared, 0)

	if _, err := Acquire(path, Exclusive, 0); !errors.Is(err, ErrLocked) {
		t.Fatalf("exclusive while shared = %v, want ErrLocked", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close first shared: %v", err)
	}
	if _, err := Acquire(path, Exclusive, 0); !errors.Is(err, ErrLocked) {
		t.Fatalf("exclusive while second shared = %v, want ErrLocked", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close second shared: %v", err)
	}

	exclusive := acquire(t, path, Exclusive, 0)
	if _, err := Acquire(path, Shared, 0); !errors.Is(err, ErrLocked) {
		t.Fatalf("shared while exclusive = %v, want ErrLocked", err)
	}
	if err := exclusive.Close(); err != nil {
		t.Fatalf("Close exclusive: %v", err)
	}
}

func TestAcquireWaitsForRelease(t *testing.T) {
	path := lockPath(t)
	first := acquire(t, path, Exclusive, 0)

	result := make(chan error, 1)
	go func() {
		second, err := Acquire(path, Exclusive, 2*time.Second)
		if err == nil {
			err = second.Close()
		}
		result <- err
	}()

	time.Sleep(30 * time.Millisecond)
	if err := first.Close(); err != nil {
		t.Fatalf("Close first: %v", err)
	}

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("waiting Acquire: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiting Acquire did not finish after release")
	}
}

func TestAcquireTimeout(t *testing.T) {
	path := lockPath(t)
	first := acquire(t, path, Exclusive, 0)
	defer func() {
		if err := first.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	start := time.Now()
	_, err := Acquire(path, Exclusive, 50*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Acquire = %v, want ErrLocked", err)
	}
	if elapsed < 20*time.Millisecond {
		t.Fatalf("Acquire returned after %s, want it to wait for timeout", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Acquire waited %s, want about 50ms", elapsed)
	}

	start = time.Now()
	if _, err := Acquire(path, Exclusive, 0); !errors.Is(err, ErrLocked) {
		t.Fatalf("zero timeout = %v, want ErrLocked", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("zero timeout took %s", elapsed)
	}
}

func TestAcquireValidation(t *testing.T) {
	path := lockPath(t)
	missing := filepath.Join(t.TempDir(), "missing", "ops.lock")

	tests := []struct {
		name    string
		path    string
		mode    Mode
		timeout time.Duration
		want    error
	}{
		{name: "empty", path: "", mode: Exclusive, want: nil},
		{name: "negative timeout", path: path, mode: Exclusive,
			timeout: -time.Second, want: nil},
		{name: "invalid mode", path: path, mode: Mode(42), want: nil},
		{name: "missing parent", path: missing, mode: Exclusive, want: os.ErrNotExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			held, err := Acquire(tt.path, tt.mode, tt.timeout)
			if err == nil {
				_ = held.Close()
				t.Fatalf("Acquire(%q, %s, %s) succeeded", tt.path, tt.mode, tt.timeout)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("validation created lock file: %v", err)
	}
}

func TestNilLock(t *testing.T) {
	var held *Lock
	if held.Path() != "" {
		t.Fatalf("nil Path() = %q", held.Path())
	}
	if err := held.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}
