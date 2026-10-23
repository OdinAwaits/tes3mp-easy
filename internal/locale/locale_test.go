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

func profile(t *testing.T) (string, string) {
	t.Helper()
	workDir := t.TempDir()
	dataFiles := filepath.Join(workDir, "servers", "srv", "data", "Data Files")
	if err := os.MkdirAll(filepath.Join(dataFiles, "Book"), 0o750); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataFiles, "Morrowind.esm"),
		[]byte("original morrowind"), 0o640); err != nil {
		t.Fatalf("write Morrowind.esm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataFiles, "Keep.esp"),
		[]byte("untouched mod"), 0o640); err != nil {
		t.Fatalf("write Keep.esp: %v", err)
	}
	return workDir, dataFiles
}

func TestInstallApplyRemoveLifecycle(t *testing.T) {
	workDir, dataFiles := profile(t)
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
		ServerID:     "srv",
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
	for _, path := range []string{
		"Data Files/Morrowind.esm",
		"Data Files/Book/Book.esp",
		"Data Files/Sound/Step.wav",
		"Data Files/Video/Intro.bik",
	} {
		full := filepath.Join(workDir, "servers", "srv", "localization", "main",
			filepath.FromSlash(path))
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("main layer missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, "servers", "srv", "localization",
		"main", "Data Files", "Documentation")); !os.IsNotExist(err) {
		t.Fatalf("top-level filtered directory leaked into the layer: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "servers", "srv", "localization",
		"voices", "Data Files", "Sound", "Voice.mp3")); err != nil {
		t.Fatalf("voice layer missing: %v", err)
	}

	applied, err := Apply(workDir, "srv")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Files != 5 {
		t.Fatalf("applied = %+v, want 5 files", applied)
	}
	body, err := os.ReadFile(filepath.Join(dataFiles, "Morrowind.esm"))
	if err != nil || string(body) != "localized morrowind" {
		t.Fatalf("localized Morrowind.esm = %q, %v", body, err)
	}
	if body, _ := os.ReadFile(filepath.Join(dataFiles, "Keep.esp")); string(body) !=
		"untouched mod" {
		t.Fatalf("unrelated profile file = %q", body)
	}

	warnings, err := RemoveOverlay(workDir, "srv")
	if err != nil {
		t.Fatalf("RemoveOverlay: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	body, err = os.ReadFile(filepath.Join(dataFiles, "Morrowind.esm"))
	if err != nil || string(body) != "original morrowind" {
		t.Fatalf("restored Morrowind.esm = %q, %v", body, err)
	}
	for _, path := range []string{"Book/Book.esp", "Sound/Step.wav",
		"Video/Intro.bik", "Sound/Voice.mp3"} {
		if _, err := os.Stat(filepath.Join(dataFiles, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("overlay file %s survived removal: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, "servers", "srv",
		AppliedName)); !os.IsNotExist(err) {
		t.Fatalf("manifest survived removal: %v", err)
	}
	if err := Remove(workDir, "srv"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestInstallSkipsWhenAlreadyInstalled(t *testing.T) {
	workDir, _ := profile(t)
	mainArchive := tarFile(t, map[string]string{
		"Data Files/Morrowind.esm": "localized",
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		_, _ = writer.Write(mainArchive)
	}))
	defer server.Close()
	options := Options{
		WorkDir:  workDir,
		CacheDir: t.TempDir(),
		ServerID: "srv",
		URL:      server.URL + "/russian.tar",
		SHA256:   archiveDigest(mainArchive),
		Client:   server.Client(),
		Attempts: 1,
	}
	if _, err := Install(context.Background(), options); err != nil {
		t.Fatalf("Install: %v", err)
	}
	second, err := Install(context.Background(), options)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !second.Skipped || !second.State.Installed {
		t.Fatalf("second result = %+v", second)
	}
}

func TestInstallWarnsWhenVoicesFail(t *testing.T) {
	workDir, _ := profile(t)
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
		ServerID:     "srv",
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

func TestUpdateEncoding(t *testing.T) {
	input := []byte("encoding=win1252\ncontent=Morrowind.esm\nencoding=win1251\n")
	enabled := UpdateEncoding(input, true)
	text := string(enabled)
	if strings.Count(text, "encoding=") != 1 ||
		!strings.Contains(text, "encoding=win1251\n") {
		t.Fatalf("enabled config:\n%s", text)
	}
	if !strings.Contains(text, "content=Morrowind.esm\n") {
		t.Fatalf("enabled config dropped unrelated line:\n%s", text)
	}
	disabled := UpdateEncoding(enabled, false)
	if strings.Contains(string(disabled), "encoding=") {
		t.Fatalf("disabled config still has encoding:\n%s", disabled)
	}
	if got := string(UpdateEncoding(input, false)); got !=
		"encoding=win1252\ncontent=Morrowind.esm\n" {
		t.Fatalf("disable should keep a non-Russian encoding: %q", got)
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
