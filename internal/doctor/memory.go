package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	minMemoryBytes int64 = 512 << 20
	// A 512 MiB swap file reports about 4 KiB less in /proc/swaps because the
	// kernel reserves one page for the swap header. Accept the 480 MiB floor
	// used by bootstrap so a freshly created swap file passes this check.
	minSwapBytes   int64 = 480 << 20
	wantSwappiness       = "10"
)

func checkMemory(procRoot, sysctlPath string) Check {
	meminfo, err := os.ReadFile(filepath.Join(procRoot, "meminfo"))
	if err != nil {
		return errorCheck("memory", "cannot read meminfo: %v", err)
	}
	values := parseMemInfo(meminfo)
	memTotal, hasMem := values["MemTotal"]
	swapTotal, hasSwap := values["SwapTotal"]
	if !hasMem {
		return errorCheck("memory", "meminfo has no MemTotal")
	}
	if !hasSwap {
		return errorCheck("memory", "meminfo has no SwapTotal")
	}

	problems := make([]string, 0, 3)
	if memTotal < minMemoryBytes {
		problems = append(problems, fmt.Sprintf(
			"RAM %s is below the 512 MiB minimum", humanBytes(memTotal)))
	}
	if swapTotal < minSwapBytes {
		problems = append(problems, fmt.Sprintf(
			"swap %s is below the 480 MiB minimum", humanBytes(swapTotal)))
	}

	sysctl, err := readRegularFile(sysctlPath, 1<<20)
	if err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", sysctlPath, err))
	} else if got, ok := sysctlValue(sysctl, "vm.swappiness"); !ok {
		problems = append(problems,
			fmt.Sprintf("%s: vm.swappiness is missing", sysctlPath))
	} else if got != wantSwappiness {
		problems = append(problems, fmt.Sprintf(
			"%s: vm.swappiness=%s, want %s", sysctlPath, got, wantSwappiness))
	}

	livePath := filepath.Join(procRoot, "sys/vm/swappiness")
	if live, err := os.ReadFile(livePath); err == nil {
		if got := strings.TrimSpace(string(live)); got != wantSwappiness {
			problems = append(problems, fmt.Sprintf(
				"%s: vm.swappiness=%s, want %s", livePath, got, wantSwappiness))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		problems = append(problems, fmt.Sprintf("%s: %v", livePath, err))
	}

	if len(problems) > 0 {
		return errorCheck("memory", "%s", joinProblems(problems))
	}
	return okCheck("memory", "%s RAM, %s swap",
		humanBytes(memTotal), humanBytes(swapTotal))
}

func parseMemInfo(data []byte) map[string]int64 {
	values := make(map[string]int64)
	for _, line := range strings.Split(string(data), "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || value < 0 {
			continue
		}
		if len(fields) > 1 && strings.EqualFold(fields[1], "kB") {
			value *= 1024
		}
		values[strings.TrimSpace(key)] = value
	}
	return values
}

func sysctlValue(data []byte, key string) (string, bool) {
	value := ""
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		name, raw, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		value = strings.TrimSpace(raw)
		found = true
	}
	return value, found
}
