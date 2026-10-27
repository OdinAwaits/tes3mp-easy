package restorepoint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// ErrInUse marks a protected restore point that cannot be removed without
// --force. The ops layer maps it to archive_in_use (exit 12).
var ErrInUse = errors.New("restore point is in use")

// RemoveOptions describes one `snapshot rm` or `checkpoint rm` call.
type RemoveOptions struct {
	Root   string
	Kind   Kind
	Names  []string
	Force  bool
	DryRun bool
}

// RemoveSkipped is one protected archive that was not removed.
type RemoveSkipped struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// RemoveResult is the data payload of `snapshot rm` and `checkpoint rm`.
type RemoveResult struct {
	Removed  []string        `json:"removed"`
	Skipped  []RemoveSkipped `json:"skipped"`
	Freed    int64           `json:"freed"`
	Warnings []string        `json:"warnings"`
}

type removeCandidate struct {
	name string
	path string
	size int64
}

// Remove deletes the named restore points. Every name is validated and checked
// before the first deletion; without Force any protected name rejects the
// whole batch.
func Remove(opts RemoveOptions) (RemoveResult, error) {
	if opts.Kind != KindSnapshot && opts.Kind != KindCheckpoint {
		return RemoveResult{}, invalidf("remove: unsupported kind %s", opts.Kind)
	}
	if len(opts.Names) == 0 {
		return RemoveResult{}, invalidf("remove: at least one archive name is required")
	}
	result := RemoveResult{
		Removed:  []string{},
		Skipped:  []RemoveSkipped{},
		Warnings: []string{},
	}
	dir := Dir(opts.Root, opts.Kind)

	names := uniqueNames(opts.Names)
	infos := make(map[string]int64, len(names))
	for _, name := range names {
		if err := ValidateName(opts.Kind, name); err != nil {
			return RemoveResult{}, err
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return RemoveResult{}, fmt.Errorf("%w: %s does not exist", fs.ErrNotExist, name)
		case err != nil:
			return RemoveResult{}, err
		case !info.Mode().IsRegular():
			return RemoveResult{}, invalidf("%s is not a regular archive", name)
		}
		infos[name] = info.Size()
	}

	latest := ""
	references := map[string][]string{}
	pinnedNames := map[string]bool{}
	switch opts.Kind {
	case KindSnapshot:
		latest = latestSnapshotName(opts.Root)
		references = snapshotReferences(opts.Root, &result)
	case KindCheckpoint:
		for _, name := range names {
			pinnedNames[name] = pinned(opts.Root, name)
			if _, _, err := ReadPin(opts.Root, name); err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("pin %s: %v", name, err))
			}
		}
	}

	blocked := make([]string, 0)
	planned := make([]removeCandidate, 0, len(names))
	for _, name := range names {
		switch {
		case opts.Kind == KindSnapshot && name == latest:
			result.Skipped = append(result.Skipped,
				RemoveSkipped{Name: name, Reason: "last_snapshot"})
		case opts.Kind == KindCheckpoint && pinnedNames[name] && !opts.Force:
			blocked = append(blocked, name+" (pinned)")
		case opts.Kind == KindSnapshot && len(references[name]) > 0 && !opts.Force:
			blocked = append(blocked, name+" (base)")
		default:
			planned = append(planned, removeCandidate{
				name: name, path: filepath.Join(dir, name), size: infos[name],
			})
		}
	}
	if len(blocked) > 0 {
		return RemoveResult{}, fmt.Errorf("%w: %s", ErrInUse,
			strings.Join(blocked, ", "))
	}
	if len(planned) == 0 {
		return RemoveResult{}, fmt.Errorf("%w: nothing to remove", ErrInUse)
	}

	for _, item := range planned {
		if deps := references[item.name]; len(deps) > 0 {
			sort.Strings(deps)
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"%s is the base of %s; those checkpoints will have available=false",
				item.name, strings.Join(deps, ", ")))
		}
		result.Removed = append(result.Removed, item.name)
		result.Freed += item.size
	}
	if opts.DryRun {
		return result, nil
	}

	removed := result.Removed[:0]
	result.Freed = 0
	for _, item := range planned {
		if err := os.Remove(item.path); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("remove %s: %v", item.name, err))
			continue
		}
		removed = append(removed, item.name)
		result.Freed += item.size
		if opts.Kind != KindCheckpoint {
			continue
		}
		if err := os.Remove(PinPath(opts.Root, item.name)); err != nil &&
			!errors.Is(err, fs.ErrNotExist) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("remove %s.pin: %v", item.name, err))
		}
	}
	result.Removed = removed
	if len(result.Removed) > 0 {
		if err := atomicfile.SyncDir(dir); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("sync %s: %v", dir, err))
		}
	}
	return result, nil
}

func uniqueNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	unique := make([]string, 0, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		unique = append(unique, name)
	}
	return unique
}

func latestSnapshotName(root string) string {
	entries, err := readArchiveDir(Dir(root, KindSnapshot), KindSnapshot)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return entries[0].name
}

func snapshotReferences(root string, result *RemoveResult) map[string][]string {
	references := map[string][]string{}
	entries, err := readArchiveDir(Dir(root, KindCheckpoint), KindCheckpoint)
	if err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("list checkpoints: %v", err))
		return references
	}
	for _, entry := range entries {
		manifest, err := ReadManifest(filepath.Join(Dir(root, KindCheckpoint), entry.name))
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("checkpoint %s: %v", entry.name, err))
			continue
		}
		if manifest.Base == nil || manifest.Base.Name == "" {
			continue
		}
		references[manifest.Base.Name] = append(
			references[manifest.Base.Name], entry.name)
	}
	return references
}
