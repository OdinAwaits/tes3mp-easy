package archive

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
	archiveTimeLayout = "2006-01-02_15-04-05"
	archiveSuffix     = ".tar.gz"

	// maxArchiveCollision keeps NextArchiveName bounded if a directory cannot be
	// inspected or contains an implausible number of collisions.
	maxArchiveCollision = 9999
)

// ArchiveKind is one of the archive naming schemes from
// docs/v2/03-contracts.md §6.7.
type ArchiveKind uint8

const (
	// KindMods is an imported mod pack: mods-<timestamp>.tar.gz.
	KindMods ArchiveKind = iota
	// KindModsInit is the bootstrap mod pack: mods-init-<timestamp>.tar.gz.
	KindModsInit
	// KindStateAuto is a timer-created state backup: state-<timestamp>.tar.gz.
	KindStateAuto
	// KindStateManual is a manual state backup: state-manual-<timestamp>.tar.gz.
	KindStateManual
	// KindStatePredeploy is a state backup made before a deploy:
	// state-predeploy-<timestamp>.tar.gz.
	KindStatePredeploy
)

// String returns the stable label used in diagnostics.
func (k ArchiveKind) String() string {
	switch k {
	case KindMods:
		return "mods"
	case KindModsInit:
		return "mods-init"
	case KindStateAuto:
		return "state"
	case KindStateManual:
		return "state-manual"
	case KindStatePredeploy:
		return "state-predeploy"
	default:
		return "unknown"
	}
}

func (k ArchiveKind) valid() bool {
	return k <= KindStatePredeploy
}

func (k ArchiveKind) prefix() string {
	switch k {
	case KindMods:
		return "mods-"
	case KindModsInit:
		return "mods-init-"
	case KindStateAuto:
		return "state-"
	case KindStateManual:
		return "state-manual-"
	case KindStatePredeploy:
		return "state-predeploy-"
	default:
		return ""
	}
}

// IsMods reports whether the archive belongs to the mods backup classes.
func (k ArchiveKind) IsMods() bool {
	return k == KindMods || k == KindModsInit
}

// IsState reports whether the archive belongs to the state backup classes.
func (k ArchiveKind) IsState() bool {
	return k == KindStateAuto || k == KindStateManual || k == KindStatePredeploy
}

// ArchiveName is a parsed archive filename. Time is always UTC and Collision
// is zero for the unsuffixed name and at least one for -01, -02, ...
type ArchiveName struct {
	Kind      ArchiveKind
	Time      time.Time
	Collision int
}

// Base returns the filename without a collision suffix.
func (n ArchiveName) Base() string {
	return n.Kind.prefix() + n.Time.UTC().Format(archiveTimeLayout) + archiveSuffix
}

// String returns the filename including the collision suffix, if any.
func (n ArchiveName) String() string {
	if n.Collision <= 0 {
		return n.Base()
	}
	stem := strings.TrimSuffix(n.Base(), archiveSuffix)
	return stem + "-" + fmt.Sprintf("%02d", n.Collision) + archiveSuffix
}

// ParseArchiveName parses one of the five names accepted by §6.6. Collision
// suffixes use at least two digits and must not be zero.
func ParseArchiveName(name string) (ArchiveName, error) {
	if err := pathguard.Base(name); err != nil {
		return ArchiveName{}, archiveInvalidf(
			"invalid archive name %q: %v", name, err)
	}
	if !strings.HasSuffix(name, archiveSuffix) {
		return ArchiveName{}, archiveInvalidf(
			"archive name %q must end with %s", name, archiveSuffix)
	}

	stem := strings.TrimSuffix(name, archiveSuffix)
	kind, rest, ok := splitArchivePrefix(stem)
	if !ok {
		return ArchiveName{}, archiveInvalidf(
			"archive name %q has an unknown prefix", name)
	}
	return parseArchiveRest(name, kind, rest)
}

func splitArchivePrefix(stem string) (ArchiveKind, string, bool) {
	prefixes := []struct {
		prefix string
		kind   ArchiveKind
	}{
		{"mods-init-", KindModsInit},
		{"state-manual-", KindStateManual},
		{"state-predeploy-", KindStatePredeploy},
		{"mods-", KindMods},
		{"state-", KindStateAuto},
	}
	for _, item := range prefixes {
		if strings.HasPrefix(stem, item.prefix) {
			return item.kind, strings.TrimPrefix(stem, item.prefix), true
		}
	}
	return 0, "", false
}

func parseArchiveRest(name string, kind ArchiveKind, rest string) (ArchiveName, error) {
	if len(rest) < len(archiveTimeLayout) {
		return ArchiveName{}, archiveInvalidf(
			"archive name %q must contain a UTC timestamp", name)
	}

	timestamp := rest[:len(archiveTimeLayout)]
	at, err := time.Parse(archiveTimeLayout, timestamp)
	if err != nil {
		return ArchiveName{}, archiveInvalidf(
			"archive name %q has an invalid timestamp: %v", name, err)
	}

	collision := 0
	if suffix := rest[len(archiveTimeLayout):]; suffix != "" {
		digits := ""
		if strings.HasPrefix(suffix, "-") {
			digits = suffix[1:]
		}
		if len(digits) < 2 || !allDigits(digits) ||
			strings.Trim(digits, "0") == "" ||
			(len(digits) > 2 && digits[0] == '0') {
			return ArchiveName{}, archiveInvalidf(
				"archive name %q has an invalid collision suffix", name)
		}
		collision, err = strconv.Atoi(digits)
		if err != nil || collision <= 0 {
			return ArchiveName{}, archiveInvalidf(
				"archive name %q has an invalid collision suffix", name)
		}
	}

	return ArchiveName{Kind: kind, Time: at, Collision: collision}, nil
}

// ValidateModsArchiveName checks the mods archive naming rules from
// docs/v2/03-contracts.md §6.7. Collision suffixes such as "-01" are accepted.
func ValidateModsArchiveName(name string) error {
	parsed, err := ParseArchiveName(name)
	if err != nil {
		return err
	}
	if !parsed.Kind.IsMods() {
		return archiveInvalidf(
			"archive name %q must start with mods- or mods-init-", name)
	}
	return nil
}

// ValidateStateArchiveName checks the state archive naming rules from
// docs/v2/03-contracts.md §6.7. Collision suffixes such as "-01" are accepted.
func ValidateStateArchiveName(name string) error {
	parsed, err := ParseArchiveName(name)
	if err != nil {
		return err
	}
	if !parsed.Kind.IsState() {
		return archiveInvalidf("archive name %q is not a state archive", name)
	}
	return nil
}

// NextArchiveName returns the first free name for kind at the given time. The
// caller is still responsible for creating the archive atomically.
func NextArchiveName(
	kind ArchiveKind,
	at time.Time,
	exists func(string) bool,
) (string, error) {
	if !kind.valid() {
		return "", fmt.Errorf("archive: unknown archive kind %d", kind)
	}
	if exists == nil {
		return "", errors.New("archive: exists callback is required")
	}
	for collision := 0; collision <= maxArchiveCollision; collision++ {
		name := ArchiveName{Kind: kind, Time: at, Collision: collision}.String()
		if !exists(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf(
		"archive: no free %s name after %d collisions", kind, maxArchiveCollision)
}

// NextArchiveNameInDir is NextArchiveName for a concrete archive directory.
func NextArchiveNameInDir(
	dir string,
	kind ArchiveKind,
	at time.Time,
) (string, error) {
	if dir == "" {
		return "", errors.New("archive: archive directory is required")
	}
	if !kind.valid() {
		return "", fmt.Errorf("archive: unknown archive kind %d", kind)
	}

	for collision := 0; collision <= maxArchiveCollision; collision++ {
		name := ArchiveName{Kind: kind, Time: at, Collision: collision}.String()
		_, err := os.Lstat(filepath.Join(dir, name))
		switch {
		case err == nil:
			continue
		case errors.Is(err, os.ErrNotExist):
			return name, nil
		default:
			return "", err
		}
	}
	return "", fmt.Errorf(
		"archive: no free %s name in %s after %d collisions",
		kind, dir, maxArchiveCollision)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
