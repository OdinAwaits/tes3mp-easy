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
	if cfg.HTTPSAddr != DefaultHTTPSAddr {
		t.Fatalf("HTTPSAddr = %q", cfg.HTTPSAddr)
	}
	if cfg.TLSMode != TLSModeACME || cfg.ACMEDirectory != DefaultACMEDirectory ||
		cfg.ACMEProfile != DefaultACMEProfile {
		t.Fatalf("TLS settings = %#v", cfg)
	}
	if cfg.CertFile != filepath.Join(DefaultRoot, "panel-data", "tls", "cert.pem") {
		t.Fatalf("CertFile = %q", cfg.CertFile)
	}
	if cfg.KeyFile != filepath.Join(DefaultRoot, "panel-data", "tls", "key.pem") {
		t.Fatalf("KeyFile = %q", cfg.KeyFile)
	}
	if cfg.ACMEAccountKeyFile !=
		filepath.Join(DefaultRoot, "panel-data", "acme", "account.key") {
		t.Fatalf("ACMEAccountKeyFile = %q", cfg.ACMEAccountKeyFile)
	}
	if cfg.RuntimeRoot != DefaultRuntimeRoot {
		t.Fatalf("RuntimeRoot = %q, want %q", cfg.RuntimeRoot, DefaultRuntimeRoot)
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
	if cfg.ACMEAccountKeyFile !=
		filepath.Join(root, "panel-data", "acme", "account.key") {
		t.Fatalf("ACMEAccountKeyFile = %q", cfg.ACMEAccountKeyFile)
	}

	explicit := Config{
		Root:               root,
		DataDir:            "/var/lib/panel",
		CertFile:           "/etc/tls/cert.pem",
		KeyFile:            "/etc/tls/key.pem",
		ACMEAccountKeyFile: "/etc/acme/account.key",
	}.Normalize()
	if explicit.DataDir != "/var/lib/panel" ||
		explicit.CertFile != "/etc/tls/cert.pem" ||
		explicit.KeyFile != "/etc/tls/key.pem" ||
		explicit.ACMEAccountKeyFile != "/etc/acme/account.key" {
		t.Fatalf("explicit paths were overwritten: %#v", explicit)
	}
}

func TestConfigFromEnv(t *testing.T) {
	env := map[string]string{
		EnvRoot:          "/srv/tes3mp",
		EnvDataDir:       "/srv/panel",
		EnvHTTPSAddr:     "127.0.0.1:8443",
		EnvPublicAddress: "203.0.113.10",
		EnvTLSMode:       TLSModeFiles,
		EnvTLSCert:       "/srv/cert.pem",
		EnvTLSKey:        "/srv/key.pem",
		EnvACMEDirectory: "https://acme-staging-v02.api.letsencrypt.org/directory",
		EnvACMEEmail:     "admin@example.test",
		EnvACMEProfile:   "shortlived",
		EnvRuntimeRoot:   "/srv/tes3mp-runtime",
	}
	cfg := ConfigFromEnv(func(key string) string { return env[key] }).Normalize()

	if cfg.Root != "/srv/tes3mp" || cfg.DataDir != "/srv/panel" ||
		cfg.HTTPSAddr != "127.0.0.1:8443" ||
		cfg.PublicAddress != "203.0.113.10" ||
		cfg.TLSMode != TLSModeFiles ||
		cfg.CertFile != "/srv/cert.pem" || cfg.KeyFile != "/srv/key.pem" ||
		cfg.ACMEDirectory != env[EnvACMEDirectory] ||
		cfg.ACMEEmail != "admin@example.test" ||
		cfg.ACMEProfile != "shortlived" ||
		cfg.RuntimeRoot != "/srv/tes3mp-runtime" {
		t.Fatalf("ConfigFromEnv = %#v", cfg)
	}
}

func TestConfigValidate(t *testing.T) {
	files := Config{
		Root:      "/tes3mp-easy",
		HTTPSAddr: ":443",
		TLSMode:   TLSModeFiles,
	}
	acme := files
	acme.TLSMode = TLSModeACME
	acme.PublicAddress = "203.0.113.10"

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{name: "files defaults", cfg: files},
		{name: "acme public ip", cfg: acme},
		{
			name: "relative runtime root",
			cfg: func() Config {
				cfg := files
				cfg.RuntimeRoot = "opt/tes3mp-easy/tes3mp"
				return cfg
			}(),
			wantErr: "not an absolute path",
		},
		{
			name: "acme requires public ip",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = ""
				return cfg
			}(),
			wantErr: "required in acme mode",
		},
		{
			name: "acme rejects private ip",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = "10.0.0.1"
				return cfg
			}(),
			wantErr: "public unicast",
		},
		{
			name: "acme rejects loopback",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = "127.0.0.1"
				return cfg
			}(),
			wantErr: "public unicast",
		},
		{
			name: "acme rejects shared address space",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = "100.64.0.1"
				return cfg
			}(),
			wantErr: "public unicast",
		},
		{
			name: "acme rejects reserved address",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = "240.0.0.1"
				return cfg
			}(),
			wantErr: "public unicast",
		},
		{
			name: "acme rejects port",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = "203.0.113.10:443"
				return cfg
			}(),
			wantErr: "bare IP",
		},
		{
			name: "acme rejects dns name",
			cfg: func() Config {
				cfg := acme
				cfg.PublicAddress = "panel.example"
				return cfg
			}(),
			wantErr: "bare IP",
		},
		{
			name: "acme rejects http directory",
			cfg: func() Config {
				cfg := acme
				cfg.ACMEDirectory = "http://acme.example/directory"
				return cfg
			}(),
			wantErr: "must use https",
		},
		{
			name: "acme rejects bad email",
			cfg: func() Config {
				cfg := acme
				cfg.ACMEEmail = "not-an-email"
				return cfg
			}(),
			wantErr: "email",
		},
		{
			name: "unknown mode",
			cfg: func() Config {
				cfg := files
				cfg.TLSMode = "bogus"
				return cfg
			}(),
			wantErr: "TLS mode",
		},
		{
			name: "bad https address",
			cfg: func() Config {
				cfg := files
				cfg.HTTPSAddr = ":"
				return cfg
			}(),
			wantErr: "https address",
		},
		{
			name: "files public address with scheme",
			cfg: func() Config {
				cfg := files
				cfg.PublicAddress = "https://panel.example"
				return cfg
			}(),
			wantErr: "URL scheme",
		},
		{
			name: "files public address with path",
			cfg: func() Config {
				cfg := files
				cfg.PublicAddress = "panel.example/admin"
				return cfg
			}(),
			wantErr: "path",
		},
		{
			name: "files public address with whitespace",
			cfg: func() Config {
				cfg := files
				cfg.PublicAddress = "panel example"
				return cfg
			}(),
			wantErr: "whitespace",
		},
		{
			name: "files public address empty host",
			cfg: func() Config {
				cfg := files
				cfg.PublicAddress = ":8443"
				return cfg
			}(),
			wantErr: "host is empty",
		},
		{
			name: "files public address bare ipv6",
			cfg: func() Config {
				cfg := files
				cfg.PublicAddress = "2001:db8::1"
				return cfg
			}(),
		},
		{
			name: "files public address ipv6 with port",
			cfg: func() Config {
				cfg := files
				cfg.PublicAddress = "[2001:db8::1]:8443"
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
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
