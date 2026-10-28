package panel

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	legoCertificate "github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

const (
	// Six-day IP certificates leave a two-day renewal window.
	acmeRenewCheckInterval = 6 * time.Hour
	acmeRetryInterval      = 30 * time.Minute
	acmeRequestTimeout     = 60 * time.Second
	acmeHTTPTimeout        = 3 * time.Minute
	acmeMaximumWindow      = 30 * 24 * time.Hour
	acmeMinimumWindow      = 48 * time.Hour
)

// certificateObtainer is the narrow ACME surface used by the manager. Tests
// replace it with an in-memory issuer; production uses legoObtainer.
type certificateObtainer interface {
	Obtain(ctx context.Context, cfg Config) (certPEM, keyPEM []byte, err error)
}

// certificateManager owns the current certificate and, in acme mode, renews it
// in the background. GetCertificate is called from TLS handshakes, so the
// current certificate is swapped atomically without restarting the listener.
type certificateManager struct {
	cfg      Config
	logger   *log.Logger
	obtainer certificateObtainer
	current  atomic.Pointer[tls.Certificate]

	mu                 sync.Mutex
	now                func() time.Time
	renewCheckInterval time.Duration
	retryInterval      time.Duration
}

func newCertificateManager(cfg Config, logger *log.Logger) (*certificateManager, error) {
	cfg = cfg.Normalize()
	manager := &certificateManager{
		cfg:                cfg,
		logger:             logger,
		obtainer:           &legoObtainer{},
		now:                time.Now,
		renewCheckInterval: acmeRenewCheckInterval,
		retryInterval:      acmeRetryInterval,
	}

	certificate, leaf, err := loadCertificatePair(cfg)
	if cfg.TLSMode == TLSModeFiles {
		if err != nil {
			return nil, err
		}
		manager.current.Store(&certificate)
		manager.logger.Printf("tls: loaded certificate %s (valid until %s)",
			cfg.CertFile, leaf.NotAfter.UTC().Format(time.RFC3339))
		return manager, nil
	}

	if err != nil {
		manager.logger.Printf("tls: no usable certificate yet: %v", err)
		return manager, nil
	}
	manager.current.Store(&certificate)
	manager.logger.Printf("tls: loaded certificate %s (valid until %s)",
		cfg.CertFile, leaf.NotAfter.UTC().Format(time.RFC3339))
	return manager, nil
}

// Run checks the certificate periodically and renews it before the shortlived
// profile expires. It exits when ctx is cancelled.
func (m *certificateManager) Run(ctx context.Context) {
	if m.cfg.TLSMode != TLSModeACME {
		return
	}

	delay := m.renewCheckInterval
	if current := m.current.Load(); current != nil {
		if leaf, err := certificateLeaf(*current); err == nil &&
			m.needsRenewal(leaf, m.now()) {
			// Ensure already tried and is serving the old certificate; retry
			// sooner than the regular six-hour check.
			delay = m.retryInterval
		}
	}
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := m.renewIfNeeded(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			m.logger.Printf("tls: ACME renewal failed: %v", err)
			delay = m.retryInterval
		} else {
			delay = m.renewCheckInterval
		}
	}
}

func (m *certificateManager) renewIfNeeded(ctx context.Context) error {
	current := m.current.Load()
	if current == nil {
		return m.obtain(ctx)
	}
	leaf, err := certificateLeaf(*current)
	if err != nil {
		return m.obtain(ctx)
	}
	if !m.needsRenewal(leaf, m.now()) {
		return nil
	}
	m.logger.Printf("tls: certificate expires at %s; renewing",
		leaf.NotAfter.UTC().Format(time.RFC3339))
	return m.obtain(ctx)
}

func (m *certificateManager) needsRenewal(leaf *x509.Certificate, now time.Time) bool {
	return !now.Add(m.renewalWindow(leaf)).Before(leaf.NotAfter)
}

// renewalWindow uses a third of the lifetime, capped at 30 days for classic
// certificates and floored at two days for the six-day shortlived profile.
func (m *certificateManager) renewalWindow(leaf *x509.Certificate) time.Duration {
	lifetime := leaf.NotAfter.Sub(leaf.NotBefore)
	if lifetime <= 0 {
		return acmeMinimumWindow
	}
	window := lifetime / 3
	if window > acmeMaximumWindow {
		window = acmeMaximumWindow
	}
	if window < acmeMinimumWindow {
		window = acmeMinimumWindow
	}
	if window > lifetime/2 {
		window = lifetime / 2
	}
	return window
}

// obtain serializes issuance so startup, renewal and a retry cannot run over
// each other or race on the ACME account files.
func (m *certificateManager) obtain(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	certPEM, keyPEM, err := m.obtainer.Obtain(ctx, m.cfg)
	if err != nil {
		return err
	}
	certificate, leaf, err := parseCertificatePair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("parse ACME certificate: %w", err)
	}
	if err := leaf.VerifyHostname(m.cfg.PublicAddress); err != nil {
		return fmt.Errorf("ACME certificate does not cover %s: %w",
			m.cfg.PublicAddress, err)
	}
	if err := writeCertificatePair(m.cfg, certPEM, keyPEM); err != nil {
		return err
	}

	m.current.Store(&certificate)
	m.logger.Printf("tls: obtained ACME certificate for %s (valid until %s)",
		m.cfg.PublicAddress, leaf.NotAfter.UTC().Format(time.RFC3339))
	return nil
}

// GetCertificate returns the atomically current certificate for a handshake.
func (m *certificateManager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificate := m.current.Load()
	if certificate == nil {
		return nil, errors.New("TLS certificate is not available")
	}
	return certificate, nil
}

// Ensure makes sure a usable certificate is loaded before HTTPS starts. In acme
// mode a missing, expired or mismatching certificate is issued now and a
// failure is fatal; an expiring but still valid certificate is kept if renewal
// fails, and Run retries in the background.
func (m *certificateManager) Ensure(ctx context.Context) error {
	if m.cfg.TLSMode == TLSModeFiles {
		if m.current.Load() == nil {
			return errors.New("TLS certificate is not loaded")
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	current := m.current.Load()
	if current != nil {
		leaf, err := certificateLeaf(*current)
		if err == nil {
			switch {
			case leaf.VerifyHostname(m.cfg.PublicAddress) != nil:
				m.logger.Printf("tls: existing certificate does not cover %s", m.cfg.PublicAddress)
			case m.now().Before(leaf.NotBefore):
				m.logger.Printf("tls: certificate is not valid before %s",
					leaf.NotBefore.UTC().Format(time.RFC3339))
			case m.now().Before(leaf.NotAfter):
				if !m.needsRenewal(leaf, m.now()) {
					return nil
				}
				m.logger.Printf("tls: certificate expires at %s; renewing",
					leaf.NotAfter.UTC().Format(time.RFC3339))
				if renewErr := m.obtain(ctx); renewErr != nil {
					m.logger.Printf("tls: renewal failed, keeping current certificate: %v",
						renewErr)
					return nil
				}
				return nil
			default:
				m.logger.Printf("tls: certificate expired at %s",
					leaf.NotAfter.UTC().Format(time.RFC3339))
			}
		} else {
			m.logger.Printf("tls: existing certificate is unreadable: %v", err)
		}
	}

	if err := m.obtain(ctx); err != nil {
		return fmt.Errorf("obtain ACME certificate for %s: %w",
			m.cfg.PublicAddress, err)
	}
	return nil
}

func loadCertificatePair(cfg Config) (tls.Certificate, *x509.Certificate, error) {
	cfg = cfg.Normalize()

	certPEM, certErr := os.ReadFile(cfg.CertFile)
	keyPEM, keyErr := os.ReadFile(cfg.KeyFile)

	switch {
	case certErr == nil && keyErr == nil:
		certificate, leaf, err := parseCertificatePair(certPEM, keyPEM)
		if err != nil {
			return tls.Certificate{}, nil, fmt.Errorf(
				"load TLS certificate %s: %w", cfg.CertFile, err)
		}
		return certificate, leaf, nil
	case errors.Is(certErr, fs.ErrNotExist) && errors.Is(keyErr, fs.ErrNotExist):
		return tls.Certificate{}, nil, fmt.Errorf(
			"TLS key pair %s and %s is missing", cfg.CertFile, cfg.KeyFile)
	case certErr != nil && !errors.Is(certErr, fs.ErrNotExist):
		return tls.Certificate{}, nil, fmt.Errorf(
			"read TLS certificate %s: %w", cfg.CertFile, certErr)
	case keyErr != nil && !errors.Is(keyErr, fs.ErrNotExist):
		return tls.Certificate{}, nil, fmt.Errorf(
			"read TLS key %s: %w", cfg.KeyFile, keyErr)
	case errors.Is(certErr, fs.ErrNotExist):
		return tls.Certificate{}, nil, fmt.Errorf(
			"incomplete TLS key pair: certificate %s is missing", cfg.CertFile)
	default:
		return tls.Certificate{}, nil, fmt.Errorf(
			"incomplete TLS key pair: private key %s is missing", cfg.KeyFile)
	}
}

func parseCertificatePair(certPEM, keyPEM []byte) (tls.Certificate, *x509.Certificate, error) {
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	if len(certificate.Certificate) == 0 {
		return tls.Certificate{}, nil, errors.New("certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certificate.Leaf = leaf
	return certificate, leaf, nil
}

func certificateLeaf(certificate tls.Certificate) (*x509.Certificate, error) {
	if certificate.Leaf != nil {
		return certificate.Leaf, nil
	}
	if len(certificate.Certificate) == 0 {
		return nil, errors.New("certificate chain is empty")
	}
	return x509.ParseCertificate(certificate.Certificate[0])
}

func writeCertificatePair(cfg Config, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create panel data directory %s: %w", cfg.DataDir, err)
	}
	tlsDir := filepath.Dir(cfg.CertFile)
	if err := os.MkdirAll(tlsDir, 0o700); err != nil {
		return fmt.Errorf("create TLS directory %s: %w", tlsDir, err)
	}
	if err := os.Chmod(tlsDir, 0o700); err != nil {
		return fmt.Errorf("set TLS directory mode %s: %w", tlsDir, err)
	}

	// Write the key first. If the certificate write fails, remove the key so
	// the next start does not see a mismatched pair. The in-memory certificate
	// stays active until a complete replacement is written.
	if err := atomicfile.WriteFile(cfg.KeyFile, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write TLS key %s: %w", cfg.KeyFile, err)
	}
	if err := atomicfile.WriteFile(cfg.CertFile, certPEM, 0o600); err != nil {
		removeErr := os.Remove(cfg.KeyFile)
		return errors.Join(
			fmt.Errorf("write TLS certificate %s: %w", cfg.CertFile, err),
			removeErr,
		)
	}
	return nil
}

// acmeCSRRequest builds the CSR used for the ACME order. The IP identifier
// must live in subjectAltName only: lego's Obtain helper puts the first domain
// in Common Name, and Let's Encrypt rejects an IP address in CN with badCSR.
// ObtainForCSR lets us keep Common Name empty and still carry the shortlived
// profile and the full chain.
func acmeCSRRequest(cfg Config) (legoCertificate.ObtainForCSRRequest, error) {
	privateKey, err := certcrypto.GeneratePrivateKey(certcrypto.EC256)
	if err != nil {
		return legoCertificate.ObtainForCSRRequest{}, fmt.Errorf(
			"generate ACME private key: %w", err)
	}
	csrDER, err := certcrypto.CreateCSR(privateKey, certcrypto.CSROptions{
		SAN: []string{cfg.PublicAddress},
	})
	if err != nil {
		return legoCertificate.ObtainForCSRRequest{}, fmt.Errorf(
			"create ACME CSR: %w", err)
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return legoCertificate.ObtainForCSRRequest{}, fmt.Errorf(
			"parse ACME CSR: %w", err)
	}
	return legoCertificate.ObtainForCSRRequest{
		CSR:        csr,
		PrivateKey: privateKey,
		Bundle:     true,
		Profile:    cfg.ACMEProfile,
	}, nil
}

// legoObtainer implements RFC 8738 certificates through lego. The HTTP-01
// provider owns :80 only while a challenge is being presented; no HTTP server
// remains after Obtain returns.
type legoObtainer struct{}

func (o *legoObtainer) Obtain(ctx context.Context, cfg Config) ([]byte, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	accountKey, created, err := loadOrCreateACMEAccountKey(cfg.ACMEAccountKeyFile)
	if err != nil {
		return nil, nil, err
	}
	user := &acmeUser{email: cfg.ACMEEmail, key: accountKey}

	legoConfig := lego.NewConfig(user)
	legoConfig.CADirURL = cfg.ACMEDirectory
	legoConfig.UserAgent = "tes3mp-easy-panel/" + Version
	legoConfig.HTTPClient.Timeout = acmeHTTPTimeout
	legoConfig.Certificate = lego.CertificateConfig{
		KeyType: certcrypto.EC256,
		Timeout: acmeRequestTimeout,
	}

	client, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create ACME client: %w", err)
	}
	if err := client.Challenge.SetHTTP01Provider(
		http01.NewProviderServer("", ACMEHTTPPort),
	); err != nil {
		return nil, nil, fmt.Errorf("configure HTTP-01 challenge: %w", err)
	}

	registered := false
	var resolveErr error
	if !created {
		_, resolveErr = client.Registration.ResolveAccountByKey()
		registered = resolveErr == nil
	}
	if !registered {
		if _, registerErr := client.Registration.Register(
			registration.RegisterOptions{TermsOfServiceAgreed: true},
		); registerErr != nil {
			if resolveErr != nil {
				return nil, nil, fmt.Errorf(
					"register ACME account: %w (resolve account: %v)",
					registerErr, resolveErr)
			}
			return nil, nil, fmt.Errorf("register ACME account: %w", registerErr)
		}
	}

	acmeRequest, err := acmeCSRRequest(cfg)
	if err != nil {
		return nil, nil, err
	}
	resource, err := client.Certificate.ObtainForCSR(acmeRequest)
	if err != nil {
		return nil, nil, err
	}
	if len(resource.Certificate) == 0 || len(resource.PrivateKey) == 0 {
		return nil, nil, errors.New("ACME server returned an empty certificate")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return resource.Certificate, resource.PrivateKey, nil
}

// acmeUser is the lego registration.User backed by the persisted account key.
// The registration URI is resolved again on every start, so only the key needs
// to be stored.
type acmeUser struct {
	email string
	key   crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }
func (u *acmeUser) GetRegistration() *registration.Resource { return nil }

func loadOrCreateACMEAccountKey(path string) (crypto.PrivateKey, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		key, parseErr := parsePrivateKey(data)
		if parseErr != nil {
			return nil, false, fmt.Errorf("load ACME account key %s: %w", path, parseErr)
		}
		if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
			return nil, false, fmt.Errorf("set ACME account key mode %s: %w",
				path, chmodErr)
		}
		return key, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, false, fmt.Errorf("read ACME account key %s: %w", path, err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, false, fmt.Errorf("generate ACME account key: %w", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, false, fmt.Errorf("marshal ACME account key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, fmt.Errorf("create ACME directory %s: %w",
			filepath.Dir(path), err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return nil, false, fmt.Errorf("set ACME directory mode %s: %w",
			filepath.Dir(path), err)
	}
	if err := atomicfile.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}),
		0o600); err != nil {
		return nil, false, fmt.Errorf("write ACME account key %s: %w", path, err)
	}
	return key, true, nil
}

func parsePrivateKey(data []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported PEM block %q", block.Type)
	}
}
