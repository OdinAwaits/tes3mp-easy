package restorepoint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

const (
	// DefaultSnapshotBudget is the backups/snapshots budget (1.5 GiB).
	DefaultSnapshotBudget int64 = 3 << 29
	// DefaultCheckpointBudget is the backups/checkpoints budget (256 MiB).
	DefaultCheckpointBudget int64 = 256 << 20

	retentionHot     = 7 * 24 * time.Hour
	retentionDaily   = 365 * 24 * time.Hour
	retentionMonthly = 3 * 365 * 24 * time.Hour
	keepManual       = 5
	keepAutomatic    = 2
	keepPredeploy    = 2
)

// RetentionOptions controls one retention run.
type RetentionOptions struct {
	Root string
	Now  time.Time

	// Zero selects the documented default. A negative value disables the
	// corresponding budget check.
	SnapshotBudget   int64
	CheckpointBudget int64
}

// RetentionResult is the list of archives removed by retention and any
// non-fatal problems encountered while planning or deleting.
type RetentionResult struct {
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

type retentionArchive struct {
	name      string
	path      string
	size      int64
	kind      Kind
	at        time.Time
	collision int
	manifest  *Manifest
	pinned    bool
	class     retentionClass
}

type retentionClass uint8

const (
	classHot retentionClass = iota
	classDaily
	classWeekly
	classMonthly
	classPredeploy
)

func (a retentionArchive) newerThan(b retentionArchive) bool {
	if !a.at.Equal(b.at) {
		return a.at.After(b.at)
	}
	if a.collision != b.collision {
		return a.collision > b.collision
	}
	return a.name > b.name
}

// ApplyRetention implements the restore point policy from
// docs/v2/03-contracts.md §6.9. It never returns an error for an individual
// unreadable archive: such archives are kept and reported in Warnings.
func ApplyRetention(opts RetentionOptions) (RetentionResult, error) {
	result := RetentionResult{Pruned: []string{}, Warnings: []string{}}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
	if opts.SnapshotBudget == 0 {
		opts.SnapshotBudget = DefaultSnapshotBudget
	}
	if opts.CheckpointBudget == 0 {
		opts.CheckpointBudget = DefaultCheckpointBudget
	}

	snapshots, err := loadRetentionArchives(opts.Root, KindSnapshot, &result.Warnings)
	if err != nil {
		return result, err
	}
	checkpoints, err := loadRetentionArchives(opts.Root, KindCheckpoint, &result.Warnings)
	if err != nil {
		return result, err
	}
	if len(snapshots) == 0 && len(checkpoints) == 0 {
		return result, nil
	}

	keepCheckpoint := planCheckpoints(checkpoints, opts.Now)
	keepSnapshot := planSnapshots(snapshots, checkpoints, keepCheckpoint)

	checkpointBytes := sumKept(checkpoints, keepCheckpoint)
	checkpointBytes = pressureCheckpoints(checkpoints, keepCheckpoint,
		checkpointBytes, opts.CheckpointBudget)

	keepSnapshot = planSnapshots(snapshots, checkpoints, keepCheckpoint)
	snapshotBytes := sumKept(snapshots, keepSnapshot)
	snapshotBytes, snapshotDrops := pressureSnapshots(snapshots, keepSnapshot,
		snapshotBytes, opts.SnapshotBudget)
	for _, dropped := range snapshotDrops {
		for _, checkpoint := range checkpoints {
			if !keepCheckpoint[checkpoint.name] || checkpoint.manifest == nil ||
				checkpoint.manifest.Base == nil {
				continue
			}
			if checkpoint.manifest.Base.Name == dropped {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"%s: base %s was pruned; checkpoint has available=false",
					checkpoint.name, dropped))
			}
		}
	}

	if opts.CheckpointBudget >= 0 && checkpointBytes > opts.CheckpointBudget {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"backups/checkpoints uses %d bytes, over the %d byte budget",
			checkpointBytes, opts.CheckpointBudget))
	}
	if opts.SnapshotBudget >= 0 && snapshotBytes > opts.SnapshotBudget {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"backups/snapshots uses %d bytes, over the %d byte budget",
			snapshotBytes, opts.SnapshotBudget))
	}

	prune := make([]retentionArchive, 0)
	for _, checkpoint := range checkpoints {
		if !keepCheckpoint[checkpoint.name] {
			prune = append(prune, checkpoint)
		}
	}
	for _, snapshot := range snapshots {
		if !keepSnapshot[snapshot.name] {
			prune = append(prune, snapshot)
		}
	}
	sort.Slice(prune, func(i, j int) bool {
		if prune[i].at.Equal(prune[j].at) {
			return prune[i].name < prune[j].name
		}
		return prune[i].at.Before(prune[j].at)
	})

	dirs := make(map[string]bool)
	for _, archive := range prune {
		if err := os.Remove(archive.path); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("remove %s: %v", archive.name, err))
			continue
		}
		result.Pruned = append(result.Pruned, archive.name)
		dirs[filepath.Dir(archive.path)] = true
	}
	dirList := make([]string, 0, len(dirs))
	for dir := range dirs {
		dirList = append(dirList, dir)
	}
	sort.Strings(dirList)
	for _, dir := range dirList {
		if err := atomicfile.SyncDir(dir); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("sync %s: %v", dir, err))
		}
	}
	sort.Strings(result.Pruned)
	return result, nil
}

func loadRetentionArchives(root string, kind Kind,
	warnings *[]string,
) ([]retentionArchive, error) {
	dir := Dir(root, kind)
	entries, err := readArchiveDir(dir, kind)
	if err != nil {
		return nil, err
	}
	archives := make([]retentionArchive, 0, len(entries))
	for _, entry := range entries {
		parsed, err := ParseName(entry.name)
		if err != nil {
			continue
		}
		archive := retentionArchive{
			name:      entry.name,
			path:      filepath.Join(dir, entry.name),
			size:      entry.size,
			kind:      parsed.Kind,
			at:        parsed.Time,
			collision: parsed.Collision,
		}
		manifest, err := ReadManifest(archive.path)
		if err != nil {
			*warnings = append(*warnings,
				fmt.Sprintf("read manifest %s: %v; keeping it", entry.name, err))
		} else {
			archive.manifest = &manifest
		}
		if kind == KindCheckpoint {
			archive.pinned = pinned(root, entry.name)
		}
		archives = append(archives, archive)
	}
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].newerThan(archives[j])
	})
	return archives, nil
}

func retentionClassFor(at, now time.Time, archiveKind Kind) retentionClass {
	if archiveKind == KindPredeploy {
		return classPredeploy
	}
	age := now.Sub(at)
	if age < 0 {
		age = 0
	}
	switch {
	case age <= retentionHot:
		return classHot
	case age <= retentionDaily:
		return classDaily
	case age <= retentionMonthly:
		return classWeekly
	default:
		return classMonthly
	}
}

func planCheckpoints(checkpoints []retentionArchive,
	now time.Time,
) map[string]bool {
	keep := make(map[string]bool, len(checkpoints))
	predeploy := make([]retentionArchive, 0)
	ordinary := make([]retentionArchive, 0, len(checkpoints))
	for i := range checkpoints {
		checkpoints[i].class = retentionClassFor(
			checkpoints[i].at, now, checkpoints[i].kind)
		if checkpoints[i].kind == KindPredeploy {
			predeploy = append(predeploy, checkpoints[i])
			continue
		}
		ordinary = append(ordinary, checkpoints[i])
	}
	sort.Slice(predeploy, func(i, j int) bool {
		return predeploy[i].newerThan(predeploy[j])
	})
	for i, checkpoint := range predeploy {
		if i < keepPredeploy || checkpoint.pinned {
			keep[checkpoint.name] = true
		}
	}

	seenDay := map[string]bool{}
	seenWeek := map[string]bool{}
	seenMonth := map[string]bool{}
	for _, checkpoint := range ordinary {
		if checkpoint.pinned || checkpoint.manifest == nil {
			keep[checkpoint.name] = true
			continue
		}
		switch checkpoint.class {
		case classHot:
			keep[checkpoint.name] = true
		case classDaily:
			key := checkpoint.at.Format("2006-01-02")
			if !seenDay[key] {
				seenDay[key] = true
				keep[checkpoint.name] = true
			}
		case classWeekly:
			year, week := checkpoint.at.ISOWeek()
			key := fmt.Sprintf("%04d-W%02d", year, week)
			if !seenWeek[key] {
				seenWeek[key] = true
				keep[checkpoint.name] = true
			}
		case classMonthly:
			key := checkpoint.at.Format("2006-01")
			if !seenMonth[key] {
				seenMonth[key] = true
				keep[checkpoint.name] = true
			}
		}
	}
	if newest := newestArchive(checkpoints); newest != nil {
		keep[newest.name] = true
	}
	return keep
}

func planSnapshots(snapshots, checkpoints []retentionArchive,
	keepCheckpoint map[string]bool,
) map[string]bool {
	keep := make(map[string]bool, len(snapshots))
	if len(snapshots) == 0 {
		return keep
	}
	keep[snapshots[0].name] = true
	for _, checkpoint := range checkpoints {
		if !keepCheckpoint[checkpoint.name] || checkpoint.manifest == nil ||
			checkpoint.manifest.Base == nil || checkpoint.manifest.Base.Name == "" {
			continue
		}
		keep[checkpoint.manifest.Base.Name] = true
	}

	manual := 0
	automatic := 0
	for _, snapshot := range snapshots {
		if snapshot.manifest == nil {
			keep[snapshot.name] = true
			continue
		}
		if snapshot.manifest.Reason == "manual" {
			if manual < keepManual {
				keep[snapshot.name] = true
			}
			manual++
			continue
		}
		if automatic < keepAutomatic {
			keep[snapshot.name] = true
		}
		automatic++
	}
	return keep
}

func sumKept(archives []retentionArchive, keep map[string]bool) int64 {
	var total int64
	for _, archive := range archives {
		if keep[archive.name] {
			total += archive.size
		}
	}
	return total
}

func pressureCheckpoints(checkpoints []retentionArchive, keep map[string]bool,
	bytes, budget int64,
) int64 {
	if budget < 0 || bytes <= budget {
		return bytes
	}
	newest := newestArchive(checkpoints)
	daily := make([]retentionArchive, 0)
	weekly := make([]retentionArchive, 0)
	for _, checkpoint := range checkpoints {
		if !keep[checkpoint.name] || checkpoint.manifest == nil ||
			checkpoint.pinned || checkpoint.kind == KindPredeploy ||
			(newest != nil && checkpoint.name == newest.name) {
			continue
		}
		switch checkpoint.class {
		case classDaily:
			daily = append(daily, checkpoint)
		case classWeekly:
			weekly = append(weekly, checkpoint)
		}
	}
	candidates := append(daily, weekly...)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[j].newerThan(candidates[i])
	})
	for _, checkpoint := range candidates {
		if bytes <= budget {
			break
		}
		delete(keep, checkpoint.name)
		bytes -= checkpoint.size
	}
	return bytes
}

func pressureSnapshots(snapshots []retentionArchive, keep map[string]bool,
	bytes, budget int64,
) (int64, []string) {
	if budget < 0 || bytes <= budget || len(snapshots) == 0 {
		return bytes, nil
	}
	newest := snapshots[0].name
	candidates := make([]retentionArchive, 0)
	for _, snapshot := range snapshots {
		if !keep[snapshot.name] || snapshot.name == newest ||
			snapshot.manifest == nil || snapshot.manifest.Reason != "base" {
			continue
		}
		candidates = append(candidates, snapshot)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[j].newerThan(candidates[i])
	})
	dropped := make([]string, 0)
	for _, snapshot := range candidates {
		if bytes <= budget {
			break
		}
		delete(keep, snapshot.name)
		bytes -= snapshot.size
		dropped = append(dropped, snapshot.name)
	}
	return bytes, dropped
}

func newestArchive(archives []retentionArchive) *retentionArchive {
	var newest *retentionArchive
	for i := range archives {
		if newest == nil || archives[i].newerThan(*newest) {
			newest = &archives[i]
		}
	}
	return newest
}
