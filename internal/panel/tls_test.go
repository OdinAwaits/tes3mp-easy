package panel

import (
	"bytes"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	return Config{
		Root:      root,
		DataDir:   filepath.Join(root, "panel-data"),
		HTTPAddr:  "127.0.0.1:0",
		HTTPSAddr: "127.0.0.1:0",
	}.Normalize()
}

func TestLoadOrCreateCertificateGeneratesPair(t *testing.T) {
	cfg := testConfig(t)
	cfg.PublicAddress = "203.0.113.10:8443"

	certificate, created, err := loadOrCreateCertificate(cfg)
	if err != nil {
		t.Fatalf("loadOrCreateCertificate: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if len(certificate.Certificate) == 0 || certificate.PrivateKey == nil {
		t.Fatal("generated certificate is incomplete")
	}

	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	if leaf.Subject.CommonName != "tes3mp-easy" {
		t.Fatalf("CommonName = %q", leaf.Subject.CommonName)
	}
	if !leaf.BasicConstraintsValid || leaf.IsCA {
		t.Fatalf("BasicConstraintsValid=%v IsCA=%v", leaf.BasicConstraintsValid, leaf.IsCA)
	}
	if !leaf.NotBefore.Before(time.Now()) ||
		!leaf.NotAfter.After(time.Now().Add(360*24*time.Hour)) {
		t.Fatalf("validity = %s..%s", leaf.NotBefore, leaf.NotAfter)
	}
	if !containsDNS(leaf.DNSNames, "localhost") {
		t.Fatalf("DNSNames = %v, want localhost", leaf.DNSNames)
	}
	if !containsIP(leaf.IPAddresses, net.ParseIP("203.0.113.10")) {
		t.Fatalf("IPAddresses = %v, want 203.0.113.10", leaf.IPAddresses)
	}
	if !containsIP(leaf.IPAddresses, net.ParseIP("127.0.0.1")) {
		t.Fatalf("IPAddresses = %v, want 127.0.0.1", leaf.IPAddresses)
	}
	if len(leaf.ExtKeyUsage) != 1 || leaf.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Fatalf("ExtKeyUsage = %v", leaf.ExtKeyUsage)
	}

	for _, path := range []string{cfg.CertFile, cfg.KeyFile} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, want 0600", path, info.Mode().Perm())
		}
	}
	info, err := os.Stat(filepath.Dir(cfg.CertFile))
	if err != nil {
		t.Fatalf("stat TLS dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("TLS dir mode = %v, want 0700", info.Mode().Perm())
	}

	again, createdAgain, err := loadOrCreateCertificate(cfg)
	if err != nil {
		t.Fatalf("second loadOrCreateCertificate: %v", err)
	}
	if createdAgain {
		t.Fatal("second call regenerated the certificate")
	}
	if !bytes.Equal(certificate.Certificate[0], again.Certificate[0]) {
		t.Fatal("second call loaded a different certificate")
	}
}

func TestLoadOrCreateCertificateRejectsIncompletePair(t *testing.T) {
	tests := []struct {
		name    string
		written string
		want    string
	}{
		{
			name:    "key only",
			written: "key",
			want:    "certificate",
		},
		{
			name:    "certificate only",
			written: "cert",
			want:    "private key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			path := cfg.CertFile
			if tt.written == "key" {
				path = cfg.KeyFile
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("junk"), 0o600); err != nil {
				t.Fatal(err)
			}

			_, _, err := loadOrCreateCertificate(cfg)
			if err == nil || !strings.Contains(err.Error(), "incomplete TLS key pair") ||
				!strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want incomplete pair mentioning %q", err, tt.want)
			}
		})
	}
}

func TestLoadOrCreateCertificateRejectsInvalidPair(t *testing.T) {
	cfg := testConfig(t)
	if err := os.MkdirAll(filepath.Dir(cfg.CertFile), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{cfg.CertFile, cfg.KeyFile} {
		if err := os.WriteFile(path, []byte("not a pem"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, _, err := loadOrCreateCertificate(cfg)
	if err == nil || !strings.Contains(err.Error(), "load TLS certificate") {
		t.Fatalf("error = %v, want invalid certificate", err)
	}
}

func containsDNS(names []string, want string) bool {
	for _, name := range names {
		if strings.EqualFold(name, want) {
			return true
		}
	}
	return false
}

func containsIP(ips []net.IP, want net.IP) bool {
	for _, ip := range ips {
		if ip.Equal(want) {
			return true
		}
	}
	return false
}
