package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/retention"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

const (
	backupTypeMods  = "mods"
	backupTypeState = "state"

	defaultBackupsLimit = 200
	maxBackupsLimit     = 500

	backupsCurrentFile = "current.txt"

	backupWarnFree int64 = 1 << 30
)

type backupsData struct {
	Type       string        `json:"type"`
	Current    *string       `json:"current"`
	Items      []backupsItem `json:"items"`
	NextBefore *string       `json:"next_before"`
}

type backupsItem struct {
	Name          string  `json:"name"`
	Size          int64   `json:"size"`
	MTime         string  `json:"mtime"`
	Current       bool    `json:"current"`
	SHA256        *string `json:"sha256,omitempty"`
	ModsArchive   *string `json:"mods_archive,omitempty"`
	ModsAvailable *bool   `json:"mods_available,omitempty"`
}

type backupsPruneData struct {
	Removed  []string `json:"removed"`
	Warnings []string `json:"warnings"`
}

type backupsListOptions struct {
	root       string
	typ        string
	limit      int
	before     string
	beforeTime time.Time
	sha256     bool
}

type backupEntry struct {
	name  string
	size  int64
	mtime time.Time
}

type backupsCreateOptions struct {
	typ       string
	manual    bool
	ifRunning bool
}

type backupsCreateData struct {
	Archive  string   `json:"archive,omitempty"`
	Size     *int64   `json:"size,omitempty"`
	SHA256   *string  `json:"sha256,omitempty"`
	Skipped  bool     `json:"skipped"`
	Reason   string   `json:"reason,omitempty"`
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

type stateBackupOptions struct {
	root    string
	kind    archive.ArchiveKind
	now     time.Time
	dryRun  bool
	protect []string
}

type stateBackupResult struct {
	archive  string
	size     int64
	sha256   string
	pruned   []string
	warnings []string
}

func runBackups(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"backups: subcommand is required; want list, create or prune"))
	}

	switch args[0] {
	case "list":
		return runBackupsList(opts, args[1:], stdout, stderr)
	case "create":
		return runBackupsCreate(opts, args[1:], client, stdout, stderr)
	case "prune":
		return runBackupsPrune(opts, args[1:], stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("backups: unknown subcommand %q", args[0])))
	}
}

func runBackupsPrune(opts options, args []string,
	stdout, stderr io.Writer,
) int {
	if len(args) != 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("backups prune: unexpected argument %q", args[0])))
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "backups prune", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	cfg := retention.Config{
		StateDir: filepath.Join(opts.root, "backups", "state"),
		ModsDir:  filepath.Join(opts.root, "backups", "mods"),
		Now:      time.Now().UTC(),
	}
	data := backupsPruneData{Removed: []string{}, Warnings: []string{}}
	if opts.dryRun {
		plan, err := retention.Build(cfg)
		if err != nil {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitInternal,
					fmt.Sprintf("backups prune: %v", err)))
		}
		for _, file := range plan.Prune {
			data.Removed = append(data.Removed, file.Name)
		}
		data.Warnings = append(data.Warnings, plan.Warnings...)
	} else {
		result, err := retention.Apply(cfg)
		if err != nil {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitInternal,
					fmt.Sprintf("backups prune: %v", err)))
		}
		data.Removed = append(data.Removed, result.Pruned...)
		data.Warnings = append(data.Warnings, result.Warnings...)
	}

	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	for _, name := range data.Removed {
		fmt.Fprintf(stdout, "removed: %s\n", name)
	}
	for _, warning := range data.Warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: backups prune: %s\n", warning)
	}
	if len(data.Removed) == 0 {
		fmt.Fprintln(stdout, "removed: -")
	}
	return int(ops.ExitOK)
}

func runBackupsCreate(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	createOpts, parseErr := parseBackupsCreateArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "backups create", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	if createOpts.ifRunning {
		if client == nil {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitInternal,
					"backups create: systemd client is not configured"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), opts.commandTimeout)
		active, err := client.IsActive(ctx, systemd.UnitTES3MPService)
		cancel()
		if err != nil {
			return writeSystemdFailure(stdout, stderr, opts.json,
				"backups create", err)
		}
		if !active {
			data := backupsCreateData{
				Skipped:  true,
				Reason:   "not_running",
				Pruned:   []string{},
				Warnings: []string{},
			}
			return writeBackupsCreate(opts, data, stdout, stderr)
		}
	}

	kind := archive.KindStateAuto
	if createOpts.manual {
		kind = archive.KindStateManual
	}
	result, createErr := createStateBackupLocked(stateBackupOptions{
		root:   opts.root,
		kind:   kind,
		now:    time.Now().UTC(),
		dryRun: opts.dryRun,
	})
	if createErr != nil {
		return writeError(stdout, stderr, opts.json, createErr)
	}

	size := result.size
	sha := result.sha256
	data := backupsCreateData{
		Archive:  result.archive,
		Size:     &size,
		SHA256:   &sha,
		Skipped:  false,
		Pruned:   result.pruned,
		Warnings: result.warnings,
	}
	return writeBackupsCreate(opts, data, stdout, stderr)
}

func parseBackupsCreateArgs(args []string) (backupsCreateOptions, *ops.Error) {
	fs := flag.NewFlagSet("backups create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	typ := fs.String("type", "", "")
	manual := fs.Bool("manual", false, "")
	ifRunning := fs.Bool("if-running", false, "")

	if err := fs.Parse(args); err != nil {
		return backupsCreateOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups create: %v", err))
	}
	if fs.NArg() != 0 {
		return backupsCreateOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups create: unexpected argument %q", fs.Arg(0)))
	}
	switch *typ {
	case backupTypeState:
	case "":
		return backupsCreateOptions{}, ops.NewError(ops.ExitInvalidArgument,
			"backups create: --type is required (state)")
	default:
		return backupsCreateOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups create: unsupported type %q; want state", *typ))
	}

	return backupsCreateOptions{
		typ:       *typ,
		manual:    *manual,
		ifRunning: *ifRunning,
	}, nil
}

// acquireOpsLock validates root and takes the shared ops lock. command is used
// in error messages so callers do not misattribute a lock failure. A dry run
// with no lock file does not create one: no writer can hold an advisory lock on
// a missing file, so there is nothing to wait for.
func acquireOpsLock(root, command string, mode lock.Mode, timeout time.Duration,
	readOnly bool,
) (*lock.Lock, *ops.Error) {
	info, err := os.Stat(root)
	switch {
	case err != nil:
		return nil, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: root %s: %v", command, root, err))
	case !info.IsDir():
		return nil, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: root %s is not a directory", command, root))
	}

	dir := filepath.Join(root, ".locks")
	path := filepath.Join(dir, "ops.lock")
	if readOnly {
		lockInfo, statErr := os.Lstat(path)
		switch {
		case errors.Is(statErr, fs.ErrNotExist):
			return nil, nil
		case statErr != nil:
			return nil, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: stat %s: %v", command, path, statErr))
		case lockInfo.Mode()&os.ModeSymlink != 0 || !lockInfo.Mode().IsRegular():
			return nil, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: %s is not a regular file", command, path))
		}
	} else {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: create %s: %v", command, dir, err))
		}
		if lockInfo, statErr := os.Lstat(path); statErr == nil &&
			(lockInfo.Mode()&os.ModeSymlink != 0 || !lockInfo.Mode().IsRegular()) {
			return nil, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: %s is not a regular file", command, path))
		} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return nil, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: stat %s: %v", command, path, statErr))
		}
	}

	held, err := lock.Acquire(path, mode, timeout)
	if err != nil {
		if errors.Is(err, lock.ErrLocked) {
			return nil, ops.NewError(ops.ExitLocked,
				fmt.Sprintf("%s: %v", command, err))
		}
		return nil, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: acquire lock: %v", command, err))
	}
	return held, nil
}

func validateStateBackupSource(src archive.StateSource, modsDir string) error {
	if _, err := os.Lstat(src.RequiredDataFiles); err != nil {
		return fmt.Errorf("%w: requiredDataFiles.json: %v", archive.ErrInvalid, err)
	}
	current, err := archive.ReadCurrentFile(src.CurrentFile)
	if err != nil {
		return fmt.Errorf("%w: %v", archive.ErrInvalid, err)
	}

	target := filepath.Join(modsDir, current.Name)
	info, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("%w: current.txt references %s: %v",
			archive.ErrInvalid, current.Name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: current.txt references %s: not a regular file",
			archive.ErrInvalid, current.Name)
	}
	return nil
}

func backupCreateError(operation string, err error) *ops.Error {
	exit := ops.ExitInternal
	switch {
	case errors.Is(err, syscall.ENOSPC):
		exit = ops.ExitNoSpace
	case errors.Is(err, fs.ErrNotExist):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, archive.ErrInvalid):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, pathguard.ErrUnsafePath):
		exit = ops.ExitUnsafePath
	}
	return ops.NewError(exit, fmt.Sprintf("backups create: %s: %v", operation, err))
}

func checkBackupSpace(stateDir string) ([]string, *ops.Error) {
	free, err := backupDiskFree(stateDir)
	if err != nil {
		return nil, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("backups create: free space: %v", err))
	}
	switch {
	case free < retention.DefaultMinFree:
		return nil, ops.NewError(ops.ExitNoSpace,
			fmt.Sprintf("backups create: free space %d is below %d bytes",
				free, retention.DefaultMinFree))
	case free < backupWarnFree:
		return []string{fmt.Sprintf(
			"free space %d is below %d bytes", free, backupWarnFree)}, nil
	default:
		return nil, nil
	}
}

// backupDiskFreeFunc is replaced in unit tests to exercise the disk-reserve
// failure modes without a small filesystem.
var backupDiskFreeFunc = retention.DiskFree

func backupDiskFree(path string) (int64, error) {
	for _, candidate := range []string{path, filepath.Dir(path)} {
		if _, err := os.Stat(candidate); err == nil {
			return backupDiskFreeFunc(candidate)
		}
	}
	return 0, fmt.Errorf("no existing directory for %s", path)
}

type countingWriter struct {
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

func writeBackupsCreate(opts options, data backupsCreateData,
	stdout, stderr io.Writer,
) int {
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeBackupsCreateHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func writeBackupsCreateHuman(w io.Writer, data backupsCreateData) error {
	pruned := "-"
	if len(data.Pruned) > 0 {
		pruned = strings.Join(data.Pruned, ", ")
	}
	warnings := "-"
	if len(data.Warnings) > 0 {
		warnings = strings.Join(data.Warnings, "; ")
	}

	var out strings.Builder
	if data.Skipped {
		fmt.Fprintf(&out, "skipped: true\nreason: %s\n", data.Reason)
	} else {
		size := int64(0)
		if data.Size != nil {
			size = *data.Size
		}
		sha := ""
		if data.SHA256 != nil {
			sha = *data.SHA256
		}
		fmt.Fprintf(&out, "archive: %s\nsize: %d\nsha256: %s\n",
			data.Archive, size, sha)
	}
	fmt.Fprintf(&out, "pruned: %s\nwarnings: %s\n", pruned, warnings)
	_, err := io.WriteString(w, out.String())
	return err
}

func applyFullRetentionLocked(root string, now time.Time,
	protectedState []string,
) ([]string, []string) {
	cfg := retention.Config{
		StateDir:       filepath.Join(root, "backups", "state"),
		ModsDir:        filepath.Join(root, "backups", "mods"),
		ProtectedState: protectedState,
		Now:            now,
	}
	result, err := retention.Apply(cfg)
	if err != nil {
		return nil, []string{fmt.Sprintf("retention: %v", err)}
	}
	return result.Pruned, result.Warnings
}

func createStateBackupLocked(opts stateBackupOptions) (stateBackupResult, *ops.Error) {
	if opts.root == "" {
		return stateBackupResult{}, ops.NewError(ops.ExitInternal,
			"backups create: root is empty")
	}
	if !opts.kind.IsState() {
		return stateBackupResult{}, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("backups create: %s is not a state archive kind", opts.kind))
	}
	now := opts.now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	stateDir := filepath.Join(opts.root, "backups", "state")
	modsDir := filepath.Join(opts.root, "backups", "mods")
	if !opts.dryRun {
		if err := os.MkdirAll(stateDir, 0o750); err != nil {
			return stateBackupResult{}, backupCreateError(
				fmt.Sprintf("create %s", stateDir), err)
		}
	}

	var pressurePruned []string
	var pressureWarnings []string
	if !opts.dryRun {
		if free, freeErr := backupDiskFree(stateDir); freeErr == nil &&
			free < backupWarnFree {
			pressurePruned, pressureWarnings = applyFullRetentionLocked(
				opts.root, now, opts.protect)
		}
	}

	warnings, spaceErr := checkBackupSpace(stateDir)
	if spaceErr != nil {
		return stateBackupResult{}, spaceErr
	}
	warnings = append(pressureWarnings, warnings...)

	name, err := archive.NextArchiveNameInDir(stateDir, opts.kind, now)
	if err != nil {
		return stateBackupResult{}, backupCreateError("choose archive name", err)
	}
	source := archive.StateSource{
		PlayersDir: filepath.Join(opts.root, "players"),
		WorldDir:   filepath.Join(opts.root, "world"),
		ConfigsDir: filepath.Join(opts.root, "configs"),
		RequiredDataFiles: filepath.Join(opts.root, "mods", "plugins",
			"requiredDataFiles.json"),
		CurrentFile: filepath.Join(modsDir, backupsCurrentFile),
	}
	if err := validateStateBackupSource(source, modsDir); err != nil {
		return stateBackupResult{}, backupCreateError("validate state source", err)
	}

	digest := sha256.New()
	counter := &countingWriter{}
	if opts.dryRun {
		if _, err := archive.WriteState(io.MultiWriter(digest, counter), source); err != nil {
			return stateBackupResult{}, backupCreateError("write state archive", err)
		}
	} else {
		target := filepath.Join(stateDir, name)
		writer, err := atomicfile.New(target, 0o640)
		if err != nil {
			return stateBackupResult{}, backupCreateError(
				fmt.Sprintf("open %s", target), err)
		}
		if _, err := archive.WriteState(
			io.MultiWriter(writer, digest, counter), source); err != nil {
			return stateBackupResult{}, backupCreateError("write state archive",
				errors.Join(err, writer.Abort()))
		}
		if counter.n > archive.MaxCompressedSize {
			return stateBackupResult{}, backupCreateError("write state archive",
				errors.Join(fmt.Errorf("%w: compressed state exceeds %d bytes",
					archive.ErrLimit, archive.MaxCompressedSize), writer.Abort()))
		}
		if err := writer.Commit(); err != nil {
			return stateBackupResult{}, backupCreateError(
				fmt.Sprintf("commit %s", target), err)
		}
	}

	result := stateBackupResult{
		archive:  name,
		size:     counter.n,
		sha256:   hex.EncodeToString(digest.Sum(nil)),
		pruned:   append([]string{}, pressurePruned...),
		warnings: append([]string{}, warnings...),
	}

	// State creation always runs state-only retention. Mods packages are
	// immutable and can only be pruned by full retention on mods changes,
	// explicit `backups prune` or disk-pressure handling.
	cfg := retention.Config{
		StateDir:       stateDir,
		ProtectedState: opts.protect,
		Now:            now,
	}
	if opts.dryRun {
		plan, err := retention.Build(cfg)
		if err != nil {
			result.warnings = append(result.warnings,
				fmt.Sprintf("retention: %v", err))
			return result, nil
		}
		for _, file := range plan.Prune {
			result.pruned = append(result.pruned, file.Name)
		}
		result.warnings = append(result.warnings, plan.Warnings...)
		return result, nil
	}

	retained, err := retention.Apply(cfg)
	if err != nil {
		result.warnings = append(result.warnings,
			fmt.Sprintf("retention: %v", err))
		return result, nil
	}
	result.pruned = append(result.pruned, retained.Pruned...)
	result.warnings = append(result.warnings, retained.Warnings...)
	return result, nil
}

func runBackupsList(opts options, args []string, stdout, stderr io.Writer) int {
	listOpts, parseErr := parseBackupsListArgs(opts.root, args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	data, listErr := listBackups(listOpts)
	if listErr != nil {
		return writeError(stdout, stderr, opts.json, listErr)
	}

	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeBackupsHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseBackupsListArgs(root string, args []string) (backupsListOptions, *ops.Error) {
	fs := flag.NewFlagSet("backups list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	typ := fs.String("type", "", "")
	limit := fs.Int("limit", defaultBackupsLimit, "")
	before := fs.String("before", "", "")
	withSHA256 := fs.Bool("sha256", false, "")

	if err := fs.Parse(args); err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: %v", err))
	}
	if fs.NArg() != 0 {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: unexpected argument %q", fs.Arg(0)))
	}

	if *typ != backupTypeMods && *typ != backupTypeState {
		if *typ == "" {
			return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
				"backups list: --type is required (mods or state)")
		}
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: unsupported type %q; want mods or state",
				*typ))
	}
	if *limit < 1 || *limit > maxBackupsLimit {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: --limit must be between 1 and %d",
				maxBackupsLimit))
	}

	beforeSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "before" {
			beforeSet = true
		}
	})

	options := backupsListOptions{
		root:   root,
		typ:    *typ,
		limit:  *limit,
		before: *before,
		sha256: *withSHA256,
	}
	if !beforeSet {
		return options, nil
	}
	if options.before == "" {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			"backups list: --before must not be empty")
	}

	if err := pathguard.Base(options.before); err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitUnsafePath,
			fmt.Sprintf("backups list: --before: %v", err))
	}
	if err := validateBackupsCursor(options.typ, options.before); err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: --before: %v", err))
	}

	parsed, err := archive.ParseArchiveName(options.before)
	if err != nil {
		return backupsListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("backups list: --before: %v", err))
	}
	options.beforeTime = parsed.Time.UTC()
	return options, nil
}

func validateBackupsCursor(typ, name string) error {
	switch typ {
	case backupTypeMods:
		return archive.ValidateModsArchiveName(name)
	case backupTypeState:
		return archive.ValidateStateArchiveName(name)
	default:
		return fmt.Errorf("unsupported type %q", typ)
	}
}

func listBackups(options backupsListOptions) (backupsData, *ops.Error) {
	dir := filepath.Join(options.root, "backups", options.typ)

	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return backupsData{}, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("backups list: read %s: %v", dir, err))
	}

	archives := make([]backupEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return backupsData{}, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("backups list: stat %s: %v",
					filepath.Join(dir, entry.Name()), infoErr))
		}
		if !info.Mode().IsRegular() {
			continue
		}

		parsed, parseErr := archive.ParseArchiveName(entry.Name())
		if parseErr != nil || !backupsKindMatches(options.typ, parsed.Kind) {
			continue
		}
		archives = append(archives, backupEntry{
			name:  entry.Name(),
			size:  info.Size(),
			mtime: info.ModTime().UTC(),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		if !archives[i].mtime.Equal(archives[j].mtime) {
			return archives[i].mtime.After(archives[j].mtime)
		}
		return archives[i].name > archives[j].name
	})

	var current *string
	if options.typ == backupTypeMods {
		value, currentErr := archive.ReadCurrentFile(
			filepath.Join(dir, backupsCurrentFile))
		switch {
		case errors.Is(currentErr, fs.ErrNotExist):
			// A fresh installation has no current.txt yet.
		case currentErr != nil:
			return backupsData{}, ops.NewError(ops.ExitArchiveInvalid,
				fmt.Sprintf("backups list: %v", currentErr))
		default:
			name := value.Name
			current = &name
		}
	}

	archives = applyBackupsCursor(archives, options)
	if len(archives) > options.limit {
		archives = archives[:options.limit]
	}

	items := make([]backupsItem, 0, len(archives))
	modsDir := filepath.Join(options.root, "backups", "mods")
	for _, entry := range archives {
		item := backupsItem{
			Name:    entry.name,
			Size:    entry.size,
			MTime:   entry.mtime.Format(time.RFC3339),
			Current: current != nil && entry.name == *current,
		}
		if options.typ == backupTypeState {
			stateCurrent, stateErr := archive.ReadStateCurrent(
				filepath.Join(dir, entry.name))
			if stateErr == nil {
				modsName := stateCurrent.Name
				item.ModsArchive = &modsName
				available := regularBackupFile(
					filepath.Join(modsDir, modsName))
				item.ModsAvailable = &available
			} else {
				available := false
				item.ModsAvailable = &available
			}
		}
		items = append(items, item)
	}

	if options.sha256 {
		for i := range items {
			sum, hashErr := backupSHA256(filepath.Join(dir, items[i].Name))
			if hashErr != nil {
				return backupsData{}, ops.NewError(ops.ExitInternal,
					fmt.Sprintf("backups list: sha256 %s: %v",
						items[i].Name, hashErr))
			}
			items[i].SHA256 = &sum
		}
	}

	var nextBefore *string
	if len(items) == options.limit {
		name := items[len(items)-1].Name
		nextBefore = &name
	}

	return backupsData{
		Type:       options.typ,
		Current:    current,
		Items:      items,
		NextBefore: nextBefore,
	}, nil
}

func backupsKindMatches(typ string, kind archive.ArchiveKind) bool {
	switch typ {
	case backupTypeMods:
		return kind.IsMods()
	case backupTypeState:
		return kind.IsState()
	default:
		return false
	}
}

// applyBackupsCursor keeps only entries older than --before. When the cursor
// archive still exists, its position in the mtime/name order is authoritative.
// If retention removed it between pages, the UTC timestamp embedded in its name
// is used as a stable fallback.
func applyBackupsCursor(entries []backupEntry, options backupsListOptions) []backupEntry {
	if options.before == "" {
		return entries
	}
	for i, entry := range entries {
		if entry.name == options.before {
			return entries[i+1:]
		}
	}

	older := make([]backupEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.mtime.Before(options.beforeTime) ||
			(entry.mtime.Equal(options.beforeTime) &&
				entry.name < options.before) {
			older = append(older, entry)
		}
	}
	return older
}

func regularBackupFile(name string) bool {
	info, err := os.Lstat(name)
	return err == nil && info.Mode().IsRegular()
}

func backupSHA256(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()

	sum := sha256.New()
	if _, err := io.CopyBuffer(sum, file, make([]byte, 32*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func writeBackupsHuman(w io.Writer, data backupsData) error {
	current := "-"
	if data.Current != nil {
		current = *data.Current
	}
	nextBefore := "-"
	if data.NextBefore != nil {
		nextBefore = *data.NextBefore
	}

	var out strings.Builder
	fmt.Fprintf(&out, "type: %s\n", data.Type)
	fmt.Fprintf(&out, "current: %s\n", current)
	for _, item := range data.Items {
		marker := "-"
		if item.Current {
			marker = "current"
		}
		fmt.Fprintf(&out, "%s\t%d\t%s\t%s",
			item.Name, item.Size, item.MTime, marker)
		if item.SHA256 != nil {
			fmt.Fprintf(&out, "\t%s", *item.SHA256)
		}
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "next_before: %s\n", nextBefore)
	_, err := io.WriteString(w, out.String())
	return err
}
