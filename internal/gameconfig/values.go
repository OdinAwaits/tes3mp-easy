package gameconfig

import (
	"fmt"
	"strconv"
	"strings"
)

type valueKind uint8

const (
	kindString valueKind = iota
	kindInt
	kindBool
)

type valueSpec struct {
	key      string
	section  string
	iniKey   string
	kind     valueKind
	min, max int64
	minLen   int
	maxLen   int
}

var serverSpecs = []valueSpec{
	{key: "hostname", section: "General", iniKey: "hostname", kind: kindString, minLen: 1, maxLen: 64},
	{key: "port", section: "General", iniKey: "port", kind: kindInt, min: 1024, max: 65535},
	{key: "maximumPlayers", section: "General", iniKey: "maximumPlayers", kind: kindInt, min: 1, max: 64},
	{key: "password", section: "General", iniKey: "password", kind: kindString},
	{key: "logLevel", section: "General", iniKey: "logLevel", kind: kindInt, min: 0, max: 4},
	{key: "masterServerEnabled", section: "MasterServer", iniKey: "enabled", kind: kindBool},
}

var luaSpecs = []valueSpec{
	{key: "gameMode", kind: kindString},
	{key: "loginTime", kind: kindInt, min: 10, max: 600},
	{key: "maxClientsPerIP", kind: kindInt, min: 1, max: 10},
	{key: "difficulty", kind: kindInt, min: -100, max: 100},
	{key: "passTimeWhenEmpty", kind: kindBool},
	{key: "nightStartHour", kind: kindInt, min: 0, max: 23},
	{key: "nightEndHour", kind: kindInt, min: 0, max: 23},
	{key: "allowConsole", kind: kindBool},
	{key: "allowBedRest", kind: kindBool},
	{key: "allowWildernessRest", kind: kindBool},
	{key: "allowWait", kind: kindBool},
	{key: "allowSuicideCommand", kind: kindBool},
	{key: "allowFixmeCommand", kind: kindBool},
	{key: "shareJournal", kind: kindBool},
	{key: "shareFactionRanks", kind: kindBool},
	{key: "shareTopics", kind: kindBool},
	{key: "shareBounty", kind: kindBool},
	{key: "shareReputation", kind: kindBool},
	{key: "enforceDataFiles", kind: kindBool},
}

func findSpec(specs []valueSpec, key string) (valueSpec, bool) {
	for _, spec := range specs {
		if spec.key == key {
			return spec, true
		}
	}
	return valueSpec{}, false
}

func parseTypedValue(spec valueSpec, raw string) (any, error) {
	switch spec.kind {
	case kindString:
		value := raw
		if strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("%s contains a control character", spec.key)
		}
		if spec.minLen > 0 && len(value) < spec.minLen {
			return nil, fmt.Errorf("%s must be at least %d characters", spec.key, spec.minLen)
		}
		if spec.maxLen > 0 && len(value) > spec.maxLen {
			return nil, fmt.Errorf("%s must be at most %d characters", spec.key, spec.maxLen)
		}
		return value, nil
	case kindInt:
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must be an integer", spec.key)
		}
		if value < spec.min || value > spec.max {
			return nil, fmt.Errorf("%s must be in %d..%d", spec.key, spec.min, spec.max)
		}
		return int(value), nil
	case kindBool:
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("%s must be true or false", spec.key)
		}
	default:
		return nil, fmt.Errorf("%s has an unsupported type", spec.key)
	}
}

func formatValue(spec valueSpec, value any) (string, error) {
	switch spec.kind {
	case kindString:
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", spec.key)
		}
		return text, nil
	case kindInt:
		number, ok := value.(int)
		if !ok {
			return "", fmt.Errorf("%s must be an integer", spec.key)
		}
		return strconv.Itoa(number), nil
	case kindBool:
		flag, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("%s must be a boolean", spec.key)
		}
		return strconv.FormatBool(flag), nil
	default:
		return "", fmt.Errorf("%s has an unsupported type", spec.key)
	}
}

func luaQuote(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch ch := value[i]; ch {
		case '\\', '"':
			out.WriteByte('\\')
			out.WriteByte(ch)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			out.WriteByte(ch)
		}
	}
	out.WriteByte('"')
	return out.String()
}

func formatLuaValue(spec valueSpec, value any) (string, error) {
	if spec.kind == kindString {
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", spec.key)
		}
		if strings.ContainsAny(text, "\x00\r\n") {
			return "", fmt.Errorf("%s contains a control character", spec.key)
		}
		return luaQuote(text), nil
	}
	return formatValue(spec, value)
}
