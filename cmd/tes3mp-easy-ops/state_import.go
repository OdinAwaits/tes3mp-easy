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
	"syscall"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/retention"
)

type stateImportArgs struct {
	file string
	name string
}

type stateImportRequest struct {
	root   string
	file   string
	name   string
	now    time.Time
	dryRun bool
}

type stateImportResult struct {
	archive    string
	sha256     string
	mods       string
	players    int
	worldFiles int
	pruned     []string
	warnings   []string
}

type stateImportData struct {
	Archive     string   `json:"archive"`
	SHA256      string   `json:"sha256"`
	ModsArchive string   `json:"mods_archive"`
	Players     int      `json:"players"`
	WorldFiles  int      `json:"world_files"`
	Pruned      []string `json:"pruned"`
	Warnings    []string `json:"warnings"`
}

var stateImportNowFunc = func() time.Time { return time.Now().UTC() }
var stateImportRenameFunc = os.Rename

func runStateImport(opts options, args []string, stdout, stderr io.Writer) int {
	importArgs, parseErr := parseStateImportArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "state import", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, importErr := importStateLocked(stateImportRequest{
		root:   opts.root,
		file:   importArgs.file,
		name:   importArgs.name,
		now:    stateImportNowFunc(),
		dryRun: opts.dryRun,
	})
	if importErr != nil {
		return writeError(stdout, stderr, opts.json, importErr)
	}

	data := stateImportData{
		Archive:     result.archive,
		SHA256:      result.sha256,
		ModsArchive: result.mods,
		Players:     result.players,
		WorldFiles:  result.worldFiles,
		Pruned:      append([]string{}, result.pruned...),
		Warnings:    append([]string{}, result.warnings...),
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeStateImportHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseStateImportArgs(args []string) (stateImportArgs, *ops.Error) {
	fs := flag.NewFlagSet("state import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	file := fs.String("file", "", "")
	name := fs.String("name", "", "")

	if err := fs.Parse(args); err != nil {
		return stateImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("state import: %v", err))
	}
	if fs.NArg() != 0 {
		return stateImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("state import: unexpected argument %q", fs.Arg(0)))
	}
	if *file == "" {
		return stateImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"state import: --file is required")
	}
	if *name != "" {
		if err := archive.ValidateStateArchiveName(*name); err != nil {
			return stateImportArgs{}, ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("state import: --name: %v", err))
		}
	}
	return stateImportArgs{file: *file, name: *name}, nil
}

func importStateLocked(req stateImportRequest) (stateImportResult, *ops.Error) {
	if req.root == "" {
		return stateImportResult{}, ops.NewError(ops.ExitInternal,
			"state import: root is empty")
	}
	now := req.now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	stateDir := filepath.Join(req.root, "backups", "state")
	modsDir := filepath.Join(req.root, "backups", "mods")
	if _, err := prepareArchiveDestination(stateDir, req.dryRun); err != nil {
		return stateImportResult{}, stateImportError("prepare destination", err)
	}

	uploadInfo, err := os.Lstat(req.file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return stateImportResult{}, ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("state import: upload %s does not exist", req.file))
	case err != nil:
		return stateImportResult{}, stateImportError("stat upload", err)
	case uploadInfo.Mode()&os.ModeSymlink != 0:
		return stateImportResult{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("state import: upload %s is a symlink, not a regular file",
				req.file))
	case !uploadInfo.Mode().IsRegular():
		return stateImportResult{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("state import: upload %s is not a regular file", req.file))
	}
	if uploadInfo.Size() > archive.MaxCompressedSize {
		return stateImportResult{}, ops.NewError(ops.ExitArchiveInvalid,
			fmt.Sprintf("state import: %v: upload exceeds %d bytes",
				archive.ErrLimit, archive.MaxCompressedSize))
	}

	name, nameErr := stateImportArchiveName(req.name, req.file, stateDir)
	if nameErr != nil {
		return stateImportResult{}, stateImportError("choose archive name", nameErr)
	}

	info, _, err := archive.ValidateStateFileWithSize(req.file, modsDir)
	if err != nil {
		return stateImportResult{}, stateImportError("validate upload", err)
	}
	sum, err := stateImportSHA256(req.file)
	if err != nil {
		return stateImportResult{}, stateImportError("hash upload", err)
	}

	result := stateImportResult{
		archive:    name,
		sha256:     sum,
		mods:       info.Current.Name,
		players:    info.Players,
		worldFiles: info.WorldFiles,
		pruned:     []string{},
		warnings:   []string{},
	}

	cfg := retention.Config{
		StateDir:       stateDir,
		ProtectedState: []string{name},
		Now:            now,
	}
	if req.dryRun {
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

	target := filepath.Join(stateDir, name)
	if err := publishUpload(req.file, target, stateImportRenameFunc); err != nil {
		return stateImportResult{}, stateImportError("publish upload", err)
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

func stateImportArchiveName(requested, file, stateDir string) (string, error) {
	name := requested
	if name == "" {
		name = filepath.Base(file)
	}
	if err := archive.ValidateStateArchiveName(name); err != nil {
		return "", err
	}
	target := filepath.Join(stateDir, name)
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("%w: %s", fs.ErrExist, target)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return name, nil
}

func stateImportSHA256(name string) (string, error) {
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

func stateImportError(operation string, err error) *ops.Error {
	exit := ops.ExitInternal
	switch {
	case errors.Is(err, archive.ErrInvalid), errors.Is(err, archive.ErrLimit):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, syscall.ENOSPC):
		exit = ops.ExitNoSpace
	case errors.Is(err, fs.ErrExist):
		exit = ops.ExitInvalidArgument
	case errors.Is(err, syscall.EXDEV):
		exit = ops.ExitInvalidArgument
	case errors.Is(err, os.ErrNotExist):
		exit = ops.ExitNotFound
	}
	return ops.NewError(exit, fmt.Sprintf("state import: %s: %v", operation, err))
}

func writeStateImportHuman(w io.Writer, data stateImportData) error {
	var out string
	out += fmt.Sprintf("archive: %s\n", data.Archive)
	out += fmt.Sprintf("sha256: %s\n", data.SHA256)
	out += fmt.Sprintf("mods_archive: %s\n", data.ModsArchive)
	out += fmt.Sprintf("players: %d\n", data.Players)
	out += fmt.Sprintf("world_files: %d\n", data.WorldFiles)
	_, err := io.WriteString(w, out)
	return err
}
