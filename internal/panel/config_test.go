package panel

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigNormalize(t *testing.T) {
	cfg := DefaultConfig().Normalize()

	if cfg.Root != DefaultRoot {
		t.Fatalf("Root = %q, want %q", cfg.Root, DefaultRoot)
	}
	if cfg.DataDir != filepath.Join(DefaultRoot, "panel-data") {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.HTTPAddr != DefaultHTTPAddr || cfg.HTTPSAddr != DefaultHTTPSAddr {
		t.Fatalf("addresses = %q, %q", cfg.HTTPAddr, cfg.HTTPSAddr)
	}
	if cfg.CertFile != filepath.Join(DefaultRoot, "panel-data", "tls", "cert.pem") {
		t.Fatalf("CertFile = %q", cfg.CertFile)
	}
	if cfg.KeyFile != filepath.Join(DefaultRoot, "panel-data", "tls", "key.pem") {
		t.Fatalf("KeyFile = %q", cfg.KeyFile)
	}
	if got := cfg.Normalize(); got != cfg {
		t.Fatalf("Normalize is not idempotent: %#v then %#v", cfg, got)
	}
}

func TestCustomRootMovesDerivedPaths(t *testing.T) {
	root := t.TempDir()
	cfg := Config{Root: root}.Normalize()

	if cfg.DataDir != filepath.Join(root, "panel-data") {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.CertFile != filepath.Join(root, "panel-data", "tls", "cert.pem") {
		t.Fatalf("CertFile = %q", cfg.CertFile)
	}
	if cfg.KeyFile != filepath.Join(root, "panel-data", "tls", "key.pem") {
		t.Fatalf("KeyFile = %q", cfg.KeyFile)
	}

	explicit := Config{
		Root:     root,
		DataDir:  "/var/lib/panel",
		CertFile: "/etc/tls/cert.pem",
		KeyFile:  "/etc/tls/key.pem",
	}.Normalize()
	if explicit.DataDir != "/var/lib/panel" ||
		explicit.CertFile != "/etc/tls/cert.pem" ||
		explicit.KeyFile != "/etc/tls/key.pem" {
		t.Fatalf("explicit paths were overwritten: %#v", explicit)
	}
}

func TestConfigFromEnv(t *testing.T) {
	env := map[string]string{
		EnvRoot:          "/srv/tes3mp",
		EnvDataDir:       "/srv/panel",
		EnvHTTPAddr:      "127.0.0.1:8080",
		EnvHTTPSAddr:     "127.0.0.1:8443",
		EnvPublicAddress: "203.0.113.10",
		EnvTLSCert:       "/srv/cert.pem",
		EnvTLSKey:        "/srv/key.pem",
	}
	cfg := ConfigFromEnv(func(key string) string { return env[key] }).Normalize()

	if cfg.Root != "/srv/tes3mp" || cfg.DataDir != "/srv/panel" ||
		cfg.HTTPAddr != "127.0.0.1:8080" || cfg.HTTPSAddr != "127.0.0.1:8443" ||
		cfg.PublicAddress != "203.0.113.10" ||
		cfg.CertFile != "/srv/cert.pem" || cfg.KeyFile != "/srv/key.pem" {
		t.Fatalf("ConfigFromEnv = %#v", cfg)
	}
}

func TestConfigValidate(t *testing.T) {
	valid := Config{
		Root:      "/tes3mp-easy",
		HTTPAddr:  ":80",
		HTTPSAddr: ":443",
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "defaults", mutate: func(*Config) {}},
		{
			name: "same address",
			mutate: func(c *Config) {
				c.HTTPAddr = "127.0.0.1:8080"
				c.HTTPSAddr = "127.0.0.1:8080"
			},
			wantErr: "must differ",
		},
		{
			name: "same ephemeral port allowed",
			mutate: func(c *Config) {
				c.HTTPAddr = "127.0.0.1:0"
				c.HTTPSAddr = "127.0.0.1:0"
			},
		},
		{
			name: "bad http address",
			mutate: func(c *Config) {
				c.HTTPAddr = "not-an-address"
			},
			wantErr: "http address",
		},
		{
			name: "bad https address",
			mutate: func(c *Config) {
				c.HTTPSAddr = ":"
			},
			wantErr: "https address",
		},
		{
			name: "public address with scheme",
			mutate: func(c *Config) {
				c.PublicAddress = "https://panel.example"
			},
			wantErr: "URL scheme",
		},
		{
			name: "public address with path",
			mutate: func(c *Config) {
				c.PublicAddress = "panel.example/admin"
			},
			wantErr: "path",
		},
		{
			name: "public address with whitespace",
			mutate: func(c *Config) {
				c.PublicAddress = "panel example"
			},
			wantErr: "whitespace",
		},
		{
			name: "public address empty host",
			mutate: func(c *Config) {
				c.PublicAddress = ":8443"
			},
			wantErr: "host is empty",
		},
		{
			name: "public address bare ipv6",
			mutate: func(c *Config) {
				c.PublicAddress = "2001:db8::1"
			},
		},
		{
			name: "public address ipv6 with port",
			mutate: func(c *Config) {
				c.PublicAddress = "[2001:db8::1]:8443"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
