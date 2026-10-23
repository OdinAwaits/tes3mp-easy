package tes3mp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

// Stage is the current installation phase reported through Progress.
type Stage string

const (
	StageDownloading Stage = "downloading"
	StageVerifying   Stage = "verifying"
	StageExtracting  Stage = "extracting"
	StageInstalling  Stage = "installing"
)

// Progress is one installation update. Downloaded/Total are meaningful during
// StageDownloading; other stages are emitted once with zero counters.
type Progress struct {
	Stage      Stage
	Downloaded int64
	Total      int64
}

// Options controls Install. Dir and CacheDir are required; empty release
// fields use the version pinned by this package.
type Options struct {
	Dir       string
	CacheDir  string
	URL       string
	SHA256    string
	Version   string
	Attempts  int
	RetryWait time.Duration
	Client    *http.Client

	Progress func(Progress)
}

// DefaultOptions returns production options for a work directory and cache.
func DefaultOptions(dir, cacheDir string) Options {
	return Options{
		Dir:       dir,
		CacheDir:  cacheDir,
		URL:       URL,
		SHA256:    SHA256,
		Version:   Version,
		Attempts:  3,
		RetryWait: time.Second,
	}
}

// Result describes a successful or skipped installation.
type Result struct {
	State      State
	Skipped    bool
	Downloaded bool
	Cached     bool
	Warnings   []string
}

// Install downloads, verifies and installs TES3MP into options.Dir. A current
// installation is left untouched. The new directory is assembled in a staging
// directory and swapped with the old one only after the executable and version
// marker are present.
func Install(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options = options.withDefaults()
	if err := options.validate(); err != nil {
		return Result{}, err
	}
	dir, err := validateDir(options.Dir)
	if err != nil {
		return Result{}, err
	}

	state, err := inspectVersion(dir, options.Version)
	if err != nil {
		return Result{}, err
	}
	if state.Installed && state.Current {
		return Result{State: state, Skipped: true}, nil
	}

	if err := os.MkdirAll(options.CacheDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return Result{}, fmt.Errorf("create work directory: %w", err)
	}

	archiveName, err := archiveFilename(options.URL)
	if err != nil {
		return Result{}, err
	}
	archivePath := filepath.Join(options.CacheDir, archiveName)
	if info, err := os.Lstat(archivePath); err == nil &&
		(info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return Result{}, fmt.Errorf("cache path %s is not a regular file", archivePath)
	}

	cached := false
	result := Result{}
	if err := verifyFile(archivePath, options.SHA256); err == nil {
		cached = true
	} else if !errors.Is(err, os.ErrNotExist) {
		if removeErr := os.Remove(archivePath); removeErr != nil {
			return Result{}, fmt.Errorf("remove invalid cache archive: %w",
				removeErr)
		}
	}
	if !cached {
		options.report(Progress{Stage: StageDownloading})
		if err := options.download(ctx, options.URL, archivePath); err != nil {
			return Result{}, err
		}
		result.Downloaded = true
	}
	result.Cached = cached

	options.report(Progress{Stage: StageVerifying})
	if err := verifyFile(archivePath, options.SHA256); err != nil {
		return Result{}, fmt.Errorf(
			"TES3MP archive checksum mismatch; delete %s and retry: %w",
			archivePath, err)
	}

	staging, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".staging-")
	if err != nil {
		return Result{}, fmt.Errorf("create TES3MP staging directory: %w", err)
	}
	installed := false
	defer func() {
		if !installed {
			_ = os.RemoveAll(staging)
		}
	}()

	options.report(Progress{Stage: StageExtracting})
	if err := archive.ExtractZipFileWithLimits(archivePath, staging,
		archive.DefaultZipLimits()); err != nil {
		return Result{}, fmt.Errorf("extract TES3MP archive: %w", err)
	}
	executable := filepath.Join(staging, Executable)
	info, err := os.Stat(executable)
	if err != nil {
		return Result{}, fmt.Errorf("TES3MP archive does not contain %s: %w",
			Executable, err)
	}
	if !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("TES3MP archive has a non-regular %s", Executable)
	}
	marker := []byte(options.Version + "\n")
	if err := atomicfile.WriteFile(filepath.Join(staging, VersionFile),
		marker, 0o644); err != nil {
		return Result{}, fmt.Errorf("write TES3MP version marker: %w", err)
	}

	options.report(Progress{Stage: StageInstalling})
	warnings, err := swapInstallation(staging, dir)
	if err != nil {
		return Result{}, err
	}
	result.Warnings = warnings
	installed = true

	state, err = inspectVersion(dir, options.Version)
	if err != nil {
		return Result{}, err
	}
	if !state.Current {
		return Result{}, fmt.Errorf(
			"TES3MP was extracted but version marker is not current")
	}
	result.State = state
	return result, nil
}

func (o Options) withDefaults() Options {
	if o.Version == "" {
		o.Version = Version
	}
	if o.URL == "" {
		o.URL = URL
	}
	if o.SHA256 == "" {
		o.SHA256 = SHA256
	}
	if o.Attempts == 0 {
		o.Attempts = 3
	}
	if o.RetryWait == 0 {
		o.RetryWait = time.Second
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 15 * time.Minute}
	}
	return o
}

func (o Options) validate() error {
	if strings.TrimSpace(o.Dir) == "" {
		return errors.New("TES3MP directory is empty")
	}
	if strings.TrimSpace(o.CacheDir) == "" {
		return errors.New("TES3MP cache directory is empty")
	}
	if o.Attempts < 1 {
		return errors.New("TES3MP download attempts must be positive")
	}
	if o.RetryWait < 0 {
		return errors.New("TES3MP retry delay must not be negative")
	}
	if len(o.SHA256) != sha256.Size*2 {
		return errors.New("TES3MP SHA256 must be 64 hex characters")
	}
	if _, err := hex.DecodeString(o.SHA256); err != nil {
		return errors.New("TES3MP SHA256 must be hexadecimal")
	}
	return nil
}

func (o Options) report(progress Progress) {
	if o.Progress != nil {
		o.Progress(progress)
	}
}

func validateDir(dir string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return "", fmt.Errorf("TES3MP directory %q: %w", dir, err)
	}
	volume := filepath.VolumeName(absolute)
	root := volume + string(filepath.Separator)
	if absolute == root || filepath.Dir(absolute) == absolute {
		return "", fmt.Errorf(
			"refusing to use filesystem root %q as TES3MP directory", absolute)
	}
	return absolute, nil
}

func archiveFilename(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse TES3MP URL: %w", err)
	}
	name := filepath.Base(parsed.Path)
	if err := pathguard.Base(name); err != nil {
		return "", fmt.Errorf("TES3MP archive name: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return "", errors.New("TES3MP archive URL must end in .zip")
	}
	return name, nil
}

func validateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid TES3MP URL: %w", err)
	}
	if parsed.User != nil {
		return errors.New("TES3MP URL must not contain credentials")
	}
	if parsed.Fragment != "" {
		return errors.New("TES3MP URL must not contain a fragment")
	}
	if parsed.Hostname() == "" {
		return errors.New("TES3MP URL has no host")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(parsed.Hostname()) {
			return nil
		}
		return errors.New("TES3MP download requires HTTPS")
	default:
		return fmt.Errorf("unsupported TES3MP URL scheme %q", parsed.Scheme)
	}
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func verifyFile(path, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 is %s, want %s", got, want)
	}
	return nil
}
