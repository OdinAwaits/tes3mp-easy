// Package panel implements the HTTPS entry point of tes3mp-easy.
//
// This step provides configuration, TLS material management, the listener pair,
// graceful lifecycle, Argon2id password authentication, in-memory sessions,
// CSRF protection, the subprocess client for tes3mp-easy-ops, the public
// server/mods/backups routes and the admin status/logs/mods/state/config/staff
// pages with streaming upload.
package panel

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/version"
)

var (
	// Version is reported by the panel binary and the public API.
	Version = version.Version
)

const (
	// DefaultRoot is the production data root.
	DefaultRoot = "/tes3mp-easy"

	// DefaultHTTPAddr is the ACME/redirect listener.
	DefaultHTTPAddr = ":80"

	// DefaultHTTPSAddr is the TLS listener.
	DefaultHTTPSAddr = ":443"
)

const (
	EnvRoot          = "TES3MP_PANEL_ROOT"
	EnvDataDir       = "TES3MP_PANEL_DATA_DIR"
	EnvHTTPAddr      = "TES3MP_PANEL_HTTP_ADDR"
	EnvHTTPSAddr     = "TES3MP_PANEL_HTTPS_ADDR"
	EnvPublicAddress = "TES3MP_PANEL_PUBLIC_ADDRESS"
	EnvTLSCert       = "TES3MP_PANEL_TLS_CERT"
	EnvTLSKey        = "TES3MP_PANEL_TLS_KEY"
)

// Config describes one panel process. Empty derived fields are filled by
// Normalize, so a custom Root automatically moves the data and TLS directories
// unless they were set explicitly.
type Config struct {
	Root          string
	DataDir       string
	HTTPAddr      string
	HTTPSAddr     string
	PublicAddress string
	CertFile      string
	KeyFile       string
}

// DefaultConfig returns production defaults. DataDir, CertFile and KeyFile are
// derived from Root by Normalize.
func DefaultConfig() Config {
	return Config{
		Root:      DefaultRoot,
		HTTPAddr:  DefaultHTTPAddr,
		HTTPSAddr: DefaultHTTPSAddr,
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
		HTTPAddr:      getenv(EnvHTTPAddr),
		HTTPSAddr:     getenv(EnvHTTPSAddr),
		PublicAddress: getenv(EnvPublicAddress),
		CertFile:      getenv(EnvTLSCert),
		KeyFile:       getenv(EnvTLSKey),
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
	if c.HTTPAddr == "" {
		c.HTTPAddr = DefaultHTTPAddr
	}
	if c.HTTPSAddr == "" {
		c.HTTPSAddr = DefaultHTTPSAddr
	}
	if c.CertFile == "" {
		c.CertFile = filepath.Join(c.DataDir, "tls", "cert.pem")
	}
	if c.KeyFile == "" {
		c.KeyFile = filepath.Join(c.DataDir, "tls", "key.pem")
	}
	return c
}

// Validate checks a normalized configuration. It intentionally does not test
// whether the ports are already in use; Listen reports that.
func (c Config) Validate() error {
	c = c.Normalize()

	switch {
	case c.Root == "":
		return errors.New("root is empty")
	case c.DataDir == "":
		return errors.New("data directory is empty")
	case c.HTTPAddr == "":
		return errors.New("http address is empty")
	case c.HTTPSAddr == "":
		return errors.New("https address is empty")
	case c.CertFile == "":
		return errors.New("TLS certificate path is empty")
	case c.KeyFile == "":
		return errors.New("TLS key path is empty")
	}

	httpHost, httpPort, err := splitAddr(c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("http address %q: %w", c.HTTPAddr, err)
	}
	httpsHost, httpsPort, err := splitAddr(c.HTTPSAddr)
	if err != nil {
		return fmt.Errorf("https address %q: %w", c.HTTPSAddr, err)
	}
	if httpPort != "0" && httpHost == httpsHost && httpPort == httpsPort {
		return errors.New("http and https addresses must differ")
	}

	if c.PublicAddress != "" {
		if err := validatePublicAddress(c.PublicAddress); err != nil {
			return fmt.Errorf("public address %q: %w", c.PublicAddress, err)
		}
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
// rejects schemes and paths because redirects add https:// themselves.
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
