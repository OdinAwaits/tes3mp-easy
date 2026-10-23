package panel

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

const selfSignedValidity = 365 * 24 * time.Hour

// loadOrCreateCertificate reuses an existing cert/key pair or creates a
// self-signed pair when both files are absent. Partial or invalid pairs are
// errors: silently replacing them would hide a broken deployment.
func loadOrCreateCertificate(cfg Config) (tls.Certificate, bool, error) {
	cfg = cfg.Normalize()

	certPEM, certErr := os.ReadFile(cfg.CertFile)
	keyPEM, keyErr := os.ReadFile(cfg.KeyFile)

	switch {
	case certErr == nil && keyErr == nil:
		certificate, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return tls.Certificate{}, false, fmt.Errorf(
				"load TLS certificate %s: %w", cfg.CertFile, err)
		}
		return certificate, false, nil
	case errors.Is(certErr, fs.ErrNotExist) && errors.Is(keyErr, fs.ErrNotExist):
		certificate, err := createSelfSigned(cfg)
		if err != nil {
			return tls.Certificate{}, false, err
		}
		return certificate, true, nil
	case certErr != nil && !errors.Is(certErr, fs.ErrNotExist):
		return tls.Certificate{}, false, fmt.Errorf(
			"read TLS certificate %s: %w", cfg.CertFile, certErr)
	case keyErr != nil && !errors.Is(keyErr, fs.ErrNotExist):
		return tls.Certificate{}, false, fmt.Errorf(
			"read TLS key %s: %w", cfg.KeyFile, keyErr)
	case errors.Is(certErr, fs.ErrNotExist):
		return tls.Certificate{}, false, fmt.Errorf(
			"incomplete TLS key pair: certificate %s is missing", cfg.CertFile)
	default:
		return tls.Certificate{}, false, fmt.Errorf(
			"incomplete TLS key pair: private key %s is missing", cfg.KeyFile)
	}
}

func createSelfSigned(cfg Config) (tls.Certificate, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return tls.Certificate{}, fmt.Errorf(
			"create panel data directory %s: %w", cfg.DataDir, err)
	}
	tlsDir := filepath.Dir(cfg.CertFile)
	if err := os.MkdirAll(tlsDir, 0o700); err != nil {
		return tls.Certificate{}, fmt.Errorf(
			"create TLS directory %s: %w", tlsDir, err)
	}

	certPEM, keyPEM, err := generateSelfSigned(cfg.PublicAddress)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Write the key first. If the certificate write fails, remove the key so
	// the next start sees a clean "both absent" state instead of a pair that
	// can never match.
	if err := atomicfile.WriteFile(cfg.KeyFile, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("write TLS key %s: %w", cfg.KeyFile, err)
	}
	if err := atomicfile.WriteFile(cfg.CertFile, certPEM, 0o600); err != nil {
		removeErr := os.Remove(cfg.KeyFile)
		return tls.Certificate{}, errors.Join(
			fmt.Errorf("write TLS certificate %s: %w", cfg.CertFile, err),
			removeErr,
		)
	}

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse generated TLS pair: %w", err)
	}
	return certificate, nil
}

func generateSelfSigned(publicAddress string) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate TLS key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("generate certificate serial: %w", err)
	}

	now := time.Now().UTC()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "tes3mp-easy",
			Organization: []string{"tes3mp-easy"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	template.DNSNames, template.IPAddresses = certificateNames(publicAddress)

	der, err := x509.CreateCertificate(rand.Reader, &template, &template,
		&key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create self-signed certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal TLS key: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}
