package fonts

import (
	"fmt"
	"html"
	"strings"
)

// XMLName is the generated MyGUI resource file in the OpenMW config directory.
const XMLName = "openmw_font.xml"

// ConfigLines returns the fallback mappings added to openmw.cfg. The resource
// names match openmw_font.xml and the values are the font family names.
func ConfigLines() []string {
	return []string{
		"fallback=Fonts_Font_0," + strings.TrimSuffix(Assets[0].Name, ".ttf"),
		"fallback=Fonts_Font_1," + strings.TrimSuffix(Assets[1].Name, ".ttf"),
		"fallback=Fonts_Font_2," + strings.TrimSuffix(Assets[2].Name, ".ttf"),
	}
}

// FontXML renders openmw_font.xml. The ranges are the legacy Latin + Cyrillic
// set: the main font covers text, the serif font the Daedric slots and the
// monospace font the console.
func FontXML() []byte {
	return fontXML(Assets)
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
			[]string{"33 126", "161 255", "1025", "1040 1103", "1105"}},
		{"Daedric", assets[1].Name, []string{"32 126"}},
		{"MonoFont", assets[2].Name, []string{"33 126", "192 382", "1025 1105"}},
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

// UpdateConfig replaces the three Fonts_Font_* fallback lines and leaves every
// other openmw.cfg line untouched. It is idempotent.
func UpdateConfig(data []byte) []byte {
	lines := splitConfig(data)
	kept := make([]string, 0, len(lines)+len(ConfigLines()))
	for _, line := range lines {
		key := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(key, "fallback=Fonts_Font_0,"),
			key == "fallback=Fonts_Font_0",
			strings.HasPrefix(key, "fallback=Fonts_Font_1,"),
			key == "fallback=Fonts_Font_1",
			strings.HasPrefix(key, "fallback=Fonts_Font_2,"),
			key == "fallback=Fonts_Font_2":
			continue
		}
		kept = append(kept, line)
	}
	kept = append(kept, ConfigLines()...)
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
