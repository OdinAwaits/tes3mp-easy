package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

type staffSetArgs struct {
	player string
	rank   int
}

type staffSetData struct {
	Player          string `json:"player"`
	Rank            int    `json:"rank"`
	RestartRequired bool   `json:"restart_required"`
}

type staffSetRequest struct {
	root    string
	player  string
	rank    int
	dryRun  bool
	writeFn func(string, []byte, os.FileMode) error
}

var staffWriteFileFunc = atomicfile.WriteFile

func runStaff(opts options, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"staff: subcommand is required; want set"))
	}
	switch args[0] {
	case "set":
		return runStaffSet(opts, args[1:], stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("staff: unknown subcommand %q", args[0])))
	}
}

func runStaffSet(opts options, args []string, stdout, stderr io.Writer) int {
	setArgs, parseErr := parseStaffSetArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}
	if err := pathguard.Base(setArgs.player); err != nil {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitUnsafePath,
				fmt.Sprintf("staff set: player %q: %v", setArgs.player, err)))
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "staff set", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, setErr := setStaffRankLocked(staffSetRequest{
		root:    opts.root,
		player:  setArgs.player,
		rank:    setArgs.rank,
		dryRun:  opts.dryRun,
		writeFn: staffWriteFileFunc,
	})
	if setErr != nil {
		return writeError(stdout, stderr, opts.json, setErr)
	}

	data := staffSetData{
		Player:          result.player,
		Rank:            result.rank,
		RestartRequired: true,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeStaffSetHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseStaffSetArgs(args []string) (staffSetArgs, *ops.Error) {
	fs := flag.NewFlagSet("staff set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	player := fs.String("player", "", "")
	rank := fs.Int("rank", 0, "")
	if err := fs.Parse(args); err != nil {
		return staffSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("staff set: %v", err))
	}
	if fs.NArg() != 0 {
		return staffSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("staff set: unexpected argument %q", fs.Arg(0)))
	}

	playerSet := false
	rankSet := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "player":
			playerSet = true
		case "rank":
			rankSet = true
		}
	})
	if !playerSet || *player == "" {
		return staffSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"staff set: --player is required and must not be empty")
	}
	if !rankSet {
		return staffSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"staff set: --rank is required")
	}
	if *rank < 0 || *rank > 3 {
		return staffSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"staff set: --rank must be in 0..3")
	}
	return staffSetArgs{player: *player, rank: *rank}, nil
}

type staffSetResult struct {
	player string
	rank   int
}

func setStaffRankLocked(req staffSetRequest) (staffSetResult, *ops.Error) {
	if req.player == "" || req.root == "" {
		return staffSetResult{}, ops.NewError(ops.ExitInternal,
			"staff set: player and root are required")
	}
	if req.writeFn == nil {
		req.writeFn = atomicfile.WriteFile
	}

	path := filepath.Join(req.root, "players", req.player+".json")
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return staffSetResult{}, ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("staff set: player %q not found", req.player))
	case err != nil:
		return staffSetResult{}, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("staff set: stat %s: %v", path, err))
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return staffSetResult{}, ops.NewError(ops.ExitUnsafePath,
			fmt.Sprintf("staff set: %s is not a regular file", path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return staffSetResult{}, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("staff set: read %s: %v", path, err))
	}

	updated, err := updateStaffJSON(data, req.rank)
	if err != nil {
		return staffSetResult{}, ops.NewError(ops.ExitConfigInvalid,
			fmt.Sprintf("staff set: %s: %v", path, err))
	}
	if !req.dryRun {
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o640
		}
		if err := req.writeFn(path, updated, mode); err != nil {
			return staffSetResult{}, ops.NewError(ops.ExitInternal,
				fmt.Sprintf("staff set: write %s: %v", path, err))
		}
	}
	return staffSetResult{player: req.player, rank: req.rank}, nil
}

func updateStaffJSON(data []byte, rank int) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty JSON object")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("invalid player JSON: %v", err)
	}
	if top == nil {
		return nil, fmt.Errorf("player JSON must be an object")
	}

	settings := make(map[string]json.RawMessage)
	if raw, ok := top["settings"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, fmt.Errorf("settings must be an object: %v", err)
		}
		if settings == nil {
			settings = make(map[string]json.RawMessage)
		}
	}
	settings["staffRank"] = json.RawMessage(strconv.Itoa(rank))
	encodedSettings, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("encode settings: %v", err)
	}
	top["settings"] = encodedSettings

	updated, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode player JSON: %v", err)
	}
	return append(updated, '\n'), nil
}

func writeStaffSetHuman(w io.Writer, data staffSetData) error {
	var out strings.Builder
	fmt.Fprintf(&out, "player: %s\nrank: %d\nrestart_required: true\n",
		data.Player, data.Rank)
	_, err := io.WriteString(w, out.String())
	return err
}
