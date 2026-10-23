package doctor

import (
	"fmt"
	"strings"
)

func parseINI(data []byte) (map[string]map[string]string, error) {
	values := make(map[string]map[string]string)
	text := strings.TrimPrefix(string(data), "\ufeff")
	section := ""

	for index, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated section", index+1)
			}
			name := strings.TrimSpace(line[1:end])
			if name == "" {
				return nil, fmt.Errorf("line %d: empty section name", index+1)
			}
			rest := strings.TrimSpace(line[end+1:])
			if rest != "" && !strings.HasPrefix(rest, "#") &&
				!strings.HasPrefix(rest, ";") {
				return nil, fmt.Errorf("line %d: unexpected text after section", index+1)
			}
			section = name
			if values[section] == nil {
				values[section] = make(map[string]string)
			}
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key=value", index+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", index+1)
		}
		if section == "" {
			return nil, fmt.Errorf("line %d: key %q is outside a section", index+1, key)
		}
		values[section][key] = strings.TrimSpace(value)
	}

	return values, nil
}
