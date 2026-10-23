package main

import (
	"context"
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

type modsDeployArgs struct {
	archive   string
	latest    bool
	current   bool
	noRestart bool
}

type modsDeployRequest struct {
	root      string
	archive   string
	latest    bool
	current   bool
	noRestart bool
	now       time.Time
	dryRun    bool
	timeout   time.Duration
}

type modsDeployResult struct {
	archive   string
	sha256    string
	previous  *string
	state     string
	restarted bool
	warnings  []string
}

type modsDeployData struct {
	Archive     string  `json:"archive"`
	SHA256      string  `json:"sha256"`
	Previous    *string `json:"previous"`
	StateBackup string  `json:"state_backup"`
	Restarted   bool    `json:"restarted"`
}

// These hooks keep deploy tests independent of the wall clock, a small test
// filesystem and atomic-write failure injection.
var (
	modsDeployNowFunc       = func() time.Time { return time.Now().UTC() }
	modsDeployDiskFreeFunc  = retention.DiskFree
	modsDeployWriteFileFunc = atomicfile.WriteFile
)

func runModsDeploy(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	deployArgs, parseErr := parseModsDeployArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "mods deploy", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, deployErr := deployModsLocked(modsDeployRequest{
		root:      opts.root,
		archive:   deployArgs.archive,
		latest:    deployArgs.latest,
		current:   deployArgs.current,
		noRestart: deployArgs.noRestart,
		now:       modsDeployNowFunc(),
		dryRun:    opts.dryRun,
		timeout:   opts.commandTimeout,
	}, client)
	if deployErr != nil {
		return writeError(stdout, stderr, opts.json, deployErr)
	}
	for _, warning := range result.warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: mods deploy: %s\n", warning)
	}

	data := modsDeployData{
		Archive:     result.archive,
		SHA256:      result.sha256,
		Previous:    result.previous,
		StateBackup: result.state,
		Restarted:   result.restarted,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeModsDeployHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseModsDeployArgs(args []string) (modsDeployArgs, *ops.Error) {
	fs := flag.NewFlagSet("mods deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	archiveName := fs.String("archive", "", "")
	latest := fs.Bool("latest", false, "")
	current := fs.Bool("current", false, "")
	noRestart := fs.Bool("no-restart", false, "")

	if err := fs.Parse(args); err != nil {
		return modsDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods deploy: %v", err))
	}
	if fs.NArg() != 0 {
		return modsDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods deploy: unexpected argument %q", fs.Arg(0)))
	}

	archiveSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "archive" {
			archiveSet = true
		}
	})
	if archiveSet && *archiveName == "" {
		return modsDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"mods deploy: --archive must not be empty")
	}

	selectors := 0
	if archiveSet {
		selectors++
	}
	if *latest {
		selectors++
	}
	if *current {
		selectors++
	}
	if selectors == 0 {
		return modsDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"mods deploy: one of --archive, --latest or --current is required")
	}
	if selectors > 1 {
		return modsDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"mods deploy: --archive, --latest and --current are mutually exclusive")
	}

	return modsDeployArgs{
		archive:   *archiveName,
		latest:    *latest,
		current:   *current,
		noRestart: *noRestart,
	}, nil
}

func writeModsDeployHuman(w io.Writer, data modsDeployData) error {
	previous := "-"
	if data.Previous != nil {
		previous = *data.Previous
	}
	restarted := "false"
	if data.Restarted {
		restarted = "true"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "archive: %s\n", data.Archive)
	fmt.Fprintf(&out, "sha256: %s\n", data.SHA256)
	fmt.Fprintf(&out, "previous: %s\n", previous)
	fmt.Fprintf(&out, "state_backup: %s\n", data.StateBackup)
	fmt.Fprintf(&out, "restarted: %s\n", restarted)
	_, err := io.WriteString(w, out.String())
	return err
}

type modsDeploySelection struct {
	name         string
	path         string
	sha256       string
	uncompressed int64
	previous     *string
}

func resolveModsDeployTarget(req modsDeployRequest) (modsDeploySelection, *ops.Error) {
	modsDir := filepath.Join(req.root, "backups", "mods")
	var previous *string
	current, currentErr := archive.VerifyCurrentMods(modsDir)
	switch {
	case currentErr == nil:
		name := current.Name
		previous = &name
	case errors.Is(currentErr, fs.ErrNotExist):
		if req.current {
			// --current requires canonical metadata; a missing file is
			// archive_invalid, not a first deploy.
			return modsDeploySelection{}, ops.NewError(ops.ExitArchiveInvalid,
				fmt.Sprintf("mods deploy: read current archive metadata: %v",
					currentErr))
		}
		// A first deploy may start without a current archive. The required
		// predeploy state archive reports the missing metadata later.
	case errors.Is(currentErr, archive.ErrInvalid):
		return modsDeploySelection{}, modsDeployError(
			"validate current archive", currentErr)
	default:
		return modsDeploySelection{}, modsDeployError(
			"read current archive metadata", currentErr)
	}

	name := req.archive
	switch {
	case req.archive != "":
		if err := pathguard.Base(req.archive); err != nil {
			return modsDeploySelection{}, ops.NewError(ops.ExitUnsafePath,
				fmt.Sprintf("mods deploy: archive %q: %v", req.archive, err))
		}
		if err := archive.ValidateModsArchiveName(req.archive); err != nil {
			return modsDeploySelection{}, modsDeployError(
				"validate archive name", err)
		}
	case req.current:
		if previous == nil {
			return modsDeploySelection{}, ops.NewError(ops.ExitNotFound,
				fmt.Sprintf("mods deploy: %s does not exist",
					filepath.Join(modsDir, backupsCurrentFile)))
		}
		name = current.Name
	case req.latest:
		latest, err := latestModsArchive(modsDir)
		if err != nil {
			return modsDeploySelection{}, err
		}
		name = latest
	}

	target := filepath.Join(modsDir, name)
	sha, uncompressed, err := inspectModsDeployArchive(target)
	if err != nil {
		return modsDeploySelection{}, err
	}
	return modsDeploySelection{
		name:         name,
		path:         target,
		sha256:       sha,
		uncompressed: uncompressed,
		previous:     previous,
	}, nil
}

func latestModsArchive(modsDir string) (string, *ops.Error) {
	entries, err := os.ReadDir(modsDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("mods deploy: %s does not exist", modsDir))
	case err != nil:
		return "", modsDeployError("read mods directory", err)
	}

	type candidate struct {
		name   string
		parsed archive.ArchiveName
	}
	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		parsed, err := archive.ParseArchiveName(name)
		if err != nil || !parsed.Kind.IsMods() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", modsDeployError("stat archive "+name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, candidate{name: name, parsed: parsed})
	}
	if len(candidates) == 0 {
		return "", ops.NewError(ops.ExitNotFound,
			"mods deploy: no mods archive is available for --latest")
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i].parsed, candidates[j].parsed
		if !left.Time.Equal(right.Time) {
			return left.Time.After(right.Time)
		}
		if left.Collision != right.Collision {
			return left.Collision > right.Collision
		}
		return candidates[i].name > candidates[j].name
	})
	return candidates[0].name, nil
}

func inspectModsDeployArchive(name string) (string, int64, *ops.Error) {
	info, err := os.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", 0, ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("mods deploy: archive %s does not exist", name))
	case err != nil:
		return "", 0, modsDeployError("stat archive", err)
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return "", 0, ops.NewError(ops.ExitArchiveInvalid,
			fmt.Sprintf("mods deploy: %s is not a regular archive file", name))
	case info.Size() > archive.MaxCompressedSize:
		return "", 0, ops.NewError(ops.ExitArchiveInvalid,
			fmt.Sprintf("mods deploy: %s exceeds %d bytes",
				name, archive.MaxCompressedSize))
	}

	_, uncompressed, err := archive.ValidateModsFileWithSize(name)
	if err != nil {
		return "", 0, modsDeployError("validate archive", err)
	}
	sha, err := backupSHA256(name)
	if err != nil {
		return "", 0, modsDeployError("hash archive", err)
	}
	return sha, uncompressed, nil
}

func modsDeployError(operation string, err error) *ops.Error {
	exit := ops.ExitInternal
	switch {
	case errors.Is(err, archive.ErrInvalid), errors.Is(err, archive.ErrLimit):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, pathguard.ErrUnsafePath):
		exit = ops.ExitUnsafePath
	case errors.Is(err, syscall.ENOSPC):
		exit = ops.ExitNoSpace
	case errors.Is(err, fs.ErrNotExist):
		exit = ops.ExitNotFound
	}
	return ops.NewError(exit,
		fmt.Sprintf("mods deploy: %s: %v", operation, err))
}

func deployModsLocked(req modsDeployRequest,
	client *systemd.Client,
) (modsDeployResult, *ops.Error) {
	if req.root == "" {
		return modsDeployResult{}, ops.NewError(ops.ExitInternal,
			"mods deploy: root is empty")
	}
	now := req.now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	selection, selectionErr := resolveModsDeployTarget(req)
	if selectionErr != nil {
		return modsDeployResult{}, selectionErr
	}

	stateDir := filepath.Join(req.root, "backups", "state")
	stateEstimate, err := estimateStateBackupSize(stateDir)
	if err != nil {
		return modsDeployResult{}, modsDeployError("estimate state backup", err)
	}
	if spaceErr := checkDeploySpace(
		"mods deploy", stateDir, selection.uncompressed+stateEstimate); spaceErr != nil {
		return modsDeployResult{}, spaceErr
	}

	predeploy, backupErr := createStateBackupLocked(stateBackupOptions{
		root:   req.root,
		kind:   archive.KindStatePredeploy,
		now:    now,
		dryRun: req.dryRun,
	})
	if backupErr != nil {
		return modsDeployResult{}, backupErr
	}
	result := modsDeployResult{
		archive:  selection.name,
		sha256:   selection.sha256,
		previous: selection.previous,
		state:    predeploy.archive,
		warnings: append([]string{}, predeploy.warnings...),
	}
	if req.dryRun {
		return result, nil
	}

	tx, err := newModsDeployTx(req.root, now)
	if err != nil {
		return modsDeployResult{}, modsDeployError("prepare transaction", err)
	}

	wasActive, err := deployIsActive(client, req.timeout)
	if err != nil {
		return modsDeployResult{}, ops.NewError(ops.ExitServiceUnavailable,
			fmt.Sprintf("mods deploy: check %s: %v",
				systemd.UnitTES3MPService, err))
	}
	if wasActive {
		if err := deployControl(client, systemd.ActionStop, req.timeout); err != nil {
			return modsDeployResult{}, ops.NewError(ops.ExitServiceUnavailable,
				fmt.Sprintf("mods deploy: stop %s: %v",
					systemd.UnitTES3MPService, err))
		}
	}

	fail := func(original *ops.Error) *ops.Error {
		var problems []string
		if rollbackErr := tx.rollback(); rollbackErr != nil {
			problems = append(problems,
				fmt.Sprintf("rollback files: %v", rollbackErr))
		}
		if wasActive && !req.noRestart {
			if err := deployControl(client, systemd.ActionStart,
				req.timeout); err != nil {
				problems = append(problems,
					fmt.Sprintf("restart %s: %v",
						systemd.UnitTES3MPService, err))
			}
		}
		if len(problems) > 0 {
			original.Message += "; " + strings.Join(problems, "; ")
		}
		return original
	}

	if err := tx.stage(selection.path); err != nil {
		return modsDeployResult{}, fail(modsDeployError("extract archive", err))
	}
	if err := tx.switchDirs(); err != nil {
		return modsDeployResult{}, fail(modsDeployError("install mods", err))
	}
	if err := tx.writeCustomScripts(); err != nil {
		return modsDeployResult{}, fail(modsDeployError(
			"generate customScripts.lua", err))
	}
	if err := tx.writeCurrent(selection.name, selection.sha256); err != nil {
		return modsDeployResult{}, fail(modsDeployError("write current.txt", err))
	}

	if wasActive && !req.noRestart {
		if err := deployControl(client, systemd.ActionStart, req.timeout); err != nil {
			return modsDeployResult{}, fail(ops.NewError(
				ops.ExitServiceUnavailable,
				fmt.Sprintf("mods deploy: start %s: %v",
					systemd.UnitTES3MPService, err)))
		}
		result.restarted = true
	}

	result.warnings = append(result.warnings, tx.commit()...)
	return result, nil
}

func deployIsActive(client *systemd.Client, timeout time.Duration) (bool, error) {
	if client == nil {
		return false, errors.New("systemd client is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return client.IsActive(ctx, systemd.UnitTES3MPService)
}

func deployControl(client *systemd.Client, action systemd.ControlAction,
	timeout time.Duration,
) error {
	if client == nil {
		return errors.New("systemd client is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return client.Control(ctx, action)
}

func estimateStateBackupSize(stateDir string) (int64, error) {
	entries, err := os.ReadDir(stateDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var largest int64
	for _, entry := range entries {
		parsed, err := archive.ParseArchiveName(entry.Name())
		if err != nil || !parsed.Kind.IsState() || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > largest {
			largest = info.Size()
		}
	}
	return largest, nil
}

func checkDeploySpace(command, stateDir string, additional int64) *ops.Error {
	path := stateDir
	for {
		if _, err := os.Stat(path); err == nil {
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: stat %s: %v", command, path, err))
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ops.NewError(ops.ExitInternal,
				fmt.Sprintf("%s: no existing directory for %s", command, stateDir))
		}
		path = parent
	}
	free, err := modsDeployDiskFreeFunc(path)
	if err != nil {
		return ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: free space: %v", command, err))
	}
	if additional < 0 ||
		additional > int64(^uint64(0)>>1)-retention.DefaultMinFree {
		return ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: disk-space estimate overflow", command))
	}
	required := retention.DefaultMinFree + additional
	if free < required {
		return ops.NewError(ops.ExitNoSpace,
			fmt.Sprintf("%s: free space %d is below %d bytes",
				command, free, required))
	}
	return nil
}
