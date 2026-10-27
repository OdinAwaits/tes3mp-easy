package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
)

type restorePointRemoveArgs struct {
	names  []string
	force  bool
	dryRun bool
}

func runRestorePointRemove(opts options, command string, kind restorepoint.Kind,
	args []string, stdout, stderr io.Writer,
) int {
	removeArgs, parseErr := parseRestorePointRemoveArgs(command, args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}
	dryRun := opts.dryRun || removeArgs.dryRun

	mode := lock.Exclusive
	if dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, command, mode,
		opts.lockTimeout, dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, err := restorepoint.Remove(restorepoint.RemoveOptions{
		Root:   opts.root,
		Kind:   kind,
		Names:  removeArgs.names,
		Force:  removeArgs.force,
		DryRun: dryRun,
	})
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitNotFound, fmt.Sprintf("%s: %v", command, err)))
		case errors.Is(err, restorepoint.ErrInUse):
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitArchiveInUse,
					fmt.Sprintf("%s: %v", command, err)))
		case errors.Is(err, restorepoint.ErrInvalid),
			errors.Is(err, pathguard.ErrUnsafePath):
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitInvalidArgument,
					fmt.Sprintf("%s: %v", command, err)))
		default:
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitInternal, fmt.Sprintf("%s: %v", command, err)))
		}
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: %s: %s\n", command, warning)
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, result)
	}
	if err := writeRestorePointRemoveHuman(stdout, result); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseRestorePointRemoveArgs(command string, args []string,
) (restorePointRemoveArgs, *ops.Error) {
	var parsed restorePointRemoveArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			parsed.names = append(parsed.names, args[i+1:]...)
			return finishRemoveArgs(command, parsed)
		case arg == "--force":
			parsed.force = true
		case arg == "--dry-run":
			parsed.dryRun = true
		case strings.HasPrefix(arg, "-"):
			return restorePointRemoveArgs{}, ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("%s: unknown flag %q", command, arg))
		default:
			parsed.names = append(parsed.names, arg)
		}
	}
	return finishRemoveArgs(command, parsed)
}

func finishRemoveArgs(command string, parsed restorePointRemoveArgs,
) (restorePointRemoveArgs, *ops.Error) {
	if len(parsed.names) == 0 {
		return restorePointRemoveArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("%s: at least one archive name is required", command))
	}
	for _, name := range parsed.names {
		if err := pathguard.Base(name); err != nil {
			return restorePointRemoveArgs{}, ops.NewError(ops.ExitUnsafePath,
				fmt.Sprintf("%s: %v", command, err))
		}
	}
	return parsed, nil
}

func writeRestorePointRemoveHuman(w io.Writer, data restorepoint.RemoveResult) error {
	if _, err := fmt.Fprintf(w, "removed: %d\nfreed: %d\nskipped: %d\nwarnings: %d\n",
		len(data.Removed), data.Freed, len(data.Skipped), len(data.Warnings)); err != nil {
		return err
	}
	for _, name := range data.Removed {
		if _, err := fmt.Fprintf(w, "removed_archive: %s\n", name); err != nil {
			return err
		}
	}
	for _, skipped := range data.Skipped {
		if _, err := fmt.Fprintf(w, "skipped_archive: %s\t%s\n",
			skipped.Name, skipped.Reason); err != nil {
			return err
		}
	}
	return nil
}
