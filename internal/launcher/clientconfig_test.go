package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepareClientConfigRewritesEndpointAndPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ClientConfigName)
	original := "[General]\r\n" +
		"destinationAddress = localhost\r\n" +
		"port = 25565\r\n" +
		"password =\r\n" +
		"# keep this comment\r\n" +
		"logLevel = 0\r\n" +
		"\r\n[Chat]\r\nkeySay = Y\r\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	result, err := PrepareClientConfig(ClientConfigOptions{
		TES3MPDir: dir,
		Address:   "203.0.113.10",
		Port:      25570,
		Password:  "hunter2",
		Now: func() time.Time {
			return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("PrepareClientConfig: %v", err)
	}
	if !result.Created || result.Backup != path+".bak-20261024-120000" {
		t.Fatalf("result = %+v", result)
	}
	backup, err := os.ReadFile(result.Backup)
	if err != nil || string(backup) != original {
		t.Fatalf("backup = %q, %v", backup, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"destinationAddress = 203.0.113.10\r\n",
		"port = 25570\r\n",
		"password = hunter2\r\n",
		"# keep this comment\r\n",
		"[Chat]\r\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "destinationAddress = localhost") {
		t.Fatalf("old address was not replaced:\n%s", text)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v", info, err)
	}
}

func TestPrepareClientConfigIsIdempotentAndCreatesMissingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ClientConfigName)
	if err := os.WriteFile(path, []byte("[General]\nlogLevel = 1\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	options := ClientConfigOptions{
		TES3MPDir: dir,
		Address:   "example.com",
		Port:      25565,
		Password:  "secret",
	}
	first, err := PrepareClientConfig(options)
	if err != nil {
		t.Fatalf("first PrepareClientConfig: %v", err)
	}
	if !first.Created {
		t.Fatal("first call did not write")
	}
	second, err := PrepareClientConfig(options)
	if err != nil {
		t.Fatalf("second PrepareClientConfig: %v", err)
	}
	if second.Created || second.Backup != "" {
		t.Fatalf("second result = %+v", second)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, want := range []string{
		"[General]\n",
		"logLevel = 1\n",
		"destinationAddress = example.com\n",
		"port = 25565\n",
		"password = secret\n",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("config does not contain %q:\n%s", want, data)
		}
	}
}

func TestPrepareClientConfigCreatesGeneralSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ClientConfigName)
	if err := os.WriteFile(path, []byte("[Chat]\nkeySay = Y\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := PrepareClientConfig(ClientConfigOptions{
		TES3MPDir: dir,
		Address:   "example.com",
		Port:      25565,
		Password:  "secret",
	}); err != nil {
		t.Fatalf("PrepareClientConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"[Chat]\nkeySay = Y\n",
		"[General]\ndestinationAddress = example.com\n",
		"port = 25565\n",
		"password = secret\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config does not contain %q:\n%s", want, text)
		}
	}
}

func TestPrepareClientConfigNeverLeaksPasswordInErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := PrepareClientConfig(ClientConfigOptions{
		TES3MPDir: dir,
		Address:   "example.com",
		Port:      0,
		Password:  "hunter2",
	})
	if err == nil {
		t.Fatal("invalid port was accepted")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error leaks password: %v", err)
	}
}
