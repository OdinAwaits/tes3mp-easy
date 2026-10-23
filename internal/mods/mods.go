// Package mods installs the current server mod pack into a client profile.
//
// A mods archive contains plugins/ and scripts/. The client keeps only plugin
// files in servers/<id>/data/Data Files; server-side Lua scripts stay on the
// server. Every archive is checked against requiredDataFiles.json, the base
// Morrowind ESM CRCs and the SHA256 reported by the panel before anything is
// activated.
package mods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

const (
	digestFile   = ".mods-sha256"
	stagingDir   = ".staging"
	archiveSub   = "archive"
	dataSub      = "data"
	dataFilesSub = "Data Files"
	profileFile  = "openmw.cfg"
	metadataName = "requiredDataFiles.json"
)

// Stage is the current update phase.
type Stage string

const (
	StageFetching    Stage = "fetching"
	StageDownloading Stage = "downloading"
	StageVerifying   Stage = "verifying"
	StageExtracting  Stage = "extracting"
	StageInstalling  Stage = "installing"
)

// Progress is one update notification. Downloaded/Total are meaningful during
// StageDownloading; other stages carry zero counters.
type Progress struct {
	Stage      Stage
	Downloaded int64
	Total      int64
}

// Options control Install. All paths and the Morrowind installation are
// required. The server API client owns HTTPS, retries and URL validation.
type Options struct {
	WorkDir   string
	CacheDir  string
	ServerID  string
	Morrowind morrowind.Installation
	Client    *serverapi.Client
	Progress  func(Progress)
}

// Result describes a completed, skipped or re-used installation.
type Result struct {
	State      State
	Skipped    bool
	Cached     bool
	Downloaded bool
	Warnings   []string
}

// State is the local mods state of one server profile.
type State struct {
	ServerDir string
	DataFiles string
	Profile   string
	SHA256    string
	Installed bool
	Plugins   int
}

// ServerDir returns the profile directory for a validated server ID.
func ServerDir(workDir, serverID string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" || !filepath.IsAbs(workDir) {
		return "", fmt.Errorf("client work directory %q is not absolute", workDir)
	}
	if err := validateServerID(serverID); err != nil {
		return "", err
	}
	return filepath.Join(filepath.Clean(workDir), "servers", serverID), nil
}

// Inspect reads the installed digest and required paths without modifying the
// profile.
func Inspect(workDir, serverID string) (State, error) {
	serverDir, err := ServerDir(workDir, serverID)
	if err != nil {
		return State{}, err
	}
	state := State{
		ServerDir: serverDir,
		DataFiles: filepath.Join(serverDir, dataSub, dataFilesSub),
		Profile:   filepath.Join(serverDir, profileFile),
	}
	info, err := os.Lstat(serverDir)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("inspect server profile: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return State{}, fmt.Errorf("server profile %q is not a directory", serverDir)
	}
	digest, err := readDigest(filepath.Join(serverDir, digestFile))
	if err != nil {
		return state, nil
	}
	state.SHA256 = digest
	dataInfo, dataErr := os.Lstat(state.DataFiles)
	profileInfo, profileErr := os.Lstat(state.Profile)
	if dataErr != nil || profileErr != nil ||
		dataInfo.Mode()&os.ModeSymlink != 0 || !dataInfo.IsDir() ||
		profileInfo.Mode()&os.ModeSymlink != 0 || !profileInfo.Mode().IsRegular() {
		return state, nil
	}
	plugins, err := countPlugins(state.DataFiles)
	if err != nil {
		return State{}, err
	}
	state.Plugins = plugins
	state.Installed = true
	return state, nil
}

// Install fetches the current mods metadata, downloads a changed archive,
// verifies it, assembles a new Data Files tree and swaps it in atomically. The
// old tree remains available for rollback until the digest is committed.
func Install(ctx context.Context, options Options) (result Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := options.validate(); err != nil {
		return Result{}, err
	}
	serverDir, err := ServerDir(options.WorkDir, options.ServerID)
	if err != nil {
		return Result{}, err
	}
	if err := ensureDirectory(serverDir); err != nil {
		return Result{}, err
	}
	dataDir := filepath.Join(serverDir, dataSub)
	targetDF := filepath.Join(dataDir, dataFilesSub)
	targetProfile := filepath.Join(serverDir, profileFile)
	if err := recoverInstallation(targetDF, targetProfile); err != nil {
		return Result{}, err
	}

	staging := filepath.Join(serverDir, stagingDir)
	if err := removeOwnedTree(staging); err != nil {
		return Result{}, err
	}
	if err := os.Mkdir(staging, 0o700); err != nil {
		return Result{}, fmt.Errorf("create mods staging directory: %w", err)
	}
	defer func() {
		if cleanupErr := removeOwnedTree(staging); cleanupErr != nil {
			if err == nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("cannot remove mods staging %s: %v", staging,
						cleanupErr))
			}
		}
	}()

	options.report(Progress{Stage: StageFetching})
	current, err := options.Client.Mods(ctx)
	if err != nil {
		return Result{}, err
	}
	storedDigest, _ := readDigest(filepath.Join(serverDir, digestFile))
	if storedDigest == current.SHA256 &&
		regularDir(targetDF) && regularFile(targetProfile) {
		state, stateErr := Inspect(options.WorkDir, options.ServerID)
		if stateErr != nil {
			return Result{}, stateErr
		}
		return Result{State: state, Skipped: true}, nil
	}

	if err := os.MkdirAll(options.CacheDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create client cache directory: %w", err)
	}
	cachePath := filepath.Join(options.CacheDir, current.Name)
	cached, err := cacheArchive(cachePath, current)
	if err != nil {
		return Result{}, err
	}
	if !cached {
		options.report(Progress{
			Stage: StageDownloading,
			Total: current.Size,
		})
		if _, err := options.Client.DownloadMods(ctx, current, cachePath,
			func(update serverapi.DownloadProgress) {
				options.report(Progress{
					Stage:      StageDownloading,
					Downloaded: update.Downloaded,
					Total:      update.Total,
				})
			}); err != nil {
			return Result{}, err
		}
	}

	options.report(Progress{Stage: StageVerifying})
	if err := verifyArchiveFile(cachePath, current); err != nil {
		_ = removeRegularFile(cachePath)
		return Result{}, err
	}
	if _, err := archive.ValidateModsFile(cachePath); err != nil {
		return Result{}, fmt.Errorf("validate mods archive: %w", err)
	}

	options.report(Progress{Stage: StageExtracting})
	archiveDir := filepath.Join(staging, archiveSub)
	if err := archive.ExtractModsFile(cachePath, archiveDir); err != nil {
		return Result{}, fmt.Errorf("extract mods archive: %w", err)
	}
	requirements, err := readRequirements(archiveDir)
	if err != nil {
		return Result{}, err
	}
	if err := checkMorrowindFiles(options.Morrowind, requirements); err != nil {
		return Result{}, err
	}

	options.report(Progress{Stage: StageInstalling})
	newDF := filepath.Join(staging, "new Data Files")
	if err := os.Mkdir(newDF, 0o700); err != nil {
		return Result{}, fmt.Errorf("create staged Data Files: %w", err)
	}
	if err := copyPlugins(filepath.Join(archiveDir, "plugins"), newDF); err != nil {
		return Result{}, err
	}
	existingProfile, err := readOptionalRegular(targetProfile)
	if err != nil {
		return Result{}, err
	}
	profileData := BuildProfile(existingProfile, options.Morrowind.DataFiles,
		targetDF, requirements)
	newProfile := filepath.Join(staging, profileFile)
	if err := os.WriteFile(newProfile, profileData, 0o600); err != nil {
		return Result{}, fmt.Errorf("write staged openmw.cfg: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create profile data directory: %w", err)
	}
	warnings, err := swapInstallation(newDF, newProfile, targetDF,
		targetProfile, filepath.Join(serverDir, digestFile), current.SHA256)
	if err != nil {
		return Result{}, err
	}
	state, err := Inspect(options.WorkDir, options.ServerID)
	if err != nil {
		return Result{}, err
	}
	return Result{
		State:      state,
		Cached:     cached,
		Downloaded: !cached,
		Warnings:   warnings,
	}, nil
}

func (o Options) validate() error {
	if o.Client == nil {
		return errors.New("server API client is nil")
	}
	if strings.TrimSpace(o.WorkDir) == "" || !filepath.IsAbs(o.WorkDir) {
		return fmt.Errorf("client work directory %q is not absolute", o.WorkDir)
	}
	if strings.TrimSpace(o.CacheDir) == "" || !filepath.IsAbs(o.CacheDir) {
		return fmt.Errorf("client cache directory %q is not absolute", o.CacheDir)
	}
	if err := validateServerID(o.ServerID); err != nil {
		return err
	}
	if strings.TrimSpace(o.Morrowind.DataFiles) == "" ||
		!filepath.IsAbs(o.Morrowind.DataFiles) {
		return fmt.Errorf("Morrowind Data Files path %q is not absolute",
			o.Morrowind.DataFiles)
	}
	for _, name := range morrowind.RequiredFiles {
		path := o.Morrowind.Files[name]
		if path == "" {
			return fmt.Errorf("Morrowind installation is missing %s", name)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
	}
	return nil
}

func (o Options) report(progress Progress) {
	if o.Progress != nil {
		o.Progress(progress)
	}
}
