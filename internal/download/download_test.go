package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func testOptions(t *testing.T, serverURL string, payload []byte) Options {
	t.Helper()
	return Options{
		URL:      serverURL,
		SHA256:   digest(payload),
		Dest:     filepath.Join(t.TempDir(), "asset.bin"),
		Attempts: 1,
	}
}

func TestFetchDownloadsAndCaches(t *testing.T) {
	payload := []byte("verified asset")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		requests++
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, payload)
	first, err := Fetch(context.Background(), options)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !first.Downloaded || first.Cached || first.Size != int64(len(payload)) {
		t.Fatalf("first result = %+v", first)
	}
	body, err := os.ReadFile(options.Dest)
	if err != nil || string(body) != string(payload) {
		t.Fatalf("destination = %q, %v", body, err)
	}

	second, err := Fetch(context.Background(), options)
	if err != nil {
		t.Fatalf("cached Fetch: %v", err)
	}
	if second.Downloaded || !second.Cached {
		t.Fatalf("second result = %+v", second)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestFetchReplacesStaleCache(t *testing.T) {
	payload := []byte("new asset")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, payload)
	if err := os.WriteFile(options.Dest, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale cache: %v", err)
	}
	result, err := Fetch(context.Background(), options)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !result.Downloaded {
		t.Fatalf("result = %+v, want download", result)
	}
	body, err := os.ReadFile(options.Dest)
	if err != nil || string(body) != string(payload) {
		t.Fatalf("destination = %q, %v", body, err)
	}
}

func TestFetchRejectsBadDigest(t *testing.T) {
	payload := []byte("payload")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, payload)
	options.SHA256 = strings.Repeat("0", 64)
	_, err := Fetch(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "SHA256 mismatch") {
		t.Fatalf("error = %v, want SHA256 mismatch", err)
	}
	if _, statErr := os.Stat(options.Dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("bad download left %s behind: %v", options.Dest, statErr)
	}
}

func TestFetchDoesNotRetryClientErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		requests++
		http.NotFound(writer, request)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, nil)
	options.Attempts = 3
	_, err := Fetch(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %v, want 404", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestFetchRetriesServerErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		requests++
		http.Error(writer, "offline", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, nil)
	options.Attempts = 2
	_, err := Fetch(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "2 attempt") {
		t.Fatalf("error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestFetchRejectsOversizeContentLength(t *testing.T) {
	payload := []byte("0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Length", "10")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, payload)
	options.MaxSize = 4
	_, err := Fetch(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "limit is 4") {
		t.Fatalf("error = %v, want size limit", err)
	}
}

func TestFetchRejectsOversizeBody(t *testing.T) {
	payload := []byte("0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.(http.Flusher).Flush()
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	options := testOptions(t, server.URL, payload)
	options.MaxSize = 4
	_, err := Fetch(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "exceeds 4 bytes") {
		t.Fatalf("error = %v, want streaming size limit", err)
	}
}

func TestFetchRequiresHTTPSOutsideLoopback(t *testing.T) {
	_, err := Fetch(context.Background(), Options{
		URL:    "http://example.com/asset",
		SHA256: strings.Repeat("0", 64),
		Dest:   filepath.Join(t.TempDir(), "asset.bin"),
	})
	if err == nil || !strings.Contains(err.Error(), "requires HTTPS") {
		t.Fatalf("error = %v, want HTTPS requirement", err)
	}
}
