// Package lock provides the advisory file lock shared by all ops mutations.
//
// The production lock file is <root>/.locks/ops.lock. Mutations take an
// exclusive lock, while --dry-run uses a shared lock. The lock is advisory:
// every writer must agree to use this package. It is released when the
// process exits, including a crash.
//
// Callers must not acquire the same lock recursively. flock is attached to the
// open file description, so a nested Acquire would wait for itself.
package lock

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	// ErrLocked reports that another process or descriptor still holds the
	// lock after the configured timeout.
	ErrLocked = errors.New("lock is held")

	// ErrUnsupported reports that this platform has no flock implementation.
	ErrUnsupported = errors.New("file locking is not supported")
)

// pollInterval is the retry interval while waiting for a busy lock. It is short
// enough to keep CLI latency low and long enough not to spin the CPU.
const pollInterval = 10 * time.Millisecond

// Mode selects shared or exclusive access.
type Mode int

const (
	// Exclusive blocks every other reader and writer. Mutations use it.
	Exclusive Mode = iota

	// Shared blocks exclusive writers but permits other shared readers.
	// --dry-run uses it.
	Shared
)

// String returns the mode name used in errors and tests.
func (m Mode) String() string {
	switch m {
	case Exclusive:
		return "exclusive"
	case Shared:
		return "shared"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

func (m Mode) valid() bool {
	return m == Exclusive || m == Shared
}

// Lock is an acquired advisory lock. Close releases it and leaves the lock
// file in place for the next process.
type Lock struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// Acquire opens path and takes a lock in the requested mode. It retries a busy
// lock every few milliseconds and returns ErrLocked when timeout expires.
// A zero timeout makes exactly one non-blocking attempt.
//
// The parent directory must already exist. Ops creates .locks during install;
// tests create it in their temporary root.
func Acquire(path string, mode Mode, timeout time.Duration) (*Lock, error) {
	switch {
	case path == "":
		return nil, errors.New("lock path is empty")
	case !mode.valid():
		return nil, fmt.Errorf("invalid lock mode %s", mode)
	case timeout < 0:
		return nil, fmt.Errorf("lock timeout must not be negative: %s", timeout)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", path, err)
	}

	deadline := time.Now().Add(timeout)
	for {
		acquired, lockErr := tryLock(file, mode)
		if lockErr != nil {
			_ = file.Close()
			return nil, fmt.Errorf("lock %s: %w", path, lockErr)
		}
		if acquired {
			return &Lock{path: path, file: file}, nil
		}
		if timeout == 0 || !time.Now().Before(deadline) {
			_ = file.Close()
			return nil, fmt.Errorf("%w: %s (timeout %s)", ErrLocked, path, timeout)
		}

		wait := pollInterval
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait > 0 {
			time.Sleep(wait)
		}
	}
}

// Path returns the lock file path.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Close releases the lock and closes its descriptor. It is safe to call Close
// more than once. The lock file itself is intentionally not removed: removing
// it would race with another process opening the same path.
func (l *Lock) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}

	file := l.file
	l.file = nil
	unlockErr := unlockFile(file)
	closeErr := file.Close()
	return errors.Join(unlockErr, closeErr)
}
