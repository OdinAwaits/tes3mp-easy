package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/gameconfig"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

type configGetData struct {
	File     string         `json:"file"`
	Values   map[string]any `json:"values"`
	ReadOnly []string       `json:"read_only"`
}

type configSetData struct {
	File            string `json:"file"`
	Key             string `json:"key"`
	Value           any    `json:"value"`
	RestartRequired bool   `json:"restart_required"`
}

type configSetArgs struct {
	file  gameconfig.File
	key   string
	value string
}

var gameConfigNowFunc = func() time.Time { return time.Now().UTC() }

func runConfig(opts options, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				"config: subcommand is required; want get or set"))
	}
	switch args[0] {
	case "get":
		return runConfigGet(opts, args[1:], stdout, stderr)
	case "set":
		return runConfigSet(opts, args[1:], stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("config: unknown subcommand %q", args[0])))
	}
}

func runConfigGet(opts options, args []string, stdout, stderr io.Writer) int {
	file, parseErr := parseConfigGetArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}
	snapshot, err := gameconfig.Read(opts.root, file)
	if err != nil {
		return writeError(stdout, stderr, opts.json,
			configCommandError("config get", err))
	}
	if snapshot.Values == nil {
		snapshot.Values = map[string]any{}
	}
	if snapshot.ReadOnly == nil {
		snapshot.ReadOnly = []string{}
	}
	data := configGetData{
		File:     snapshot.File.String(),
		Values:   snapshot.Values,
		ReadOnly: snapshot.ReadOnly,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeConfigGetHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseConfigGetArgs(args []string) (gameconfig.File, *ops.Error) {
	fs := flag.NewFlagSet("config get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	raw := fs.String("file", "", "")
	if err := fs.Parse(args); err != nil {
		return "", ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("config get: %v", err))
	}
	if fs.NArg() != 0 {
		return "", ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("config get: unexpected argument %q", fs.Arg(0)))
	}
	fileSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "file" {
			fileSet = true
		}
	})
	if !fileSet || *raw == "" {
		return "", ops.NewError(ops.ExitInvalidArgument,
			"config get: --file is required")
	}
	file, err := gameconfig.ParseFile(*raw)
	if err != nil {
		return "", ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("config get: %v", err))
	}
	return file, nil
}

func runConfigSet(opts options, args []string, stdout, stderr io.Writer) int {
	setArgs, parseErr := parseConfigSetArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, "config set", mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	result, err := gameconfig.Set(opts.root, setArgs.file, setArgs.key,
		setArgs.value, gameConfigNowFunc(), opts.dryRun)
	if err != nil {
		return writeError(stdout, stderr, opts.json,
			configCommandError("config set", err))
	}
	data := configSetData{
		File:            result.File.String(),
		Key:             result.Key,
		Value:           result.Value,
		RestartRequired: true,
	}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeConfigSetHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseConfigSetArgs(args []string) (configSetArgs, *ops.Error) {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	rawFile := fs.String("file", "", "")
	key := fs.String("key", "", "")
	value := fs.String("value", "", "")
	if err := fs.Parse(args); err != nil {
		return configSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("config set: %v", err))
	}
	if fs.NArg() != 0 {
		return configSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("config set: unexpected argument %q", fs.Arg(0)))
	}

	fileSet := false
	keySet := false
	valueSet := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "file":
			fileSet = true
		case "key":
			keySet = true
		case "value":
			valueSet = true
		}
	})
	if !fileSet || *rawFile == "" {
		return configSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"config set: --file is required")
	}
	if !keySet || *key == "" {
		return configSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"config set: --key is required and must not be empty")
	}
	if !valueSet {
		return configSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			"config set: --value is required; use --value= for an empty string")
	}
	file, err := gameconfig.ParseFile(*rawFile)
	if err != nil {
		return configSetArgs{}, ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("config set: %v", err))
	}
	return configSetArgs{file: file, key: *key, value: *value}, nil
}

func configCommandError(command string, err error) *ops.Error {
	var invalid *gameconfig.InvalidError
	if errors.As(err, &invalid) {
		result := ops.NewError(ops.ExitConfigInvalid,
			fmt.Sprintf("%s: %v", command, err))
		details := map[string]any{}
		if invalid.File != "" {
			details["file"] = invalid.File.String()
		}
		if invalid.Key != "" {
			details["key"] = invalid.Key
		}
		if len(details) > 0 {
			result.Details = details
		}
		return result
	}
	if errors.Is(err, fs.ErrNotExist) {
		return ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("%s: %v", command, err))
	}
	return ops.NewError(ops.ExitInternal,
		fmt.Sprintf("%s: %v", command, err))
}

func writeConfigGetHuman(w io.Writer, data configGetData) error {
	keys := make([]string, 0, len(data.Values))
	for key := range data.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	fmt.Fprintf(&out, "file: %s\nvalues:\n", data.File)
	for _, key := range keys {
		fmt.Fprintf(&out, "  %s=%s\n", key, humanConfigValue(data.Values[key]))
	}
	readOnly := "-"
	if len(data.ReadOnly) > 0 {
		readOnly = strings.Join(data.ReadOnly, ", ")
	}
	fmt.Fprintf(&out, "read_only: %s\n", readOnly)
	_, err := io.WriteString(w, out.String())
	return err
}

func humanConfigValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}

func writeConfigSetHuman(w io.Writer, data configSetData) error {
	var out strings.Builder
	fmt.Fprintf(&out, "file: %s\nkey: %s\nvalue: %s\nrestart_required: true\n",
		data.File, data.Key, humanConfigValue(data.Value))
	_, err := io.WriteString(w, out.String())
	return err
}
