package gameconfig

import (
	"strings"
)

type iniRef struct {
	section string
	key     string
}

type iniEntry struct {
	line  int
	value string
}

type iniDoc struct {
	lines   []string
	entries map[iniRef]iniEntry
}

func parseServerDoc(path string, data []byte) (iniDoc, error) {
	doc := iniDoc{
		lines:   strings.Split(string(data), "\n"),
		entries: make(map[iniRef]iniEntry),
	}
	section := ""

	for index, raw := range doc.lines {
		body := strings.TrimSuffix(raw, "\r")
		line := strings.TrimSpace(body)
		if line == "" || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				return iniDoc{}, invalidf(FileServer, "",
					"%s:%d: unterminated section", path, index+1)
			}
			name := strings.TrimSpace(line[1:end])
			if name == "" {
				return iniDoc{}, invalidf(FileServer, "",
					"%s:%d: empty section name", path, index+1)
			}
			rest := strings.TrimSpace(line[end+1:])
			if rest != "" && !strings.HasPrefix(rest, "#") &&
				!strings.HasPrefix(rest, ";") {
				return iniDoc{}, invalidf(FileServer, "",
					"%s:%d: unexpected text after section", path, index+1)
			}
			section = name
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return iniDoc{}, invalidf(FileServer, "",
				"%s:%d: expected key=value", path, index+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return iniDoc{}, invalidf(FileServer, "",
				"%s:%d: empty key", path, index+1)
		}
		if section == "" {
			return iniDoc{}, invalidf(FileServer, "",
				"%s:%d: key %q is outside a section", path, index+1, key)
		}
		ref := iniRef{section: section, key: key}
		if _, exists := doc.entries[ref]; exists {
			return iniDoc{}, invalidf(FileServer, "",
				"%s:%d: duplicate key [%s] %s", path, index+1, section, key)
		}
		parsed, _ := splitINIComment(value)
		doc.entries[ref] = iniEntry{
			line:  index,
			value: parsed,
		}
	}
	return doc, nil
}

// splitINIComment separates an INI value from a trailing "#" or ";" comment.
// A marker only starts a comment at the beginning of the value or after
// whitespace, so values such as "server#1" stay intact.
func splitINIComment(value string) (parsed, comment string) {
	for index := 0; index < len(value); index++ {
		if value[index] != '#' && value[index] != ';' {
			continue
		}
		if index == 0 || value[index-1] == ' ' || value[index-1] == '\t' {
			return strings.TrimSpace(value[:index]), value[index:]
		}
	}
	return strings.TrimSpace(value), ""
}

func readServer(path string, data []byte) (Snapshot, error) {
	doc, err := parseServerDoc(path, data)
	if err != nil {
		return Snapshot{}, err
	}

	values := make(map[string]any)
	for _, spec := range serverSpecs {
		entry, ok := doc.entries[iniRef{section: spec.section, key: spec.iniKey}]
		if !ok {
			continue
		}
		value, err := parseTypedValue(spec, entry.value)
		if err != nil {
			return Snapshot{}, invalidf(FileServer, spec.key,
				"%s:%d: %v", path, entry.line+1, err)
		}
		values[spec.key] = value
	}
	return Snapshot{
		File:     FileServer,
		Values:   values,
		ReadOnly: []string{},
	}, nil
}

func setServer(path string, data []byte, key, rawValue string,
) ([]byte, any, error) {
	doc, err := parseServerDoc(path, data)
	if err != nil {
		return nil, nil, err
	}
	spec, ok := serverSpecFor(key)
	if !ok {
		return nil, nil, invalidf(FileServer, key,
			"key is not in the editable whitelist")
	}
	entry, ok := doc.entries[iniRef{section: spec.section, key: spec.iniKey}]
	if !ok {
		return nil, nil, invalidf(FileServer, spec.key,
			"%s has no [%s] %s setting", path, spec.section, spec.iniKey)
	}
	value, err := parseTypedValue(spec, rawValue)
	if err != nil {
		return nil, nil, invalidf(FileServer, spec.key, "%v", err)
	}
	formatted, err := formatValue(spec, value)
	if err != nil {
		return nil, nil, invalidf(FileServer, spec.key, "%v", err)
	}

	raw := doc.lines[entry.line]
	hasCR := strings.HasSuffix(raw, "\r")
	body := strings.TrimSuffix(raw, "\r")
	eq := strings.IndexByte(body, '=')
	if eq < 0 {
		return nil, nil, invalidf(FileServer, spec.key,
			"%s:%d: expected key=value", path, entry.line+1)
	}
	valueStart := eq + 1
	for valueStart < len(body) && (body[valueStart] == ' ' || body[valueStart] == '\t') {
		valueStart++
	}
	valueEnd := len(body)
	for index := valueStart; index < len(body); index++ {
		if body[index] != '#' && body[index] != ';' {
			continue
		}
		if index == 0 || body[index-1] == ' ' || body[index-1] == '\t' {
			valueEnd = index
			for valueEnd > valueStart &&
				(body[valueEnd-1] == ' ' || body[valueEnd-1] == '\t') {
				valueEnd--
			}
			break
		}
	}
	updatedLine := body[:valueStart] + formatted + body[valueEnd:]
	if hasCR {
		updatedLine += "\r"
	}
	lines := append([]string(nil), doc.lines...)
	lines[entry.line] = updatedLine

	return []byte(strings.Join(lines, "\n")), value, nil
}

func serverSpecFor(key string) (valueSpec, bool) {
	switch key {
	case "masterServer.enabled", "MasterServer.enabled", "enabled":
		key = "masterServerEnabled"
	}
	return findSpec(serverSpecs, key)
}
