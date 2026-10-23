package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

// controlData is the `data` object for start, stop and restart. The unit name
// is informational: rootctl owns the fixed unit and ops never passes it.
type controlData struct {
	Unit   string `json:"unit"`
	Action string `json:"action"`
	DryRun bool   `json:"dry_run,omitempty"`
}

func runControl(opts options, action systemd.ControlAction, args []string,
	client *systemd.Client, stdout, stderr io.Writer,
) int {
	if err := parseControlArgs(action, args); err != nil {
		return writeError(stdout, stderr, opts.json, err)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, string(action), mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	if !opts.dryRun {
		ctx, cancel := context.WithTimeout(context.Background(), opts.commandTimeout)
		err := client.Control(ctx, action)
		cancel()
		if err != nil {
			return writeSystemdFailure(stdout, stderr, opts.json, string(action), err)
		}
	}

	data := controlData{
		Unit:   systemd.UnitTES3MPService,
		Action: string(action),
		DryRun: opts.dryRun,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	verb := "ok"
	if data.DryRun {
		verb = "planned"
	}
	if _, err := fmt.Fprintf(stdout,
		"unit: %s\naction: %s\ndry_run: %t\nresult: %s\n",
		data.Unit, data.Action, data.DryRun, verb); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseControlArgs(action systemd.ControlAction, args []string) *ops.Error {
	fs := flag.NewFlagSet(string(action), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	if err := fs.Parse(args); err != nil {
		return ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: %v", action, err))
	}
	if fs.NArg() != 0 {
		return ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: unexpected argument %q", action, fs.Arg(0)))
	}
	return nil
}
