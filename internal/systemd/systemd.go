// Package systemd is the thin adapter over systemctl, journalctl and the fixed
// rootctl helper described in docs/v2/04-ops.md §9.
//
// The package never invokes a shell: commands and arguments stay separate all
// the way to exec.CommandContext. Command output is returned as bytes; failures
// carry the command, exit status and captured stderr in CommandError.
package systemd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Unit names and the installed rootctl path are fixed by
// docs/v2/02-architecture.md and docs/v2/04-ops.md.
const (
	UnitTES3MPService = "tes3mp.service"
	UnitPanelService  = "tes3mp-panel.service"
	UnitBackupService = "tes3mp-easy-backup.service"
	UnitBackupTimer   = "tes3mp-easy-backup.timer"

	RootctlPath = "/usr/local/libexec/tes3mp-easy-rootctl"
)

// ShowProperties is the property set used by status and doctor.
const ShowProperties = "ActiveState,SubState,ActiveEnterTimestamp,LastTriggerUSec,LoadState,UnitFileState"

const (
	showPropertyArg  = "--property=" + ShowProperties
	showTimestampArg = "--timestamp=utc"
)

// waitDelay bounds how long a child may keep stdout/stderr open after the
// context is canceled or the process exits.
const waitDelay = 5 * time.Second

// Errors for caller-side validation. Commands map these to invalid_argument
// before reaching systemd; the adapter itself never guesses.
var (
	ErrInvalidUnit   = errors.New("invalid unit")
	ErrInvalidAction = errors.New("invalid control action")
)

// Runner executes one command and returns its stdout. Implementations must not
// invoke a shell. ExecRunner is the production implementation; systemdtest
// provides a recording fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// CommandError describes a command that could not be executed or exited with a
// non-zero status. Stderr is captured separately so the caller can show a
// useful message without parsing stdout.
type CommandError struct {
	Name     string
	Args     []string
	ExitCode int
	Stderr   string
	Err      error
}

// Error implements error.
func (e *CommandError) Error() string {
	if e == nil {
		return "<nil>"
	}

	command := strings.TrimSpace(strings.Join(append([]string{e.Name}, e.Args...), " "))
	if command == "" {
		command = "command"
	}

	message := "failed"
	if e.Err != nil {
		message = e.Err.Error()
	}
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		message += ": " + stderr
	}
	return command + ": " + message
}

// Unwrap exposes the underlying exec or context error.
func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ExecRunner runs a command with exec.CommandContext. No shell is involved.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("systemd: command name is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = waitDelay

	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}

	cause := err
	if ctxErr := ctx.Err(); ctxErr != nil {
		cause = ctxErr
	}

	commandErr := &CommandError{
		Name:   name,
		Args:   append([]string(nil), args...),
		Stderr: stderr.String(),
		Err:    cause,
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		commandErr.ExitCode = exitErr.ExitCode()
	}
	return stdout.Bytes(), commandErr
}

// Client converts typed operations into systemctl, journalctl and rootctl
// calls. It holds no state except the Runner and is safe for concurrent use if
// the Runner is.
type Client struct {
	runner Runner
}

// New returns a client using runner. A nil runner is rejected on the first
// operation instead of silently falling back to the real system.
func New(runner Runner) *Client {
	return &Client{runner: runner}
}

// Properties is the parsed output of systemctl show.
type Properties map[string]string

// Get returns the property value or an empty string when it is absent.
func (p Properties) Get(name string) string {
	return p[name]
}

// Show reads the fixed property set for unit. Timestamps are requested in UTC
// so status can turn them into RFC3339 without guessing the host time zone.
func (c *Client) Show(ctx context.Context, unit string) (Properties, error) {
	if err := ValidateUnit(unit); err != nil {
		return nil, err
	}

	out, err := c.run(ctx, "systemctl", "show", unit, showPropertyArg, showTimestampArg)
	if err != nil {
		return nil, err
	}

	properties, err := parseProperties(out)
	if err != nil {
		return nil, fmt.Errorf("systemd: parse systemctl show %s: %w", unit, err)
	}
	return properties, nil
}

// ShowProperties reads an explicit property set for unit. Doctor uses it for
// cgroup limits so it does not need the timestamp fields requested by status.
func (c *Client) ShowProperties(ctx context.Context, unit string,
	properties ...string,
) (Properties, error) {
	if err := ValidateUnit(unit); err != nil {
		return nil, err
	}
	if len(properties) == 0 {
		return nil, errors.New("systemd: at least one property is required")
	}
	for _, property := range properties {
		if !validPropertyName(property) {
			return nil, fmt.Errorf("systemd: invalid property name %q", property)
		}
	}

	out, err := c.run(ctx, "systemctl", "show", unit,
		"--property="+strings.Join(properties, ","))
	if err != nil {
		return nil, err
	}

	parsed, err := parseProperties(out)
	if err != nil {
		return nil, fmt.Errorf("systemd: parse systemctl show %s: %w", unit, err)
	}
	return parsed, nil
}

// Version returns the first line of systemctl --version, for example
// "systemd 261 (261.2-1-arch)". It is used by doctor to distinguish a working
// systemd from a host where systemctl is missing or unusable.
func (c *Client) Version(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "systemctl", "--version")
	if err != nil {
		return "", err
	}

	line, _, _ := strings.Cut(string(out), "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errors.New("systemd: systemctl --version returned no version")
	}
	if !strings.HasPrefix(line, "systemd ") {
		return "", fmt.Errorf("systemd: unexpected systemctl --version output %q", line)
	}
	return line, nil
}

// IsActive reports whether systemctl considers unit active. A clean
// inactive/unknown state is not an error; a systemd failure is.
func (c *Client) IsActive(ctx context.Context, unit string) (bool, error) {
	if err := ValidateUnit(unit); err != nil {
		return false, err
	}

	out, err := c.run(ctx, "systemctl", "is-active", unit)
	state := strings.TrimSpace(string(out))
	if err == nil {
		return state == "active", nil
	}

	var commandErr *CommandError
	if errors.As(err, &commandErr) &&
		commandErr.ExitCode != 0 &&
		strings.TrimSpace(commandErr.Stderr) == "" &&
		isKnownActiveState(state) {
		return false, nil
	}
	return false, err
}

// Logs returns the last tail journal lines for unit as plain messages.
func (c *Client) Logs(ctx context.Context, unit string, tail int) ([]string, error) {
	if err := ValidateUnit(unit); err != nil {
		return nil, err
	}
	if tail <= 0 {
		return nil, fmt.Errorf("systemd: log tail must be positive, got %d", tail)
	}

	out, err := c.run(ctx, "journalctl",
		"-u", unit,
		"-n", strconv.Itoa(tail),
		"--no-pager",
		"-o", "cat",
	)
	if err != nil {
		return nil, err
	}
	return splitLogLines(out), nil
}

// ControlAction is one of the verbs accepted by tes3mp-easy-rootctl.
type ControlAction string

const (
	ActionStart   ControlAction = "start"
	ActionStop    ControlAction = "stop"
	ActionRestart ControlAction = "restart"
)

// Control invokes the fixed rootctl helper through sudo -n. The helper owns the
// unit name; callers cannot pass another unit.
func (c *Client) Control(ctx context.Context, action ControlAction) error {
	switch action {
	case ActionStart, ActionStop, ActionRestart:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidAction, action)
	}

	_, err := c.run(ctx, "sudo", "-n", RootctlPath, string(action))
	return err
}

// ValidateUnit rejects empty, option-like and path-like unit names. The
// commands only pass known constants, but validating here keeps a future
// argument from becoming a systemctl option.
func ValidateUnit(unit string) error {
	if unit == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidUnit)
	}
	if len(unit) > 256 {
		return fmt.Errorf("%w: %q is too long", ErrInvalidUnit, unit)
	}
	if strings.HasPrefix(unit, "-") || strings.HasPrefix(unit, ".") {
		return fmt.Errorf("%w: %q", ErrInvalidUnit, unit)
	}

	for i := 0; i < len(unit); i++ {
		ch := unit[i]
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '.', ch == '_', ch == '-', ch == '@', ch == ':', ch == '\\':
		default:
			return fmt.Errorf("%w: %q", ErrInvalidUnit, unit)
		}
	}
	return nil
}

func (c *Client) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if c == nil || c.runner == nil {
		return nil, errors.New("systemd: runner is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	out, err := c.runner.Run(ctx, name, args...)
	if err == nil {
		return out, nil
	}

	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		return out, err
	}
	return out, &CommandError{
		Name: name,
		Args: append([]string(nil), args...),
		Err:  err,
	}
}

func validPropertyName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		default:
			return false
		}
	}
	return true
}

func parseProperties(data []byte) (Properties, error) {
	properties := make(Properties)
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	for index, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || key != strings.TrimSpace(key) {
			return nil, fmt.Errorf("line %d is not KEY=VALUE: %q", index+1, line)
		}
		properties[key] = value
	}
	return properties, nil
}

func splitLogLines(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func isKnownActiveState(state string) bool {
	switch state {
	case "inactive", "failed", "unknown",
		"activating", "deactivating", "reloading",
		"maintenance", "refreshing":
		return true
	default:
		return false
	}
}
