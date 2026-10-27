package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type restoreArgs struct {
	archive        string
	latest         bool
	acceptMismatch bool
	noRestart      bool
}

type restoreTarget struct {
	name     string
	path     string
	manifest restorepoint.Manifest
}

func parseRestoreArgs(command string, args []string, allowPredeploy bool,
) (restoreArgs, *ops.Error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	archiveName := fs.String("archive", "", "")
	latest := fs.Bool("latest", false, "")
	acceptMismatch := fs.Bool("accept-mod-mismatch", false, "")
	noRestart := fs.Bool("no-restart", false, "")
	if err := fs.Parse(args); err != nil {
		return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: %v", command, err))
	}
	if fs.NArg() != 0 {
		return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: unexpected argument %q", command, fs.Arg(0)))
	}
	archiveSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "archive" {
			archiveSet = true
		}
	})
	if archiveSet && *archiveName == "" {
		return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: --archive must not be empty", command))
	}
	if archiveSet && *latest {
		return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: --archive and --latest are mutually exclusive", command))
	}
	if !archiveSet && !*latest {
		return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: one of --archive or --latest is required", command))
	}
	if *archiveName != "" {
		if err := pathguard.Base(*archiveName); err != nil {
			return restoreArgs{}, ops.NewError(ops.ExitUnsafePath,
				fmt.Sprintf("%s: archive: %v", command, err))
		}
		parsed, err := restorepoint.ParseName(*archiveName)
		if err != nil {
			return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("%s: archive: %v", command, err))
		}
		valid := parsed.Kind == restorepoint.KindSnapshot
		if allowPredeploy {
			valid = parsed.Kind.IsCheckpoint()
		}
		if !valid {
			return restoreArgs{}, ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("%s: archive %q has the wrong kind", command, *archiveName))
		}
	}
	return restoreArgs{
		archive: *archiveName, latest: *latest,
		acceptMismatch: *acceptMismatch, noRestart: *noRestart,
	}, nil
}

func resolveCheckpointTarget(root string, args restoreArgs) (restoreTarget, *ops.Error) {
	if args.archive != "" {
		return readRestoreTarget(root, restorepoint.KindCheckpoint, args.archive)
	}
	items, err := restorepoint.ListCheckpoints(root)
	if err != nil {
		return restoreTarget{}, restorePointError("checkpoint restore", err)
	}
	for _, item := range items {
		if item.Reason == "predeploy" || !item.Base.Available {
			continue
		}
		return readRestoreTarget(root, restorepoint.KindCheckpoint, item.Name)
	}
	return restoreTarget{}, ops.NewError(ops.ExitNotFound,
		"checkpoint restore: no available checkpoint")
}

func resolveSnapshotTarget(root string, args restoreArgs) (restoreTarget, *ops.Error) {
	if args.archive != "" {
		return readRestoreTarget(root, restorepoint.KindSnapshot, args.archive)
	}
	items, err := restorepoint.ListSnapshots(root)
	if err != nil {
		return restoreTarget{}, restorePointError("snapshot restore", err)
	}
	if len(items) == 0 {
		return restoreTarget{}, ops.NewError(ops.ExitNotFound,
			"snapshot restore: no snapshot archive")
	}
	return readRestoreTarget(root, restorepoint.KindSnapshot, items[0].Name)
}

func readRestoreTarget(root string, kind restorepoint.Kind, name string,
) (restoreTarget, *ops.Error) {
	parsed, err := restorepoint.ParseName(name)
	if err != nil {
		return restoreTarget{}, restorePointError("restore", err)
	}
	valid := parsed.Kind == kind
	if kind == restorepoint.KindCheckpoint {
		valid = parsed.Kind.IsCheckpoint()
	}
	if !valid {
		return restoreTarget{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("restore: archive %q has the wrong kind", name))
	}
	path := filepath.Join(restorepoint.Dir(root, kind), name)
	manifest, err := restorepoint.ReadManifest(path)
	if err != nil {
		return restoreTarget{}, restorePointError("restore: read manifest", err)
	}
	return restoreTarget{name: name, path: path, manifest: manifest}, nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return false, fmt.Errorf("%s: not a regular file", path)
	default:
		return true, nil
	}
}

func restoreStagingDir(root, prefix string) (string, *ops.Error) {
	parent := filepath.Join(root, ".staging")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", ops.NewError(ops.ExitInternal,
			fmt.Sprintf("create %s: %v", parent, err))
	}
	dir, err := os.MkdirTemp(parent, prefix+"-")
	if err != nil {
		return "", ops.NewError(ops.ExitInternal,
			fmt.Sprintf("create restore staging: %v", err))
	}
	return dir, nil
}

func installModsFromDir(root, modsDir string, now time.Time,
	timeout time.Duration, client *systemd.Client,
) (string, []string, *ops.Error) {
	const command = "mods deploy"
	backupsDir := filepath.Join(root, "backups", "mods")
	if err := os.MkdirAll(backupsDir, 0o750); err != nil {
		return "", nil, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: create %s: %v", command, backupsDir, err))
	}
	name, err := archive.NextArchiveNameInDir(backupsDir, archive.KindMods, now)
	if err != nil {
		return "", nil, restorePointError(command, err)
	}
	target := filepath.Join(backupsDir, name)
	writer, err := atomicfile.New(target, 0o640)
	if err != nil {
		return "", nil, restorePointError(
			fmt.Sprintf("%s: open %s", command, target), err)
	}
	if err := archive.WriteModsDir(writer, modsDir, now); err != nil {
		return "", nil, restorePointError(
			fmt.Sprintf("%s: write archive", command),
			errors.Join(err, writer.Abort()))
	}
	if err := writer.Commit(); err != nil {
		return "", nil, restorePointError(
			fmt.Sprintf("%s: commit %s", command, target), err)
	}

	result, deployErr := deployModsLocked(modsDeployRequest{
		root:      root,
		archive:   name,
		noRestart: true,
		now:       now,
		dryRun:    false,
		timeout:   timeout,
	}, client)
	if deployErr != nil {
		return "", nil, deployErr
	}
	return name, result.warnings, nil
}

func missingPlugins(checkpointRoot, liveMods string) ([]string, *ops.Error) {
	required, err := os.ReadFile(filepath.Join(checkpointRoot, "requiredDataFiles.json"))
	if err != nil {
		return nil, restorePointError("checkpoint restore", err)
	}
	wanted, err := archive.ParseRequiredDataFiles(required)
	if err != nil {
		return nil, restorePointError("checkpoint restore", err)
	}
	liveData, err := os.ReadFile(filepath.Join(liveMods, "plugins", "requiredDataFiles.json"))
	if err != nil {
		return nil, restorePointError("checkpoint restore", err)
	}
	live, err := archive.ParseRequiredDataFiles(liveData)
	if err != nil {
		return nil, restorePointError("checkpoint restore", err)
	}
	have := make(map[string]bool, len(live))
	for _, file := range live {
		have[file.Name] = true
	}
	var missing []string
	for _, file := range wanted {
		if !have[file.Name] {
			missing = append(missing, file.Name)
		}
	}
	return missing, nil
}
