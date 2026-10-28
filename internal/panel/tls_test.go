package panel

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
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
		HTTPSAddr: "127.0.0.1:0",
		TLSMode:   TLSModeFiles,
	}.Normalize()
}

func testConfigWithCertificate(t *testing.T) Config {
	t.Helper()
	cfg := testConfig(t)
	certPEM, keyPEM := generateTestCertificate(t, "127.0.0.1",
		time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour))
	writeTestCertificate(t, cfg, certPEM, keyPEM)
	return cfg
}

func generateTestCertificate(t *testing.T, ip string, notBefore, notAfter time.Time) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	caSerial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		t.Fatalf("generate CA serial: %v", err)
	}

	ca := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "tes3mp-easy-test-ca"},
		NotBefore:             notBefore.Add(-time.Hour),
		NotAfter:              notAfter.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca,
		&caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "tes3mp-easy-test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		template.IPAddresses = []net.IP{parsed}
	} else {
		template.DNSNames = []string{ip}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert,
		&key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func writeTestCertificate(t *testing.T, cfg Config, certPEM, keyPEM []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(cfg.CertFile), 0o700); err != nil {
		t.Fatalf("create TLS dir: %v", err)
	}
	if err := os.WriteFile(cfg.CertFile, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(cfg.KeyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

func TestLoadCertificatePair(t *testing.T) {
	cfg := testConfigWithCertificate(t)

	certificate, leaf, err := loadCertificatePair(cfg)
	if err != nil {
		t.Fatalf("loadCertificatePair: %v", err)
	}
	if len(certificate.Certificate) == 0 || certificate.PrivateKey == nil {
		t.Fatal("loaded certificate is incomplete")
	}
	if err := leaf.VerifyHostname("127.0.0.1"); err != nil {
		t.Fatalf("VerifyHostname: %v", err)
	}
}

func TestLoadCertificatePairRejectsIncomplete(t *testing.T) {
	tests := []struct {
		name    string
		written string
		want    string
	}{
		{name: "key only", written: "key", want: "certificate"},
		{name: "certificate only", written: "cert", want: "private key"},
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

			_, _, err := loadCertificatePair(cfg)
			if err == nil || !strings.Contains(err.Error(), "incomplete TLS key pair") ||
				!strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want incomplete pair mentioning %q", err, tt.want)
			}
		})
	}
}

func TestLoadCertificatePairRejectsInvalid(t *testing.T) {
	cfg := testConfig(t)
	writeTestCertificate(t, cfg, []byte("not a pem"), []byte("not a pem"))

	_, _, err := loadCertificatePair(cfg)
	if err == nil || !strings.Contains(err.Error(), "load TLS certificate") {
		t.Fatalf("error = %v, want invalid certificate", err)
	}
}

type fakeObtainer struct {
	certPEM []byte
	keyPEM  []byte
	err     error
	calls   int
}

func (f *fakeObtainer) Obtain(context.Context, Config) ([]byte, []byte, error) {
	f.calls++
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.certPEM, f.keyPEM, nil
}

func testCertificateManager(t *testing.T, cfg Config, obtainer certificateObtainer,
	now func() time.Time,
) *certificateManager {
	t.Helper()
	if now == nil {
		now = time.Now
	}
	return &certificateManager{
		cfg:                cfg.Normalize(),
		logger:             log.New(io.Discard, "", 0),
		obtainer:           obtainer,
		now:                now,
		renewCheckInterval: time.Hour,
		retryInterval:      time.Minute,
	}
}

func TestCertificateManagerObtainsACMEForIP(t *testing.T) {
	cfg := Config{
		Root:          t.TempDir(),
		DataDir:       filepath.Join(t.TempDir(), "panel-data"),
		HTTPSAddr:     "127.0.0.1:0",
		TLSMode:       TLSModeACME,
		PublicAddress: "203.0.113.10",
	}.Normalize()
	certPEM, keyPEM := generateTestCertificate(t, cfg.PublicAddress,
		time.Now().Add(-time.Minute), time.Now().Add(6*24*time.Hour))
	obtainer := &fakeObtainer{certPEM: certPEM, keyPEM: keyPEM}
	manager := testCertificateManager(t, cfg, obtainer, nil)

	if err := manager.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if obtainer.calls != 1 {
		t.Fatalf("obtain calls = %d, want 1", obtainer.calls)
	}
	current := manager.current.Load()
	if current == nil {
		t.Fatal("current certificate is nil")
	}
	leaf, err := certificateLeaf(*current)
	if err != nil {
		t.Fatalf("certificateLeaf: %v", err)
	}
	if err := leaf.VerifyHostname("203.0.113.10"); err != nil {
		t.Fatalf("certificate does not cover IP: %v", err)
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
}

func TestCertificateManagerRejectsWrongIP(t *testing.T) {
	cfg := Config{
		Root:          t.TempDir(),
		DataDir:       filepath.Join(t.TempDir(), "panel-data"),
		HTTPSAddr:     "127.0.0.1:0",
		TLSMode:       TLSModeACME,
		PublicAddress: "203.0.113.10",
	}.Normalize()
	certPEM, keyPEM := generateTestCertificate(t, "198.51.100.20",
		time.Now().Add(-time.Minute), time.Now().Add(6*24*time.Hour))
	manager := testCertificateManager(t, cfg,
		&fakeObtainer{certPEM: certPEM, keyPEM: keyPEM}, nil)

	err := manager.Ensure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "does not cover") {
		t.Fatalf("Ensure = %v, want hostname mismatch", err)
	}
	if _, statErr := os.Stat(cfg.CertFile); !os.IsNotExist(statErr) {
		t.Fatalf("certificate was written despite mismatch: %v", statErr)
	}
}

func TestCertificateManagerKeepsValidExpiringCertificateOnRenewalError(t *testing.T) {
	root := t.TempDir()
	cfg := Config{
		Root:          root,
		DataDir:       filepath.Join(root, "panel-data"),
		HTTPSAddr:     "127.0.0.1:0",
		TLSMode:       TLSModeACME,
		PublicAddress: "203.0.113.10",
	}.Normalize()
	notBefore := time.Now().Add(-5 * 24 * time.Hour)
	certPEM, keyPEM := generateTestCertificate(t, cfg.PublicAddress,
		notBefore, notBefore.Add(6*24*time.Hour))
	writeTestCertificate(t, cfg, certPEM, keyPEM)

	manager, err := newCertificateManager(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("newCertificateManager: %v", err)
	}
	obtainer := &fakeObtainer{err: errors.New("ACME unavailable")}
	manager.obtainer = obtainer
	manager.now = func() time.Time { return notBefore.Add(5*24*time.Hour + 12*time.Hour) }

	if err := manager.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure = %v, want current certificate to be kept", err)
	}
	if obtainer.calls != 1 {
		t.Fatalf("obtain calls = %d, want 1", obtainer.calls)
	}
	if manager.current.Load() == nil {
		t.Fatal("current certificate was dropped")
	}
}

func TestCertificateManagerRenewalWindow(t *testing.T) {
	manager := &certificateManager{}
	tests := []struct {
		lifetime time.Duration
		want     time.Duration
	}{
		{lifetime: 6 * 24 * time.Hour, want: 48 * time.Hour},
		{lifetime: 90 * 24 * time.Hour, want: 30 * 24 * time.Hour},
		{lifetime: 3 * 24 * time.Hour, want: 36 * time.Hour},
	}
	for _, tt := range tests {
		leaf := &x509.Certificate{
			NotBefore: time.Unix(0, 0),
			NotAfter:  time.Unix(0, 0).Add(tt.lifetime),
		}
		if got := manager.renewalWindow(leaf); got != tt.want {
			t.Errorf("lifetime %s: window = %s, want %s", tt.lifetime, got, tt.want)
		}
	}
}

func TestLoadOrCreateACMEAccountKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acme", "account.key")
	key, created, err := loadOrCreateACMEAccountKey(path)
	if err != nil {
		t.Fatalf("create account key: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat account key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat acme dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("acme dir mode = %v, want 0700", dirInfo.Mode().Perm())
	}

	loaded, createdAgain, err := loadOrCreateACMEAccountKey(path)
	if err != nil {
		t.Fatalf("load account key: %v", err)
	}
	if createdAgain {
		t.Fatal("second call regenerated the account key")
	}
	firstPub, err := x509.MarshalPKIXPublicKey(key.(*ecdsa.PrivateKey).Public())
	if err != nil {
		t.Fatal(err)
	}
	secondPub, err := x509.MarshalPKIXPublicKey(loaded.(*ecdsa.PrivateKey).Public())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstPub, secondPub) {
		t.Fatal("loaded account key differs from the generated key")
	}
}

func TestEnsureHonoursCancelledContext(t *testing.T) {
	cfg := Config{
		Root:          t.TempDir(),
		HTTPSAddr:     "127.0.0.1:0",
		TLSMode:       TLSModeACME,
		PublicAddress: "203.0.113.10",
	}.Normalize()
	obtainer := &fakeObtainer{err: errors.New("must not be called")}
	manager := testCertificateManager(t, cfg, obtainer, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := manager.Ensure(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Ensure = %v, want context.Canceled", err)
	}
	if obtainer.calls != 0 {
		t.Fatalf("obtain calls = %d, want 0", obtainer.calls)
	}
}

func TestACMECSRUsesIPSANWithoutCommonName(t *testing.T) {
	cfg := Config{
		PublicAddress: "203.0.113.10",
		ACMEProfile:   DefaultACMEProfile,
	}.Normalize()

	request, err := acmeCSRRequest(cfg)
	if err != nil {
		t.Fatalf("acmeCSRRequest: %v", err)
	}
	if request.CSR == nil {
		t.Fatal("CSR = nil")
	}
	if request.PrivateKey == nil {
		t.Fatal("PrivateKey = nil")
	}
	if request.CSR.Subject.CommonName != "" {
		t.Fatalf("CommonName = %q, want empty for an IP identifier",
			request.CSR.Subject.CommonName)
	}
	if len(request.CSR.DNSNames) != 0 {
		t.Fatalf("DNSNames = %v, want empty", request.CSR.DNSNames)
	}
	if len(request.CSR.IPAddresses) != 1 ||
		request.CSR.IPAddresses[0].String() != "203.0.113.10" {
		t.Fatalf("IPAddresses = %v", request.CSR.IPAddresses)
	}
	if request.Profile != "shortlived" {
		t.Fatalf("Profile = %q, want shortlived", request.Profile)
	}
	if !request.Bundle {
		t.Fatal("Bundle = false, want full chain")
	}
}
