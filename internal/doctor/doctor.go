// Package doctor implements the read-only diagnostics behind the doctor
// command. It never writes to the data root and never takes the ops lock.
package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/retention"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

// DefaultOSReleasePath is the host OS description read by doctor.
const DefaultOSReleasePath = "/etc/os-release"

// Default host paths used by the deep checks. Tests override them through
// Options; the CLI exposes environment overrides for contract tests.
const (
	DefaultProcRoot           = "/proc"
	DefaultSysctlPath         = "/etc/sysctl.d/99-tes3mp-easy.conf"
	DefaultJournaldConfigPath = "/etc/systemd/journald.conf.d/tes3mp-easy.conf"
	DefaultRuntimeRoot        = "/opt/tes3mp-easy/tes3mp"
)

// DefaultCommandTimeout bounds one systemctl invocation.
const DefaultCommandTimeout = 2 * time.Minute

// Status is the state of one check or of the whole report.
type Status string

const (
	StatusOK    Status = "ok"
	StatusWarn  Status = "warn"
	StatusError Status = "error"
)

// Check is one doctor result. Message is omitted for a clean check.
type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// Report is the data payload of the doctor command.
type Report struct {
	Status Status  `json:"status"`
	Checks []Check `json:"checks"`
}

// Options controls one doctor run. The function fields let unit tests replace
// host probes without touching the real system.
type Options struct {
	Root           string
	OSReleasePath  string
	ProcRoot       string
	SysctlPath     string
	JournaldConfig string
	RuntimeRoot    string
	CommandTimeout time.Duration
	Systemd        *systemd.Client

	Uname      func(context.Context) (string, error)
	DiskFree   func(string) (int64, error)
	DeviceOf   func(string) (uint64, error)
	RunCommand func(context.Context, string, ...string) ([]byte, error)

	OwnerNames    ownerNamesFunc
	SkipOwnership bool
}

// Run executes all checks in the stable order documented in
// docs/v2/04-ops.md §6 and aggregates their status.
func Run(ctx context.Context, opts Options) Report {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = opts.withDefaults()

	checks := make([]Check, 0, 19)
	checks = append(checks, checkRoot(opts.Root))
	checks = append(checks, checkOS(opts.OSReleasePath))
	checks = append(checks, checkArch(ctx, opts.Uname))

	systemdCheck := checkSystemd(ctx, opts.Systemd, opts.CommandTimeout)
	systemdOK := systemdCheck.Status == StatusOK
	checks = append(checks, systemdCheck)
	checks = append(checks, checkUnits(ctx, opts.Systemd,
		opts.CommandTimeout, systemdOK))
	checks = append(checks, checkDisk(opts.Root, opts.DiskFree))

	checks = append(checks, checkOwnership(opts.Root, opts.OwnerNames,
		opts.SkipOwnership))
	checks = append(checks, checkJournald(ctx, opts.Systemd,
		opts.CommandTimeout, systemdOK))
	checks = append(checks, checkRuntime(ctx, opts.RuntimeRoot,
		opts.CommandTimeout, opts.RunCommand))
	checks = append(checks, checkMemory(opts.ProcRoot, opts.SysctlPath))
	checks = append(checks, checkCgroup(ctx, opts.Systemd,
		opts.CommandTimeout, systemdOK))
	checks = append(checks, checkFilesystems(opts.Root, opts.DeviceOf))
	checks = append(checks, checkBudgets(opts.Root))
	checks = append(checks, checkCurrent(opts.Root))
	checks = append(checks, checkDataFiles(opts.Root))
	checks = append(checks, checkArchives(opts.Root))
	checks = append(checks, checkConfigs(opts.Root))
	checks = append(checks, checkCleanup(opts.Root))
	checks = append(checks, checkJournaldConfig(opts.JournaldConfig))

	return Report{Status: summarize(checks), Checks: checks}
}

func (o Options) withDefaults() Options {
	if o.OSReleasePath == "" {
		o.OSReleasePath = DefaultOSReleasePath
	}
	if o.ProcRoot == "" {
		o.ProcRoot = DefaultProcRoot
	}
	if o.SysctlPath == "" {
		o.SysctlPath = DefaultSysctlPath
	}
	if o.JournaldConfig == "" {
		o.JournaldConfig = DefaultJournaldConfigPath
	}
	if o.RuntimeRoot == "" {
		o.RuntimeRoot = DefaultRuntimeRoot
	}
	if o.CommandTimeout <= 0 {
		o.CommandTimeout = DefaultCommandTimeout
	}
	if o.Uname == nil {
		o.Uname = defaultUname
	}
	if o.DiskFree == nil {
		o.DiskFree = retention.DiskFree
	}
	if o.DeviceOf == nil {
		o.DeviceOf = deviceOfPath
	}
	if o.RunCommand == nil {
		o.RunCommand = defaultRunCommand
	}
	if o.OwnerNames == nil {
		o.OwnerNames = defaultOwnerNames
	}
	return o
}

func defaultRunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return out, fmt.Errorf("%s: %w", name, err)
		}
		return out, fmt.Errorf("%s: %w: %s", name, err, message)
	}
	return out, nil
}

func summarize(checks []Check) Status {
	status := StatusOK
	for _, check := range checks {
		switch check.Status {
		case StatusError:
			return StatusError
		case StatusWarn:
			status = StatusWarn
		}
	}
	return status
}

func defaultUname(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "uname", "-m")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("uname -m: %w", err)
	}
	machine := strings.TrimSpace(string(out))
	if machine == "" {
		return "", fmt.Errorf("uname -m: empty output")
	}
	return machine, nil
}
