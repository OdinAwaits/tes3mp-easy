// Package fonts installs the fixed OpenMW font pack into the OpenMW
// configuration directory and generates openmw_font.xml.
//
// Font files never touch Morrowind or a server profile. A failed download is a
// warning rather than a launch error: the game keeps its standard font.
package fonts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/download"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

const fontRoot = "https://raw.githubusercontent.com/google/fonts/main/ofl"

// Asset is one pinned font file. The three assets map to Magic Cards, Daedric
// and MonoFont in openmw_font.xml.
type Asset struct {
	Name   string
	URL    string
	SHA256 string
}

// Assets is the default set (SetNoto): Noto Sans, Noto Serif and Roboto Mono.
// All three are OFL-licensed Google Fonts; the files themselves are pinned in
// sets.go together with the rest of the catalog.
var Assets = []Asset{notoSans, notoSerif, robotoMono}

// Stage is the current installation phase.
type Stage string

const (
	StageDownloading Stage = "downloading"
	StageInstalling  Stage = "installing"
	StageVerifying   Stage = "verifying"
)

// Progress is one installation update. Asset is set for per-font stages.
type Progress struct {
	Stage      Stage
	Asset      string
	Downloaded int64
	Total      int64
}

// Options controls Install. ConfigDir is the OpenMW configuration directory
// (".../My Games/OpenMW"); fonts are written to ConfigDir/Fonts.
type Options struct {
	ConfigDir string
	CacheDir  string
	// Set selects the catalog entry. An empty value means DefaultSet; Assets
	// overrides it for a custom set (tests).
	Set       string
	Assets    []Asset
	Client    *http.Client
	Attempts  int
	RetryWait time.Duration

	Progress func(Progress)
}

// State describes the installed font set.
type State struct {
	// Set is the resolved catalog identifier the state was checked against.
	Set       string
	FontsDir  string
	XMLPath   string
	Installed bool
	Missing   []string
	Invalid   []string
}

// Result describes one Install call.
type Result struct {
	State      State
	Downloaded bool
	Cached     bool
	Warnings   []string
}

// DefaultOptions returns production options for an OpenMW config directory. The
// asset list is resolved from Set by Install, so the caller only chooses the
// catalog entry.
func DefaultOptions(configDir, cacheDir string) Options {
	return Options{
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		Attempts:  3,
		RetryWait: time.Second,
	}
}

// Install fetches all three fonts and writes openmw_font.xml when every font
// is present and verified. Individual download failures are returned as
// warnings with Installed=false; only invalid options are errors.
func Install(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options = options.withDefaults()
	if err := options.validate(); err != nil {
		return Result{}, err
	}
	fontsDir := filepath.Join(options.ConfigDir, "Fonts")
	if err := os.MkdirAll(fontsDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create fonts directory: %w", err)
	}

	result := Result{}
	for _, asset := range options.Assets {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fetched, err := fetchAsset(ctx, options, asset)
		if err != nil {
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("font %s: %v", asset.Name, err))
			continue
		}
		result.Downloaded = result.Downloaded || fetched.Downloaded
		result.Cached = result.Cached || fetched.Cached

		options.report(Progress{Stage: StageInstalling, Asset: asset.Name})
		target := filepath.Join(fontsDir, asset.Name)
		if err := installFile(fetched.Path, target); err != nil {
			return Result{}, fmt.Errorf("install font %s: %w", asset.Name, err)
		}
	}

	state, err := inspect(fontsDir, options.Assets)
	if err != nil {
		return Result{}, err
	}
	state.Set = options.Set
	if state.Installed {
		options.report(Progress{Stage: StageVerifying})
		xmlPath := filepath.Join(fontsDir, XMLName)
		if err := atomicfile.WriteFile(xmlPath, fontXML(options.Assets), 0o600); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", XMLName, err)
		}
		state.XMLPath = xmlPath
		removeStaleFonts(fontsDir, options.Assets)
		pruneFontCache(filepath.Join(options.CacheDir, "fonts"), options.Assets)
	} else {
		for _, asset := range options.Assets {
			path := filepath.Join(fontsDir, asset.Name)
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"font %s is not installed", asset.Name))
			}
		}
	}
	result.State = state
	return result, nil
}

// EnsureCached downloads the files of one set into CacheDir/fonts and verifies
// them, without writing anything into the OpenMW directory. The font preview
// uses it to show the chosen set before Finish; Install reads the same cache,
// so the install that follows needs no network. Failures are warnings, like in
// Install: a preview without files is not an error of the launcher.
func EnsureCached(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options = options.withDefaults()
	if err := options.validate(); err != nil {
		return Result{}, err
	}
	result := Result{}
	for _, asset := range options.Assets {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fetched, err := fetchAsset(ctx, options, asset)
		if err != nil {
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("font %s: %v", asset.Name, err))
			continue
		}
		result.Downloaded = result.Downloaded || fetched.Downloaded
		result.Cached = result.Cached || fetched.Cached
	}
	return result, nil
}

// fetchAsset fills the cache with one font and reports its byte progress, so
// Install and EnsureCached download exactly the same way.
func fetchAsset(ctx context.Context, options Options,
	asset Asset,
) (download.Result, error) {
	options.report(Progress{Stage: StageDownloading, Asset: asset.Name})
	return download.Fetch(ctx, download.Options{
		URL:       asset.URL,
		SHA256:    asset.SHA256,
		Dest:      filepath.Join(options.CacheDir, "fonts", asset.Name),
		Client:    options.Client,
		Attempts:  options.Attempts,
		RetryWait: options.RetryWait,
		Progress: func(update download.Progress) {
			options.report(Progress{
				Stage:      StageDownloading,
				Asset:      asset.Name,
				Downloaded: update.Downloaded,
				Total:      update.Total,
			})
		},
	})
}

// InspectSet verifies the files of one catalog set. Files of another set count
// as missing, which is what the Installation row reports after a set switch.
func InspectSet(fontsDir, setID string) (State, error) {
	set := ResolveSet(setID)
	state, err := inspect(fontsDir, set.Assets)
	if err != nil {
		return State{}, err
	}
	state.Set = set.ID
	return state, nil
}

func inspect(fontsDir string, assets []Asset) (State, error) {
	if strings.TrimSpace(fontsDir) == "" {
		return State{}, errors.New("fonts directory is empty")
	}
	absolute, err := filepath.Abs(fontsDir)
	if err != nil {
		return State{}, fmt.Errorf("fonts directory %q: %w", fontsDir, err)
	}
	state := State{FontsDir: absolute}
	for _, asset := range assets {
		path := filepath.Join(absolute, asset.Name)
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			state.Missing = append(state.Missing, asset.Name)
			continue
		case err != nil:
			return State{}, fmt.Errorf("inspect font %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
			state.Invalid = append(state.Invalid, asset.Name)
			continue
		}
		digest, _, err := download.FileSHA256(path)
		if err != nil {
			return State{}, fmt.Errorf("verify font %s: %w", path, err)
		}
		if !strings.EqualFold(digest, asset.SHA256) {
			state.Invalid = append(state.Invalid, asset.Name)
		}
	}
	state.Installed = len(state.Missing) == 0 && len(state.Invalid) == 0
	return state, nil
}

func (o Options) withDefaults() Options {
	o.Set = strings.ToLower(strings.TrimSpace(o.Set))
	if o.Set == "" {
		o.Set = DefaultSet
	}
	if len(o.Assets) == 0 {
		o.Assets = append([]Asset(nil), ResolveSet(o.Set).Assets...)
	}
	if o.Attempts == 0 {
		o.Attempts = 3
	}
	return o
}

func (o Options) validate() error {
	if !KnownSet(o.Set) {
		return fmt.Errorf("unknown font set %q", o.Set)
	}
	if strings.TrimSpace(o.ConfigDir) == "" {
		return errors.New("OpenMW config directory is empty")
	}
	if !filepath.IsAbs(o.ConfigDir) {
		return fmt.Errorf("OpenMW config directory %q is not absolute", o.ConfigDir)
	}
	if strings.TrimSpace(o.CacheDir) == "" {
		return errors.New("client cache directory is empty")
	}
	if !filepath.IsAbs(o.CacheDir) {
		return fmt.Errorf("client cache directory %q is not absolute", o.CacheDir)
	}
	if len(o.Assets) != 3 {
		return fmt.Errorf("font set must contain exactly 3 files, got %d",
			len(o.Assets))
	}
	seen := make(map[string]struct{}, len(o.Assets))
	for _, asset := range o.Assets {
		if err := pathguard.Base(asset.Name); err != nil {
			return fmt.Errorf("font name: %w", err)
		}
		if !strings.HasSuffix(strings.ToLower(asset.Name), ".ttf") {
			return fmt.Errorf("font %s must be a .ttf file", asset.Name)
		}
		key := strings.ToLower(asset.Name)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate font %s", asset.Name)
		}
		seen[key] = struct{}{}
		if len(asset.SHA256) != 64 {
			return fmt.Errorf("font %s SHA256 must be 64 hex characters",
				asset.Name)
		}
		if strings.TrimSpace(asset.URL) == "" {
			return fmt.Errorf("font %s URL is empty", asset.Name)
		}
	}
	if o.Attempts < 1 {
		return errors.New("font download attempts must be positive")
	}
	if o.RetryWait < 0 {
		return errors.New("font download retry delay must not be negative")
	}
	return nil
}

func (o Options) report(progress Progress) {
	if o.Progress != nil {
		o.Progress(progress)
	}
}

// removeStaleFonts deletes the files of a previous set after a successful
// install. Only names the launcher itself writes are considered, so a font the
// user dropped into Fonts/ is never touched. A failed removal is ignored: the
// generated openmw_font.xml already stopped referencing the file.
func removeStaleFonts(fontsDir string, assets []Asset) {
	keep := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		keep[strings.ToLower(asset.Name)] = struct{}{}
	}
	for _, name := range ManagedAssetNames() {
		if _, ok := keep[strings.ToLower(name)]; ok {
			continue
		}
		path := filepath.Join(fontsDir, name)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 ||
			!info.Mode().IsRegular() {
			continue
		}
		_ = os.Remove(path)
	}
}

// pruneFontCache drops cached files of other sets after a successful install.
// Ten sets would otherwise keep tens of megabytes in cache/fonts, while a cache
// hit only saves a download when the user switches back to the same set. The
// files of the installed set stay, so a retry of the same set is offline. A
// failure is ignored: the cache is a convenience, not a contract.
func pruneFontCache(cacheDir string, assets []Asset) {
	keep := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		keep[strings.ToLower(asset.Name)] = struct{}{}
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := keep[strings.ToLower(entry.Name())]; ok {
			continue
		}
		_ = os.Remove(filepath.Join(cacheDir, entry.Name()))
	}
}

func installFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	writer, err := atomicfile.New(target, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(writer, in); err != nil {
		return errors.Join(err, writer.Abort())
	}
	return writer.Commit()
}
