// Package restorepoint implements the snapshot and checkpoint archive format
// from docs/v2/03-contracts.md §6.2 and §6.3.
package restorepoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

const (
	timeLayout = "2006-01-02_15-04-05"
	suffix     = ".tar.gz"

	maxCollision = 9999
)

var (
	// ErrInvalid marks a malformed restore point archive or manifest.
	ErrInvalid = errors.New("invalid restore point")
	// ErrLimit marks an archive that exceeds a configured limit.
	ErrLimit = errors.New("restore point limit exceeded")
)

// Kind is one of the restore point naming schemes.
type Kind uint8

const (
	// KindSnapshot is a self-contained snapshot-<timestamp>.tar.gz archive.
	KindSnapshot Kind = iota
	// KindCheckpoint is a checkpoint-<timestamp>.tar.gz archive.
	KindCheckpoint
	// KindPredeploy is an internal checkpoint-predeploy-<timestamp>.tar.gz.
	KindPredeploy
)

// String returns the stable label used in JSON and diagnostics.
func (k Kind) String() string {
	switch k {
	case KindSnapshot:
		return "snapshot"
	case KindCheckpoint:
		return "checkpoint"
	case KindPredeploy:
		return "checkpoint-predeploy"
	default:
		return "unknown"
	}
}

func (k Kind) prefix() string {
	switch k {
	case KindSnapshot:
		return "snapshot-"
	case KindCheckpoint:
		return "checkpoint-"
	case KindPredeploy:
		return "checkpoint-predeploy-"
	default:
		return ""
	}
}

// IsCheckpoint reports whether the kind is a world checkpoint.
func (k Kind) IsCheckpoint() bool {
	return k == KindCheckpoint || k == KindPredeploy
}

// Name is a parsed restore point filename.
type Name struct {
	Kind      Kind
	Time      time.Time
	Collision int
}

// Base returns the filename without a collision suffix.
func (n Name) Base() string {
	return n.Kind.prefix() + n.Time.UTC().Format(timeLayout) + suffix
}

// String returns the filename including the collision suffix.
func (n Name) String() string {
	if n.Collision <= 0 {
		return n.Base()
	}
	stem := strings.TrimSuffix(n.Base(), suffix)
	return stem + "-" + fmt.Sprintf("%02d", n.Collision) + suffix
}

// ParseName parses a snapshot, checkpoint or predeploy archive name.
func ParseName(name string) (Name, error) {
	if err := pathguard.Base(name); err != nil {
		return Name{}, invalidf("invalid archive name %q: %v", name, err)
	}
	if !strings.HasSuffix(name, suffix) {
		return Name{}, invalidf("archive name %q must end with %s", name, suffix)
	}
	stem := strings.TrimSuffix(name, suffix)

	kind := KindSnapshot
	rest := ""
	switch {
	case strings.HasPrefix(stem, KindPredeploy.prefix()):
		kind = KindPredeploy
		rest = strings.TrimPrefix(stem, KindPredeploy.prefix())
	case strings.HasPrefix(stem, KindCheckpoint.prefix()):
		kind = KindCheckpoint
		rest = strings.TrimPrefix(stem, KindCheckpoint.prefix())
	case strings.HasPrefix(stem, KindSnapshot.prefix()):
		kind = KindSnapshot
		rest = strings.TrimPrefix(stem, KindSnapshot.prefix())
	default:
		return Name{}, invalidf("archive name %q has an unknown prefix", name)
	}
	return parseRest(name, kind, rest)
}

func parseRest(name string, kind Kind, rest string) (Name, error) {
	if len(rest) < len(timeLayout) {
		return Name{}, invalidf("archive name %q must contain a UTC timestamp", name)
	}
	at, err := time.Parse(timeLayout, rest[:len(timeLayout)])
	if err != nil {
		return Name{}, invalidf("archive name %q has an invalid timestamp: %v", name, err)
	}

	collision := 0
	if tail := rest[len(timeLayout):]; tail != "" {
		digits := ""
		if strings.HasPrefix(tail, "-") {
			digits = tail[1:]
		}
		if len(digits) < 2 || !allDigits(digits) ||
			strings.Trim(digits, "0") == "" ||
			(len(digits) > 2 && digits[0] == '0') {
			return Name{}, invalidf("archive name %q has an invalid collision suffix", name)
		}
		collision, err = strconv.Atoi(digits)
		if err != nil || collision <= 0 {
			return Name{}, invalidf("archive name %q has an invalid collision suffix", name)
		}
	}
	return Name{Kind: kind, Time: at.UTC(), Collision: collision}, nil
}

func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ValidateName checks that name belongs to kind. Predeploy archives are a
// separate kind so an ordinary checkpoint selector cannot pick one up.
func ValidateName(kind Kind, name string) error {
	parsed, err := ParseName(name)
	if err != nil {
		return err
	}
	if parsed.Kind != kind {
		return invalidf("archive name %q is not a %s archive", name, kind)
	}
	return nil
}

// NextName returns a free archive name for kind. A collision suffix is added
// when an archive with the same second already exists.
func NextName(dir string, kind Kind, now time.Time) (string, error) {
	if kind.prefix() == "" {
		return "", errors.New("restore point: unknown archive kind")
	}
	base := Name{Kind: kind, Time: now.UTC()}
	for collision := 0; collision <= maxCollision; collision++ {
		base.Collision = collision
		name := base.String()
		_, err := os.Lstat(filepath.Join(dir, name))
		switch {
		case errors.Is(err, os.ErrNotExist):
			return name, nil
		case err != nil:
			return "", err
		}
	}
	return "", fmt.Errorf("restore point: no free archive name for %s", base.Base())
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
