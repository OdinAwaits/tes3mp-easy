package openmwcfg

import (
	"bytes"
	"strconv"
	"strings"
)

// document is a settings.cfg kept as raw lines so that everything the launcher
// does not manage survives a rewrite byte for byte.
type document struct {
	bom      bool
	eol      string
	lines    []string
	finalEOL bool
}

func parseDocument(data []byte) document {
	doc := document{eol: "\n"}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		doc.bom = true
		data = data[3:]
	}
	if bytes.Contains(data, []byte("\r\n")) {
		doc.eol = "\r\n"
	}
	text := string(data)
	if text == "" {
		return doc
	}
	doc.finalEOL = strings.HasSuffix(text, "\n")
	if doc.finalEOL {
		text = strings.TrimSuffix(text, "\n")
	}
	doc.lines = strings.Split(text, "\n")
	for i, line := range doc.lines {
		doc.lines[i] = strings.TrimSuffix(line, "\r")
	}
	return doc
}

func (d document) bytes() []byte {
	var buffer bytes.Buffer
	if d.bom {
		buffer.WriteString("\ufeff")
	}
	for i, line := range d.lines {
		if i > 0 {
			buffer.WriteString(d.eol)
		}
		buffer.WriteString(line)
	}
	if d.finalEOL && len(d.lines) > 0 {
		buffer.WriteString(d.eol)
	}
	return buffer.Bytes()
}

// section is the half-open line range of a section body.
type section struct {
	header int
	end    int
	found  bool
}

func sectionHeader(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 || trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return "", false
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	return name, name != ""
}

func (d *document) findSection(name string) section {
	for i, line := range d.lines {
		found, ok := sectionHeader(line)
		if !ok || !strings.EqualFold(found, name) {
			continue
		}
		end := len(d.lines)
		for j := i + 1; j < len(d.lines); j++ {
			if _, ok := sectionHeader(d.lines[j]); ok {
				end = j
				break
			}
		}
		return section{header: i, end: end, found: true}
	}
	return section{header: -1, end: len(d.lines)}
}

// entry splits "key = value" and skips comments and blank lines.
func entry(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") ||
		strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	index := strings.Index(trimmed, "=")
	if index <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(trimmed[:index])
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(trimmed[index+1:]), true
}

// lookup returns the first value of key inside the section.
func (d *document) lookup(name, key string) (string, bool) {
	sec := d.findSection(name)
	if !sec.found {
		return "", false
	}
	for i := sec.header + 1; i < sec.end; i++ {
		found, value, ok := entry(d.lines[i])
		if ok && strings.EqualFold(found, key) {
			return value, true
		}
	}
	return "", false
}

func (d *document) intValue(name, key string, fallback int) (int, bool, error) {
	raw, ok := d.lookup(name, key)
	if !ok {
		return fallback, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, true, invalidf(key, "value %q is not an integer", raw)
	}
	return value, true, nil
}

func (d *document) floatValue(name, key string) (float64, bool, error) {
	raw, ok := d.lookup(name, key)
	if !ok {
		return 0, false, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, true, invalidf(key, "value %q is not a number", raw)
	}
	return value, true, nil
}

// set replaces the first occurrence of key, removes later duplicates and
// appends the section or the key when they are absent.
func (d *document) set(name, key, value string) {
	sec := d.findSection(name)
	if !sec.found {
		if len(d.lines) > 0 && strings.TrimSpace(d.lines[len(d.lines)-1]) != "" {
			d.lines = append(d.lines, "")
		}
		d.lines = append(d.lines, "["+name+"]", key+" = "+value)
		d.finalEOL = true
		return
	}
	first := -1
	duplicates := make(map[int]struct{})
	for i := sec.header + 1; i < sec.end; i++ {
		found, _, ok := entry(d.lines[i])
		if !ok || !strings.EqualFold(found, key) {
			continue
		}
		if first < 0 {
			first = i
			continue
		}
		duplicates[i] = struct{}{}
	}
	if first >= 0 {
		line := d.lines[first]
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		d.lines[first] = indent + key + " = " + value
		d.dropLines(duplicates)
		return
	}
	at := sec.header + 1
	for i := sec.header + 1; i < sec.end; i++ {
		if strings.TrimSpace(d.lines[i]) != "" {
			at = i + 1
		}
	}
	d.insert(at, key+" = "+value)
}

func (d *document) insert(at int, line string) {
	d.lines = append(d.lines, "")
	copy(d.lines[at+1:], d.lines[at:])
	d.lines[at] = line
	d.finalEOL = true
}

func (d *document) dropLines(drop map[int]struct{}) {
	if len(drop) == 0 {
		return
	}
	kept := d.lines[:0]
	for i, line := range d.lines {
		if _, ok := drop[i]; ok {
			continue
		}
		kept = append(kept, line)
	}
	d.lines = kept
}
