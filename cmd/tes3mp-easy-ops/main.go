// Command tes3mp-easy-ops is the administrative CLI for tes3mp-easy.
//
// This step implements the process contract, the read-only doctor, status,
// logs, backups and config get commands, the mutating mods/state import and
// deploy, staff set and config set commands, and fixed start/stop/restart
// through tes3mp-easy-rootctl from docs/v2/04-ops.md.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/contract"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

const (
	defaultRoot           = "/tes3mp-easy"
	defaultLockTimeout    = 30 * time.Second
	defaultCommandTimeout = 2 * time.Minute
)

const usage = `Usage: tes3mp-easy-ops [global flags] <command> [command flags]

Global flags:
  --root <path>              data root (default /tes3mp-easy)
  --json                     write exactly one JSON object to stdout
  --dry-run                  validate and print the plan, change nothing
  --lock-timeout <duration>  wait for the ops lock (default 30s)
  --command-timeout <duration>
                             timeout for one systemctl/journalctl call (default 2m)

Commands:
  doctor
  status [--unit tes3mp.service|tes3mp-easy-backup.timer]
  start
  stop
  restart
  logs --tail <n>
  backups list --type mods|state [--limit <n>] [--before <name>] [--sha256]
  backups create --type state [--manual] [--if-running]
  backups prune [--dry-run]
  mods import --file <path> [--name <archive>]
  mods deploy (--current|--latest|--archive <name>) [--no-restart]
  state import --file <path> [--name <archive>]
  state deploy (--latest|--archive <name>) [--no-restart]
  staff set --player <name> --rank 0..3
  config get --file server|config|banlist
  config set --file server|config|banlist --key <key> --value <value>

start, stop and restart call sudo -n /usr/local/libexec/tes3mp-easy-rootctl
and take the ops lock; --dry-run only prints the plan.
See docs/v2/04-ops.md for the target command set.
`

// knownCommands distinguishes a known but unfinished command from a typo.
var knownCommands = map[string]struct{}{
	"doctor":  {},
	"status":  {},
	"start":   {},
	"stop":    {},
	"restart": {},
	"logs":    {},
	"backups": {},
	"mods":    {},
	"state":   {},
	"staff":   {},
	"config":  {},
}

type options struct {
	root           string
	json           bool
	dryRun         bool
	lockTimeout    time.Duration
	commandTimeout time.Duration
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithClient(args, stdout, stderr,
		systemd.New(systemd.ExecRunner{}))
}

func runWithClient(args []string, stdout, stderr io.Writer, client *systemd.Client) int {
	opts, rest, help, parseErr := parseGlobal(args)
	if help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	if len(rest) == 0 {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument, "command is required"))
	}

	command := rest[0]
	if strings.HasPrefix(command, "-") {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("unknown flag %q", command)))
	}
	if _, ok := knownCommands[command]; !ok {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInvalidArgument,
				fmt.Sprintf("unknown command %q", command)))
	}

	switch command {
	case "doctor":
		return runDoctor(opts, rest[1:], client, stdout, stderr)
	case "status":
		return runStatus(opts, rest[1:], client, stdout, stderr)
	case "start":
		return runControl(opts, systemd.ActionStart, rest[1:], client, stdout, stderr)
	case "stop":
		return runControl(opts, systemd.ActionStop, rest[1:], client, stdout, stderr)
	case "restart":
		return runControl(opts, systemd.ActionRestart, rest[1:], client, stdout, stderr)
	case "logs":
		return runLogs(opts, rest[1:], client, stdout, stderr)
	case "backups":
		return runBackups(opts, rest[1:], client, stdout, stderr)
	case "mods":
		return runMods(opts, rest[1:], client, stdout, stderr)
	case "state":
		return runState(opts, rest[1:], client, stdout, stderr)
	case "staff":
		return runStaff(opts, rest[1:], stdout, stderr)
	case "config":
		return runConfig(opts, rest[1:], stdout, stderr)
	default:
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitInternal,
				fmt.Sprintf("command %q is not implemented yet", command)))
	}
}

func writeError(stdout, stderr io.Writer, jsonMode bool, err *ops.Error) int {
	if jsonMode {
		if writeErr := contract.Write(stdout, err.Envelope()); writeErr != nil {
			fmt.Fprintf(stderr, "tes3mp-easy-ops: write json: %v\n", writeErr)
			return int(ops.ExitInternal)
		}
		return int(err.Exit)
	}

	fmt.Fprintf(stderr, "tes3mp-easy-ops: %s\n", err.Message)
	return int(err.Exit)
}

func parseGlobal(args []string) (options, []string, bool, *ops.Error) {
	globalArgs, rest, help := splitGlobal(args)
	if help {
		return options{}, rest, true, nil
	}

	opts := options{
		root:           defaultRoot,
		lockTimeout:    defaultLockTimeout,
		commandTimeout: defaultCommandTimeout,
	}

	fs := flag.NewFlagSet("tes3mp-easy-ops", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	root := fs.String("root", defaultRoot, "")
	jsonMode := fs.Bool("json", false, "")
	dryRun := fs.Bool("dry-run", false, "")
	lockTimeout := fs.Duration("lock-timeout", defaultLockTimeout, "")
	commandTimeout := fs.Duration("command-timeout", defaultCommandTimeout, "")

	if err := fs.Parse(globalArgs); err != nil {
		// The output mode must still be known when parsing fails, otherwise a
		// malformed --lock-timeout followed by --json would print human text.
		opts.json = wantsJSON(args)
		return opts, rest, false, ops.NewError(ops.ExitInvalidArgument, err.Error())
	}

	opts = options{
		root:           *root,
		json:           *jsonMode,
		dryRun:         *dryRun,
		lockTimeout:    *lockTimeout,
		commandTimeout: *commandTimeout,
	}

	switch {
	case opts.root == "":
		return opts, rest, false, ops.NewError(
			ops.ExitInvalidArgument, "--root must not be empty")
	case opts.lockTimeout < 0:
		return opts, rest, false, ops.NewError(
			ops.ExitInvalidArgument, "--lock-timeout must not be negative")
	case opts.commandTimeout <= 0:
		return opts, rest, false, ops.NewError(
			ops.ExitInvalidArgument, "--command-timeout must be positive")
	}

	return opts, rest, false, nil
}

// splitGlobal extracts global flags regardless of whether they appear before
// or after the command. The remaining arguments stay untouched for the
// command parser.
func splitGlobal(args []string) (global, rest []string, help bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if isHelpFlag(arg) {
			help = true
			continue
		}

		name, hasValue, ok := flagToken(arg)
		if !ok {
			rest = append(rest, arg)
			continue
		}

		known, takesValue := globalFlag(name)
		if !known {
			rest = append(rest, arg)
			continue
		}

		global = append(global, arg)
		if takesValue && !hasValue && i+1 < len(args) {
			i++
			global = append(global, args[i])
		}
	}

	return global, rest, help
}

func globalFlag(name string) (known, takesValue bool) {
	switch name {
	case "root":
		return true, true
	case "lock-timeout", "command-timeout":
		return true, true
	case "json", "dry-run":
		return true, false
	default:
		return false, false
	}
}

func flagToken(arg string) (name string, hasValue, ok bool) {
	if arg == "" || arg == "-" || arg == "--" || !strings.HasPrefix(arg, "-") {
		return "", false, false
	}

	trimmed := strings.TrimPrefix(arg, "-")
	trimmed = strings.TrimPrefix(trimmed, "-")
	name, _, hasValue = strings.Cut(trimmed, "=")
	if name == "" {
		return "", false, false
	}
	return name, hasValue, true
}

func isHelpFlag(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "-help"
}

// wantsJSON is a fallback for parse errors. It mirrors the global flag grammar
// closely enough to decide where a failed invocation should write its error.
func wantsJSON(args []string) bool {
	jsonMode := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if isHelpFlag(arg) {
			continue
		}

		name, hasValue, ok := flagToken(arg)
		if !ok {
			continue
		}
		known, takesValue := globalFlag(name)
		if !known {
			continue
		}
		if takesValue && !hasValue {
			i++
			continue
		}
		if name != "json" {
			continue
		}
		if !hasValue {
			jsonMode = true
			continue
		}

		_, value, _ := strings.Cut(arg, "=")
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			jsonMode = true
			continue
		}
		jsonMode = parsed
	}
	return jsonMode
}
