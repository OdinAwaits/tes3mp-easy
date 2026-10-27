package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type checkpointCreateArgs struct {
	label     string
	reason    string
	ifRunning bool
}

type checkpointBaseData struct {
	Name       string `json:"name"`
	SHA256     string `json:"sha256"`
	ModsSHA256 string `json:"mods_sha256"`
}

type checkpointCreateData struct {
	Archive  string              `json:"archive,omitempty"`
	Size     *int64              `json:"size,omitempty"`
	SHA256   *string             `json:"sha256,omitempty"`
	Reason   string              `json:"reason"`
	Label    *string             `json:"label"`
	Base     *checkpointBaseData `json:"base,omitempty"`
	Players  *int                `json:"players,omitempty"`
	Skipped  bool                `json:"skipped"`
	Pruned   []string            `json:"pruned"`
	Warnings []string            `json:"warnings"`
}

type checkpointPinData struct {
	Archive string `json:"archive"`
	Pinned  bool   `json:"pinned"`
}

func runCheckpoint(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"checkpoint: subcommand is required; want create, list, pin, unpin, restore or rm"))
	}
	switch args[0] {
	case "create":
		return runCheckpointCreate(opts, args[1:], client, stdout, stderr)
	case "list":
		return runCheckpointList(opts, args[1:], stdout, stderr)
	case "pin", "unpin":
		return runCheckpointPin(opts, args[0], args[1:], stdout, stderr)
	case "restore":
		return runCheckpointRestore(opts, args[1:], client, stdout, stderr)
	case "rm":
		return runRestorePointRemove(opts, "checkpoint rm", restorepoint.KindCheckpoint,
			args[1:], stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("checkpoint: unknown subcommand %q", args[0])))
	}
}

func runCheckpointCreate(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	createArgs, parseErr := parseCheckpointCreateArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "checkpoint create", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	if createArgs.ifRunning {
		active, activeErr := checkpointServiceActive(client, opts.commandTimeout)
		if activeErr != nil {
			return writeError(stdout, stderr, opts.json, activeErr)
		}
		if !active {
			data := checkpointCreateData{
				Reason:   "not_running",
				Skipped:  true,
				Pruned:   []string{},
				Warnings: []string{},
			}
			return writeCheckpointCreate(opts, data, stdout, stderr)
		}
	}

	result, createErr := createCheckpointLocked(opts.root, time.Now().UTC(),
		createArgs.reason, createArgs.label, opts.dryRun)
	if createErr != nil {
		return writeError(stdout, stderr, opts.json, createErr)
	}
	for _, warning := range result.warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: checkpoint create: %s\n", warning)
	}

	data := checkpointCreateData{
		Archive:  result.name,
		Reason:   createArgs.reason,
		Skipped:  false,
		Pruned:   nonNil(result.pruned),
		Warnings: nonNil(result.warnings),
	}
	if result.manifest.Format != "" {
		size := result.size
		sha := result.sha256
		players := 0
		if result.manifest.Players != nil {
			players = *result.manifest.Players
		}
		data.Size = &size
		data.SHA256 = &sha
		data.Reason = result.manifest.Reason
		data.Label = optionalLabel(result.manifest.Label)
		data.Players = &players
		if result.manifest.Base != nil {
			data.Base = &checkpointBaseData{
				Name:       result.baseName,
				SHA256:     result.baseSHA,
				ModsSHA256: result.manifest.Base.ModsSHA256,
			}
		}
	}
	return writeCheckpointCreate(opts, data, stdout, stderr)
}

func parseCheckpointCreateArgs(args []string) (checkpointCreateArgs, *ops.Error) {
	fs := flag.NewFlagSet("checkpoint create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	label := fs.String("label", "", "")
	reason := fs.String("reason", "manual", "")
	ifRunning := fs.Bool("if-running", false, "")
	if err := fs.Parse(args); err != nil {
		return checkpointCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("checkpoint create: %v", err))
	}
	if fs.NArg() != 0 {
		return checkpointCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("checkpoint create: unexpected argument %q", fs.Arg(0)))
	}
	if len(*label) > 64 {
		return checkpointCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"checkpoint create: --label must not exceed 64 characters")
	}
	switch *reason {
	case "timer", "manual", "predeploy":
	default:
		return checkpointCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("checkpoint create: unsupported reason %q", *reason))
	}
	return checkpointCreateArgs{
		label: *label, reason: *reason, ifRunning: *ifRunning,
	}, nil
}

func createCheckpointLocked(root string, now time.Time, reason, label string,
	dryRun bool,
) (checkpointCreateResult, *ops.Error) {
	const command = "checkpoint create"
	baseName, baseManifest, baseSHA, warnings, baseErr :=
		selectCheckpointBase(root, now, dryRun)
	if baseErr != nil {
		return checkpointCreateResult{}, baseErr
	}

	dir, dirErr := ensureRestorePointDir(root, restorepoint.KindCheckpoint, dryRun)
	if dirErr != nil {
		return checkpointCreateResult{}, dirErr
	}
	spacePath := dir
	if dryRun {
		spacePath = root
	}
	spaceWarnings, spaceErr := checkRestorePointSpace(command, spacePath)
	if spaceErr != nil {
		return checkpointCreateResult{}, spaceErr
	}
	warnings = append(warnings, spaceWarnings...)

	nameKind := restorepoint.KindCheckpoint
	if reason == "predeploy" {
		nameKind = restorepoint.KindPredeploy
	}
	name, err := restorepoint.NextName(dir, nameKind, now)
	if err != nil {
		return checkpointCreateResult{}, restorePointError(command, err)
	}
	if baseManifest == nil {
		return checkpointCreateResult{
			name: name, baseName: baseName, warnings: warnings,
		}, nil
	}

	target := filepath.Join(dir, name)
	counter := &countingWriter{}
	var destination io.Writer = io.Discard
	var writer *atomicfile.Writer
	if !dryRun {
		writer, err = atomicfile.New(target, 0o640)
		if err != nil {
			return checkpointCreateResult{}, restorePointError(
				fmt.Sprintf("%s: open %s", command, target), err)
		}
		destination = writer
	}

	manifest, writeErr := restorepoint.WriteCheckpoint(
		io.MultiWriter(destination, counter), restorepoint.CheckpointOptions{
			Root:              root,
			Now:               now,
			Reason:            reason,
			Label:             label,
			Base:              *baseManifest,
			BaseName:          baseName,
			BaseArchiveSHA256: baseSHA,
		})
	if writeErr != nil {
		if writer != nil {
			writeErr = errors.Join(writeErr, writer.Abort())
		}
		return checkpointCreateResult{}, restorePointError(
			fmt.Sprintf("%s: write archive", command), writeErr)
	}
	limit := restorepoint.Limits(restorepoint.KindCheckpoint)
	if counter.n > limit.CompressedSize {
		err := fmt.Errorf("%w: compressed checkpoint exceeds %d bytes",
			restorepoint.ErrLimit, limit.CompressedSize)
		if writer != nil {
			err = errors.Join(err, writer.Abort())
		}
		return checkpointCreateResult{}, restorePointError(
			fmt.Sprintf("%s: write archive", command), err)
	}
	if writer != nil {
		if err := writer.Commit(); err != nil {
			return checkpointCreateResult{}, restorePointError(
				fmt.Sprintf("%s: commit %s", command, target), err)
		}
	}

	result := checkpointCreateResult{
		name:     name,
		size:     counter.n,
		manifest: manifest,
		baseName: baseName,
		baseSHA:  baseSHA,
		warnings: warnings,
	}
	if !dryRun {
		sum, err := backupSHA256(target)
		if err != nil {
			return checkpointCreateResult{}, restorePointError(
				fmt.Sprintf("%s: sha256 %s", command, target), err)
		}
		result.sha256 = sum
	}
	if !dryRun {
		applyRestorePointRetention(root, now, &result.pruned, &result.warnings)
	}
	return result, nil
}

func selectCheckpointBase(root string, now time.Time, dryRun bool,
) (string, *restorepoint.Manifest, string, []string, *ops.Error) {
	const command = "checkpoint create"
	liveDigest, err := restorepoint.LiveModsDigest(root)
	if err != nil {
		return "", nil, "", nil, restorePointError(
			fmt.Sprintf("%s: scan mods", command), err)
	}
	snapshots, err := restorepoint.ListSnapshots(root)
	if err != nil {
		return "", nil, "", nil, restorePointError(
			fmt.Sprintf("%s: list snapshots", command), err)
	}
	for _, snapshot := range snapshots {
		if snapshot.ModsSHA256 != liveDigest {
			continue
		}
		path := filepath.Join(restorepoint.Dir(root, restorepoint.KindSnapshot),
			snapshot.Name)
		manifest, err := restorepoint.ReadManifest(path)
		if err != nil {
			return "", nil, "", nil, restorePointError(
				fmt.Sprintf("%s: read base %s", command, snapshot.Name), err)
		}
		sum, err := backupSHA256(path)
		if err != nil {
			return "", nil, "", nil, restorePointError(
				fmt.Sprintf("%s: sha256 %s", command, path), err)
		}
		return snapshot.Name, &manifest, sum, nil, nil
	}

	if dryRun {
		return "", nil, "", []string{
			"no matching base snapshot; checkpoint create would create snapshot reason=base",
		}, nil
	}
	created, createErr := createSnapshotLocked(root, now, "base", "", false)
	if createErr != nil {
		return "", nil, "", nil, createErr
	}
	manifest := created.manifest
	return created.name, &manifest, created.sha256, nil, nil
}

func checkpointServiceActive(client *systemd.Client, timeout time.Duration,
) (bool, *ops.Error) {
	if client == nil {
		return false, ops.NewError(ops.ExitInternal,
			"checkpoint create: systemd client is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	active, err := client.IsActive(ctx, systemd.UnitTES3MPService)
	if err != nil {
		return false, ops.NewError(ops.ExitServiceUnavailable,
			fmt.Sprintf("checkpoint create: check %s: %v",
				systemd.UnitTES3MPService, err))
	}
	return active, nil
}

func runCheckpointPin(opts options, action string, args []string,
	stdout, stderr io.Writer,
) int {
	command := "checkpoint " + action
	if len(args) != 1 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("%s: archive name is required", command)))
	}
	name := args[0]
	if err := pathguard.Base(name); err != nil {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitUnsafePath, fmt.Sprintf("%s: %v", command, err)))
	}
	if err := restorepoint.ValidateName(restorepoint.KindCheckpoint, name); err != nil {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument, fmt.Sprintf("%s: %v", command, err)))
	}

	held, lockErr := acquireOpsLock(opts.root, command, lock.Exclusive,
		opts.lockTimeout, false)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	pinned := action == "pin"
	var err error
	if pinned {
		err = restorepoint.Pin(opts.root, name, time.Now().UTC())
	} else {
		err = restorepoint.Unpin(opts.root, name)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitNotFound,
					fmt.Sprintf("%s: %s does not exist", command, name)))
		}
		return writeError(stdout, stderr, opts.json,
			restorePointError(command, err))
	}

	data := checkpointPinData{Archive: name, Pinned: pinned}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if _, err := fmt.Fprintf(stdout, "archive: %s\npinned: %t\n",
		data.Archive, data.Pinned); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func writeCheckpointCreate(opts options, data checkpointCreateData,
	stdout, stderr io.Writer,
) int {
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if data.Skipped {
		if _, err := fmt.Fprintf(stdout, "skipped: true\nreason: %s\n",
			data.Reason); err != nil {
			return writeOutputFailure(stderr, err)
		}
		return int(ops.ExitOK)
	}
	label := "-"
	if data.Label != nil {
		label = *data.Label
	}
	base := "-"
	if data.Base != nil {
		base = data.Base.Name
	}
	size := int64(0)
	if data.Size != nil {
		size = *data.Size
	}
	sha := ""
	if data.SHA256 != nil {
		sha = *data.SHA256
	}
	players := 0
	if data.Players != nil {
		players = *data.Players
	}
	if _, err := fmt.Fprintf(stdout,
		"archive: %s\nsize: %d\nsha256: %s\nreason: %s\nlabel: %s\n"+
			"base: %s\nplayers: %d\npruned: %d\nwarnings: %d\n",
		data.Archive, size, sha, data.Reason, label, base, players,
		len(data.Pruned), len(data.Warnings)); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}
