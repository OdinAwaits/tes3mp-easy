package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

// Sentinel errors identify local preconditions that the UI can explain.
var (
	ErrMorrowindNotConfigured   = errors.New("Morrowind folder is not configured")
	ErrModsNotInstalled         = errors.New("server mods are not installed")
	ErrLocalizationNotInstalled = errors.New("Russian localization is selected but not installed")
	ErrUnknownFontSet           = errors.New("unknown font set")
	ErrWorkDir                  = errors.New("client work directory is invalid")
	ErrGameExited               = errors.New("TES3MP exited unexpectedly")
	ErrLaunchInFlight           = errors.New("TES3MP is already starting")
	ErrGameRunning              = errors.New("TES3MP is already running")
)

// FailureKind is a stable reason code used to choose a localized explanation.
type FailureKind string

const (
	FailureUnknown                FailureKind = "unknown"
	FailureMorrowindNotFound      FailureKind = "morrowind_not_found"
	FailureMorrowindIncomplete    FailureKind = "morrowind_incomplete"
	FailureMorrowindNotConfigured FailureKind = "morrowind_not_configured"
	FailureNetwork                FailureKind = "network"
	FailureTimeout                FailureKind = "timeout"
	FailureCertificate            FailureKind = "certificate"
	FailureNoMods                 FailureKind = "no_mods"
	FailureServerAPI              FailureKind = "server_api"
	FailureFileSystem             FailureKind = "file_system"
	FailureWorkDir                FailureKind = "work_dir"
	FailureModsNotInstalled       FailureKind = "mods_not_installed"
	FailureLocalization           FailureKind = "localization_not_installed"
	FailureGameMissing            FailureKind = "game_missing"
	FailureGameProton             FailureKind = "game_proton"
	FailureGameConfig             FailureKind = "game_config"
	FailureGameStart              FailureKind = "game_start"
	FailureGameExited             FailureKind = "game_exited"
	FailureGameStarting           FailureKind = "game_starting"
	FailureGameRunning            FailureKind = "game_running"
)

// FailureCode is the stable, untranslated identifier shown in dialogs and logs.
// It never changes when UI copy or classification internals are refactored.
type FailureCode string

const (
	CodeUnknown             FailureCode = "T3E-APP-99"
	CodeMorrowindNotFound   FailureCode = "T3E-ENV-01"
	CodeMorrowindIncomplete FailureCode = "T3E-ENV-02"
	CodeMorrowindNotSet     FailureCode = "T3E-ENV-03"
	CodeWorkDir             FailureCode = "T3E-ENV-04"
	CodeFileSystem          FailureCode = "T3E-ENV-05"
	CodeNetwork             FailureCode = "T3E-NET-01"
	CodeTimeout             FailureCode = "T3E-NET-02"
	CodeCertificate         FailureCode = "T3E-NET-03"
	CodeServerAPI           FailureCode = "T3E-API-01"
	CodeInvalidResponse     FailureCode = "T3E-API-02"
	CodeNoMods              FailureCode = "T3E-API-03"
	CodeModsNotInstalled    FailureCode = "T3E-MOD-01"
	CodeLocalization        FailureCode = "T3E-LOC-01"
	CodeGameMissing         FailureCode = "T3E-GAME-01"
	CodeGameProton          FailureCode = "T3E-GAME-02"
	CodeGameConfig          FailureCode = "T3E-GAME-03"
	CodeGameStart           FailureCode = "T3E-GAME-04"
	CodeGameExited          FailureCode = "T3E-GAME-05"
	CodeGameStarting        FailureCode = "T3E-GAME-06"
	CodeGameRunning         FailureCode = "T3E-GAME-07"
)

// Failure contains a stable reason, a support code and the original technical
// detail. Detail never contains a server password because app errors do not
// include profiles.
type Failure struct {
	Kind   FailureKind
	Code   FailureCode
	Detail string
}

// Explain classifies an error for a user-facing dialog. Unknown errors keep
// their original text so a support log still has actionable detail.
func Explain(err error) Failure {
	if err == nil {
		return Failure{}
	}
	failure := Failure{
		Kind:   FailureUnknown,
		Code:   CodeUnknown,
		Detail: err.Error(),
	}
	switch {
	case errors.Is(err, ErrGameExited):
		failure.Kind, failure.Code = FailureGameExited, CodeGameExited
	case errors.Is(err, ErrLaunchInFlight):
		failure.Kind, failure.Code = FailureGameStarting, CodeGameStarting
	case errors.Is(err, ErrGameRunning):
		failure.Kind, failure.Code = FailureGameRunning, CodeGameRunning
	case errors.Is(err, launcher.ErrTES3MPMissing):
		failure.Kind, failure.Code = FailureGameMissing, CodeGameMissing
	case errors.Is(err, launcher.ErrProton):
		failure.Kind, failure.Code = FailureGameProton, CodeGameProton
	case errors.Is(err, launcher.ErrConfig):
		failure.Kind, failure.Code = FailureGameConfig, CodeGameConfig
	case errors.Is(err, launcher.ErrStart):
		failure.Kind, failure.Code = FailureGameStart, CodeGameStart
	case errors.Is(err, serverapi.ErrInvalidResponse):
		failure.Kind, failure.Code = FailureServerAPI, CodeInvalidResponse
	case errors.Is(err, ErrMorrowindNotConfigured):
		failure.Kind, failure.Code = FailureMorrowindNotConfigured, CodeMorrowindNotSet
	case errors.Is(err, morrowind.ErrNotFound):
		failure.Kind, failure.Code = FailureMorrowindNotFound, CodeMorrowindNotFound
	case errors.Is(err, morrowind.ErrIncomplete):
		failure.Kind, failure.Code = FailureMorrowindIncomplete, CodeMorrowindIncomplete
	case errors.Is(err, serverapi.ErrNoMods):
		failure.Kind, failure.Code = FailureNoMods, CodeNoMods
	case errors.Is(err, ErrModsNotInstalled):
		failure.Kind, failure.Code = FailureModsNotInstalled, CodeModsNotInstalled
	case errors.Is(err, ErrLocalizationNotInstalled):
		failure.Kind, failure.Code = FailureLocalization, CodeLocalization
	case errors.Is(err, ErrWorkDir):
		failure.Kind, failure.Code = FailureWorkDir, CodeWorkDir
	case isCertificateError(err):
		failure.Kind, failure.Code = FailureCertificate, CodeCertificate
	case errors.Is(err, context.DeadlineExceeded):
		failure.Kind, failure.Code = FailureTimeout, CodeTimeout
	case errors.Is(err, openmwcfg.ErrInvalid):
		failure.Kind, failure.Code = FailureGameConfig, CodeGameConfig
	case isFilesystemError(err):
		failure.Kind, failure.Code = FailureFileSystem, CodeFileSystem
	case isNetworkError(err):
		failure.Kind, failure.Code = FailureNetwork, CodeNetwork
	default:
		var apiError *serverapi.APIError
		if errors.As(err, &apiError) {
			failure.Kind, failure.Code = FailureServerAPI, CodeServerAPI
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
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var opError *net.OpError
	if errors.As(err, &opError) {
		return true
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return true
	}
	var addrError *net.AddrError
	if errors.As(err, &addrError) {
		return true
	}
	var urlError *url.Error
	return errors.As(err, &urlError)
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
