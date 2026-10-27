package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
)

type stateArchive struct {
	path      string
	name      string
	timestamp string
	collision int
}

func checkArchives(root string) Check {
	legacy, legacyChecked, problems := checkLegacyArchives(root)
	if len(problems) > 0 {
		return errorCheck("archives", "%s", joinProblems(problems))
	}

	snapshots, err := restorepoint.ListSnapshots(root)
	if err != nil {
		return errorCheck("archives", "%v", err)
	}
	checkpoints, err := restorepoint.ListCheckpoints(root)
	if err != nil {
		return errorCheck("archives", "%v", err)
	}
	if len(snapshots) > 0 {
		path := filepath.Join(restorepoint.Dir(root, restorepoint.KindSnapshot),
			snapshots[0].Name)
		if err := verifyRestorePoint(path); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(checkpoints) > 0 {
		path := filepath.Join(restorepoint.Dir(root, restorepoint.KindCheckpoint),
			checkpoints[0].Name)
		if err := verifyRestorePoint(path); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return errorCheck("archives", "%s", joinProblems(problems))
	}

	warnings := make([]string, 0)
	missing := make([]string, 0)
	for _, checkpoint := range checkpoints {
		if checkpoint.Base.Available {
			continue
		}
		missing = append(missing, checkpoint.Name)
	}
	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d checkpoints have a missing base: %s",
			len(missing), summarizeNames(missing)))
	}

	if len(snapshots) == 0 && len(checkpoints) == 0 && legacy == 0 {
		return warnCheck("archives", "no restore point archives yet")
	}
	if len(warnings) > 0 {
		return warnCheck("archives", "%s", strings.Join(warnings, "; "))
	}
	parts := make([]string, 0, 2)
	if len(snapshots) > 0 || len(checkpoints) > 0 {
		parts = append(parts, fmt.Sprintf("%d snapshots and %d checkpoints",
			len(snapshots), len(checkpoints)))
	}
	if legacy > 0 {
		parts = append(parts, fmt.Sprintf("validated %d of %d legacy state archives",
			legacyChecked, legacy))
	}
	return okCheck("archives", "validated %s", strings.Join(parts, "; "))
}

func checkLegacyArchives(root string) (count, checked int, problems []string) {
	dir := filepath.Join(root, "backups", "state")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("cannot read %s: %v", dir, err)}
	}

	archives := make([]stateArchive, 0, len(entries))
	for _, entry := range entries {
		parsed, err := archive.ParseArchiveName(entry.Name())
		if err != nil || !parsed.Kind.IsState() {
			continue
		}
		info, err := os.Lstat(filepath.Join(dir, entry.Name()))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		archives = append(archives, stateArchive{
			path:      filepath.Join(dir, entry.Name()),
			name:      entry.Name(),
			timestamp: parsed.Time.UTC().Format("2006-01-02T15:04:05Z"),
			collision: parsed.Collision,
		})
	}
	if len(archives) == 0 {
		return 0, 0, nil
	}
	sort.Slice(archives, func(i, j int) bool {
		if archives[i].timestamp != archives[j].timestamp {
			return archives[i].timestamp > archives[j].timestamp
		}
		if archives[i].collision != archives[j].collision {
			return archives[i].collision > archives[j].collision
		}
		return archives[i].name > archives[j].name
	})

	samples := archiveSamples(len(archives))
	for _, index := range samples {
		item := archives[index]
		reader, err := archive.Open(item.path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", item.name, err))
			continue
		}
		_, validateErr := archive.ValidateState(reader)
		closeErr := reader.Close()
		switch {
		case validateErr != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", item.name, validateErr))
		case closeErr != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", item.name, closeErr))
		}
	}
	return len(archives), len(samples), problems
}

func verifyRestorePoint(path string) error {
	dir, err := os.MkdirTemp("", "tes3mp-easy-doctor-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if _, err := restorepoint.Extract(path, dir); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func summarizeNames(names []string) string {
	const limit = 3
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:limit], ", ") +
		fmt.Sprintf(" and %d more", len(names)-limit)
}

// archiveSamples chooses the newest, oldest and middle archive. The selection
// is deterministic so repeated doctor runs report the same result.
func archiveSamples(count int) []int {
	if count <= 3 {
		indexes := make([]int, count)
		for i := range indexes {
			indexes[i] = i
		}
		return indexes
	}
	return []int{0, count / 2, count - 1}
}
