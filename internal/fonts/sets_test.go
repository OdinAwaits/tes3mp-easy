package fonts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogIsComplete(t *testing.T) {
	catalog := Sets()
	if len(catalog) != 3 {
		t.Fatalf("len(Sets()) = %d, want 3 sets in P1", len(catalog))
	}
	if ids := SetIDs(); len(ids) != 3 || ids[0] != DefaultSet {
		t.Fatalf("SetIDs() = %v, want %s first", ids, DefaultSet)
	}
	seenSets := make(map[string]bool, len(catalog))
	seenNames := make(map[string]string, len(catalog)*3)
	for _, set := range catalog {
		if set.ID == "" {
			t.Fatal("set without id")
		}
		if seenSets[set.ID] {
			t.Fatalf("duplicate set %q", set.ID)
		}
		seenSets[set.ID] = true
		if len(set.Assets) != 3 {
			t.Fatalf("set %s has %d assets, want 3", set.ID, len(set.Assets))
		}
		for _, asset := range set.Assets {
			if !strings.HasSuffix(asset.Name, ".ttf") {
				t.Fatalf("%s: %s is not a .ttf", set.ID, asset.Name)
			}
			if owner, ok := seenNames[asset.Name]; ok {
				t.Fatalf("font %s is shared by %s and %s", asset.Name, owner,
					set.ID)
			}
			seenNames[asset.Name] = set.ID
			if len(asset.SHA256) != 64 ||
				strings.ToLower(asset.SHA256) != asset.SHA256 {
				t.Fatalf("%s: SHA256 %q is not 64 lowercase hex characters",
					asset.Name, asset.SHA256)
			}
			if !strings.HasPrefix(asset.URL, "https://") {
				t.Fatalf("%s: URL %q is not HTTPS", asset.Name, asset.URL)
			}
		}
	}
	if len(ManagedAssetNames()) != len(seenNames) {
		t.Fatalf("ManagedAssetNames() = %v", ManagedAssetNames())
	}
}

func TestDefaultSetKeepsLegacyAssets(t *testing.T) {
	set, ok := LookupSet(DefaultSet)
	if !ok {
		t.Fatalf("LookupSet(%q) failed", DefaultSet)
	}
	for index, asset := range set.Assets {
		if asset != Assets[index] {
			t.Fatalf("asset %d = %+v, want %+v", index, asset, Assets[index])
		}
	}
	if ResolveSet("").ID != DefaultSet || ResolveSet("unknown").ID != DefaultSet {
		t.Fatalf("ResolveSet must fall back to %s", DefaultSet)
	}
	if _, ok := LookupSet("  INTER "); !ok {
		t.Fatal("LookupSet must ignore case and spaces")
	}
	if !KnownSet("") {
		t.Fatal("empty set must be known")
	}
	if KnownSet("bogus") {
		t.Fatal("bogus set must be unknown")
	}
}

func TestInstallRejectsUnknownSet(t *testing.T) {
	_, err := Install(context.Background(), Options{
		ConfigDir: t.TempDir(),
		CacheDir:  t.TempDir(),
		Set:       "bogus",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown font set") {
		t.Fatalf("Install error = %v, want unknown font set", err)
	}
}

func TestUpdateConfigForSwitchesFamily(t *testing.T) {
	input := []byte("fallback=Fonts_Font_0,NotoSans\nfallback=Fonts_Font_1\n" +
		"fallback=other=1\n")
	first := UpdateConfigFor(input, SetInter)
	text := string(first)
	if strings.Contains(text, "NotoSans") {
		t.Fatalf("old family survived:\n%s", text)
	}
	if !strings.Contains(text, "fallback=Fonts_Font_0,Inter\n") ||
		!strings.Contains(text, "fallback=Fonts_Font_2,JetBrainsMono\n") {
		t.Fatalf("inter fallback lines are missing:\n%s", text)
	}
	if !strings.Contains(text, "fallback=other=1\n") {
		t.Fatalf("unrelated line was dropped:\n%s", text)
	}
	if second := string(UpdateConfigFor(first, SetInter)); second != text {
		t.Fatalf("UpdateConfigFor is not idempotent:\n%s", second)
	}
	if lines := ConfigLinesFor("bogus"); len(lines) != 3 ||
		!strings.Contains(lines[0], "NotoSans") {
		t.Fatalf("ConfigLinesFor(bogus) = %v, want the default set", lines)
	}
	if xml := string(FontXMLFor(SetPlex)); !strings.Contains(xml,
		`value="IBMPlexSans.ttf"`) {
		t.Fatalf("FontXMLFor(plex) does not name its font:\n%s", xml)
	}
}

// localAssets pins the catalog file names of one set to a test server, so the
// test exercises the real names without touching the network.
func localAssets(t *testing.T, setID, serverURL string,
	bodies map[string][]byte,
) []Asset {
	t.Helper()
	set, ok := LookupSet(setID)
	if !ok {
		t.Fatalf("LookupSet(%q) failed", setID)
	}
	assets := make([]Asset, 0, len(set.Assets))
	for _, asset := range set.Assets {
		body, ok := bodies[asset.Name]
		if !ok {
			t.Fatalf("missing body for %s", asset.Name)
		}
		assets = append(assets, Asset{
			Name:   asset.Name,
			URL:    serverURL + "/" + asset.Name,
			SHA256: payloadDigest(body),
		})
	}
	return assets
}

func TestInstallSwitchesSetAndRemovesStaleFiles(t *testing.T) {
	bodies := map[string][]byte{}
	for _, setID := range SetIDs() {
		set, _ := LookupSet(setID)
		for _, asset := range set.Assets {
			bodies[asset.Name] = []byte("payload of " + asset.Name)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		body, ok := bodies[strings.TrimPrefix(request.URL.Path, "/")]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(body)
	}))
	defer server.Close()

	configDir := t.TempDir()
	cacheDir := t.TempDir()
	fontsDir := filepath.Join(configDir, "Fonts")
	install := func(setID string) Result {
		t.Helper()
		result, err := Install(context.Background(), Options{
			ConfigDir: configDir,
			CacheDir:  cacheDir,
			Set:       setID,
			Assets:    localAssets(t, setID, server.URL, bodies),
			Client:    server.Client(),
			Attempts:  1,
		})
		if err != nil {
			t.Fatalf("Install(%s): %v", setID, err)
		}
		if !result.State.Installed || result.State.Set != setID {
			t.Fatalf("Install(%s) state = %+v", setID, result.State)
		}
		return result
	}

	install(SetNoto)
	// A font the user put into Fonts/ by hand must survive a set switch.
	userFont := filepath.Join(fontsDir, "UserFont.ttf")
	if err := os.WriteFile(userFont, []byte("user"), 0o600); err != nil {
		t.Fatalf("write user font: %v", err)
	}

	install(SetInter)
	for _, old := range Assets {
		if _, err := os.Stat(filepath.Join(fontsDir, old.Name)); !os.IsNotExist(err) {
			t.Fatalf("%s of the previous set is still installed", old.Name)
		}
	}
	inter, _ := LookupSet(SetInter)
	for _, asset := range inter.Assets {
		if _, err := os.Stat(filepath.Join(fontsDir, asset.Name)); err != nil {
			t.Fatalf("%s is not installed: %v", asset.Name, err)
		}
	}
	if _, err := os.Stat(userFont); err != nil {
		t.Fatalf("user font was removed: %v", err)
	}
	xml, err := os.ReadFile(filepath.Join(fontsDir, XMLName))
	if err != nil {
		t.Fatalf("read %s: %v", XMLName, err)
	}
	if !strings.Contains(string(xml), `value="Inter.ttf"`) {
		t.Fatalf("%s still points at the old set:\n%s", XMLName, xml)
	}

	state, err := InspectSet(fontsDir, SetNoto)
	if err != nil {
		t.Fatalf("InspectSet(noto): %v", err)
	}
	if state.Installed || len(state.Missing) != 3 {
		t.Fatalf("state = %+v, want the noto files missing", state)
	}
}
