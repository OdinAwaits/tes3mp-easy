package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type snapshotCreateArgs struct {
	label  string
	reason string
}

type snapshotCreateData struct {
	Archive     string   `json:"archive"`
	Size        int64    `json:"size"`
	SHA256      string   `json:"sha256"`
	Reason      string   `json:"reason"`
	Label       *string  `json:"label"`
	ModsArchive string   `json:"mods_archive"`
	Pruned      []string `json:"pruned"`
	Warnings    []string `json:"warnings"`
}

func runSnapshot(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"snapshot: subcommand is required; want create, list, restore or rm"))
	}
	switch args[0] {
	case "create":
		return runSnapshotCreate(opts, args[1:], stdout, stderr)
	case "list":
		return runSnapshotList(opts, args[1:], stdout, stderr)
	case "restore":
		return runSnapshotRestore(opts, args[1:], client, stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("snapshot: unknown subcommand %q", args[0])))
	}
}

func runSnapshotCreate(opts options, args []string, stdout, stderr io.Writer) int {
	createArgs, parseErr := parseSnapshotCreateArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "snapshot create", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, createErr := createSnapshotLocked(opts.root, time.Now().UTC(),
		createArgs.reason, createArgs.label, opts.dryRun)
	if createErr != nil {
		return writeError(stdout, stderr, opts.json, createErr)
	}
	for _, warning := range result.warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: snapshot create: %s\n", warning)
	}

	data := snapshotCreateData{
		Archive:     result.name,
		Size:        result.size,
		SHA256:      result.sha256,
		Reason:      result.manifest.Reason,
		Label:       optionalLabel(result.manifest.Label),
		ModsArchive: result.mods,
		Pruned:      []string{},
		Warnings:    nonNil(result.warnings),
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeSnapshotCreateHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseSnapshotCreateArgs(args []string) (snapshotCreateArgs, *ops.Error) {
	fs := flag.NewFlagSet("snapshot create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	label := fs.String("label", "", "")
	reason := fs.String("reason", "manual", "")
	if err := fs.Parse(args); err != nil {
		return snapshotCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("snapshot create: %v", err))
	}
	if fs.NArg() != 0 {
		return snapshotCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("snapshot create: unexpected argument %q", fs.Arg(0)))
	}
	if len(*label) > 64 {
		return snapshotCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"snapshot create: --label must not exceed 64 characters")
	}
	switch *reason {
	case "manual", "mods", "base":
	default:
		return snapshotCreateArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("snapshot create: unsupported reason %q", *reason))
	}
	return snapshotCreateArgs{label: *label, reason: *reason}, nil
}

func writeSnapshotCreateHuman(w io.Writer, data snapshotCreateData) error {
	label := "-"
	if data.Label != nil {
		label = *data.Label
	}
	_, err := fmt.Fprintf(w,
		"archive: %s\nsize: %d\nsha256: %s\nreason: %s\nlabel: %s\n"+
			"mods_archive: %s\npruned: -\nwarnings: %d\n",
		data.Archive, data.Size, data.SHA256, data.Reason, label,
		data.ModsArchive, len(data.Warnings))
	return err
}
