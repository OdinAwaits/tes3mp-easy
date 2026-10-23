package mods

import (
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

var (
	baseESMNames = map[string]struct{}{
		"morrowind.esm": {},
		"tribunal.esm":  {},
		"bloodmoon.esm": {},
	}

	fallbackArchives = []string{
		"Morrowind.bsa",
		"Tribunal.bsa",
		"Bloodmoon.bsa",
	}
)

// BuildProfile returns the server profile openmw.cfg. Data and content lines
// are regenerated; every other line (encoding, fallback, fonts and user
// options) is preserved. The base fallback archives are added only when the
// file does not already define any.
func BuildProfile(existing []byte, morrowindDataFiles, serverDataFiles string,
	required []archive.RequiredFile,
) []byte {
	var preserved []string
	hasFallback := false
	for _, raw := range strings.Split(string(existing), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "data="):
			continue
		case strings.HasPrefix(trimmed, "content="):
			continue
		case strings.HasPrefix(trimmed, "fallback-archive="):
			hasFallback = true
		}
		if line != "" {
			preserved = append(preserved, line)
		}
	}

	lines := make([]string, 0, len(preserved)+len(required)+8)
	lines = append(lines, preserved...)
	if !hasFallback {
		for _, name := range fallbackArchives {
			lines = append(lines, "fallback-archive="+name)
		}
	}
	lines = append(lines,
		`data="`+toConfigPath(morrowindDataFiles)+`"`,
		`data="`+toConfigPath(serverDataFiles)+`"`,
	)
	for _, file := range required {
		lines = append(lines, "content="+file.Name)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func toConfigPath(path string) string {
	path = strings.ReplaceAll(path, `\`, "/")
	return strings.TrimSuffix(path, "/")
}

func isBaseESM(name string) bool {
	_, ok := baseESMNames[strings.ToLower(name)]
	return ok
}
