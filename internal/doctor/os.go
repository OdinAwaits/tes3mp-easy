package doctor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func checkOS(path string) Check {
	data, err := os.ReadFile(path)
	if err != nil {
		return errorCheck("os", "cannot read %s: %v", path, err)
	}
	values, err := parseOSRelease(string(data))
	if err != nil {
		return errorCheck("os", "%s: %v", path, err)
	}

	id := strings.ToLower(strings.TrimSpace(values["ID"]))
	version := strings.TrimSpace(values["VERSION_ID"])
	if id == "" {
		return errorCheck("os", "ID is missing from %s", path)
	}

	var (
		name       string
		minVersion string
	)
	switch id {
	case "debian":
		name, minVersion = "Debian", "12"
	case "ubuntu":
		name, minVersion = "Ubuntu", "24.04"
	default:
		return warnCheck("os",
			"unsupported OS %q (need Debian 12+ or Ubuntu 24.04+)", id)
	}

	if version == "" {
		return errorCheck("os", "VERSION_ID is missing for %s", name)
	}
	ok, err := versionAtLeast(minVersion, version)
	if err != nil {
		return warnCheck("os", "cannot compare %s VERSION_ID %q: %v",
			name, version, err)
	}
	if !ok {
		return warnCheck("os", "%s %s is older than %s %s",
			name, version, name, minVersion)
	}
	return okCheck("os", "%s %s", name, version)
}

func parseOSRelease(text string) (map[string]string, error) {
	values := make(map[string]string)
	for lineNumber, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d is not KEY=VALUE", lineNumber+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("line %d has an empty key", lineNumber+1)
		}
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid quoted value: %w",
					lineNumber+1, err)
			}
			value = unquoted
		} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values, nil
}

func versionAtLeast(required, actual string) (bool, error) {
	requiredParts := strings.Split(required, ".")
	actualParts := strings.Split(actual, ".")

	length := len(requiredParts)
	if len(actualParts) > length {
		length = len(actualParts)
	}
	for i := 0; i < length; i++ {
		want := int64(0)
		got := int64(0)
		var err error
		if i < len(requiredParts) {
			want, err = strconv.ParseInt(requiredParts[i], 10, 64)
			if err != nil {
				return false, fmt.Errorf("invalid required version %q", required)
			}
		}
		if i < len(actualParts) {
			got, err = strconv.ParseInt(actualParts[i], 10, 64)
			if err != nil {
				return false, fmt.Errorf("invalid version %q", actual)
			}
		}
		if got > want {
			return true, nil
		}
		if got < want {
			return false, nil
		}
	}
	return true, nil
}
