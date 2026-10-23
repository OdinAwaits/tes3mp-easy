package gameconfig

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type luaEntry struct {
	line       int
	valueStart int
	valueEnd   int
	complex    bool
}

type luaDoc struct {
	lines   []string
	entries map[string]luaEntry
}

func parseLuaDoc(path string, data []byte) (luaDoc, error) {
	doc := luaDoc{
		lines:   strings.Split(string(data), "\n"),
		entries: make(map[string]luaEntry),
	}
	for index, raw := range doc.lines {
		body := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(body)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if !strings.HasPrefix(trimmed, "config.") {
			continue
		}
		keyStart := strings.Index(body, "config.") + len("config.")
		keyEnd := keyStart
		for keyEnd < len(body) && isLuaIdent(body[keyEnd]) {
			keyEnd++
		}
		if keyEnd == keyStart {
			continue
		}
		key := body[keyStart:keyEnd]
		pos := keyEnd
		for pos < len(body) && (body[pos] == ' ' || body[pos] == '\t') {
			pos++
		}
		if pos >= len(body) || body[pos] != '=' {
			return luaDoc{}, invalidf(FileConfig, "",
				"%s:%d: expected config.%s = value", path, index+1, key)
		}
		pos++
		for pos < len(body) && (body[pos] == ' ' || body[pos] == '\t') {
			pos++
		}
		if pos >= len(body) {
			return luaDoc{}, invalidf(FileConfig, key,
				"%s:%d: missing value", path, index+1)
		}
		comment := luaCommentIndex(body[pos:])
		region := body[pos:]
		if comment >= 0 {
			region = region[:comment]
		}
		expr := strings.TrimSpace(region)
		if expr == "" {
			return luaDoc{}, invalidf(FileConfig, key,
				"%s:%d: missing value", path, index+1)
		}
		exprStart := pos + strings.Index(region, expr)
		entry := luaEntry{
			line:       index,
			valueStart: exprStart,
			valueEnd:   exprStart + len(expr),
			complex:    expr[0] == '{',
		}
		if _, exists := doc.entries[key]; exists {
			return luaDoc{}, invalidf(FileConfig, key,
				"%s:%d: duplicate assignment", path, index+1)
		}
		doc.entries[key] = entry
	}
	return doc, nil
}

func isLuaIdent(ch byte) bool {
	return ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' ||
		ch >= '0' && ch <= '9'
}

func luaCommentIndex(value string) int {
	var quote byte
	escaped := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '-' && i+1 < len(value) && value[i+1] == '-' {
			return i
		}
	}
	return -1
}

func parseLuaScalar(expr string) (any, bool, error) {
	if expr == "" {
		return nil, false, fmt.Errorf("empty value")
	}
	if expr[0] == '{' {
		return nil, true, nil
	}
	if expr[0] == '"' || expr[0] == '\'' {
		value, end, err := parseLuaString(expr)
		if err != nil {
			return nil, false, err
		}
		if strings.TrimSpace(expr[end:]) != "" {
			return nil, true, nil
		}
		return value, false, nil
	}
	switch expr {
	case "true":
		return true, false, nil
	case "false":
		return false, false, nil
	}
	value, err := strconv.ParseInt(expr, 10, 64)
	if err == nil {
		return int(value), false, nil
	}
	return nil, true, nil
}

func parseLuaString(expr string) (string, int, error) {
	quote := expr[0]
	var out strings.Builder
	for index := 1; index < len(expr); index++ {
		ch := expr[index]
		switch ch {
		case quote:
			return out.String(), index + 1, nil
		case '\\':
			index++
			if index >= len(expr) {
				return "", 0, fmt.Errorf("unterminated escape")
			}
			switch expr[index] {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			case '\\', '"', '\'':
				out.WriteByte(expr[index])
			case '0':
				out.WriteByte(0)
			default:
				return "", 0, fmt.Errorf("unsupported escape \\%c", expr[index])
			}
		default:
			out.WriteByte(ch)
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}

func readLua(path string, data []byte) (Snapshot, error) {
	doc, err := parseLuaDoc(path, data)
	if err != nil {
		return Snapshot{}, err
	}
	values := make(map[string]any)
	readOnly := make(map[string]bool)
	for key, entry := range doc.entries {
		spec, known := findSpec(luaSpecs, key)
		if !known {
			if entry.complex {
				readOnly[key] = true
			}
			continue
		}
		if entry.complex {
			return Snapshot{}, invalidf(FileConfig, key,
				"%s:%d: %s must be a string, number or boolean",
				path, entry.line+1, key)
		}
		expr := luaEntryText(doc.lines[entry.line], entry)
		parsed, complex, err := parseLuaScalar(expr)
		if err != nil {
			return Snapshot{}, invalidf(FileConfig, key,
				"%s:%d: %v", path, entry.line+1, err)
		}
		if complex {
			return Snapshot{}, invalidf(FileConfig, key,
				"%s:%d: %s must be a string, number or boolean",
				path, entry.line+1, key)
		}
		value, err := coerceLuaValue(spec, parsed)
		if err != nil {
			return Snapshot{}, invalidf(FileConfig, key,
				"%s:%d: %v", path, entry.line+1, err)
		}
		values[key] = value
	}
	return Snapshot{
		File:     FileConfig,
		Values:   values,
		ReadOnly: sortedKeys(readOnly),
	}, nil
}

func luaEntryText(raw string, entry luaEntry) string {
	body := strings.TrimSuffix(raw, "\r")
	if entry.valueStart > len(body) || entry.valueEnd > len(body) ||
		entry.valueStart > entry.valueEnd {
		return ""
	}
	return body[entry.valueStart:entry.valueEnd]
}

func coerceLuaValue(spec valueSpec, value any) (any, error) {
	switch spec.kind {
	case kindString:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a string", spec.key)
		}
		if strings.ContainsAny(text, "\x00\r\n") {
			return nil, fmt.Errorf("%s contains a control character", spec.key)
		}
		if spec.minLen > 0 && len(text) < spec.minLen {
			return nil, fmt.Errorf("%s must be at least %d characters", spec.key, spec.minLen)
		}
		if spec.maxLen > 0 && len(text) > spec.maxLen {
			return nil, fmt.Errorf("%s must be at most %d characters", spec.key, spec.maxLen)
		}
		return text, nil
	case kindInt:
		number, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("%s must be an integer", spec.key)
		}
		if int64(number) < spec.min || int64(number) > spec.max {
			return nil, fmt.Errorf("%s must be in %d..%d", spec.key, spec.min, spec.max)
		}
		return number, nil
	case kindBool:
		flag, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be true or false", spec.key)
		}
		return flag, nil
	default:
		return nil, fmt.Errorf("%s has an unsupported type", spec.key)
	}
}

func setLua(path string, data []byte, key, rawValue string,
) ([]byte, any, error) {
	doc, err := parseLuaDoc(path, data)
	if err != nil {
		return nil, nil, err
	}
	spec, ok := findSpec(luaSpecs, key)
	if !ok {
		return nil, nil, invalidf(FileConfig, key,
			"key is not in the editable whitelist")
	}
	entry, ok := doc.entries[key]
	if !ok {
		return nil, nil, invalidf(FileConfig, key,
			"%s has no config.%s assignment", path, key)
	}
	if entry.complex {
		return nil, nil, invalidf(FileConfig, key,
			"%s:%d: tables and lists are read-only", path, entry.line+1)
	}
	value, err := parseTypedValue(spec, rawValue)
	if err != nil {
		return nil, nil, invalidf(FileConfig, key, "%v", err)
	}
	formatted, err := formatLuaValue(spec, value)
	if err != nil {
		return nil, nil, invalidf(FileConfig, key, "%v", err)
	}

	raw := doc.lines[entry.line]
	hasCR := strings.HasSuffix(raw, "\r")
	body := strings.TrimSuffix(raw, "\r")
	updatedLine := body[:entry.valueStart] + formatted + body[entry.valueEnd:]
	if hasCR {
		updatedLine += "\r"
	}
	lines := append([]string(nil), doc.lines...)
	lines[entry.line] = updatedLine

	return []byte(strings.Join(lines, "\n")), value, nil
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
