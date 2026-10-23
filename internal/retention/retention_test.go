package retention

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

var testNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func testConfig(stateDir, modsDir string) Config {
	return Config{
		StateDir:    stateDir,
		ModsDir:     modsDir,
		Now:         testNow,
		StateBudget: 1 << 40,
		ModsBudget:  1 << 40,
		MinFree:     -1,
	}
}

func archiveName(kind archive.ArchiveKind, at time.Time, collision int) string {
	return archive.ArchiveName{
		Kind: kind, Time: at, Collision: collision,
	}.String()
}

func writeArchive(t *testing.T, dir string, kind archive.ArchiveKind,
	at time.Time, collision int, size int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := archiveName(kind, at, collision)
	data := bytes.Repeat([]byte("x"), size)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func writeNamedFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func planFor(t *testing.T, cfg Config) Plan {
	t.Helper()
	plan, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return plan
}

func names(files []File) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, file := range files {
		out[file.Name] = true
	}
	return out
}

func warningContains(plan Plan, sub string) bool {
	for _, warning := range plan.Warnings {
		if strings.Contains(warning, sub) {
			return true
		}
	}
	return false
}

func TestDiskFree(t *testing.T) {
	free, err := DiskFree(t.TempDir())
	if err != nil {
		t.Fatalf("DiskFree: %v", err)
	}
	if free < 0 {
		t.Fatalf("DiskFree = %d", free)
	}
	if _, err := DiskFree(""); err == nil {
		t.Fatal("DiskFree accepted an empty path")
	}
}

func TestRetentionEmptyAndMissingDirectories(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(
		filepath.Join(root, "missing-state"),
		filepath.Join(root, "missing-mods"),
	)
	plan := planFor(t, cfg)
	if len(plan.Keep) != 0 || len(plan.Prune) != 0 {
		t.Fatalf("empty plan = keep %d prune %d", len(plan.Keep), len(plan.Prune))
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("empty plan warnings = %v", plan.Warnings)
	}
	if plan.StateBytes != 0 || plan.ModsBytes != 0 {
		t.Fatalf("empty plan bytes = state %d mods %d",
			plan.StateBytes, plan.ModsBytes)
	}

	result, err := Apply(cfg)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Pruned) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("empty Apply = %+v", result)
	}
}

func TestRetentionFreeSpaceUsesExistingParent(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(
		filepath.Join(root, "missing-state"),
		filepath.Join(root, "missing-mods"),
	)
	cfg.MinFree = 512 << 20
	checked := ""
	cfg.FreeSpace = func(path string) (int64, error) {
		checked = path
		return 1 << 40, nil
	}
	plan := planFor(t, cfg)
	if checked != root {
		t.Fatalf("free-space path = %q, want %q", checked, root)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("warnings = %v", plan.Warnings)
	}
}

func TestRetentionManualAndPredeployCounts(t *testing.T) {
	stateDir := t.TempDir()
	for i := 0; i < 7; i++ {
		writeArchive(t, stateDir, archive.KindStateManual,
			testNow.Add(-time.Duration(i)*time.Hour), 0, 1)
	}
	for i := 0; i < 4; i++ {
		writeArchive(t, stateDir, archive.KindStatePredeploy,
			testNow.Add(-time.Duration(i+1)*time.Hour), 0, 1)
	}

	plan := planFor(t, testConfig(stateDir, ""))
	if got := len(filterKind(plan.Keep, archive.KindStateManual)); got != 5 {
		t.Fatalf("manual kept = %d, want 5", got)
	}
	if got := len(filterKind(plan.Keep, archive.KindStatePredeploy)); got != 2 {
		t.Fatalf("predeploy kept = %d, want 2", got)
	}
	if got := len(plan.Prune); got != 4 {
		t.Fatalf("pruned = %d, want 4", got)
	}
	// The globally newest state archive is protected.
	newest := archiveName(archive.KindStateManual, testNow, 0)
	if !names(plan.Keep)[newest] {
		t.Fatalf("newest state %s was pruned", newest)
	}
}

func TestRetentionProtectedStateArchiveIsKept(t *testing.T) {
	stateDir := t.TempDir()
	var oldest string
	for i := 0; i < 7; i++ {
		name := writeArchive(t, stateDir, archive.KindStateManual,
			testNow.Add(-time.Duration(i)*time.Hour), 0, 1)
		if i == 6 {
			oldest = name
		}
	}

	cfg := testConfig(stateDir, "")
	cfg.ProtectedState = []string{oldest}
	plan := planFor(t, cfg)
	if !names(plan.Keep)[oldest] {
		t.Fatalf("protected archive %s was pruned", oldest)
	}
	if got := len(filterKind(plan.Keep, archive.KindStateManual)); got != 6 {
		t.Fatalf("manual kept = %d, want 6", got)
	}
}

func TestRetentionCurrentModsArchiveIsProtected(t *testing.T) {
	modsDir := t.TempDir()
	var namesByAge []string
	for i := 1; i <= 7; i++ {
		name := writeArchive(t, modsDir, archive.KindMods,
			testNow.AddDate(0, 0, -i), 0, 1)
		namesByAge = append(namesByAge, name)
	}
	for i := 1; i <= 3; i++ {
		writeArchive(t, modsDir, archive.KindModsInit,
			testNow.AddDate(0, 0, -i), 0, 1)
	}

	current := namesByAge[6] // the oldest non-init archive
	writeNamedFile(t, modsDir, "current.txt",
		[]byte(strings.Repeat("a", 64)+" "+current+"\n"))

	plan := planFor(t, testConfig("", modsDir))
	kept := names(plan.Keep)
	if !kept[current] {
		t.Fatalf("current archive %s was pruned", current)
	}
	if got := len(filterKind(plan.Keep, archive.KindMods)); got != 6 {
		t.Fatalf("mods kept = %d, want 6 (5 newest + current)", got)
	}
	if got := len(filterKind(plan.Keep, archive.KindModsInit)); got != 2 {
		t.Fatalf("mods-init kept = %d, want 2", got)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("warnings = %v", plan.Warnings)
	}

	result, err := Apply(testConfig("", modsDir))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Apply warnings = %v", result.Warnings)
	}
	if _, err := os.Stat(filepath.Join(modsDir, "current.txt")); err != nil {
		t.Fatalf("current.txt was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir, current)); err != nil {
		t.Fatalf("current archive was removed: %v", err)
	}
}

func TestRetentionInvalidCurrentProtectsAllMods(t *testing.T) {
	modsDir := t.TempDir()
	for i := 1; i <= 7; i++ {
		writeArchive(t, modsDir, archive.KindMods,
			testNow.AddDate(0, 0, -i), 0, 1)
	}
	writeNamedFile(t, modsDir, "current.txt", []byte("not canonical\n"))

	plan := planFor(t, testConfig("", modsDir))
	if len(plan.Prune) != 0 {
		t.Fatalf("pruned with invalid current = %v", names(plan.Prune))
	}
	if !warningContains(plan, "current.txt") {
		t.Fatalf("warnings = %v, want current.txt warning", plan.Warnings)
	}
}

func parseDir(t *testing.T, dir string) []File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []File
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		parsed, err := archive.ParseArchiveName(entry.Name())
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, File{
			Name: entry.Name(), Path: filepath.Join(dir, entry.Name()),
			Kind: parsed.Kind, Time: parsed.Time,
			Collision: parsed.Collision, Size: info.Size(),
		})
	}
	return files
}

func TestRetentionFiveYearHistory(t *testing.T) {
	stateDir := t.TempDir()
	start := time.Date(testNow.Year()-5, testNow.Month(), testNow.Day(),
		0, 1, 0, 0, time.UTC)
	for day := start; !day.After(testNow); day = day.AddDate(0, 0, 1) {
		writeArchive(t, stateDir, archive.KindStateAuto, day, 0, 1)
	}
	for at := testNow.Add(-72 * time.Hour); !at.After(testNow); at = at.Add(2 * time.Hour) {
		writeArchive(t, stateDir, archive.KindStateAuto, at, 0, 1)
	}

	plan := planFor(t, testConfig(stateDir, ""))
	kept := names(plan.Keep)
	if len(plan.Prune) == 0 {
		t.Fatal("five-year history produced no prunes")
	}

	days := make(map[string]string)
	weeks := make(map[string]string)
	months := make(map[string]string)
	for _, file := range parseDir(t, stateDir) {
		if file.Kind != archive.KindStateAuto {
			continue
		}
		age := testNow.Sub(file.Time)
		switch {
		case age < 0:
			t.Fatalf("future archive %s", file.Name)
		case age < hotWindow:
			if !kept[file.Name] {
				t.Fatalf("hot archive %s was pruned", file.Name)
			}
		case age < dailyWindow:
			day := file.Time.UTC().Format("2006-01-02")
			if !kept[file.Name] {
				break
			}
			if other := days[day]; other != "" {
				t.Fatalf("two daily representatives for %s: %s and %s",
					day, other, file.Name)
			}
			days[day] = file.Name
		case age < monthlyWindow:
			year, week := file.Time.UTC().ISOWeek()
			key := fmt.Sprintf("%04d-W%02d", year, week)
			if !kept[file.Name] {
				break
			}
			if other := weeks[key]; other != "" {
				t.Fatalf("two weekly representatives for %s: %s and %s",
					key, other, file.Name)
			}
			weeks[key] = file.Name
		default:
			month := file.Time.UTC().Format("2006-01")
			if !kept[file.Name] {
				break
			}
			if other := months[month]; other != "" {
				t.Fatalf("two monthly representatives for %s: %s and %s",
					month, other, file.Name)
			}
			months[month] = file.Name
		}
	}
	if len(days) == 0 || len(weeks) == 0 || len(months) == 0 {
		t.Fatalf("buckets not exercised: days=%d weeks=%d months=%d",
			len(days), len(weeks), len(months))
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("warnings = %v", plan.Warnings)
	}
}

func TestRetentionUsesNamesNotMTime(t *testing.T) {
	stateDir := t.TempDir()
	sameMTime := testNow.AddDate(-2, 0, 0)
	day := testNow.AddDate(0, 0, -10)
	var newest string
	for i := 0; i < 3; i++ {
		newest = writeArchive(t, stateDir, archive.KindStateAuto,
			day.Add(time.Duration(i)*time.Hour), 0, 1)
		if err := os.Chtimes(filepath.Join(stateDir, newest), sameMTime, sameMTime); err != nil {
			t.Fatal(err)
		}
	}
	plan := planFor(t, testConfig(stateDir, ""))
	if got := len(plan.Keep); got != 1 {
		t.Fatalf("kept = %d, want 1 daily representative", got)
	}
	if !names(plan.Keep)[newest] {
		t.Fatalf("kept = %v, want newest timestamp %s", names(plan.Keep), newest)
	}
}

func TestRetentionDailyUsesLastOfDay(t *testing.T) {
	stateDir := t.TempDir()
	day := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	last := writeArchive(t, stateDir, archive.KindStateAuto,
		day.Add(23*time.Hour), 0, 1)
	older := writeArchive(t, stateDir, archive.KindStateAuto, day, 0, 1)

	plan := planFor(t, testConfig(stateDir, ""))
	kept := names(plan.Keep)
	if !kept[last] || kept[older] {
		t.Fatalf("kept = %v, want only the last file of the day %s", kept, last)
	}
}

func TestRetentionCollisionSuffix(t *testing.T) {
	stateDir := t.TempDir()
	hotAt := testNow.Add(-time.Hour)
	hotBase := writeArchive(t, stateDir, archive.KindStateAuto, hotAt, 0, 1)
	hotNext := writeArchive(t, stateDir, archive.KindStateAuto, hotAt, 1, 1)

	oldAt := testNow.AddDate(0, 0, -10)
	oldBase := writeArchive(t, stateDir, archive.KindStateAuto, oldAt, 0, 1)
	oldNext := writeArchive(t, stateDir, archive.KindStateAuto, oldAt, 1, 1)

	plan := planFor(t, testConfig(stateDir, ""))
	kept := names(plan.Keep)
	if !kept[hotBase] || !kept[hotNext] {
		t.Fatalf("hot collisions not kept: %v", kept)
	}
	if !kept[oldNext] || kept[oldBase] {
		t.Fatalf("daily collision kept %v, want %s", kept, oldNext)
	}
}

func monthlyRepresentatives(t *testing.T, files []File) map[string]File {
	t.Helper()
	reps := make(map[string]File)
	sortNewest(files)
	for _, file := range files {
		if file.Kind != archive.KindStateAuto ||
			ageAt(file, testNow) < monthlyWindow {
			continue
		}
		month := file.Time.UTC().Format("2006-01")
		if _, ok := reps[month]; !ok {
			reps[month] = file
		}
	}
	if len(reps) == 0 {
		t.Fatal("test history has no monthly representatives")
	}
	return reps
}

func TestRetentionPressureKeepsMonthly(t *testing.T) {
	stateDir := t.TempDir()
	start := time.Date(testNow.Year()-5, testNow.Month(), testNow.Day(),
		0, 1, 0, 0, time.UTC)
	for day := start; !day.After(testNow); day = day.AddDate(0, 0, 1) {
		writeArchive(t, stateDir, archive.KindStateAuto, day, 0, 1)
	}

	cfg := testConfig(stateDir, "")
	cfg.StateBudget = 1
	plan := planFor(t, cfg)
	if !warningContains(plan, "state budget exceeded") {
		t.Fatalf("warnings = %v, want state budget warning", plan.Warnings)
	}
	if !warningContains(plan, "monthly archives are always kept") {
		t.Fatalf("warnings = %v, want monthly protection explanation", plan.Warnings)
	}

	kept := names(plan.Keep)
	pruned := names(plan.Prune)
	reps := monthlyRepresentatives(t, parseDir(t, stateDir))
	for month, file := range reps {
		if !kept[file.Name] {
			t.Fatalf("monthly representative %s (%s) was pruned", file.Name, month)
		}
		if pruned[file.Name] {
			t.Fatalf("monthly representative %s (%s) is in prune list", file.Name, month)
		}
	}
	if len(plan.Prune) <= len(reps) {
		t.Fatalf("pruned = %d, want pressure beyond %d monthly archives",
			len(plan.Prune), len(reps))
	}
}

func TestRetentionPressureOrder(t *testing.T) {
	stateDir := t.TempDir()
	hotDay := time.Date(testNow.Year(), testNow.Month(), testNow.Day()-2,
		0, 0, 0, 0, time.UTC)
	hotOld := writeArchive(t, stateDir, archive.KindStateAuto,
		hotDay, 0, 1)
	hotLast := writeArchive(t, stateDir, archive.KindStateAuto,
		hotDay.Add(5*time.Minute), 0, 1)
	newest := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.Add(-time.Hour), 0, 1)
	dailyOld := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.AddDate(0, 0, -20), 0, 1)
	dailyNew := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.AddDate(0, 0, -10), 0, 1)
	weeklyOld := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.AddDate(0, 0, -410), 0, 1)
	weeklyNew := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.AddDate(0, 0, -400), 0, 1)
	monthlyOld := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.AddDate(-5, 0, 0), 0, 1)
	monthlyNew := writeArchive(t, stateDir, archive.KindStateAuto,
		testNow.AddDate(-4, 0, 0), 0, 1)

	cfg := testConfig(stateDir, "")
	cfg.StateBudget = 1
	plan := planFor(t, cfg)

	want := []string{hotOld, dailyOld, dailyNew, weeklyOld, weeklyNew}
	got := make([]string, len(plan.Prune))
	for i, file := range plan.Prune {
		got[i] = file.Name
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pressure order = %v, want %v", got, want)
	}
	kept := names(plan.Keep)
	for _, name := range []string{hotLast, newest, monthlyOld, monthlyNew} {
		if !kept[name] {
			t.Fatalf("protected archive %s was pruned", name)
		}
	}
}

func TestRetentionFreeSpacePressure(t *testing.T) {
	stateDir := t.TempDir()
	for i := 2; i <= 6; i++ {
		writeArchive(t, stateDir, archive.KindStateAuto,
			testNow.AddDate(0, 0, -i*10), 0, 1)
	}

	cfg := testConfig(stateDir, "")
	cfg.MinFree = 1 << 40
	cfg.FreeSpace = func(string) (int64, error) { return 0, nil }
	plan := planFor(t, cfg)
	if plan.FreeBytes != 0 {
		t.Fatalf("FreeBytes = %d, want 0", plan.FreeBytes)
	}
	if got := len(plan.Prune); got != 4 {
		t.Fatalf("pruned = %d, want 4 (newest state protected)", got)
	}
	if !warningContains(plan, "free space below reserve") {
		t.Fatalf("warnings = %v, want free-space warning", plan.Warnings)
	}
}

func TestRetentionModsBudgetWarning(t *testing.T) {
	modsDir := t.TempDir()
	for i := 1; i <= 7; i++ {
		writeArchive(t, modsDir, archive.KindMods,
			testNow.AddDate(0, 0, -i), 0, 1)
	}
	for i := 1; i <= 3; i++ {
		writeArchive(t, modsDir, archive.KindModsInit,
			testNow.AddDate(0, 0, -i), 0, 1)
	}

	cfg := testConfig("", modsDir)
	cfg.ModsBudget = 1
	plan := planFor(t, cfg)
	if !warningContains(plan, "mods budget exceeded") {
		t.Fatalf("warnings = %v, want mods budget warning", plan.Warnings)
	}
	if got := len(filterKind(plan.Keep, archive.KindMods)); got != 5 {
		t.Fatalf("mods kept = %d, want 5", got)
	}
	if got := len(filterKind(plan.Keep, archive.KindModsInit)); got != 2 {
		t.Fatalf("mods-init kept = %d, want 2", got)
	}
}

func TestApplyRemovesPlannedArchives(t *testing.T) {
	stateDir := t.TempDir()
	var all []string
	for i := 0; i < 7; i++ {
		all = append(all, writeArchive(t, stateDir, archive.KindStateManual,
			testNow.Add(-time.Duration(i)*time.Hour), 0, 1))
	}

	result, err := Apply(testConfig(stateDir, ""))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Pruned) != 2 {
		t.Fatalf("Pruned = %v, want 2", result.Pruned)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %v", result.Warnings)
	}
	for _, name := range result.Pruned {
		if _, err := os.Stat(filepath.Join(stateDir, name)); !os.IsNotExist(err) {
			t.Fatalf("pruned archive %s still exists (err=%v)", name, err)
		}
	}
	for _, name := range all {
		if names(result.Keep)[name] {
			if _, err := os.Stat(filepath.Join(stateDir, name)); err != nil {
				t.Fatalf("kept archive %s: %v", name, err)
			}
		}
	}
}
