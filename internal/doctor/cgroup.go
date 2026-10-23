package doctor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type cgroupLimit struct {
	unit string
	high int64
	max  int64
}

var cgroupLimits = []cgroupLimit{
	{unit: systemd.UnitTES3MPService, high: 384 << 20, max: 448 << 20},
	{unit: systemd.UnitPanelService, high: 48 << 20, max: 64 << 20},
	{unit: systemd.UnitBackupService, high: 48 << 20, max: 64 << 20},
}

func checkCgroup(ctx context.Context, client *systemd.Client, timeout time.Duration,
	systemdOK bool,
) Check {
	if !systemdOK {
		return errorCheck("cgroup", "not checked: systemd is unavailable")
	}
	if client == nil {
		return errorCheck("cgroup", "systemd client is not configured")
	}

	problems := make([]string, 0, len(cgroupLimits)*2)
	for _, limit := range cgroupLimits {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		properties, err := client.ShowProperties(callCtx, limit.unit,
			"MemoryHigh", "MemoryMax")
		cancel()
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", limit.unit, err))
			continue
		}

		for _, property := range []struct {
			name string
			want int64
		}{
			{name: "MemoryHigh", want: limit.high},
			{name: "MemoryMax", want: limit.max},
		} {
			raw := strings.TrimSpace(properties.Get(property.name))
			got, err := parseMemoryLimit(raw)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s %s=%q: %v",
					limit.unit, property.name, raw, err))
				continue
			}
			if got != property.want {
				problems = append(problems, fmt.Sprintf(
					"%s %s=%s, want %d", limit.unit, property.name,
					humanBytes(got), property.want))
			}
		}
	}

	if len(problems) > 0 {
		return errorCheck("cgroup", "%s", joinProblems(problems))
	}
	return Check{Name: "cgroup", Status: StatusOK}
}

func parseMemoryLimit(raw string) (int64, error) {
	if raw == "" {
		return 0, fmt.Errorf("value is empty")
	}
	if strings.EqualFold(raw, "infinity") {
		return 0, fmt.Errorf("limit is not set")
	}

	multiplier := int64(1)
	number := raw
	if suffix := raw[len(raw)-1]; suffix < '0' || suffix > '9' {
		switch suffix {
		case 'K', 'k':
			multiplier = 1 << 10
		case 'M', 'm':
			multiplier = 1 << 20
		case 'G', 'g':
			multiplier = 1 << 30
		case 'T', 't':
			multiplier = 1 << 40
		default:
			return 0, fmt.Errorf("unsupported suffix")
		}
		number = raw[:len(raw)-1]
	}
	value, err := strconv.ParseInt(number, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid byte value")
	}
	return value * multiplier, nil
}
