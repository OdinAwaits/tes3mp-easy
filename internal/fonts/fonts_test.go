package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func payloadDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func testAssets(t *testing.T, serverURL string) ([]Asset, map[string][]byte) {
	t.Helper()
	bodies := map[string][]byte{
		"Text.ttf":    []byte("text font"),
		"Serif.ttf":   []byte("serif font"),
		"Console.ttf": []byte("console font"),
	}
	assets := make([]Asset, 0, len(bodies))
	for _, name := range []string{"Text.ttf", "Serif.ttf", "Console.ttf"} {
		assets = append(assets, Asset{
			Name:   name,
			URL:    serverURL + "/" + name,
			SHA256: payloadDigest(bodies[name]),
		})
	}
	return assets, bodies
}

func TestUpdateConfigIsIdempotent(t *testing.T) {
	input := []byte("data=\"Data Files\"\nfallback=Fonts_Font_0,Old\n" +
		"content=Morrowind.esm\nfallback=Fonts_Font_1\n")
	first := UpdateConfig(input)
	text := string(first)
	for _, line := range ConfigLines() {
		if !strings.Contains(text, line+"\n") {
			t.Fatalf("config missing %q:\n%s", line, text)
		}
	}
	if strings.Contains(text, "Fonts_Font_0,Old") ||
		strings.Contains(text, "fallback=Fonts_Font_1\n") {
		t.Fatalf("old fallback lines survived:\n%s", text)
	}
	if !strings.Contains(text, "content=Morrowind.esm\n") {
		t.Fatalf("unrelated line was dropped:\n%s", text)
	}
	second := UpdateConfig(first)
	if string(second) != text {
		t.Fatalf("UpdateConfig is not idempotent:\nfirst:\n%s\nsecond:\n%s",
			text, second)
	}
	if count := strings.Count(string(second), "fallback=Fonts_Font_0,"); count != 1 {
		t.Fatalf("fallback count = %d, want 1", count)
	}
}

func TestFontXMLHasLatinAndCyrillicRanges(t *testing.T) {
	xml := string(FontXML())
	for _, want := range []string{
		`<Resource type="ResourceTrueTypeFont" name="Magic Cards">`,
		`<Code range="1040 1103"/>`,
		`<Code range="1025 1105"/>`,
		`<Property key="Source" value="NotoSans.ttf"/>`,
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("FontXML missing %q:\n%s", want, xml)
		}
	}
}

func TestInstallWritesFontsAndXML(t *testing.T) {
	var requested []string
	assets, bodies := testAssets(t, "")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		name := strings.TrimPrefix(request.URL.Path, "/")
		requested = append(requested, name)
		body, ok := bodies[name]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(body)
	}))
	defer server.Close()

	assets, _ = testAssets(t, server.URL)
	configDir := t.TempDir()
	options := Options{
		ConfigDir: configDir,
		CacheDir:  t.TempDir(),
		Assets:    assets,
		Client:    server.Client(),
		Attempts:  1,
	}
	result, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !result.Downloaded || result.Cached {
		t.Fatalf("result = %+v", result)
	}
	if !result.State.Installed {
		t.Fatalf("state = %+v, want installed", result.State)
	}
	if len(requested) != 3 {
		t.Fatalf("requested = %v, want three fonts", requested)
	}
	for name, body := range bodies {
		installed, err := os.ReadFile(filepath.Join(configDir, "Fonts", name))
		if err != nil || string(installed) != string(body) {
			t.Fatalf("font %s = %q, %v", name, installed, err)
		}
	}
	xmlPath := filepath.Join(configDir, "Fonts", XMLName)
	xml, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatalf("read %s: %v", xmlPath, err)
	}
	if result.State.XMLPath != xmlPath {
		t.Fatalf("XMLPath = %q, want %q", result.State.XMLPath, xmlPath)
	}
	if _, err := os.Stat(filepath.Join(configDir, XMLName)); !os.IsNotExist(err) {
		t.Fatalf("%s must not be written to the OpenMW config root", XMLName)
	}

	if !strings.Contains(string(xml), `value="Serif.ttf"`) {
		t.Fatalf("generated XML does not mention custom assets:\n%s", xml)
	}

	second, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if second.Downloaded || !second.Cached || !second.State.Installed {
		t.Fatalf("second result = %+v", second)
	}
}

func TestInstallWarnsWhenAFontFails(t *testing.T) {
	assets, bodies := testAssets(t, "")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		name := strings.TrimPrefix(request.URL.Path, "/")
		if name == "Console.ttf" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(bodies[name])
	}))
	defer server.Close()
	assets, _ = testAssets(t, server.URL)

	configDir := t.TempDir()
	result, err := Install(context.Background(), Options{
		ConfigDir: configDir,
		CacheDir:  t.TempDir(),
		Assets:    assets,
		Client:    server.Client(),
		Attempts:  1,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.State.Installed {
		t.Fatalf("state = %+v, want incomplete", result.State)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings (%d) = %q", len(result.Warnings), result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], "Console.ttf") ||
		!strings.Contains(result.Warnings[0], "404") ||
		!strings.Contains(result.Warnings[1], "Console.ttf is not installed") {
		t.Fatalf("warnings = %q", result.Warnings)
	}
	if _, err := os.Stat(filepath.Join(configDir, "Fonts", XMLName)); !os.IsNotExist(err) {
		t.Fatalf("%s was written for an incomplete font set", XMLName)
	}
}

func TestInspectRejectsModifiedFont(t *testing.T) {
	fontsDir := t.TempDir()
	assets, bodies := testAssets(t, "")
	for name, body := range bodies {
		if err := os.WriteFile(filepath.Join(fontsDir, name), body, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	state, err := inspect(fontsDir, assets)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !state.Installed {
		t.Fatalf("state = %+v", state)
	}
	if err := os.WriteFile(filepath.Join(fontsDir, "Text.ttf"),
		[]byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	state, err = inspect(fontsDir, assets)
	if err != nil {
		t.Fatalf("inspect modified: %v", err)
	}
	if state.Installed || len(state.Invalid) != 1 || state.Invalid[0] != "Text.ttf" {
		t.Fatalf("state = %+v, want Text.ttf invalid", state)
	}
}
