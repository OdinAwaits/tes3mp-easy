package config

import (
	"strings"
	"testing"
)

func validServer() Server {
	return Server{
		Name:     "Morrowind with friends",
		Address:  "203.0.113.10",
		Port:     25565,
		Password: "secret",
		PanelURL: "https://203.0.113.10",
	}
}

func TestServerID(t *testing.T) {
	tests := []struct {
		address string
		port    int
		want    string
	}{
		{address: "example.com", port: 25565, want: "example.com_25565"},
		{address: "MX.Example.COM", port: 25565, want: "mx.example.com_25565"},
		{address: "2001:db8::1", port: 25565, want: "2001_db8_1_25565"},
		{address: " bad host ", port: 1, want: "bad_host_1"},
		{address: "", port: 25565, want: ""},
		{address: "example.com", port: 0, want: ""},
		{address: "example.com", port: 65536, want: ""},
	}
	for _, test := range tests {
		if got := ServerID(test.address, test.port); got != test.want {
			t.Errorf("ServerID(%q, %d) = %q, want %q",
				test.address, test.port, got, test.want)
		}
	}
}

func TestValidateAddress(t *testing.T) {
	valid := []string{
		"203.0.113.10",
		"2001:db8::1",
		"example.com",
		"localhost",
		"my-server.example",
	}
	for _, address := range valid {
		if err := ValidateAddress(address); err != nil {
			t.Errorf("ValidateAddress(%q) = %v", address, err)
		}
	}

	invalid := map[string]string{
		"":                       "empty",
		"example.com:25565":      "port",
		"https://example.com":    "scheme",
		"example.com/path":       "path",
		"[::1]":                  "brackets",
		"bad host":               "whitespace",
		"example..com":           "empty DNS label",
		"-example.com":           "starting or ending",
		"user@example.com":       "credentials",
		strings.Repeat("a", 254): "too long",
	}
	for address, want := range invalid {
		err := ValidateAddress(address)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateAddress(%q) = %v, want %q", address, err, want)
		}
	}
}

func TestNormalizePanelURL(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "https://Example.COM/", want: "https://example.com"},
		{raw: " https://203.0.113.10 ", want: "https://203.0.113.10"},
		{raw: "http://localhost:8080", want: "http://localhost:8080"},
		{raw: "http://127.0.0.1", want: "http://127.0.0.1"},
		{raw: "http://[::1]:8080", want: "http://[::1]:8080"},
	}
	for _, test := range tests {
		got, err := NormalizePanelURL(test.raw)
		if err != nil {
			t.Fatalf("NormalizePanelURL(%q): %v", test.raw, err)
		}
		if got != test.want {
			t.Errorf("NormalizePanelURL(%q) = %q, want %q", test.raw, got, test.want)
		}
	}

	invalid := map[string]string{
		"":                              "empty",
		"http://203.0.113.10":           "https outside localhost",
		"ftp://example.com":             "must use https",
		"https://example.com/path":      "path",
		"https://example.com?query=yes": "query",
		"https://user:pass@example.com": "credentials",
		"https://":                      "no host",
	}
	for raw, want := range invalid {
		_, err := NormalizePanelURL(raw)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("NormalizePanelURL(%q) = %v, want %q", raw, err, want)
		}
	}
}

func TestServerNormalizeAndValidate(t *testing.T) {
	server := validServer()
	server.Name = "  My server  "
	server.Address = "  Example.COM  "
	server.PanelURL = " https://Example.COM/ "
	server.Normalize()

	if server.Name != "My server" {
		t.Fatalf("Name = %q", server.Name)
	}
	if server.Address != "Example.COM" {
		t.Fatalf("Address = %q", server.Address)
	}
	if server.ID != "example.com_25565" {
		t.Fatalf("ID = %q", server.ID)
	}
	if server.PanelURL != "https://example.com" {
		t.Fatalf("PanelURL = %q", server.PanelURL)
	}
	if err := server.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	badName := server
	badName.Name = ""
	if err := badName.Validate(); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("empty name error = %v", err)
	}

	badPort := server
	badPort.Port = 0
	if err := badPort.Validate(); err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("bad port error = %v", err)
	}

	badID := server
	badID.ID = "other"
	if err := badID.Validate(); err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("bad id error = %v", err)
	}

	badPanel := server
	badPanel.PanelURL = "http://example.com"
	if err := badPanel.Validate(); err == nil || !strings.Contains(err.Error(), "panel URL") {
		t.Fatalf("bad panel error = %v", err)
	}
}

func TestServerValidationDoesNotLeakPassword(t *testing.T) {
	server := validServer()
	server.Address = "bad host"
	err := server.Validate()
	if err == nil {
		t.Fatal("Validate returned nil")
	}
	if strings.Contains(err.Error(), server.Password) {
		t.Fatalf("error leaks password: %v", err)
	}
}

func TestConfigNormalizeAndValidate(t *testing.T) {
	cfg := Default()
	cfg.Language = " RU_ru.UTF-8 "
	cfg.Servers = []Server{validServer()}
	cfg.ActiveServer = "missing"
	cfg.Normalize()

	if cfg.Language != "ru" {
		t.Fatalf("Language = %q", cfg.Language)
	}
	if cfg.Servers[0].ID != "203.0.113.10_25565" {
		t.Fatalf("server ID = %q", cfg.Servers[0].ID)
	}
	if cfg.ActiveServer != "" {
		t.Fatalf("ActiveServer = %q, want empty", cfg.ActiveServer)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	duplicate := cfg.Clone()
	duplicate.Servers = append(duplicate.Servers, duplicate.Servers[0])
	if err := duplicate.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}

	unsupported := cfg.Clone()
	unsupported.Language = "de"
	if err := unsupported.Validate(); err == nil || !strings.Contains(err.Error(), "language") {
		t.Fatalf("language error = %v", err)
	}

	version := cfg.Clone()
	version.Version = CurrentVersion + 1
	if err := version.Validate(); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("version error = %v", err)
	}
}
