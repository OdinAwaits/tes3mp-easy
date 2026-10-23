package locale

import "strings"

// UpdateEncoding sets encoding=win1251 for the Russian localization and removes
// only that value when the localization is disabled. Other encoding values and
// every unrelated openmw.cfg line are preserved.
func UpdateEncoding(data []byte, enabled bool) []byte {
	lines := splitConfig(data)
	kept := make([]string, 0, len(lines)+1)
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		key, value, hasValue := strings.Cut(trimmed, "=")
		if strings.TrimSpace(key) == "encoding" && hasValue {
			if enabled {
				if !found {
					kept = append(kept, "encoding="+Encoding)
					found = true
				}
				continue
			}
			if strings.EqualFold(strings.TrimSpace(value), Encoding) {
				continue
			}
		}
		kept = append(kept, line)
	}
	if enabled && !found {
		kept = append(kept, "encoding="+Encoding)
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
