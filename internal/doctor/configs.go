package doctor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var requireLine = regexp.MustCompile(
	`^require\s*\(\s*["']custom\.([A-Za-z0-9_-]+)["']\s*\)\s*(?:--.*)?$`)

func checkConfigs(root string) Check {
	dir := filepath.Join(root, "configs")
	problems := make([]string, 0, 4)

	serverPath := filepath.Join(dir, "tes3mp-server-default.cfg")
	if data, err := readRegularFile(serverPath, 4<<20); err != nil {
		problems = append(problems, err.Error())
	} else if _, err := parseINI(data); err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", serverPath, err))
	}

	luaPath := filepath.Join(dir, "config.lua")
	if data, err := readRegularFile(luaPath, 4<<20); err != nil {
		problems = append(problems, err.Error())
	} else if err := validateLuaConfig(data); err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", luaPath, err))
	}

	banlistPath := filepath.Join(dir, "banlist.json")
	if data, err := readRegularFile(banlistPath, 4<<20); err != nil {
		problems = append(problems, err.Error())
	} else if err := validateBanlist(data); err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", banlistPath, err))
	}

	problems = append(problems, checkCustomScripts(root)...)
	if len(problems) > 0 {
		return errorCheck("configs", "%s", joinProblems(problems))
	}
	return Check{Name: "configs", Status: StatusOK}
}

func validateBanlist(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var banlist struct {
		PlayerNames *[]string `json:"playerNames"`
		IPAddresses *[]string `json:"ipAddresses"`
	}
	if err := decoder.Decode(&banlist); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data after JSON object")
	}
	if banlist.PlayerNames == nil {
		return fmt.Errorf("playerNames must be an array")
	}
	if banlist.IPAddresses == nil {
		return fmt.Errorf("ipAddresses must be an array")
	}
	return nil
}

func checkCustomScripts(root string) []string {
	scriptsDir := filepath.Join(root, "mods", "scripts")
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", scriptsDir, err)}
	}

	problems := make([]string, 0)
	expected := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			problems = append(problems,
				fmt.Sprintf("mods/scripts/%s: %v", name, err))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			problems = append(problems,
				fmt.Sprintf("mods/scripts/%s is not a regular file", name))
			continue
		}
		if !strings.HasSuffix(name, ".lua") {
			problems = append(problems,
				fmt.Sprintf("mods/scripts/%s is not a .lua file", name))
			continue
		}
		stem := strings.TrimSuffix(name, ".lua")
		if !scriptName(stem) {
			problems = append(problems,
				fmt.Sprintf("mods/scripts/%s has an invalid script name", name))
			continue
		}
		expected = append(expected, "custom."+stem)
	}

	customPath := filepath.Join(root, "configs", "customScripts.lua")
	data, err := readRegularFile(customPath, 4<<20)
	if err != nil {
		return append(problems, err.Error())
	}
	actual := make([]string, 0)
	for index, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		match := requireLine.FindStringSubmatch(line)
		if match == nil {
			problems = append(problems, fmt.Sprintf(
				"configs/customScripts.lua line %d is not a generated require", index+1))
			continue
		}
		actual = append(actual, "custom."+match[1])
	}
	if !equalStrings(expected, actual) {
		problems = append(problems, fmt.Sprintf(
			"configs/customScripts.lua does not match mods/scripts: want %v, got %v",
			expected, actual))
	}
	return problems
}

func scriptName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_', ch == '-':
		default:
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
