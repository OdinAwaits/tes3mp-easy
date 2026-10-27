package restorepoint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SnapshotItem is one row of `snapshot list`.
type SnapshotItem struct {
	Name        string
	Size        int64
	Mtime       time.Time
	Reason      string
	Label       *string
	ModsSHA256  string
	StateSHA256 string
	UsedBy      int
	Protected   bool
}

// SnapshotBase is the base reference shown by `checkpoint list`.
type SnapshotBase struct {
	Name      string
	SHA256    string
	Available bool
}

// CheckpointItem is one row of `checkpoint list`.
type CheckpointItem struct {
	Name    string
	Size    int64
	Mtime   time.Time
	Label   *string
	Reason  string
	Pinned  bool
	Players int
	Base    SnapshotBase
}

// Dir returns the directory holding kind archives.
func Dir(root string, kind Kind) string {
	if kind == KindSnapshot {
		return filepath.Join(root, "backups", "snapshots")
	}
	return filepath.Join(root, "backups", "checkpoints")
}

// PinPath returns the sidecar path for a checkpoint.
func PinPath(root, name string) string {
	return filepath.Join(Dir(root, KindCheckpoint), name+".pin")
}

// ListSnapshots reads every snapshot manifest and counts checkpoint references.
func ListSnapshots(root string) ([]SnapshotItem, error) {
	dir := Dir(root, KindSnapshot)
	entries, err := readArchiveDir(dir, KindSnapshot)
	if err != nil {
		return nil, err
	}
	items := make([]SnapshotItem, 0, len(entries))
	for _, entry := range entries {
		manifest, err := ReadManifest(filepath.Join(dir, entry.name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
		items = append(items, SnapshotItem{
			Name:        entry.name,
			Size:        entry.size,
			Mtime:       entry.mtime,
			Reason:      manifest.Reason,
			Label:       optionalString(manifest.Label),
			ModsSHA256:  manifest.ModsSHA256,
			StateSHA256: manifest.StateSHA256,
		})
	}

	checkpoints, err := ListCheckpoints(root)
	if err != nil {
		return nil, err
	}
	used := make(map[string]int)
	for _, checkpoint := range checkpoints {
		if checkpoint.Reason == "predeploy" {
			continue
		}
		used[checkpoint.Base.Name]++
	}
	for i := range items {
		items[i].UsedBy = used[items[i].Name]
	}
	if len(items) > 0 {
		items[0].Protected = true
	}
	for i := range items {
		if items[i].UsedBy > 0 {
			items[i].Protected = true
		}
	}
	return items, nil
}

// ListCheckpoints reads every checkpoint manifest, its pin sidecar and whether
// the referenced base snapshot still exists.
func ListCheckpoints(root string) ([]CheckpointItem, error) {
	dir := Dir(root, KindCheckpoint)
	entries, err := readArchiveDir(dir, KindCheckpoint)
	if err != nil {
		return nil, err
	}
	items := make([]CheckpointItem, 0, len(entries))
	for _, entry := range entries {
		manifest, err := ReadManifest(filepath.Join(dir, entry.name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.name, err)
		}
		players := 0
		if manifest.Players != nil {
			players = *manifest.Players
		}
		available, err := fileExists(filepath.Join(Dir(root, KindSnapshot), manifest.Base.Name))
		if err != nil {
			return nil, err
		}
		_, isPinned, err := ReadPin(root, entry.name)
		if err != nil {
			return nil, err
		}
		items = append(items, CheckpointItem{
			Name:    entry.name,
			Size:    entry.size,
			Mtime:   entry.mtime,
			Label:   optionalString(manifest.Label),
			Reason:  manifest.Reason,
			Pinned:  isPinned,
			Players: players,
			Base: SnapshotBase{
				Name:      manifest.Base.Name,
				SHA256:    manifest.Base.SHA256,
				Available: available,
			},
		})
	}
	return items, nil
}

type archiveEntry struct {
	name  string
	size  int64
	mtime time.Time
}

func readArchiveDir(dir string, kind Kind) ([]archiveEntry, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	items := make([]archiveEntry, 0, len(entries))
	for _, entry := range entries {
		parsed, err := ParseName(entry.Name())
		if err != nil {
			continue
		}
		matches := parsed.Kind == kind
		if kind == KindCheckpoint {
			matches = parsed.Kind.IsCheckpoint()
		}
		if !matches || !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		items = append(items, archiveEntry{
			name:  entry.Name(),
			size:  info.Size(),
			mtime: info.ModTime().UTC(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].mtime.Equal(items[j].mtime) {
			return items[i].name > items[j].name
		}
		return items[i].mtime.After(items[j].mtime)
	})
	return items, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}
