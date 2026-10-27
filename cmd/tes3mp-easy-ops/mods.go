package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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

type modsImportArgs struct {
	file string
}

type modsImportRequest struct {
	root   string
	file   string
	now    time.Time
	dryRun bool
}

type modsImportResult struct {
	archive  string
	sha256   string
	plugins  int
	scripts  int
	pruned   []string
	warnings []string
}

type modsImportData struct {
	Archive  string   `json:"archive"`
	SHA256   string   `json:"sha256"`
	Plugins  int      `json:"plugins"`
	Scripts  int      `json:"scripts"`
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

// These hooks keep the command testable without changing archive limits,
// mounting a second filesystem or reproducing a systemd mount namespace.
var (
	modsImportNowFunc          = func() time.Time { return time.Now().UTC() }
	modsImportMaxCompressed    = archive.MaxCompressedSize
	modsImportDeviceOfPathFunc = deviceOfPath
	modsImportRenameFunc       = os.Rename
)

func runMods(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"mods: subcommand is required; want import or deploy"))
	}

	switch args[0] {
	case "import":
		return runModsImport(opts, args[1:], stdout, stderr)
	case "deploy":
		return runModsDeploy(opts, args[1:], client, stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("mods: unknown subcommand %q", args[0])))
	}
}

func runModsImport(opts options, args []string, stdout, stderr io.Writer) int {
	importArgs, parseErr := parseModsImportArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "mods import", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, importErr := importModsLocked(modsImportRequest{
		root:   opts.root,
		file:   importArgs.file,
		now:    modsImportNowFunc(),
		dryRun: opts.dryRun,
	})
	if importErr != nil {
		return writeError(stdout, stderr, opts.json, importErr)
	}

	data := modsImportData{
		Archive:  result.archive,
		SHA256:   result.sha256,
		Plugins:  result.plugins,
		Scripts:  result.scripts,
		Pruned:   result.pruned,
		Warnings: result.warnings,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeModsImportHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseModsImportArgs(args []string) (modsImportArgs, *ops.Error) {
	fs := flag.NewFlagSet("mods import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	file := fs.String("file", "", "")

	if err := fs.Parse(args); err != nil {
		return modsImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods import: %v", err))
	}
	if fs.NArg() != 0 {
		return modsImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods import: unexpected argument %q", fs.Arg(0)))
	}
	if *file == "" {
		return modsImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"mods import: --file is required")
	}
	return modsImportArgs{file: *file}, nil
}

// importModsLocked validates the upload and moves it into backups/mods. The
// caller must already hold the ops lock; future deploy operations can reuse it
// without taking the lock again.
func importModsLocked(req modsImportRequest) (modsImportResult, *ops.Error) {
	if req.root == "" {
		return modsImportResult{}, ops.NewError(ops.ExitInternal,
			"mods import: root is empty")
	}
	now := req.now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	modsDir := filepath.Join(req.root, "backups", "mods")
	stateDir := filepath.Join(req.root, "backups", "state")
	devicePath, err := prepareModsDestination(modsDir, req.dryRun)
	if err != nil {
		return modsImportResult{}, modsImportError("prepare destination", err)
	}

	uploadInfo, err := os.Lstat(req.file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return modsImportResult{}, ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("mods import: upload %s does not exist", req.file))
	case err != nil:
		return modsImportResult{}, modsImportError("stat upload", err)
	case uploadInfo.Mode()&os.ModeSymlink != 0:
		return modsImportResult{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods import: upload %s is a symlink, not a regular file",
				req.file))
	case !uploadInfo.Mode().IsRegular():
		return modsImportResult{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods import: upload %s is not a regular file", req.file))
	}

	uploadDev, err := modsImportDeviceOfPathFunc(req.file)
	if err != nil {
		return modsImportResult{}, modsImportError("inspect upload filesystem", err)
	}
	destDev, err := modsImportDeviceOfPathFunc(devicePath)
	if err != nil {
		return modsImportResult{}, modsImportError(
			"inspect backup filesystem", err)
	}
	if uploadDev != destDev {
		return modsImportResult{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("mods import: upload %s and %s are on different filesystems (device %d and %d)",
				req.file, devicePath, uploadDev, destDev))
	}

	mods, sha, err := validateModsUpload(req.file)
	if err != nil {
		return modsImportResult{}, modsImportError("validate upload", err)
	}

	name, err := archive.NextArchiveNameInDir(modsDir, archive.KindMods, now)
	if err != nil {
		return modsImportResult{}, modsImportError("choose archive name", err)
	}
	result := modsImportResult{
		archive:  name,
		sha256:   sha,
		plugins:  mods.Plugins,
		scripts:  mods.Scripts,
		pruned:   []string{},
		warnings: []string{},
	}

	cfg := retention.Config{
		StateDir: stateDir,
		ModsDir:  modsDir,
		Now:      now,
	}
	if req.dryRun {
		// A dry run may not have backups/mods yet. Check free space on the
		// existing destination instead of a path retention cannot stat.
		freePath := devicePath
		cfg.FreeSpace = func(string) (int64, error) {
			return retention.DiskFree(freePath)
		}
		plan, planErr := retention.Build(cfg)
		if planErr != nil {
			result.warnings = append(result.warnings,
				fmt.Sprintf("retention: %v", planErr))
			return result, nil
		}
		for _, file := range plan.Prune {
			result.pruned = append(result.pruned, file.Name)
		}
		result.warnings = append(result.warnings, plan.Warnings...)
		return result, nil
	}

	target := filepath.Join(modsDir, name)
	if err := publishModsUpload(req.file, target); err != nil {
		return modsImportResult{}, modsImportError("publish upload", err)
	}

	retained, retentionErr := retention.Apply(cfg)
	if retentionErr != nil {
		result.warnings = append(result.warnings,
			fmt.Sprintf("retention: %v", retentionErr))
		return result, nil
	}
	result.pruned = append(result.pruned, retained.Pruned...)
	result.warnings = append(result.warnings, retained.Warnings...)
	return result, nil
}

// publishModsUpload moves a validated upload into backups/mods. Rename is the
// fast path. It can return EXDEV even when both paths share st_dev: systemd
// implements ReadWritePaths as separate bind mounts, and Linux forbids rename
// across mount points. In that case copy into a .part file in the destination
// directory, fsync, and rename within that directory.
func publishModsUpload(src, target string) error {
	dir := filepath.Dir(target)
	err := modsImportRenameFunc(src, target)
	if err == nil {
		if syncErr := atomicfile.SyncDir(dir); syncErr != nil {
			return errors.Join(
				fmt.Errorf("sync backup directory: %w", syncErr),
				modsImportRenameFunc(target, src),
			)
		}
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("rename upload: %w", err)
	}

	if copyErr := copyModsUpload(src, target); copyErr != nil {
		return fmt.Errorf("rename upload: %v; copy fallback: %w", err, copyErr)
	}
	if removeErr := os.Remove(src); removeErr != nil {
		return errors.Join(
			fmt.Errorf("remove upload after copy: %w", removeErr),
			removeModsTarget(target),
			atomicfile.SyncDir(dir),
		)
	}
	return nil
}

// copyModsUpload writes target through atomicfile so a crash cannot expose a
// partial backup. The caller removes src only after the destination is durable.
func copyModsUpload(src, target string) error {
	input, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open upload: %w", err)
	}
	writer, err := atomicfile.New(target, 0o640)
	if err != nil {
		return errors.Join(fmt.Errorf("create backup: %w", err), input.Close())
	}
	_, copyErr := io.Copy(writer, input)
	closeErr := input.Close()
	if copyErr != nil {
		return errors.Join(
			fmt.Errorf("copy upload: %w", copyErr),
			closeErr,
			writer.Abort(),
		)
	}
	if closeErr != nil {
		return errors.Join(
			fmt.Errorf("close upload: %w", closeErr),
			writer.Abort(),
		)
	}
	if err := writer.Commit(); err != nil {
		return errors.Join(
			fmt.Errorf("commit backup: %w", err),
			removeModsTarget(target),
			atomicfile.SyncDir(filepath.Dir(target)),
		)
	}
	return nil
}

func removeModsTarget(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove backup %s: %w", path, err)
	}
	return nil
}

// prepareModsDestination creates backups/mods for a real import. A dry run
// must not create anything, so it returns the nearest existing parent for the
// same-filesystem check while NextArchiveNameInDir still targets modsDir.
func prepareModsDestination(dir string, dryRun bool) (string, error) {
	info, err := os.Lstat(dir)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("%s is not a directory", dir)
		}
		return dir, nil
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}

	if !dryRun {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", err
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("%s is not a directory", dir)
		}
		return dir, nil
	}

	return existingAncestor(dir)
}

func existingAncestor(path string) (string, error) {
	for {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("no existing directory for %s", path)
		}
		path = parent
	}
}

// validateModsUpload streams the archive once, hashing the compressed bytes
// while ValidateMods checks the layout, metadata and plugin CRCs.
func validateModsUpload(name string) (archive.ModsInfo, string, error) {
	file, err := os.Open(name)
	if err != nil {
		return archive.ModsInfo{}, "", err
	}

	info, err := file.Stat()
	if err != nil {
		return archive.ModsInfo{}, "", errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() {
		return archive.ModsInfo{}, "", errors.Join(
			fmt.Errorf("%s is not a regular file", name), file.Close())
	}
	if info.Size() > modsImportMaxCompressed {
		return archive.ModsInfo{}, "", errors.Join(
			fmt.Errorf("%w: compressed size exceeds %d bytes",
				archive.ErrLimit, modsImportMaxCompressed),
			file.Close())
	}

	digest := sha256.New()
	reader, err := archive.NewReader(io.TeeReader(file, digest))
	if err != nil {
		return archive.ModsInfo{}, "", errors.Join(err, file.Close())
	}

	mods, validateErr := archive.ValidateMods(reader)
	readerErr := reader.Close()
	fileErr := file.Close()
	if validateErr != nil {
		return archive.ModsInfo{}, "",
			errors.Join(validateErr, readerErr, fileErr)
	}
	if readerErr != nil || fileErr != nil {
		return archive.ModsInfo{}, "", errors.Join(readerErr, fileErr)
	}
	return mods, hex.EncodeToString(digest.Sum(nil)), nil
}

func modsImportError(operation string, err error) *ops.Error {
	exit := ops.ExitInternal
	switch {
	case errors.Is(err, archive.ErrInvalid), errors.Is(err, archive.ErrLimit):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, pathguard.ErrUnsafePath):
		exit = ops.ExitUnsafePath
	case errors.Is(err, syscall.ENOSPC):
		exit = ops.ExitNoSpace
	case errors.Is(err, syscall.EXDEV):
		exit = ops.ExitInvalidArgument
	}
	return ops.NewError(exit, fmt.Sprintf("mods import: %s: %v", operation, err))
}

func writeModsImportHuman(w io.Writer, data modsImportData) error {
	pruned := "-"
	if len(data.Pruned) > 0 {
		pruned = strings.Join(data.Pruned, ", ")
	}
	warnings := "-"
	if len(data.Warnings) > 0 {
		warnings = strings.Join(data.Warnings, "; ")
	}

	var out strings.Builder
	fmt.Fprintf(&out, "archive: %s\nsha256: %s\nplugins: %d\nscripts: %d\n",
		data.Archive, data.SHA256, data.Plugins, data.Scripts)
	fmt.Fprintf(&out, "pruned: %s\nwarnings: %s\n", pruned, warnings)
	_, err := io.WriteString(w, out.String())
	return err
}
