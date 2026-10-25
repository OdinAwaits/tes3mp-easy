// Package locale installs optional Morrowind language packs into one global
// client overlay.
//
// The original Morrowind installation is read-only. A pack archive provides
// Data Files, the optional voice archive adds Sound, and both are assembled in
// <workDir>/localization. The launcher adds the pack Data Files directory to
// every server profile at launch, so all servers share one localization.
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
	// LanguageRussian is the supported downloadable language pack.
	LanguageRussian = "russian"
	// Language is kept as the Russian pack alias for older callers.
	Language = LanguageRussian
	// Encoding is required by the Cyrillic Morrowind data files.
	Encoding = "win1251"

	// ReleaseURL is the pinned GitHub release with both Russian archives.
	ReleaseURL = "https://github.com/Saagilat/tes3mp-easy/releases/download/russian-v1.0"

	// ArchiveName and VoicesArchiveName are the release asset filenames.
	ArchiveName       = "russian.tar"
	VoicesArchiveName = "voices_russian.tar"

	// ArchiveSHA256 and VoicesSHA256 pin the release assets.
	ArchiveSHA256 = "1376aa1ec46d31fa9c12e605941d596b1e8c0cfb7592d0593acfb112c3d25e61"
	// VoicesSHA256 pins the optional Russian voice archive.
	VoicesSHA256 = "7abc6591fc5507221c732df08fff7fc1ac5d7447e783660d25ed8cc7b885d6c7"

	// DirName is the global localization directory below the client work dir.
	DirName    = "localization"
	markerName = ".locale.json"
	mainDir    = "main"
	voicesDir  = "voices"
)

// Pack describes one downloadable game language.
type Pack struct {
	Language          string
	ArchiveName       string
	VoicesArchiveName string
	URL               string
	VoicesURL         string
	SHA256            string
	VoicesSHA256      string
	Encoding          string
}

var packs = map[string]Pack{
	LanguageRussian: {
		Language:          LanguageRussian,
		ArchiveName:       ArchiveName,
		VoicesArchiveName: VoicesArchiveName,
		URL:               ReleaseURL + "/" + ArchiveName,
		VoicesURL:         ReleaseURL + "/" + VoicesArchiveName,
		SHA256:            ArchiveSHA256,
		VoicesSHA256:      VoicesSHA256,
		Encoding:          Encoding,
	},
}

// Packages returns every downloadable localization in display order.
func Packages() []Pack {
	return []Pack{packs[LanguageRussian]}
}

// ParseLanguage accepts a persisted game language. An empty value is not a
// supported identifier.
func ParseLanguage(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := packs[value]; ok {
		return value, true
	}
	return "", false
}

// PackFor returns the language pack metadata.
func PackFor(language string) (Pack, bool) {
	language, ok := ParseLanguage(language)
	if !ok {
		return Pack{}, false
	}
	return packs[language], true
}

// EncodingFor returns the OpenMW encoding required by the language, or an
// empty string when the base game encoding should be kept.
func EncodingFor(language string) string {
	pack, ok := PackFor(language)
	if !ok {
		return ""
	}
	return pack.Encoding
}

// IsPackEncoding reports whether encoding belongs to one of the language packs.
func IsPackEncoding(encoding string) bool {
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	for _, pack := range packs {
		if pack.Encoding != "" && pack.Encoding == encoding {
			return true
		}
	}
	return false
}

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
	Language  string
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

// State describes the installed global overlay.
type State struct {
	Dir          string
	DataFiles    string
	Language     string
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

// DefaultOptions returns production options for one language.
func DefaultOptions(workDir, cacheDir, language string,
	voiceover bool,
) Options {
	language, _ = ParseLanguage(language)
	if language == "" {
		language = LanguageRussian
	}
	pack, _ := PackFor(language)
	return Options{
		WorkDir:      workDir,
		CacheDir:     cacheDir,
		Language:     language,
		Voiceover:    voiceover,
		URL:          pack.URL,
		SHA256:       pack.SHA256,
		VoicesURL:    pack.VoicesURL,
		VoicesSHA256: pack.VoicesSHA256,
		Attempts:     3,
		RetryWait:    time.Second,
	}
}

// Dir returns the global localization directory below workDir.
func Dir(workDir string) string {
	return filepath.Join(workDir, DirName)
}

// DataFiles returns the Data Files layer of an installed pack.
func DataFiles(workDir string) string {
	return filepath.Join(Dir(workDir), mainDir, "Data Files")
}

// Inspect reports whether the overlay and its marker are present.
func Inspect(workDir string) (State, error) {
	root, err := validateWorkDir(workDir)
	if err != nil {
		return State{}, err
	}
	state := State{
		Dir:       filepath.Join(root, DirName),
		DataFiles: filepath.Join(root, DirName, mainDir, "Data Files"),
	}
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
	if meta.Language == "" {
		return State{}, errors.New("localization marker is invalid")
	}
	if _, ok := ParseLanguage(meta.Language); !ok || meta.MainSHA256 == "" {
		return State{}, errors.New("localization marker is invalid")
	}
	if info, err := os.Lstat(state.DataFiles); err != nil ||
		info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return State{}, fmt.Errorf("inspect localization: %w", err)
		}
		return state, nil
	}
	state.Language = meta.Language
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
	root, err := validateWorkDir(options.WorkDir)
	if err != nil {
		return Result{}, err
	}
	pack, _ := PackFor(options.Language)
	state, err := Inspect(root)
	if err != nil {
		return Result{}, err
	}
	voicesMatch := !options.Voiceover ||
		strings.EqualFold(state.VoicesSHA256, options.VoicesSHA256)
	if state.Installed && strings.EqualFold(state.Language, options.Language) &&
		strings.EqualFold(state.MainSHA256, options.SHA256) &&
		state.Voiceover == options.Voiceover && voicesMatch {
		return Result{State: state, Skipped: true}, nil
	}
	if err := os.MkdirAll(options.CacheDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Result{}, fmt.Errorf("create client work directory: %w", err)
	}

	staging := filepath.Join(root, DirName+".staging")
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
		filepath.Join(options.CacheDir, pack.ArchiveName), pack.ArchiveName)
	if err != nil {
		return Result{}, fmt.Errorf("localization archive: %w", err)
	}
	result.Downloaded = mainFetched.Downloaded
	result.Cached = mainFetched.Cached
	options.report(Progress{Stage: StageExtracting, Name: pack.ArchiveName})
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
		Language:   options.Language,
		MainSHA256: options.SHA256,
		Voiceover:  false,
	}
	if options.Voiceover {
		options.report(Progress{Stage: StageDownloading, Name: pack.VoicesArchiveName})
		voicesFetched, voiceErr := options.fetch(ctx, options.VoicesURL,
			options.VoicesSHA256,
			filepath.Join(options.CacheDir, pack.VoicesArchiveName),
			pack.VoicesArchiveName)
		if voiceErr != nil {
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"voiceover was not installed: %v", voiceErr))
		} else {
			result.Downloaded = result.Downloaded || voicesFetched.Downloaded
			result.Cached = result.Cached || voicesFetched.Cached
			options.report(Progress{Stage: StageExtracting, Name: pack.VoicesArchiveName})
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
					"voiceover was not installed: %v", extractErr))
				_ = os.RemoveAll(voiceTarget)
			} else {
				meta.Voiceover = true
				meta.VoicesSHA256 = options.VoicesSHA256
			}
		}
	}

	options.report(Progress{Stage: StageInstalling, Name: pack.ArchiveName})
	if err := writeMarker(filepath.Join(staging, markerName), meta); err != nil {
		return Result{}, err
	}
	warnings, err := swapInstallation(staging, filepath.Join(root, DirName))
	if err != nil {
		return Result{}, err
	}
	committed = true
	result.Warnings = append(result.Warnings, warnings...)
	state, err = Inspect(root)
	if err != nil {
		return Result{}, err
	}
	if !state.Installed {
		return Result{}, errors.New("localization was extracted but is not installed")
	}
	result.State = state
	return result, nil
}

// Remove deletes the global localization overlay and any stale copies.
func Remove(workDir string) error {
	root, err := validateWorkDir(workDir)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, DirName)
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
