package locale

import (
	"path"
	"strings"
)

// UpdateDataFiles inserts the global localization Data Files directory after
// the first data= line and removes any previous data= line below root. Passing
// an empty overlay removes the managed line without adding a new one. Unrelated
// data= lines are preserved.
func UpdateDataFiles(data []byte, overlay, root string) []byte {
	overlay = strings.TrimSpace(overlay)
	root = strings.TrimSpace(root)
	lines := splitConfig(data)
	kept := make([]string, 0, len(lines)+1)
	inserted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data=") {
			value := configDataValue(trimmed)
			if root != "" && withinConfigRoot(value, root) {
				if overlay != "" && !inserted {
					kept = append(kept, configDataLine(overlay))
					inserted = true
				}
				continue
			}
			kept = append(kept, line)
			if overlay != "" && !inserted {
				kept = append(kept, configDataLine(overlay))
				inserted = true
			}
			continue
		}
		kept = append(kept, line)
	}
	if overlay != "" && !inserted {
		kept = append(kept, configDataLine(overlay))
	}
	return joinConfig(kept)
}

func configDataValue(line string) string {
	value := strings.TrimSpace(strings.TrimPrefix(line, "data="))
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return value
}

func configDataLine(value string) string {
	return "data=\"" + strings.TrimSuffix(toConfigPath(value), "/") + "\""
}

func toConfigPath(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
}

func withinConfigRoot(value, root string) bool {
	value = path.Clean(toConfigPath(value))
	root = path.Clean(toConfigPath(root))
	if strings.EqualFold(value, root) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(value), strings.ToLower(root)+"/")
}

// UpdateEncoding writes one OpenMW encoding= line for the selected language
// pack. An empty encoding removes only encodings owned by language packs, so a
// user's original encoding and every unrelated openmw.cfg line are preserved.
func UpdateEncoding(data []byte, encoding string) []byte {
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	lines := splitConfig(data)
	kept := make([]string, 0, len(lines)+1)
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		key, value, hasValue := strings.Cut(trimmed, "=")
		if strings.TrimSpace(key) == "encoding" && hasValue {
			if encoding != "" {
				if !found {
					kept = append(kept, "encoding="+encoding)
					found = true
				}
				continue
			}
			if IsPackEncoding(value) {
				continue
			}
		}
		kept = append(kept, line)
	}
	if encoding != "" && !found {
		kept = append(kept, "encoding="+encoding)
	}
	return joinConfig(kept)
}

func splitConfig(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func joinConfig(lines []string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
