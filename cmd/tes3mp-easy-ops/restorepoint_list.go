package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
)

type restorePointListOptions struct {
	limit      int
	before     string
	beforeTime time.Time
}

func parseRestorePointListArgs(command string, args []string,
	kind restorepoint.Kind,
) (restorePointListOptions, *ops.Error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	limit := fs.Int("limit", defaultBackupsLimit, "")
	before := fs.String("before", "", "")
	if err := fs.Parse(args); err != nil {
		return restorePointListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: %v", command, err))
	}
	if fs.NArg() != 0 {
		return restorePointListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: unexpected argument %q", command, fs.Arg(0)))
	}
	if *limit < 1 || *limit > maxBackupsLimit {
		return restorePointListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: --limit must be between 1 and %d",
				command, maxBackupsLimit))
	}
	if *before == "" {
		return restorePointListOptions{limit: *limit}, nil
	}
	if err := pathguard.Base(*before); err != nil {
		return restorePointListOptions{}, ops.NewError(ops.ExitUnsafePath,
			fmt.Sprintf("%s: --before: %v", command, err))
	}
	parsed, err := restorepoint.ParseName(*before)
	if err != nil {
		return restorePointListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: --before: %v", command, err))
	}
	matches := parsed.Kind == kind
	if kind == restorepoint.KindCheckpoint {
		matches = parsed.Kind.IsCheckpoint()
	}
	if !matches {
		return restorePointListOptions{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: --before: archive name %q is not a %s archive",
				command, *before, kind))
	}
	return restorePointListOptions{
		limit:      *limit,
		before:     *before,
		beforeTime: parsed.Time,
	}, nil
}

func pageSnapshotItems(items []snapshotListItem, opts restorePointListOptions,
) ([]snapshotListItem, *string) {
	start := pageStart(len(items), opts,
		func(i int) (string, time.Time) {
			at, _ := time.Parse(time.RFC3339, items[i].Mtime)
			return items[i].Name, at
		})
	if start < 0 {
		return []snapshotListItem{}, nil
	}
	items = items[start:]
	var next *string
	if len(items) > opts.limit {
		items = items[:opts.limit]
	}
	if len(items) == opts.limit {
		name := items[len(items)-1].Name
		next = &name
	}
	if items == nil {
		items = []snapshotListItem{}
	}
	return items, next
}

func pageStart(length int, opts restorePointListOptions,
	entry func(int) (string, time.Time),
) int {
	if opts.before == "" {
		return 0
	}
	for i := 0; i < length; i++ {
		name, _ := entry(i)
		if name == opts.before {
			return i + 1
		}
	}
	for i := 0; i < length; i++ {
		name, at := entry(i)
		if at.Before(opts.beforeTime) ||
			(at.Equal(opts.beforeTime) && name < opts.before) {
			return i
		}
	}
	return -1
}

func optionalLabel(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
