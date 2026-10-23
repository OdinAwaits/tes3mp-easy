package tes3mp

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

type zipEntry struct {
	name string
	body string
	mode os.FileMode
}

func makeZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fakeRelease(t *testing.T, entries ...zipEntry) ([]byte, string) {
	t.Helper()
	data := makeZip(t, entries)
	return data, digest(data)
}

func testOptions(t *testing.T, serverURL string, data []byte, sum string) Options {
	t.Helper()
	root := t.TempDir()
	options := DefaultOptions(filepath.Join(root, "work", "tes3mp"),
		filepath.Join(root, "cache"))
	options.URL = serverURL + "/tes3mp.zip"
	options.SHA256 = sum
	options.Version = "test-1"
	options.RetryWait = time.Millisecond
	return options
}

func TestInspect(t *testing.T) {
	root := t.TempDir()
	state, err := Inspect(filepath.Join(root, "tes3mp"))
	if err != nil || state.Installed || state.Current {
		t.Fatalf("missing state = %+v, %v", state, err)
	}

	dir := filepath.Join(root, "tes3mp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, Executable), []byte("exe"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err = Inspect(dir)
	if err != nil || !state.Installed || state.Current {
		t.Fatalf("unversioned state = %+v, %v", state, err)
	}
	if err := os.WriteFile(filepath.Join(dir, VersionFile),
		[]byte(Version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err = Inspect(dir)
	if err != nil || !state.Installed || !state.Current {
		t.Fatalf("current state = %+v, %v", state, err)
	}
}

func TestInspectRejectsInvalidPaths(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(file); err == nil {
		t.Fatal("Inspect accepted a file")
	}
	if _, err := Inspect(""); err == nil {
		t.Fatal("Inspect accepted an empty path")
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "link")
		if err := os.Symlink(root, link); err != nil {
			t.Fatal(err)
		}
		if _, err := Inspect(link); err == nil {
			t.Fatal("Inspect accepted a symlink")
		}
	}
}

func TestInstallAndSkip(t *testing.T) {
	data, sum := fakeRelease(t,
		zipEntry{name: "tes3mp.exe", body: "exe", mode: 0o755},
		zipEntry{name: "resources/data.bin", body: "data", mode: 0o644},
	)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, data, sum)
	var stages []Stage
	options.Progress = func(progress Progress) {
		if len(stages) == 0 || stages[len(stages)-1] != progress.Stage {
			stages = append(stages, progress.Stage)
		}
	}
	result, err := Install(t.Context(), options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Skipped || !result.Downloaded || result.Cached {
		t.Fatalf("result = %+v", result)
	}
	if !result.State.Current || result.State.Version != "test-1" {
		t.Fatalf("state = %+v", result.State)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	for _, stage := range []Stage{
		StageDownloading, StageVerifying, StageExtracting, StageInstalling,
	} {
		found := false
		for _, got := range stages {
			if got == stage {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing stage %q in %v", stage, stages)
		}
	}
	got, err := os.ReadFile(filepath.Join(options.Dir, "resources", "data.bin"))
	if err != nil || string(got) != "data" {
		t.Fatalf("installed data = %q, %v", got, err)
	}

	server.Close()
	result, err = Install(t.Context(), options)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !result.Skipped || result.Downloaded {
		t.Fatalf("second result = %+v", result)
	}
	if requests != 1 {
		t.Fatalf("skip made a request, requests = %d", requests)
	}
}

func TestInstallUsesCacheWhenOffline(t *testing.T) {
	data, sum := fakeRelease(t,
		zipEntry{name: "tes3mp.exe", body: "exe", mode: 0o755})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	options := testOptions(t, server.URL, data, sum)
	if _, err := Install(t.Context(), options); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	server.Close()
	if err := os.RemoveAll(options.Dir); err != nil {
		t.Fatal(err)
	}

	result, err := Install(t.Context(), options)
	if err != nil {
		t.Fatalf("offline Install: %v", err)
	}
	if !result.Cached || result.Downloaded || !result.State.Current {
		t.Fatalf("result = %+v", result)
	}
}

func TestInstallRetriesServerErrors(t *testing.T) {
	data, sum := fakeRelease(t,
		zipEntry{name: "tes3mp.exe", body: "exe", mode: 0o755})
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, data, sum)
	if _, err := Install(t.Context(), options); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	data, _ := fakeRelease(t,
		zipEntry{name: "tes3mp.exe", body: "exe", mode: 0o755})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, data,
		strings.Repeat("0", sha256.Size*2))
	_, err := Install(t.Context(), options)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Install = %v, want checksum error", err)
	}
	if _, statErr := os.Stat(options.Dir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination exists after failed checksum: %v", statErr)
	}
}

func TestInstallRejectsBadArchives(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipEntry
		want    string
	}{
		{
			name: "no executable",
			entries: []zipEntry{
				{name: "readme.txt", body: "x", mode: 0o644},
			},
			want: "does not contain tes3mp.exe",
		},
		{
			name: "traversal",
			entries: []zipEntry{
				{name: "../escape", body: "x", mode: 0o644},
				{name: "tes3mp.exe", body: "exe", mode: 0o755},
			},
			want: "unsafe path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, sum := fakeRelease(t, test.entries...)
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write(data)
				}))
			defer server.Close()
			options := testOptions(t, server.URL, data, sum)
			_, err := Install(t.Context(), options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Install = %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(options.Dir); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("destination exists after failure: %v", statErr)
			}
			entries, readErr := os.ReadDir(filepath.Dir(options.Dir))
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				if strings.Contains(entry.Name(), ".staging-") {
					t.Fatalf("staging left behind: %s", entry.Name())
				}
			}
		})
	}
}

func TestInstallRejectsHTTPAndInvalidInput(t *testing.T) {
	tests := []Options{
		{},
		{
			Dir: "/tmp/tes3mp", CacheDir: "/tmp/cache",
			URL: "http://example.com/tes3mp.zip",
		},
		{
			Dir: "/tmp/tes3mp", CacheDir: "/tmp/cache",
			URL: "https://example.com/file.tar.gz",
		},
		{
			Dir: "/tmp/tes3mp", CacheDir: "/tmp/cache",
			URL: "https://example.com/x.zip", SHA256: "abc",
		},
		{
			Dir: "/tmp/tes3mp", CacheDir: "/tmp/cache",
			URL: URL, Attempts: -1,
		},
		{
			Dir: "/tmp/tes3mp", CacheDir: "/tmp/cache",
			URL: URL, RetryWait: -1,
		},
	}
	for index, options := range tests {
		if _, err := Install(t.Context(), options); err == nil {
			t.Fatalf("case %d: Install succeeded", index)
		}
	}
}

func TestSwapInstallationRecoversBackup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tes3mp")
	backup := dir + ".old"
	staging := filepath.Join(root, "staging")
	if err := os.MkdirAll(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "new"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := swapInstallation(staging, dir); err != nil {
		t.Fatalf("swapInstallation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("new installation: %v", err)
	}
	if _, err := os.Lstat(backup); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup survived: %v", err)
	}
}

func TestSwapInstallationRejectsBackupSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "tes3mp")
	backup := dir + ".old"
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, backup); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(root, "staging")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := swapInstallation(staging, dir); err == nil {
		t.Fatal("swapInstallation accepted a symlink backup")
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination changed: %v", err)
	}
}

func TestSwapInstallationRestoresOldDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tes3mp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := swapInstallation(filepath.Join(root, "missing"), dir); err == nil {
		t.Fatal("swapInstallation succeeded with missing staging")
	}
	got, err := os.ReadFile(filepath.Join(dir, "old"))
	if err != nil || string(got) != "old" {
		t.Fatalf("old directory = %q, %v", got, err)
	}
}

func TestInstallReplacesOldInstallation(t *testing.T) {
	data, sum := fakeRelease(t,
		zipEntry{name: "tes3mp.exe", body: "new", mode: 0o755})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, data, sum)
	if err := os.MkdirAll(options.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(options.Dir, "old"),
		[]byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(t.Context(), options); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(options.Dir, "old")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file survived installation: %v", err)
	}
	if _, err := os.Stat(options.Dir + ".old"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup survived installation: %v", err)
	}
}

func TestValidateURL(t *testing.T) {
	for _, raw := range []string{
		URL,
		"http://127.0.0.1:8080/tes3mp.zip",
		"http://localhost/tes3mp.zip",
		"http://[::1]/tes3mp.zip",
	} {
		if err := validateURL(raw); err != nil {
			t.Fatalf("validateURL(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{
		"http://example.com/tes3mp.zip",
		"ftp://example.com/tes3mp.zip",
		"https://user:pass@example.com/tes3mp.zip",
		"https://example.com/tes3mp.zip#fragment",
		"not a URL",
	} {
		if err := validateURL(raw); err == nil {
			t.Fatalf("validateURL(%q) succeeded", raw)
		}
	}
}

// TestOfficialArchive is opt-in because the release is about 65 MiB. It checks
// the embedded checksum and the real archive layout without network access:
//
//	TES3MP_ARCHIVE=/path/to/tes3mp.Win64.release.0.8.1.zip go test ./internal/tes3mp
func TestOfficialArchive(t *testing.T) {
	path := os.Getenv("TES3MP_ARCHIVE")
	if path == "" {
		t.Skip("TES3MP_ARCHIVE is not set")
	}
	if err := verifyFile(path, SHA256); err != nil {
		t.Fatalf("official archive checksum: %v", err)
	}
	dest := t.TempDir()
	if err := archive.ExtractZipFileWithLimits(path, dest,
		archive.DefaultZipLimits()); err != nil {
		t.Fatalf("extract official archive: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, Executable))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("official archive has no %s: %v", Executable, err)
	}
}
