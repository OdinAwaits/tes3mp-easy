package locale

import (
	"archive/tar"
	"bytes"
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

func archiveDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func tarFile(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, body := range entries {
		header := &tar.Header{
			Name:     name,
			Mode:     0o640,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if strings.HasSuffix(name, "/") {
			header.Mode = 0o750
			header.Typeflag = tar.TypeDir
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %q: %v", name, err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := writer.Write([]byte(body)); err != nil {
				t.Fatalf("write tar body %q: %v", name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buffer.Bytes()
}

func TestInstallRemoveLifecycle(t *testing.T) {
	workDir := t.TempDir()
	mainArchive := tarFile(t, map[string]string{
		"Data Files/":              "",
		"Data Files/Morrowind.esm": "localized morrowind",
		"Data Files/Book/Book.esp": "localized book",
		"Sound/":                   "",
		"Sound/Step.wav":           "footstep",
		"Video/":                   "",
		"Video/Intro.bik":          "intro",
		"Documentation/Readme.txt": "ignored",
	})
	voiceArchive := tarFile(t, map[string]string{
		"Sound/Voice.mp3": "voice line",
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		switch request.URL.Path {
		case "/russian.tar":
			_, _ = writer.Write(mainArchive)
		case "/voices.tar":
			_, _ = writer.Write(voiceArchive)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	options := Options{
		WorkDir:      workDir,
		CacheDir:     t.TempDir(),
		Voiceover:    true,
		URL:          server.URL + "/russian.tar",
		SHA256:       archiveDigest(mainArchive),
		VoicesURL:    server.URL + "/voices.tar",
		VoicesSHA256: archiveDigest(voiceArchive),
		Client:       server.Client(),
		Attempts:     1,
	}
	result, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !result.State.Installed || !result.State.Voiceover {
		t.Fatalf("state = %+v", result.State)
	}
	if !result.Downloaded || result.Cached {
		t.Fatalf("result = %+v", result)
	}
	root := filepath.Join(workDir, DirName)
	if result.State.Dir != root ||
		result.State.DataFiles != filepath.Join(root, mainDir, "Data Files") {
		t.Fatalf("state paths = %+v", result.State)
	}
	for _, path := range []string{
		"Data Files/Morrowind.esm",
		"Data Files/Book/Book.esp",
		"Data Files/Sound/Step.wav",
		"Data Files/Video/Intro.bik",
	} {
		full := filepath.Join(root, mainDir, filepath.FromSlash(path))
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("main layer missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, mainDir, "Data Files",
		"Documentation")); !os.IsNotExist(err) {
		t.Fatalf("top-level filtered directory leaked into the layer: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, voicesDir, "Data Files",
		"Sound", "Voice.mp3")); err != nil {
		t.Fatalf("voice layer missing: %v", err)
	}

	second, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !second.Skipped || !second.State.Installed {
		t.Fatalf("second result = %+v", second)
	}
	if err := Remove(workDir); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("localization directory survived removal: %v", err)
	}
	if _, err := os.Stat(root + ".staging"); !os.IsNotExist(err) {
		t.Fatalf("staging directory survived removal: %v", err)
	}
}

func TestInstallWarnsWhenVoicesFail(t *testing.T) {
	workDir := t.TempDir()
	mainArchive := tarFile(t, map[string]string{
		"Data Files/Morrowind.esm": "localized",
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path == "/voices.tar" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(mainArchive)
	}))
	defer server.Close()
	options := Options{
		WorkDir:      workDir,
		CacheDir:     t.TempDir(),
		Voiceover:    true,
		URL:          server.URL + "/russian.tar",
		SHA256:       archiveDigest(mainArchive),
		VoicesURL:    server.URL + "/voices.tar",
		VoicesSHA256: archiveDigest(mainArchive),
		Client:       server.Client(),
		Attempts:     1,
	}
	result, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.State.Voiceover {
		t.Fatalf("state = %+v, want main-only install", result.State)
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "voiceover was not installed") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestLanguagePackages(t *testing.T) {
	packs := Packages()
	if len(packs) != 1 || packs[0].Language != LanguageRussian {
		t.Fatalf("Packages() = %+v", packs)
	}
	russian, ok := PackFor(LanguageRussian)
	if !ok || russian.ArchiveName != ArchiveName ||
		russian.Encoding != Encoding ||
		russian.URL == "" || russian.SHA256 != ArchiveSHA256 {
		t.Fatalf("Russian pack = %+v, %v", russian, ok)
	}
	if _, ok := PackFor("english"); ok {
		t.Fatal("English is not a downloadable package")
	}
	if _, ok := PackFor("klingon"); ok {
		t.Fatal("unsupported language accepted")
	}
	if got := EncodingFor(LanguageRussian); got != Encoding {
		t.Fatalf("Russian encoding = %q", got)
	}
	options := DefaultOptions("/work", "/cache", "", true)
	if options.Language != LanguageRussian || options.URL != russian.URL ||
		!options.Voiceover {
		t.Fatalf("DefaultOptions() = %+v", options)
	}
}

func TestUpdateEncoding(t *testing.T) {
	input := []byte("encoding=win1252\ncontent=Morrowind.esm\nencoding=win1251\n")
	enabled := UpdateEncoding(input, Encoding)
	text := string(enabled)
	if strings.Count(text, "encoding=") != 1 ||
		!strings.Contains(text, "encoding=win1251\n") {
		t.Fatalf("enabled config:\n%s", text)
	}
	if !strings.Contains(text, "content=Morrowind.esm\n") {
		t.Fatalf("enabled config dropped unrelated line:\n%s", text)
	}
	disabled := UpdateEncoding(enabled, "")
	if strings.Contains(string(disabled), "encoding=win1251") {
		t.Fatalf("disabled config still has pack encoding:\n%s", disabled)
	}
	if got := string(UpdateEncoding(input, "")); got !=
		"encoding=win1252\ncontent=Morrowind.esm\n" {
		t.Fatalf("disable should keep a non-pack encoding: %q", got)
	}
}

func TestUpdateDataFiles(t *testing.T) {
	root := "/work/localization"
	overlay := filepath.Join(root, mainDir, "Data Files")
	input := []byte("data=\"/games/Morrowind/Data Files\"\n" +
		"data=\"/work/servers/srv/data/Data Files\"\ncontent=Morrowind.esm\n")
	got := string(UpdateDataFiles(input, overlay, root))
	want := "data=\"/games/Morrowind/Data Files\"\n" +
		"data=\"/work/localization/main/Data Files\"\n" +
		"data=\"/work/servers/srv/data/Data Files\"\ncontent=Morrowind.esm\n"
	if got != want {
		t.Fatalf("UpdateDataFiles() =\n%s\nwant:\n%s", got, want)
	}
	again := string(UpdateDataFiles([]byte(got), overlay, root))
	if again != want {
		t.Fatalf("UpdateDataFiles is not idempotent:\n%s", again)
	}
	disabled := string(UpdateDataFiles([]byte(got), "", root))
	if strings.Contains(disabled, root) {
		t.Fatalf("managed data line survived disable:\n%s", disabled)
	}
}

func TestMapEntryKeepsTopLevelDataDirectories(t *testing.T) {
	cases := []struct {
		name   string
		dir    bool
		target string
		ok     bool
	}{
		{"Data Files/", true, "", true},
		{"Data Files/Book/Book.esp", false, "Book/Book.esp", true},
		{"Sound/", true, "", true},
		{"Sound/Voice.mp3", false, "Sound/Voice.mp3", true},
		{"Video/Intro.bik", false, "Video/Intro.bik", true},
		{"Music/Theme.mp3", false, "Music/Theme.mp3", true},
		{"Documentation/Readme.txt", false, "", false},
	}
	for _, test := range cases {
		target, ok, err := mapEntry(test.name, test.dir)
		if err != nil {
			t.Fatalf("mapEntry(%q): %v", test.name, err)
		}
		if target != test.target || ok != test.ok {
			t.Fatalf("mapEntry(%q) = %q, %v; want %q, %v",
				test.name, target, ok, test.target, test.ok)
		}
	}
}

func TestPackMetadataDescribesBothVariants(t *testing.T) {
	pack, ok := PackFor(LanguageRussian)
	if !ok {
		t.Fatal("the Russian pack is missing")
	}
	if !pack.HasVoices || pack.VoicesSize <= 0 || pack.TextSize <= 0 {
		t.Fatalf("pack metadata = %+v", pack)
	}
	if got := pack.DownloadSize(false); got != pack.TextSize {
		t.Fatalf("text download size = %d, want %d", got, pack.TextSize)
	}
	if got := pack.DownloadSize(true); got != pack.TextSize+pack.VoicesSize {
		t.Fatalf("voice download size = %d, want %d",
			got, pack.TextSize+pack.VoicesSize)
	}
	quiet := Pack{TextSize: pack.TextSize}
	if got := quiet.DownloadSize(true); got != quiet.TextSize {
		t.Fatalf("pack without voices reported %d, want %d",
			got, quiet.TextSize)
	}
}
