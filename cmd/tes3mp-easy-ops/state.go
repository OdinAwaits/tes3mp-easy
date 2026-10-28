package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type stateDeployArgs struct {
	archive   string
	latest    bool
	noRestart bool
}

type stateDeployData struct {
	Archive         string `json:"archive"`
	ModsArchive     string `json:"mods_archive"`
	PredeployBackup string `json:"predeploy_backup"`
	Restarted       bool   `json:"restarted"`
}

func runState(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"state: subcommand is required; want import or deploy"))
	}

	switch args[0] {
	case "import":
		return runStateImport(opts, args[1:], stdout, stderr)
	case "deploy":
		return runStateDeploy(opts, args[1:], client, stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("state: unknown subcommand %q", args[0])))
	}
}

func runStateDeploy(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	deployArgs, parseErr := parseStateDeployArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "state deploy", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, deployErr := deployStateLocked(stateDeployRequest{
		root:      opts.root,
		archive:   deployArgs.archive,
		latest:    deployArgs.latest,
		noRestart: deployArgs.noRestart,
		now:       stateDeployNowFunc(),
		dryRun:    opts.dryRun,
		timeout:   opts.commandTimeout,
	}, client)
	if deployErr != nil {
		return writeError(stdout, stderr, opts.json, deployErr)
	}
	for _, warning := range result.warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: state deploy: %s\n", warning)
	}

	data := stateDeployData{
		Archive:         result.archive,
		ModsArchive:     result.mods,
		PredeployBackup: result.predeploy,
		Restarted:       result.restarted,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeStateDeployHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseStateDeployArgs(args []string) (stateDeployArgs, *ops.Error) {
	fs := flag.NewFlagSet("state deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	archiveName := fs.String("archive", "", "")
	latest := fs.Bool("latest", false, "")
	noRestart := fs.Bool("no-restart", false, "")

	if err := fs.Parse(args); err != nil {
		return stateDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("state deploy: %v", err))
	}
	if fs.NArg() != 0 {
		return stateDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("state deploy: unexpected argument %q", fs.Arg(0)))
	}

	archiveSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "archive" {
			archiveSet = true
		}
	})
	if archiveSet && *archiveName == "" {
		return stateDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"state deploy: --archive must not be empty")
	}
	if archiveSet && *latest {
		return stateDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"state deploy: --archive and --latest are mutually exclusive")
	}
	if !archiveSet && !*latest {
		return stateDeployArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"state deploy: one of --archive or --latest is required")
	}

	return stateDeployArgs{
		archive:   *archiveName,
		latest:    *latest,
		noRestart: *noRestart,
	}, nil
}

func writeStateDeployHuman(w io.Writer, data stateDeployData) error {
	restarted := "false"
	if data.Restarted {
		restarted = "true"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "archive: %s\n", data.Archive)
	fmt.Fprintf(&out, "mods_archive: %s\n", data.ModsArchive)
	fmt.Fprintf(&out, "predeploy_backup: %s\n", data.PredeployBackup)
	fmt.Fprintf(&out, "restarted: %s\n", restarted)
	_, err := io.WriteString(w, out.String())
	return err
}
