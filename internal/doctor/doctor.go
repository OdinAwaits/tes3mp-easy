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
	CommandTimeout time.Duration
	Systemd        *systemd.Client

	Uname    func(context.Context) (string, error)
	DiskFree func(string) (int64, error)
}

// Run executes all O03 checks in a stable order and aggregates their status.
func Run(ctx context.Context, opts Options) Report {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = opts.withDefaults()

	checks := make([]Check, 0, 6)
	checks = append(checks, checkRoot(opts.Root))
	checks = append(checks, checkOS(opts.OSReleasePath))
	checks = append(checks, checkArch(ctx, opts.Uname))

	systemdCheck := checkSystemd(ctx, opts.Systemd, opts.CommandTimeout)
	checks = append(checks, systemdCheck)
	checks = append(checks, checkUnits(ctx, opts.Systemd,
		opts.CommandTimeout, systemdCheck.Status == StatusOK))
	checks = append(checks, checkDisk(opts.Root, opts.DiskFree))

	return Report{Status: summarize(checks), Checks: checks}
}

func (o Options) withDefaults() Options {
	if o.OSReleasePath == "" {
		o.OSReleasePath = DefaultOSReleasePath
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
	return o
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
