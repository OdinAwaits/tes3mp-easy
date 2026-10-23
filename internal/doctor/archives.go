package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

type stateArchive struct {
	path      string
	name      string
	timestamp string
	collision int
}

func checkArchives(root string) Check {
	dir := filepath.Join(root, "backups", "state")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errorCheck("archives", "cannot read %s: %v", dir, err)
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
		return warnCheck("archives", "no state archives yet")
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
	problems := make([]string, 0, len(samples))
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

	if len(problems) > 0 {
		return errorCheck("archives", "%s", joinProblems(problems))
	}
	return okCheck("archives", "validated %d of %d state archives",
		len(samples), len(archives))
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
