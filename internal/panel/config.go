// Package panel implements the HTTPS entry point of tes3mp-easy.
//
// This step provides configuration, TLS material management, the TLS listener,
// graceful lifecycle, Argon2id password authentication, in-memory sessions,
// CSRF protection, the subprocess client for tes3mp-easy-ops, the public
// server/mods/backups routes and the admin status/logs/mods/state/config/staff
// pages with streaming upload.
package panel

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/version"
)

var (
	// Version is reported by the panel binary and the public API.
	Version = version.Server
)

const (
	// DefaultRoot is the production data root.
	DefaultRoot = "/tes3mp-easy"

	// DefaultHTTPSAddr is the TLS listener. Port 80 is not kept open: the panel
	// binds it only while an ACME HTTP-01 challenge is being solved.
	DefaultHTTPSAddr = ":443"

	// TLSModeACME uses the embedded ACME client.
	TLSModeACME = "acme"

	// TLSModeFiles loads an existing certificate/key pair and never contacts
	// an ACME server.
	TLSModeFiles = "files"

	// DefaultTLSMode issues and renews a Let's Encrypt certificate for the
	// configured public IP.
	DefaultTLSMode = TLSModeACME

	// DefaultACMEDirectory is the Let's Encrypt production directory.
	DefaultACMEDirectory = "https://acme-v02.api.letsencrypt.org/directory"

	// DefaultACMEProfile requests six-day IP certificates (RFC 8738). Let's
	// Encrypt requires this profile for IP identifiers.
	DefaultACMEProfile = "shortlived"

	// ACMEHTTPPort is the only port Let's Encrypt uses for HTTP-01. The
	// listener exists only for the duration of a challenge.
	ACMEHTTPPort = "80"
)

const (
	EnvRoot          = "TES3MP_PANEL_ROOT"
	EnvDataDir       = "TES3MP_PANEL_DATA_DIR"
	EnvHTTPSAddr     = "TES3MP_PANEL_HTTPS_ADDR"
	EnvPublicAddress = "TES3MP_PANEL_PUBLIC_ADDRESS"
	EnvTLSMode       = "TES3MP_PANEL_TLS_MODE"
	EnvTLSCert       = "TES3MP_PANEL_TLS_CERT"
	EnvTLSKey        = "TES3MP_PANEL_TLS_KEY"
	EnvACMEDirectory = "TES3MP_PANEL_ACME_DIRECTORY"
	EnvACMEEmail     = "TES3MP_PANEL_ACME_EMAIL"
	EnvACMEProfile   = "TES3MP_PANEL_ACME_PROFILE"

	// EnvLanguage selects the default interface language. The admin header can
	// switch it at runtime; the choice is then stored in panel-data and wins
	// over this variable.
	EnvLanguage = "TES3MP_PANEL_LANGUAGE"

	// DefaultLanguage is the documented default interface language.
	DefaultLanguage = i18n.English
)

// Config describes one panel process. Empty derived fields are filled by
// Normalize, so a custom Root automatically moves the data, TLS and ACME
// directories unless they were set explicitly.
type Config struct {
	Root          string
	DataDir       string
	HTTPSAddr     string
	PublicAddress string
	TLSMode       string
	CertFile      string
	KeyFile       string

	ACMEDirectory      string
	ACMEEmail          string
	ACMEProfile        string
	ACMEAccountKeyFile string

	// Language is the default interface language of the panel. The admin header
	// can switch it; the stored choice in panel-data wins after the first switch.
	Language string
}

// DefaultConfig returns production defaults. DataDir and the TLS/ACME paths
// are derived from Root by Normalize.
func DefaultConfig() Config {
	return Config{
		Root:      DefaultRoot,
		HTTPSAddr: DefaultHTTPSAddr,
		TLSMode:   DefaultTLSMode,
	}
}

// ConfigFromEnv reads the documented TES3MP_PANEL_* variables. An unset or
// empty variable keeps the default and is filled by Normalize.
func ConfigFromEnv(getenv func(string) string) Config {
	if getenv == nil {
		getenv = os.Getenv
	}
	return Config{
		Root:          getenv(EnvRoot),
		DataDir:       getenv(EnvDataDir),
		HTTPSAddr:     getenv(EnvHTTPSAddr),
		PublicAddress: getenv(EnvPublicAddress),
		TLSMode:       getenv(EnvTLSMode),
		CertFile:      getenv(EnvTLSCert),
		KeyFile:       getenv(EnvTLSKey),
		ACMEDirectory: getenv(EnvACMEDirectory),
		ACMEEmail:     getenv(EnvACMEEmail),
		ACMEProfile:   getenv(EnvACMEProfile),
		Language:      getenv(EnvLanguage),
	}
}

// Normalize fills production defaults and paths derived from Root. It is
// idempotent.
func (c Config) Normalize() Config {
	if c.Root == "" {
		c.Root = DefaultRoot
	}
	if c.DataDir == "" {
		c.DataDir = filepath.Join(c.Root, "panel-data")
	}
	if c.HTTPSAddr == "" {
		c.HTTPSAddr = DefaultHTTPSAddr
	}
	if c.TLSMode == "" {
		c.TLSMode = DefaultTLSMode
	}
	if c.CertFile == "" {
		c.CertFile = filepath.Join(c.DataDir, "tls", "cert.pem")
	}
	if c.KeyFile == "" {
		c.KeyFile = filepath.Join(c.DataDir, "tls", "key.pem")
	}
	if c.ACMEDirectory == "" {
		c.ACMEDirectory = DefaultACMEDirectory
	}
	if c.ACMEProfile == "" {
		c.ACMEProfile = DefaultACMEProfile
	}
	if c.ACMEAccountKeyFile == "" {
		c.ACMEAccountKeyFile = filepath.Join(c.DataDir, "acme", "account.key")
	}
	if c.Language == "" {
		c.Language = string(DefaultLanguage)
	}
	return c
}

// Validate checks a normalized configuration. It intentionally does not test
// whether ports are already in use; Listen reports that.
func (c Config) Validate() error {
	c = c.Normalize()

	switch {
	case c.Root == "":
		return errors.New("root is empty")
	case c.DataDir == "":
		return errors.New("data directory is empty")
	case c.HTTPSAddr == "":
		return errors.New("https address is empty")
	case c.CertFile == "":
		return errors.New("TLS certificate path is empty")
	case c.KeyFile == "":
		return errors.New("TLS key path is empty")
	}

	if _, _, err := splitAddr(c.HTTPSAddr); err != nil {
		return fmt.Errorf("https address %q: %w", c.HTTPSAddr, err)
	}

	if _, ok := i18n.Parse(c.Language); !ok {
		return fmt.Errorf("language %q: must be %q or %q",
			c.Language, i18n.English, i18n.Russian)
	}

	switch c.TLSMode {
	case TLSModeACME:
		if c.ACMEAccountKeyFile == "" {
			return errors.New("ACME account key path is empty")
		}
		if err := validateACMEIdentifier(c.PublicAddress); err != nil {
			return fmt.Errorf("public address %q: %w", c.PublicAddress, err)
		}
		if err := validateACMEDirectory(c.ACMEDirectory); err != nil {
			return fmt.Errorf("ACME directory %q: %w", c.ACMEDirectory, err)
		}
		if strings.TrimSpace(c.ACMEProfile) == "" {
			return errors.New("ACME profile is empty")
		}
		if err := validateACMEEmail(c.ACMEEmail); err != nil {
			return fmt.Errorf("ACME email %q: %w", c.ACMEEmail, err)
		}
	case TLSModeFiles:
		if c.PublicAddress != "" {
			if err := validatePublicAddress(c.PublicAddress); err != nil {
				return fmt.Errorf("public address %q: %w", c.PublicAddress, err)
			}
		}
	default:
		return fmt.Errorf("TLS mode %q: must be %q or %q",
			c.TLSMode, TLSModeACME, TLSModeFiles)
	}
	return nil
}

func splitAddr(addr string) (host, port string, err error) {
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", err
	}
	if port == "" {
		return "", "", errors.New("port is empty")
	}
	return host, port, nil
}

// validatePublicAddress accepts an IP, a DNS name, or either with a port. It
// rejects schemes and paths because the value is a host, not a URL.
func validatePublicAddress(address string) error {
	if strings.IndexFunc(address, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) >= 0 {
		return errors.New("must not contain whitespace or control characters")
	}
	if strings.Contains(address, "://") {
		return errors.New("must not contain a URL scheme")
	}
	if strings.ContainsAny(address, "/?#") {
		return errors.New("must not contain a path, query or fragment")
	}

	if host, port, err := net.SplitHostPort(address); err == nil {
		if host == "" {
			return errors.New("host is empty")
		}
		if port == "" {
			return errors.New("port is empty")
		}
		return nil
	}

	if strings.HasPrefix(address, "[") || strings.HasSuffix(address, "]") {
		return errors.New("brackets are only allowed around a host with a port")
	}
	if net.ParseIP(address) != nil {
		return nil
	}
	if strings.Contains(address, ":") {
		return errors.New("invalid host or host:port")
	}
	return nil
}

// validateACMEIdentifier accepts the bare public IP that will be placed in the
// certificate. A port, scheme or DNS name is rejected because the embedded
// client only implements RFC 8738 IP certificates.
func validateACMEIdentifier(address string) error {
	if address == "" {
		return errors.New("is required in acme mode")
	}
	if strings.IndexFunc(address, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) >= 0 {
		return errors.New("must not contain whitespace or control characters")
	}
	ip := net.ParseIP(address)
	if ip == nil {
		return errors.New("must be a bare IP address without a port")
	}
	if !isPublicIP(ip) {
		return errors.New("must be a public unicast IP address")
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!isSharedIPv4(ip) &&
		!isReservedIPv4(ip)
}

// isSharedIPv4 recognizes RFC 6598 carrier-grade NAT space (100.64.0.0/10).
// It is not globally routable, so Let's Encrypt cannot validate it.
func isSharedIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}

// isReservedIPv4 rejects 0.0.0.0/8 and 240.0.0.0/4. Go's IsGlobalUnicast
// treats most of these as global, but they can never be a public server address.
func isReservedIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && (ip4[0] == 0 || ip4[0] >= 240)
}

func validateACMEDirectory(directory string) error {
	parsed, err := url.Parse(directory)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("must use https")
	}
	if parsed.Host == "" {
		return errors.New("host is empty")
	}
	if parsed.Fragment != "" {
		return errors.New("must not contain a fragment")
	}
	return nil
}

func validateACMEEmail(email string) error {
	if email == "" {
		return nil
	}
	if strings.IndexFunc(email, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) >= 0 {
		return errors.New("must not contain whitespace or control characters")
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return errors.New("must be a valid email address")
	}
	return nil
}
