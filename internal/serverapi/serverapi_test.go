package serverapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func newTestClient(t *testing.T, base string, attempts int) *Client {
	t.Helper()
	client, err := New(Options{
		BaseURL:   base,
		Attempts:  attempts,
		RetryWait: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func TestNewValidatesPanelURL(t *testing.T) {
	valid := []string{
		"https://203.0.113.10",
		"https://panel.example",
		"http://127.0.0.1:8080",
		"http://localhost:8080/",
	}
	for _, value := range valid {
		if _, err := New(Options{BaseURL: value}); err != nil {
			t.Errorf("New(%q): %v", value, err)
		}
	}
	invalid := []string{
		"",
		"http://203.0.113.10",
		"ftp://203.0.113.10",
		"https://user@panel.example",
		"https://panel.example/api",
		"https://panel.example?x=1",
		"https://panel.example#x",
	}
	for _, value := range invalid {
		if _, err := New(Options{BaseURL: value}); err == nil {
			t.Errorf("New(%q) accepted invalid URL", value)
		}
	}
	if _, err := New(Options{
		BaseURL:  "https://panel.example",
		Attempts: -1,
	}); err == nil {
		t.Fatal("negative attempts accepted")
	}
}

func TestServerAndMods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request,
	) {
		switch r.URL.Path {
		case ServerPath:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w,
				`{"name":"Friends","address":"203.0.113.10","port":25565,"status":"running","mods_sha256":%q,"panel_version":"0.1.0"}`,
				testSHA)
		case ModsPath:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w,
				`{"name":"mods-2026-10-23_12-00-00.tar.gz","sha256":%q,"size":42,"url":%q}`,
				testSHA, ModsArchivePath)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 1)

	info, err := client.Server(context.Background())
	if err != nil {
		t.Fatalf("Server: %v", err)
	}
	if info.Name != "Friends" || info.Port != 25565 ||
		info.ModsSHA256 == nil || *info.ModsSHA256 != testSHA {
		t.Fatalf("server = %+v", info)
	}
	mods, err := client.Mods(context.Background())
	if err != nil {
		t.Fatalf("Mods: %v", err)
	}
	if mods.Name != "mods-2026-10-23_12-00-00.tar.gz" ||
		mods.Size != 42 || mods.SHA256 != testSHA {
		t.Fatalf("mods = %+v", mods)
	}
	resolved, err := client.ArchiveURL(mods)
	if err != nil {
		t.Fatalf("ArchiveURL: %v", err)
	}
	if resolved.Path != ModsArchivePath {
		t.Fatalf("archive URL = %s", resolved)
	}
}

func TestModsNoModsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w,
			`{"error":{"code":"no_mods","message":"Текущий архив модов не найден"}}`)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 1)
	_, err := client.Mods(context.Background())
	if !errors.Is(err, ErrNoMods) {
		t.Fatalf("Mods error = %v, want ErrNoMods", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("APIError = %#v", apiErr)
	}
}

func TestModsRejectsInvalidMetadata(t *testing.T) {
	bodies := []string{
		`{"name":"mods-2026-10-23_12-00-00.tar.gz","sha256":"bad","size":42,"url":"/api/v1/mods/current/archive"}`,
		`{"name":"state-2026-10-23_12-00-00.tar.gz","sha256":"` + testSHA + `","size":42,"url":"/api/v1/mods/current/archive"}`,
		`{"name":"mods-2026-10-23_12-00-00.tar.gz","sha256":"` + testSHA + `","size":0,"url":"/api/v1/mods/current/archive"}`,
		`{"name":"mods-2026-10-23_12-00-00.tar.gz","sha256":"` + testSHA + `","size":42,"url":"https://evil.example/api/v1/mods/current/archive"}`,
		`{"name":"mods-2026-10-23_12-00-00.tar.gz","sha256":"` + testSHA + `","size":42,"url":"/other"}`,
	}
	for _, body := range bodies {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
			_ *http.Request,
		) {
			fmt.Fprint(w, body)
		}))
		client := newTestClient(t, server.URL, 1)
		_, err := client.Mods(context.Background())
		server.Close()
		if err == nil {
			t.Errorf("Mods accepted %s", body)
		} else if !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("Mods(%s) error = %v, want ErrInvalidResponse", body, err)
		}
	}
}

func TestServerRejectsMalformedAndRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 2)
	_, err := client.Server(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty address") {
		t.Fatalf("Server error = %v", err)
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("Server error = %v, want ErrInvalidResponse", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}

	body := `{"name":"x","address":"host","port":25565,"status":"running"} trailing`
	trailing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		fmt.Fprint(w, body)
	}))
	defer trailing.Close()
	if _, err := newTestClient(t, trailing.URL, 1).Server(
		context.Background()); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestServerAcceptsEmptyName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		fmt.Fprint(w,
			`{"name":"","address":"203.0.113.10","port":25565,"status":"running"}`)
	}))
	defer server.Close()

	info, err := newTestClient(t, server.URL, 1).Server(context.Background())
	if err != nil {
		t.Fatalf("Server: %v", err)
	}
	if info.Name != "" || info.Address != "203.0.113.10" {
		t.Fatalf("server = %+v", info)
	}
}

func TestClientRejectsCrossHostRedirect(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		fmt.Fprint(w, `{"name":"x","address":"host","port":25565,"status":"running"}`)
	}))
	defer evil.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request,
	) {
		http.Redirect(w, r, evil.URL+ServerPath, http.StatusFound)
	}))
	defer redirector.Close()
	if _, err := newTestClient(t, redirector.URL, 1).Server(
		context.Background()); err == nil {
		t.Fatal("cross-host redirect accepted")
	}
}

func TestArchiveURLRejectsArbitraryTargets(t *testing.T) {
	client := newTestClient(t, "https://panel.example", 1)
	bad := []string{
		"",
		"https://evil.example/api/v1/mods/current/archive",
		"http://panel.example/api/v1/mods/current/archive",
		"/api/v1/mods/current/archive?x=1",
		"/api/v1/mods/current/archive#x",
		"/api/v1/server",
	}
	for _, raw := range bad {
		_, err := client.ArchiveURL(Mods{URL: raw})
		if err == nil {
			t.Errorf("ArchiveURL(%q) accepted", raw)
			continue
		}
		if !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("ArchiveURL(%q) error = %v, want ErrInvalidResponse", raw, err)
		}
	}
}

func TestDownloadModsVerifiesSizeAndReportsProgress(t *testing.T) {
	body := []byte("synthetic tar.gz body")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 1)
	mods := Mods{
		Name:   "mods-2026-10-23_12-00-00.tar.gz",
		SHA256: testSHA,
		Size:   int64(len(body)),
		URL:    ModsArchivePath,
	}
	dest := filepath.Join(t.TempDir(), "cache", mods.Name)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		t.Fatal(err)
	}
	var progress DownloadProgress
	written, err := client.DownloadMods(context.Background(), mods, dest,
		func(update DownloadProgress) { progress = update })
	if err != nil {
		t.Fatalf("DownloadMods: %v", err)
	}
	if written != int64(len(body)) || progress.Downloaded != int64(len(body)) ||
		progress.Total != int64(len(body)) {
		t.Fatalf("written/progress = %d %+v", written, progress)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(body) {
		t.Fatalf("downloaded body = %q, %v", got, err)
	}
}

func TestDownloadModsRejectsWrongContentLength(t *testing.T) {
	body := []byte("short")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request,
	) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)+10))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 1)
	mods := Mods{
		Name:   "mods-2026-10-23_12-00-00.tar.gz",
		SHA256: testSHA,
		Size:   int64(len(body)),
		URL:    ModsArchivePath,
	}
	dest := filepath.Join(t.TempDir(), mods.Name)
	if _, err := client.DownloadMods(context.Background(), mods, dest, nil); err == nil {
		t.Fatal("wrong Content-Length accepted")
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial archive exists: %v", err)
	}
	if _, err := os.Stat(dest + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial temp file exists: %v", err)
	}
}
