// Package retention implements the backup retention policy from
// docs/v2/03-contracts.md §6.7. Build is a pure planner; Apply performs the
// planned deletions and reports failures as warnings.
package retention

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

const (
	// DefaultStateBudget is the backups/state directory budget (1 GiB).
	DefaultStateBudget int64 = 1 << 30
	// DefaultModsBudget is the backups/mods directory budget (512 MiB).
	DefaultModsBudget int64 = 512 << 20
	// DefaultMinFree is the free-space reserve (512 MiB).
	DefaultMinFree int64 = 512 << 20

	hotWindow      = 7 * 24 * time.Hour
	dailyWindow    = 365 * 24 * time.Hour
	monthlyWindow  = 3 * 365 * 24 * time.Hour
	pressureWindow = 24 * time.Hour

	keepManual    = 5
	keepPredeploy = 2
	keepMods      = 5
	keepModsInit  = 2
)

// FreeSpaceFunc reports bytes available on the filesystem containing path.
type FreeSpaceFunc func(path string) (int64, error)

// DiskFree reports bytes available on the filesystem containing path.
func DiskFree(path string) (int64, error) {
	if path == "" {
		return 0, retentionErrorf("disk free: path is required")
	}
	return diskFree(path)
}

// Config controls one retention run. Zero values for the three budgets select
// their documented defaults; a negative budget or MinFree disables that check.
// StateDir and ModsDir may be empty, but at least one must be set.
type Config struct {
	StateDir string
	ModsDir  string

	// CurrentMods is an optional override for <ModsDir>/current.txt. It is
	// useful when the caller already parsed current.txt.
	CurrentMods string

	Now         time.Time
	StateBudget int64
	ModsBudget  int64
	MinFree     int64
	FreeSpace   FreeSpaceFunc
}

// File is one recognized archive in a backup directory.
type File struct {
	Name      string
	Path      string
	Kind      archive.ArchiveKind
	Time      time.Time
	Collision int
	Size      int64
}

// Plan describes the result of Build without modifying the filesystem. The
// *After fields are estimates assuming every planned deletion succeeds.
type Plan struct {
	Keep  []File
	Prune []File

	StateBytes      int64
	StateBytesAfter int64
	ModsBytes       int64
	ModsBytesAfter  int64

	// FreeBytes and FreeBytesAfter are -1 when free space is unknown.
	FreeBytes      int64
	FreeBytesAfter int64

	Warnings []string
}

// Result contains the executed plan and the archives actually removed.
type Result struct {
	Plan
	Pruned []string
}

func (c Config) withDefaults() (Config, error) {
	if c.StateDir == "" && c.ModsDir == "" {
		return Config{}, retentionErrorf("state or mods directory is required")
	}
	if c.Now.IsZero() {
		c.Now = time.Now().UTC()
	} else {
		c.Now = c.Now.UTC()
	}
	if c.StateBudget == 0 {
		c.StateBudget = DefaultStateBudget
	}
	if c.ModsBudget == 0 {
		c.ModsBudget = DefaultModsBudget
	}
	if c.MinFree == 0 {
		c.MinFree = DefaultMinFree
	}
	return c, nil
}

func (c Config) freeSpacePath() string {
	for _, dir := range []string{c.StateDir, c.ModsDir} {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		// The archive directory may not exist yet; checking its parent still
		// answers the question for the filesystem that will hold it.
		parent := filepath.Dir(dir)
		if parent != "" {
			if _, err := os.Stat(parent); err == nil {
				return parent
			}
		}
	}
	if c.StateDir != "" {
		return c.StateDir
	}
	return c.ModsDir
}

type automaticBuckets struct {
	prune       map[string]bool
	lastOfDay   map[string]string
	weeklyReps  map[string]bool
	monthlyReps map[string]bool
}

func (f File) newerThan(other File) bool {
	if !f.Time.Equal(other.Time) {
		return f.Time.After(other.Time)
	}
	if f.Collision != other.Collision {
		return f.Collision > other.Collision
	}
	return f.Name > other.Name
}

func sortNewest(files []File) {
	sortFiles(files, func(a, b File) bool { return a.newerThan(b) })
}

func sortOldest(files []File) {
	sortFiles(files, func(a, b File) bool { return b.newerThan(a) })
}

func sortFiles(files []File, less func(a, b File) bool) {
	sort.Slice(files, func(i, j int) bool { return less(files[i], files[j]) })
}

func retentionErrorf(format string, args ...any) error {
	return fmt.Errorf("retention: "+format, args...)
}

func addBytes(a, b int64) int64 {
	if a < 0 {
		return a
	}
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
