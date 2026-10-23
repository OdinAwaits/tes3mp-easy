package doctor

import (
	"context"
	"fmt"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

func checkJournald(ctx context.Context, client *systemd.Client, timeout time.Duration,
	systemdOK bool,
) Check {
	if !systemdOK {
		return errorCheck("journald", "not checked: systemd is unavailable")
	}
	if client == nil {
		return errorCheck("journald", "systemd client is not configured")
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := client.Logs(callCtx, systemd.UnitTES3MPService, 1); err != nil {
		return errorCheck("journald", "cannot read journal: %v", err)
	}
	return Check{Name: "journald", Status: StatusOK}
}

func checkJournaldConfig(path string) Check {
	data, err := readRegularFile(path, 1<<20)
	if err != nil {
		return errorCheck("journald-config", "%v", err)
	}
	values, err := parseINI(data)
	if err != nil {
		return errorCheck("journald-config", "%s: %v", path, err)
	}
	section := values["Journal"]
	if section == nil {
		return errorCheck("journald-config", "%s: missing [Journal] section", path)
	}

	problems := make([]string, 0, 2)
	if got := section["SystemMaxUse"]; got != "100M" {
		problems = append(problems, fmt.Sprintf(
			"SystemMaxUse=%q, want %q", got, "100M"))
	}
	if got := section["RuntimeMaxUse"]; got != "32M" {
		problems = append(problems, fmt.Sprintf(
			"RuntimeMaxUse=%q, want %q", got, "32M"))
	}
	if len(problems) > 0 {
		return errorCheck("journald-config", "%s: %s", path,
			joinProblems(problems))
	}
	return Check{Name: "journald-config", Status: StatusOK}
}
