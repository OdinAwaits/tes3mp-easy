package doctor

import (
	"fmt"
	"strings"
)

func validateLuaConfig(data []byte) error {
	if err := validateLuaBalance(data); err != nil {
		return err
	}

	for index, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if !strings.HasPrefix(line, "config.") {
			continue
		}
		assignment := strings.TrimPrefix(line, "config.")
		key, value, ok := strings.Cut(assignment, "=")
		if !ok {
			return fmt.Errorf("line %d: config.%s has no value", index+1, key)
		}
		key = strings.TrimSpace(key)
		if key == "" || !luaIdentifier(key) {
			return fmt.Errorf("line %d: invalid config key %q", index+1, key)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("line %d: config.%s has no value", index+1, key)
		}
	}
	return nil
}

func luaIdentifier(value string) bool {
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_':
		default:
			return false
		}
	}
	return value != ""
}

func validateLuaBalance(data []byte) error {
	var stack []byte
	for i := 0; i < len(data); {
		ch := data[i]

		if ch == '-' && i+1 < len(data) && data[i+1] == '-' {
			if level, start, ok := luaLongBracket(data, i+2); ok {
				end, ok := luaLongBracketEnd(data, start, level)
				if !ok {
					return fmt.Errorf("unterminated long comment")
				}
				i = end
				continue
			}
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}

		if ch == '[' {
			if level, start, ok := luaLongBracket(data, i); ok {
				end, ok := luaLongBracketEnd(data, start, level)
				if !ok {
					return fmt.Errorf("unterminated long string")
				}
				i = end
				continue
			}
			stack = append(stack, '[')
			i++
			continue
		}

		switch ch {
		case '"', '\'':
			next, err := skipLuaString(data, i)
			if err != nil {
				return err
			}
			i = next
			continue
		case '{', '(':
			stack = append(stack, ch)
		case '}', ')':
			want := byte('{')
			if ch == ')' {
				want = '('
			}
			if len(stack) == 0 || stack[len(stack)-1] != want {
				return fmt.Errorf("unexpected %q", ch)
			}
			stack = stack[:len(stack)-1]
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return fmt.Errorf("unexpected ']'")
			}
			stack = stack[:len(stack)-1]
		}
		i++
	}
	if len(stack) > 0 {
		return fmt.Errorf("unclosed %q", stack[len(stack)-1])
	}
	return nil
}

func luaLongBracket(data []byte, start int) (level, content int, ok bool) {
	if start >= len(data) || data[start] != '[' {
		return 0, 0, false
	}
	i := start + 1
	for i < len(data) && data[i] == '=' {
		level++
		i++
	}
	if i >= len(data) || data[i] != '[' {
		return 0, 0, false
	}
	return level, i + 1, true
}

func luaLongBracketEnd(data []byte, start, level int) (int, bool) {
	close := "]" + strings.Repeat("=", level) + "]"
	index := strings.Index(string(data[start:]), close)
	if index < 0 {
		return 0, false
	}
	return start + index + len(close), true
}

func skipLuaString(data []byte, start int) (int, error) {
	quote := data[start]
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case quote:
			return i + 1, nil
		case '\n':
			return 0, fmt.Errorf("unterminated string")
		}
	}
	return 0, fmt.Errorf("unterminated string")
}
