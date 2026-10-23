// Package locale installs the optional Russian Morrowind localization as an
// overlay in a server profile.
//
// The original Morrowind installation is read-only. The main archive provides
// Data Files, the optional voice archive adds Sound, and both are assembled in
// servers/<id>/localization before being copied over the profile Data Files.
package locale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

const (
	// Language is the supported localization identifier.
	Language = "russian"
	// Encoding is required by the Cyrillic Morrowind data files.
	Encoding = "win1251"

	// ReleaseURL is the pinned GitHub release with both archives.
	ReleaseURL = "https://github.com/Saagilat/tes3mp-easy/releases/download/russian-v1.0"

	// ArchiveName and VoicesArchiveName are the release asset filenames.
	ArchiveName       = "russian.tar"
	VoicesArchiveName = "voices_russian.tar"

	// ArchiveSHA256 and VoicesSHA256 pin the release assets.
	ArchiveSHA256 = "1376aa1ec46d31fa9c12e605941d596b1e8c0cfb7592d0593acfb112c3d25e61"
	// VoicesSHA256 pins the optional Russian voice archive.
	VoicesSHA256 = "7abc6591fc5507221c732df08fff7fc1ac5d7447e783660d25ed8cc7b885d6c7"

	// DirName is the per-server localization directory.
	DirName = "localization"
	// AppliedName records files copied into Data Files.
	AppliedName = ".locale-applied.json"
	markerName  = ".locale.json"
	mainDir     = "main"
	voicesDir   = "voices"
)

// Stage is the current installation phase.
type Stage string

const (
	StageDownloading Stage = "downloading"
	StageExtracting  Stage = "extracting"
	StageInstalling  Stage = "installing"
)

// Progress is one installation update. Name identifies the archive.
type Progress struct {
	Stage      Stage
	Name       string
	Downloaded int64
	Total      int64
}

// Options controls Install.
type Options struct {
	WorkDir   string
	CacheDir  string
	ServerID  string
	Voiceover bool

	URL          string
	SHA256       string
	VoicesURL    string
	VoicesSHA256 string
	Client       *http.Client
	Attempts     int
	RetryWait    time.Duration
	Progress     func(Progress)
}

// State describes the installed overlay.
type State struct {
	Dir          string
	Installed    bool
	Voiceover    bool
	MainSHA256   string
	VoicesSHA256 string
}

// Result describes one Install call.
type Result struct {
	State      State
	Skipped    bool
	Downloaded bool
	Cached     bool
	Warnings   []string
}

type marker struct {
	Language     string `json:"language"`
	MainSHA256   string `json:"main_sha256"`
	VoicesSHA256 string `json:"voices_sha256,omitempty"`
	Voiceover    bool   `json:"voiceover"`
}

// DefaultOptions returns production options for a server profile.
func DefaultOptions(workDir, cacheDir, serverID string, voiceover bool) Options {
	return Options{
		WorkDir:      workDir,
		CacheDir:     cacheDir,
		ServerID:     serverID,
		Voiceover:    voiceover,
		URL:          ReleaseURL + "/" + ArchiveName,
		SHA256:       ArchiveSHA256,
		VoicesURL:    ReleaseURL + "/" + VoicesArchiveName,
		VoicesSHA256: VoicesSHA256,
		Attempts:     3,
		RetryWait:    time.Second,
	}
}

// ServerDir returns servers/<id> below workDir.
func ServerDir(workDir, serverID string) string {
	return filepath.Join(workDir, "servers", serverID)
}

// Dir returns the localization directory for a server profile.
func Dir(workDir, serverID string) string {
	return filepath.Join(ServerDir(workDir, serverID), DirName)
}

// OverlayDirs returns the existing Data Files overlay layers in copy order.
func OverlayDirs(workDir, serverID string) []string {
	root := Dir(workDir, serverID)
	candidates := []string{
		filepath.Join(root, mainDir, "Data Files"),
		filepath.Join(root, voicesDir, "Data Files"),
	}
	var dirs []string
	for _, candidate := range candidates {
		if info, err := os.Lstat(candidate); err == nil &&
			info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
			dirs = append(dirs, candidate)
		}
	}
	return dirs
}

// Inspect reports whether the overlay and its marker are present.
func Inspect(workDir, serverID string) (State, error) {
	dir, err := validateProfile(workDir, serverID)
	if err != nil {
		return State{}, err
	}
	state := State{Dir: filepath.Join(dir, DirName)}
	data, err := os.ReadFile(filepath.Join(state.Dir, markerName))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read localization marker: %w", err)
	}
	var meta marker
	if err := json.Unmarshal(data, &meta); err != nil {
		return State{}, fmt.Errorf("parse localization marker: %w", err)
	}
	if meta.Language != Language || meta.MainSHA256 == "" {
		return State{}, errors.New("localization marker is invalid")
	}
	mainPath := filepath.Join(state.Dir, mainDir, "Data Files")
	if info, err := os.Lstat(mainPath); err != nil ||
		info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return State{}, fmt.Errorf("inspect localization: %w", err)
		}
		return state, nil
	}
	state.Installed = true
	state.Voiceover = meta.Voiceover
	state.MainSHA256 = meta.MainSHA256
	state.VoicesSHA256 = meta.VoicesSHA256
	return state, nil
}

// Install downloads, verifies and activates the localization overlay. A voice
// archive failure is a warning: the main localization remains installed.
func Install(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options = options.withDefaults()
	if err := options.validate(); err != nil {
		return Result{}, err
	}
	serverDir, err := validateProfile(options.WorkDir, options.ServerID)
	if err != nil {
		return Result{}, err
	}
	state, err := Inspect(options.WorkDir, options.ServerID)
	if err != nil {
		return Result{}, err
	}
	voicesMatch := !options.Voiceover ||
		strings.EqualFold(state.VoicesSHA256, options.VoicesSHA256)
	if state.Installed && strings.EqualFold(state.MainSHA256, options.SHA256) &&
		state.Voiceover == options.Voiceover && voicesMatch {
		return Result{State: state, Skipped: true}, nil
	}
	if err := os.MkdirAll(options.CacheDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create server directory: %w", err)
	}

	staging := filepath.Join(serverDir, DirName+".staging")
	if err := os.RemoveAll(staging); err != nil {
		return Result{}, fmt.Errorf("remove stale localization staging: %w", err)
	}
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return Result{}, fmt.Errorf("create localization staging: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	result := Result{}
	mainFetched, err := options.fetch(ctx, options.URL, options.SHA256,
		filepath.Join(options.CacheDir, ArchiveName), ArchiveName)
	if err != nil {
		return Result{}, fmt.Errorf("localization archive: %w", err)
	}
	result.Downloaded = mainFetched.Downloaded
	result.Cached = mainFetched.Cached
	options.report(Progress{Stage: StageExtracting, Name: ArchiveName})
	mainTarget := filepath.Join(staging, mainDir, "Data Files")
	if err := os.MkdirAll(mainTarget, 0o700); err != nil {
		return Result{}, fmt.Errorf("create localization layer: %w", err)
	}
	if err := archive.ExtractOverlayFile(mainFetched.Path, mainTarget,
		mapEntry); err != nil {
		return Result{}, fmt.Errorf("extract localization: %w", err)
	}
	if count, err := countFiles(mainTarget); err != nil {
		return Result{}, err
	} else if count == 0 {
		return Result{}, errors.New(
			"localization archive contains no Data Files entries")
	}

	meta := marker{
		Language:   Language,
		MainSHA256: options.SHA256,
		Voiceover:  false,
	}
	if options.Voiceover {
		options.report(Progress{Stage: StageDownloading, Name: VoicesArchiveName})
		voicesFetched, voiceErr := options.fetch(ctx, options.VoicesURL,
			options.VoicesSHA256,
			filepath.Join(options.CacheDir, VoicesArchiveName), VoicesArchiveName)
		if voiceErr != nil {
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Russian voiceover was not installed: %v", voiceErr))
		} else {
			result.Downloaded = result.Downloaded || voicesFetched.Downloaded
			result.Cached = result.Cached || voicesFetched.Cached
			options.report(Progress{Stage: StageExtracting, Name: VoicesArchiveName})
			voiceTarget := filepath.Join(staging, voicesDir, "Data Files")
			if err := os.MkdirAll(voiceTarget, 0o700); err != nil {
				return Result{}, fmt.Errorf("create voiceover layer: %w", err)
			}
			voiceCount, extractErr := 0, archive.ExtractOverlayFile(
				voicesFetched.Path, voiceTarget, mapEntry)
			if extractErr == nil {
				voiceCount, extractErr = countFiles(voiceTarget)
				if extractErr == nil && voiceCount == 0 {
					extractErr = errors.New("voice archive contains no Data Files entries")
				}
			}
			if extractErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"Russian voiceover was not installed: %v", extractErr))
				_ = os.RemoveAll(voiceTarget)
			} else {
				meta.Voiceover = true
				meta.VoicesSHA256 = options.VoicesSHA256
			}
		}
	}

	options.report(Progress{Stage: StageInstalling, Name: ArchiveName})
	if err := writeMarker(filepath.Join(staging, markerName), meta); err != nil {
		return Result{}, err
	}
	warnings, err := swapInstallation(staging, filepath.Join(serverDir, DirName))
	if err != nil {
		return Result{}, err
	}
	committed = true
	result.Warnings = append(result.Warnings, warnings...)
	state, err = Inspect(options.WorkDir, options.ServerID)
	if err != nil {
		return Result{}, err
	}
	if !state.Installed {
		return Result{}, errors.New("localization was extracted but is not installed")
	}
	result.State = state
	return result, nil
}

// Remove deletes the localization layers for one server profile. Files already
// copied into Data Files are handled by RemoveOverlay.
func Remove(workDir, serverID string) error {
	serverDir, err := validateProfile(workDir, serverID)
	if err != nil {
		return err
	}
	dir := filepath.Join(serverDir, DirName)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove localization: %w", err)
	}
	for _, stale := range []string{dir + ".old", dir + ".staging"} {
		if err := os.RemoveAll(stale); err != nil {
			return fmt.Errorf("remove stale localization %s: %w", stale, err)
		}
	}
	return nil
}
