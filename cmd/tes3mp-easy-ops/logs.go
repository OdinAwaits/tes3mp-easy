package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

const maxLogTail = 10000

type logsData struct {
	Lines []string `json:"lines"`
}

func runLogs(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	tail, parseErr := parseLogsArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	logCtx, cancel := context.WithTimeout(context.Background(), opts.commandTimeout)
	lines, err := client.Logs(logCtx, systemd.UnitTES3MPService, tail)
	cancel()
	if err != nil {
		return writeSystemdFailure(stdout, stderr, opts.json, "logs", err)
	}
	if lines == nil {
		lines = []string{}
	}

	if opts.json {
		return writeJSONSuccess(stdout, stderr, logsData{Lines: lines})
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return writeOutputFailure(stderr, err)
		}
	}
	return int(ops.ExitOK)
}

func parseLogsArgs(args []string) (int, *ops.Error) {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	tail := -1
	fs.IntVar(&tail, "tail", -1, "")
	if err := fs.Parse(args); err != nil {
		return 0, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("logs: %v", err))
	}
	if fs.NArg() != 0 {
		return 0, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("logs: unexpected argument %q", fs.Arg(0)))
	}

	tailSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "tail" {
			tailSet = true
		}
	})
	switch {
	case !tailSet:
		return 0, ops.NewError(ops.ExitInvalidArgument,
			"logs: --tail is required")
	case tail < 1:
		return 0, ops.NewError(ops.ExitInvalidArgument,
			"logs: --tail must be positive")
	case tail > maxLogTail:
		return 0, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("logs: --tail must not exceed %d", maxLogTail))
	}

	return tail, nil
}
