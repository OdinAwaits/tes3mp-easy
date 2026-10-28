package fonts

import (
	"fmt"
	"html"
	"strings"
)

// XMLName is the generated MyGUI resource file in the OpenMW config directory.
const XMLName = "openmw_font.xml"

// ConfigLines returns the fallback mappings of the default set, added to
// openmw.cfg. The resource names match openmw_font.xml and the values are the
// font family names.
func ConfigLines() []string {
	return ConfigLinesFor(DefaultSet)
}

// ConfigLinesFor returns the fallback mappings of one catalog set. An unknown
// identifier falls back to the default set, like ResolveSet.
func ConfigLinesFor(setID string) []string {
	assets := ResolveSet(setID).Assets
	lines := make([]string, 0, len(assets))
	for index, asset := range assets {
		lines = append(lines, fmt.Sprintf("fallback=Fonts_Font_%d,%s", index,
			strings.TrimSuffix(asset.Name, ".ttf")))
	}
	return lines
}

// FontXML renders openmw_font.xml for the default set. The ranges are the
// legacy Latin + Cyrillic set: the main font covers text, the serif font the
// Daedric slots and the monospace font the console.
func FontXML() []byte {
	return fontXML(Assets)
}

// FontXMLFor renders openmw_font.xml for one catalog set.
func FontXMLFor(setID string) []byte {
	return fontXML(ResolveSet(setID).Assets)
}

func fontXML(assets []Asset) []byte {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(`<MyGUI type="Resource" version="1.1">` + "\n")
	fonts := []struct {
		resource string
		name     string
		ranges   []string
	}{
		{"Magic Cards", assets[0].Name,
			[]string{"33 126", "161 255", "8211 8212", "8216 8217",
				"8230", "8470", "1025 1105"}},
		{"Daedric", assets[1].Name, []string{"32 126"}},
		{"MonoFont", assets[2].Name,
			[]string{"33 126", "192 382", "8211 8212", "8230", "8470",
				"1025 1105"}},
	}
	for _, font := range fonts {
		builder.WriteString(fmt.Sprintf(
			"    <Resource type=\"ResourceTrueTypeFont\" name=\"%s\">\n",
			html.EscapeString(font.resource)))
		builder.WriteString(fmt.Sprintf(
			"        <Property key=\"Source\" value=\"%s\"/>\n",
			html.EscapeString(font.name)))
		builder.WriteString("        <Property key=\"Antialias\" value=\"false\"/>\n")
		builder.WriteString("        <Property key=\"TabWidth\" value=\"8\"/>\n")
		builder.WriteString("        <Property key=\"OffsetHeight\" value=\"0\"/>\n")
		builder.WriteString("        <Codes>\n")
		for _, codeRange := range font.ranges {
			builder.WriteString(fmt.Sprintf(
				"            <Code range=\"%s\"/>\n", codeRange))
		}
		builder.WriteString("        </Codes>\n")
		builder.WriteString("    </Resource>\n")
	}
	builder.WriteString("</MyGUI>\n")
	return []byte(builder.String())
}

// UpdateConfig replaces the three Fonts_Font_* fallback lines of the default
// set and leaves every other openmw.cfg line untouched. It is idempotent.
func UpdateConfig(data []byte) []byte {
	return UpdateConfigFor(data, DefaultSet)
}

// UpdateConfigFor is UpdateConfig for one catalog set: a set switch replaces the
// family names instead of appending a second mapping.
func UpdateConfigFor(data []byte, setID string) []byte {
	lines := splitConfig(data)
	fallbacks := ConfigLinesFor(setID)
	kept := make([]string, 0, len(lines)+len(fallbacks))
	for _, line := range lines {
		key := strings.TrimSpace(line)
		managed := false
		for index := range fallbacks {
			prefix := fmt.Sprintf("fallback=Fonts_Font_%d,", index)
			if strings.HasPrefix(key, prefix) ||
				key == strings.TrimSuffix(prefix, ",") {
				managed = true
				break
			}
		}
		if managed {
			continue
		}
		kept = append(kept, line)
	}
	kept = append(kept, fallbacks...)
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
