//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package lock

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAcquireUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ops.lock")
	if _, err := Acquire(path, Exclusive, 0); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Acquire = %v, want ErrUnsupported", err)
	}
}
