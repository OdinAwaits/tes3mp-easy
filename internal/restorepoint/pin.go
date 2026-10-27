package restorepoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

const maxPinSize = 4 << 10

type pinInfo struct {
	PinnedAt time.Time `json:"pinned_at"`
}

// Pin writes <name>.pin atomically. Re-pinning is idempotent.
func Pin(root, name string, now time.Time) error {
	if err := ValidateName(KindCheckpoint, name); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	data, err := json.Marshal(pinInfo{PinnedAt: now.UTC()})
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicfile.WriteFile(PinPath(root, name), data, 0o640)
}

// Unpin removes <name>.pin. A missing sidecar is not an error.
func Unpin(root, name string) error {
	if err := ValidateName(KindCheckpoint, name); err != nil {
		return err
	}
	err := os.Remove(PinPath(root, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func pinned(root, name string) bool {
	path := PinPath(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

// ReadPin reads and validates a pin sidecar. It is used by ops listing to
// reject a corrupted sidecar instead of silently treating it as unpinned.
func ReadPin(root, name string) (time.Time, bool, error) {
	path := PinPath(root, name)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	if len(data) > maxPinSize {
		return time.Time{}, false, fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, path, maxPinSize)
	}
	var info pinInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return time.Time{}, false, invalidf("%s: %v", path, err)
	}
	if info.PinnedAt.IsZero() {
		return time.Time{}, false, invalidf("%s: pinned_at is missing", path)
	}
	return info.PinnedAt.UTC(), true, nil
}
