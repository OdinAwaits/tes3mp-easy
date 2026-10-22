package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

const (
	stateRunning   = "running"
	stateStopped   = "stopped"
	stateStarting  = "starting"
	stateUnhealthy = "unhealthy"
	stateUnknown   = "unknown"

	healthHealthy   = "healthy"
	healthUnhealthy = "unhealthy"
	healthUnknown   = "unknown"
)

type statusData struct {
	Unit      string  `json:"unit"`
	State     string  `json:"state"`
	Health    string  `json:"health"`
	StartedAt *string `json:"started_at"`
}

func runStatus(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	unit, parseErr := parseStatusArgs(args)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	showCtx, cancelShow := context.WithTimeout(context.Background(), opts.commandTimeout)
	properties, err := client.Show(showCtx, unit)
	cancelShow()
	if err != nil {
		return writeSystemdFailure(stdout, stderr, opts.json, "status", err)
	}

	activeCtx, cancelActive := context.WithTimeout(context.Background(), opts.commandTimeout)
	active, activeErr := client.IsActive(activeCtx, unit)
	cancelActive()

	state, health, err := classifyStatus(unit, properties, active, activeErr)
	if err != nil {
		return writeSystemdFailure(stdout, stderr, opts.json, "status", err)
	}

	data := statusData{
		Unit:      unit,
		State:     state,
		Health:    health,
		StartedAt: statusStartedAt(unit, properties),
	}

	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeStatusHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func parseStatusArgs(args []string) (string, *ops.Error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	unit := fs.String("unit", systemd.UnitTES3MPService, "")
	if err := fs.Parse(args); err != nil {
		return "", ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("status: %v", err))
	}
	if fs.NArg() != 0 {
		return "", ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("status: unexpected argument %q", fs.Arg(0)))
	}

	switch *unit {
	case systemd.UnitTES3MPService, systemd.UnitBackupTimer:
		return *unit, nil
	default:
		return "", ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("status: unsupported unit %q; want %s or %s",
				*unit, systemd.UnitTES3MPService, systemd.UnitBackupTimer))
	}
}

func classifyStatus(unit string, properties systemd.Properties,
	isActive bool, activeErr error,
) (string, string, error) {
	loadState := strings.ToLower(strings.TrimSpace(properties.Get("LoadState")))
	activeState := strings.ToLower(strings.TrimSpace(properties.Get("ActiveState")))
	isActiveKnown := activeErr == nil

	switch loadState {
	case "not-found":
		return "", "", fmt.Errorf("unit %s is not installed", unit)
	case "error":
		return stateUnhealthy, healthUnhealthy, nil
	case "masked":
		return stateStopped, healthUnhealthy, nil
	}

	var state string
	if unit == systemd.UnitBackupTimer {
		state = timerState(activeState, isActive, isActiveKnown)
	} else {
		state = serviceState(activeState, isActive, isActiveKnown)
	}

	health := healthHealthy
	switch {
	case activeErr != nil:
		health = healthUnhealthy
	case state == stateUnhealthy:
		health = healthUnhealthy
	case state == stateUnknown:
		health = healthUnknown
	}

	return state, health, nil
}

func serviceState(activeState string, isActive, isActiveKnown bool) string {
	switch activeState {
	case "failed":
		return stateUnhealthy
	case "activating", "reloading":
		return stateStarting
	case "deactivating":
		return stateStopped
	case "active":
		if !isActiveKnown || isActive {
			return stateRunning
		}
		return stateStopped
	case "inactive":
		if isActiveKnown && isActive {
			return stateRunning
		}
		return stateStopped
	case "unknown", "":
		return stateUnknown
	default:
		if isActiveKnown && isActive {
			return stateRunning
		}
		return stateUnknown
	}
}

// timerState follows 04-ops.md: a timer is running while it is active and
// stopped otherwise. Transitional states are not reported for timers.
func timerState(activeState string, isActive, isActiveKnown bool) string {
	switch activeState {
	case "failed":
		return stateUnhealthy
	case "unknown", "":
		return stateUnknown
	case "active":
		if !isActiveKnown || isActive {
			return stateRunning
		}
		return stateStopped
	default:
		if isActiveKnown && isActive {
			return stateRunning
		}
		return stateStopped
	}
}

func statusStartedAt(unit string, properties systemd.Properties) *string {
	property := "ActiveEnterTimestamp"
	if unit == systemd.UnitBackupTimer {
		property = "LastTriggerUSec"
	}
	return parseSystemdTimestamp(properties.Get(property))
}

var systemdTimeLayouts = []string{
	"Mon 2006-01-02 15:04:05 MST",
	"Mon 2006-01-02 15:04:05.999999999 MST",
	"2006-01-02 15:04:05 MST",
	"Mon 2006-01-02 15:04:05 -0700",
	time.RFC3339Nano,
}

func parseSystemdTimestamp(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "n/a" {
		return nil
	}

	var parsed time.Time
	if strings.HasPrefix(raw, "@") {
		seconds, err := strconv.ParseFloat(strings.TrimPrefix(raw, "@"), 64)
		if err != nil {
			return nil
		}
		parsed = time.Unix(0, int64(seconds*float64(time.Second)))
	} else {
		for _, layout := range systemdTimeLayouts {
			candidate, err := time.Parse(layout, raw)
			if err == nil {
				parsed = candidate
				break
			}
		}
		if parsed.IsZero() {
			return nil
		}
	}

	formatted := parsed.UTC().Format(time.RFC3339)
	return &formatted
}

func writeStatusHuman(w io.Writer, data statusData) error {
	startedAt := "-"
	if data.StartedAt != nil {
		startedAt = *data.StartedAt
	}

	var out strings.Builder
	fmt.Fprintf(&out, "unit: %s\n", data.Unit)
	fmt.Fprintf(&out, "state: %s\n", data.State)
	fmt.Fprintf(&out, "health: %s\n", data.Health)
	fmt.Fprintf(&out, "started_at: %s\n", startedAt)
	_, err := io.WriteString(w, out.String())
	return err
}
