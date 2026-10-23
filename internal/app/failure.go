package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"

	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

// Sentinel errors identify local preconditions that the UI can explain.
var (
	ErrMorrowindNotConfigured   = errors.New("Morrowind folder is not configured")
	ErrModsNotInstalled         = errors.New("server mods are not installed")
	ErrLocalizationNotInstalled = errors.New("Russian localization is enabled but not installed")
	ErrWorkDir                  = errors.New("client work directory is invalid")
)

// FailureKind is a stable reason code used to choose a localized explanation.
type FailureKind string

const (
	FailureUnknown                FailureKind = "unknown"
	FailureMorrowindNotFound      FailureKind = "morrowind_not_found"
	FailureMorrowindIncomplete    FailureKind = "morrowind_incomplete"
	FailureMorrowindNotConfigured FailureKind = "morrowind_not_configured"
	FailureNetwork                FailureKind = "network"
	FailureCertificate            FailureKind = "certificate"
	FailureNoMods                 FailureKind = "no_mods"
	FailureServerAPI              FailureKind = "server_api"
	FailureFileSystem             FailureKind = "file_system"
	FailureWorkDir                FailureKind = "work_dir"
	FailureModsNotInstalled       FailureKind = "mods_not_installed"
	FailureLocalization           FailureKind = "localization_not_installed"
)

// Failure contains a stable reason and the original technical detail. Detail
// never contains a server password because app errors do not include profiles.
type Failure struct {
	Kind   FailureKind
	Detail string
}

// Explain classifies an error for a user-facing dialog. Unknown errors keep
// their original text so a support log still has actionable detail.
func Explain(err error) Failure {
	if err == nil {
		return Failure{}
	}
	failure := Failure{Kind: FailureUnknown, Detail: err.Error()}
	switch {
	case errors.Is(err, ErrMorrowindNotConfigured):
		failure.Kind = FailureMorrowindNotConfigured
	case errors.Is(err, morrowind.ErrNotFound):
		failure.Kind = FailureMorrowindNotFound
	case errors.Is(err, morrowind.ErrIncomplete):
		failure.Kind = FailureMorrowindIncomplete
	case errors.Is(err, serverapi.ErrNoMods):
		failure.Kind = FailureNoMods
	case errors.Is(err, ErrModsNotInstalled):
		failure.Kind = FailureModsNotInstalled
	case errors.Is(err, ErrLocalizationNotInstalled):
		failure.Kind = FailureLocalization
	case errors.Is(err, ErrWorkDir):
		failure.Kind = FailureWorkDir
	case isCertificateError(err):
		failure.Kind = FailureCertificate
	case isNetworkError(err):
		failure.Kind = FailureNetwork
	case isFilesystemError(err):
		failure.Kind = FailureFileSystem
	default:
		var apiError *serverapi.APIError
		if errors.As(err, &apiError) {
			failure.Kind = FailureServerAPI
		}
	}
	return failure
}

func isCertificateError(err error) bool {
	var verification *tls.CertificateVerificationError
	if errors.As(err, &verification) {
		return true
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return true
	}
	var hostname x509.HostnameError
	if errors.As(err, &hostname) {
		return true
	}
	var invalid x509.CertificateInvalidError
	return errors.As(err, &invalid)
}

func isNetworkError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	var dnsError *net.DNSError
	return errors.As(err, &dnsError)
}

func isFilesystemError(err error) bool {
	if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, fs.ErrInvalid) {
		return true
	}
	var pathError *os.PathError
	return errors.As(err, &pathError)
}

// WrapWorkDir adds a stable classification while preserving the original
// message for the log.
func WrapWorkDir(err error) error {
	if err == nil || errors.Is(err, ErrWorkDir) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrWorkDir, err)
}
