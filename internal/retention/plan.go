package retention

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

const dayLayout = "2006-01-02"

// Build scans the archive directories and returns the retention plan. It never
// modifies the filesystem. Missing directories are treated as empty.
func Build(cfg Config) (Plan, error) {
	cfg, err := cfg.withDefaults()
	if err != nil {
		return Plan{}, err
	}

	state, err := scanDir(cfg.StateDir, archive.ArchiveKind.IsState)
	if err != nil {
		return Plan{}, err
	}
	mods, err := scanDir(cfg.ModsDir, archive.ArchiveKind.IsMods)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		StateBytes:      state.bytes,
		StateBytesAfter: state.bytes,
		ModsBytes:       mods.bytes,
		ModsBytesAfter:  mods.bytes,
		FreeBytes:       -1,
		FreeBytesAfter:  -1,
		Warnings:        append([]string(nil), state.warnings...),
	}
	plan.Warnings = append(plan.Warnings, mods.warnings...)

	sortNewest(state.files)
	sortNewest(mods.files)

	modsByName := make(map[string]File, len(mods.files))
	for _, file := range mods.files {
		modsByName[file.Name] = file
	}

	currentName, protectAllMods, currentWarnings := currentArchive(cfg)
	plan.Warnings = append(plan.Warnings, currentWarnings...)
	if currentName != "" {
		if _, ok := modsByName[currentName]; !ok {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("current mods archive %s is missing", currentName))
			currentName = ""
		}
	}

	referencedMods, protectReferencedAll, referenceWarnings :=
		stateModsReferences(cfg, state.files, modsByName)
	plan.Warnings = append(plan.Warnings, referenceWarnings...)
	protectAllMods = protectAllMods || protectReferencedAll

	prune := make(map[string]bool)
	automatic := classifyAutomatic(state.files, cfg.Now)
	for name := range automatic.prune {
		prune[name] = true
	}

	for _, file := range excess(filterKind(state.files, archive.KindStateManual), keepManual) {
		prune[file.Name] = true
	}
	for _, file := range excess(filterKind(state.files, archive.KindStatePredeploy), keepPredeploy) {
		prune[file.Name] = true
	}
	for _, file := range excess(filterKind(mods.files, archive.KindMods), keepMods) {
		prune[file.Name] = true
	}
	for _, file := range excess(filterKind(mods.files, archive.KindModsInit), keepModsInit) {
		prune[file.Name] = true
	}

	newestState := ""
	if len(state.files) > 0 {
		newestState = state.files[0].Name
	}
	protected := map[string]bool{newestState: true, currentName: true}
	for name := range referencedMods {
		protected[name] = true
	}
	for _, name := range cfg.ProtectedState {
		protected[name] = true
	}
	for name := range automatic.monthlyReps {
		protected[name] = true
	}
	if protectAllMods {
		for _, file := range mods.files {
			protected[file.Name] = true
		}
	}
	for name := range protected {
		if name != "" {
			delete(prune, name)
		}
	}

	normalPrune := collectPrune(state.files, mods.files, prune)
	stateFreed, modsFreed := freedBytes(normalPrune)
	stateAfter := state.bytes - stateFreed
	modsAfter := mods.bytes - modsFreed

	free, freeWarnings := freeBytes(cfg)
	plan.FreeBytes = free
	plan.FreeBytesAfter = free
	if free >= 0 {
		plan.FreeBytesAfter = addBytes(free, stateFreed+modsFreed)
	}
	plan.Warnings = append(plan.Warnings, freeWarnings...)

	var pressurePrune []File
	if pressureNeeded(cfg, stateAfter, plan.FreeBytesAfter) {
		candidates := pressureCandidates(
			state.files, automatic, prune, protected, newestState, cfg.Now)
		for _, file := range candidates {
			if !pressureNeeded(cfg, stateAfter, plan.FreeBytesAfter) {
				break
			}
			prune[file.Name] = true
			pressurePrune = append(pressurePrune, file)
			stateAfter -= file.Size
			plan.FreeBytesAfter = addBytes(plan.FreeBytesAfter, file.Size)
		}
	}

	plan.Prune = append(normalPrune, pressurePrune...)
	plan.StateBytesAfter = stateAfter
	plan.ModsBytesAfter = modsAfter

	if cfg.StateBudget > 0 && stateAfter > cfg.StateBudget {
		msg := fmt.Sprintf(
			"state budget exceeded after pressure pruning: %d > %d bytes",
			stateAfter, cfg.StateBudget)
		if len(automatic.monthlyReps) > 0 {
			msg += "; monthly archives are always kept"
		}
		plan.Warnings = append(plan.Warnings, msg)
	}
	if cfg.ModsBudget > 0 && modsAfter > cfg.ModsBudget {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"mods budget exceeded: %d > %d bytes; fixed keep counts applied",
			modsAfter, cfg.ModsBudget))
	}
	if cfg.MinFree > 0 && plan.FreeBytesAfter >= 0 &&
		plan.FreeBytesAfter < cfg.MinFree {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"free space below reserve after pressure pruning: %d < %d bytes",
			plan.FreeBytesAfter, cfg.MinFree))
	}

	for _, file := range state.files {
		if !prune[file.Name] {
			plan.Keep = append(plan.Keep, file)
		}
	}
	for _, file := range mods.files {
		if !prune[file.Name] {
			plan.Keep = append(plan.Keep, file)
		}
	}
	return plan, nil
}

type inventory struct {
	files    []File
	bytes    int64
	warnings []string
}

func scanDir(dir string, want func(archive.ArchiveKind) bool) (inventory, error) {
	if dir == "" {
		return inventory{}, nil
	}

	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return inventory{}, nil
	case err != nil:
		return inventory{}, retentionErrorf("read %s: %w", dir, err)
	}

	var inv inventory
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			inv.warnings = append(inv.warnings,
				fmt.Sprintf("stat %s: %v", path, err))
			continue
		}
		if !info.Mode().IsRegular() {
			if info.Mode()&os.ModeSymlink != 0 {
				if _, err := archive.ParseArchiveName(entry.Name()); err == nil {
					inv.warnings = append(inv.warnings,
						fmt.Sprintf("ignore symlink archive %s", path))
				}
			}
			continue
		}
		inv.bytes = addBytes(inv.bytes, info.Size())

		parsed, err := archive.ParseArchiveName(entry.Name())
		if err != nil || !want(parsed.Kind) {
			continue
		}
		inv.files = append(inv.files, File{
			Name:      entry.Name(),
			Path:      path,
			Kind:      parsed.Kind,
			Time:      parsed.Time,
			Collision: parsed.Collision,
			Size:      info.Size(),
		})
	}
	return inv, nil
}

func currentArchive(cfg Config) (string, bool, []string) {
	if cfg.ModsDir == "" {
		return "", false, nil
	}
	if cfg.CurrentMods != "" {
		if err := archive.ValidateModsArchiveName(cfg.CurrentMods); err != nil {
			return "", true, []string{fmt.Sprintf(
				"current mods archive name is invalid: %v", err)}
		}
		return cfg.CurrentMods, false, nil
	}

	current, err := archive.ReadCurrentFile(
		filepath.Join(cfg.ModsDir, "current.txt"))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", false, nil
	case err != nil:
		return "", true, []string{fmt.Sprintf("current.txt: %v", err)}
	default:
		return current.Name, false, nil
	}
}

// stateModsReferences protects every mods package referenced by a live state
// archive. It runs only for full retention (StateDir and ModsDir both set), so
// the 15-minute state-only path never opens state archives. Unreadable state
// metadata protects all mods: the safe direction is to keep too much.
func stateModsReferences(cfg Config, states []File,
	modsByName map[string]File,
) (map[string]bool, bool, []string) {
	refs := make(map[string]bool)
	if cfg.StateDir == "" || cfg.ModsDir == "" || len(states) == 0 {
		return refs, false, nil
	}

	var warnings []string
	protectAll := false
	for _, file := range states {
		current, err := archive.ReadStateCurrent(file.Path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"%s: cannot read current.txt: %v", file.Name, err))
			protectAll = true
			continue
		}
		if _, ok := modsByName[current.Name]; !ok {
			warnings = append(warnings, fmt.Sprintf(
				"%s references missing mods archive %s",
				file.Name, current.Name))
			continue
		}
		refs[current.Name] = true
	}
	return refs, protectAll, warnings
}

func classifyAutomatic(files []File, now time.Time) automaticBuckets {
	out := automaticBuckets{
		prune:       make(map[string]bool),
		lastOfDay:   make(map[string]string),
		weeklyReps:  make(map[string]bool),
		monthlyReps: make(map[string]bool),
	}
	days := make(map[string]bool)
	weeks := make(map[string]bool)
	months := make(map[string]bool)

	for _, file := range files {
		if file.Kind != archive.KindStateAuto {
			continue
		}
		at := file.Time.UTC()
		age := ageAt(file, now)
		day := at.Format(dayLayout)
		if _, ok := out.lastOfDay[day]; !ok {
			out.lastOfDay[day] = file.Name
		}

		switch {
		case age < hotWindow:
			continue
		case age < dailyWindow:
			// The newest file of a UTC day wins even when it is still in the
			// hot window and an older file of the same day is not.
			if days[day] || out.lastOfDay[day] != file.Name {
				out.prune[file.Name] = true
				continue
			}
			days[day] = true
		case age < monthlyWindow:
			year, week := at.ISOWeek()
			key := fmt.Sprintf("%04d-W%02d", year, week)
			if weeks[key] {
				out.prune[file.Name] = true
				continue
			}
			weeks[key] = true
			out.weeklyReps[file.Name] = true
		default:
			month := at.Format("2006-01")
			if months[month] {
				out.prune[file.Name] = true
				continue
			}
			months[month] = true
			out.monthlyReps[file.Name] = true
		}
	}
	return out
}

func filterKind(files []File, kind archive.ArchiveKind) []File {
	filtered := make([]File, 0, len(files))
	for _, file := range files {
		if file.Kind == kind {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func excess(files []File, keep int) []File {
	if keep < 0 {
		keep = 0
	}
	sorted := append([]File(nil), files...)
	sortNewest(sorted)
	if len(sorted) <= keep {
		return nil
	}
	return append([]File(nil), sorted[keep:]...)
}

func collectPrune(state, mods []File, prune map[string]bool) []File {
	var out []File
	for _, file := range state {
		if prune[file.Name] {
			out = append(out, file)
		}
	}
	for _, file := range mods {
		if prune[file.Name] {
			out = append(out, file)
		}
	}
	sortOldest(out)
	return out
}

func freedBytes(files []File) (state, mods int64) {
	for _, file := range files {
		switch {
		case file.Kind.IsState():
			state = addBytes(state, file.Size)
		case file.Kind.IsMods():
			mods = addBytes(mods, file.Size)
		}
	}
	return state, mods
}

func freeBytes(cfg Config) (int64, []string) {
	if cfg.MinFree <= 0 {
		return -1, nil
	}
	free := cfg.FreeSpace
	if free == nil {
		free = DiskFree
	}
	bytes, err := free(cfg.freeSpacePath())
	if err != nil {
		return -1, []string{fmt.Sprintf("free space: %v", err)}
	}
	if bytes < 0 {
		return -1, []string{fmt.Sprintf(
			"free space: invalid negative value %d", bytes)}
	}
	return bytes, nil
}

func pressureNeeded(cfg Config, stateAfter, freeAfter int64) bool {
	if cfg.StateBudget > 0 && stateAfter > cfg.StateBudget {
		return true
	}
	return cfg.MinFree > 0 && freeAfter >= 0 && freeAfter < cfg.MinFree
}

func pressureCandidates(
	files []File,
	automatic automaticBuckets,
	prune map[string]bool,
	protected map[string]bool,
	newestState string,
	now time.Time,
) []File {
	var step1, step2, step3 []File
	for _, file := range files {
		if file.Kind != archive.KindStateAuto ||
			file.Name == newestState ||
			prune[file.Name] ||
			protected[file.Name] ||
			automatic.monthlyReps[file.Name] {
			continue
		}

		age := ageAt(file, now)
		if age <= pressureWindow {
			continue
		}
		lastOfDay := automatic.lastOfDay[file.Time.UTC().Format(dayLayout)] == file.Name
		switch {
		case !lastOfDay:
			step1 = append(step1, file)
		case age >= hotWindow && age < dailyWindow:
			step2 = append(step2, file)
		case age >= dailyWindow && age < monthlyWindow &&
			automatic.weeklyReps[file.Name]:
			step3 = append(step3, file)
		}
	}
	sortOldest(step1)
	sortOldest(step2)
	sortOldest(step3)
	return append(append(step1, step2...), step3...)
}

func ageAt(file File, now time.Time) time.Duration {
	age := now.Sub(file.Time.UTC())
	if age < 0 {
		return 0
	}
	return age
}
