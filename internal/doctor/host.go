package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

var checkedUnits = []struct {
	name  string
	timer bool
}{
	{name: systemd.UnitTES3MPService},
	{name: systemd.UnitPanelService},
	{name: systemd.UnitBackupTimer, timer: true},
}

func checkArch(ctx context.Context, uname func(context.Context) (string, error)) Check {
	machine, err := uname(ctx)
	if err != nil {
		return errorCheck("arch", "cannot determine architecture: %v", err)
	}
	machine = strings.TrimSpace(machine)
	if machine == "" {
		return errorCheck("arch", "uname -m returned an empty architecture")
	}
	if machine != "x86_64" {
		return errorCheck("arch", "architecture %s is not supported (need x86_64)",
			machine)
	}
	return Check{Name: "arch", Status: StatusOK}
}

func checkSystemd(ctx context.Context, client *systemd.Client,
	timeout time.Duration,
) Check {
	if client == nil {
		return errorCheck("systemd", "systemd client is not configured")
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	version, err := client.Version(callCtx)
	if err != nil {
		return errorCheck("systemd", "%v", err)
	}
	return okCheck("systemd", "%s", version)
}

func checkUnits(ctx context.Context, client *systemd.Client, timeout time.Duration,
	systemdOK bool,
) Check {
	if !systemdOK {
		return errorCheck("units", "not checked: systemd is unavailable")
	}
	if client == nil {
		return errorCheck("units", "systemd client is not configured")
	}

	problems := make([]string, 0, len(checkedUnits))
	for _, unit := range checkedUnits {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		properties, err := client.Show(callCtx, unit.name)
		cancel()
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", unit.name, err))
			continue
		}

		loadState := strings.ToLower(strings.TrimSpace(properties.Get("LoadState")))
		activeState := strings.ToLower(strings.TrimSpace(properties.Get("ActiveState")))
		if loadState != "loaded" {
			if loadState == "not-found" {
				problems = append(problems,
					fmt.Sprintf("%s is not installed", unit.name))
			} else {
				problems = append(problems,
					fmt.Sprintf("%s has LoadState=%s", unit.name, loadState))
			}
			continue
		}

		if activeState == "failed" {
			problems = append(problems, fmt.Sprintf("%s is failed", unit.name))
			continue
		}
		if !unit.timer {
			continue
		}

		if activeState != "active" {
			problems = append(problems,
				fmt.Sprintf("%s is not active (%s)", unit.name, activeState))
		}
		unitFileState := strings.ToLower(strings.TrimSpace(
			properties.Get("UnitFileState")))
		if unitFileState != "enabled" {
			problems = append(problems,
				fmt.Sprintf("%s is not enabled (%s)", unit.name, unitFileState))
		}
	}

	if len(problems) > 0 {
		return errorCheck("units", "%s", strings.Join(problems, "; "))
	}
	return Check{Name: "units", Status: StatusOK}
}
