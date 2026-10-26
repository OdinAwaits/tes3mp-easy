package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUninstallEmpty reports an uninstall that selects nothing. The UI checks
// the scope first; the error keeps the app API honest for other callers.
var ErrUninstallEmpty = errors.New("no uninstall items selected")

// UninstallScope selects the client-owned directories an uninstall removes.
// The Morrowind installation is never part of any scope.
type UninstallScope struct {
	// GameFiles removes the work directory: TES3MP, servers, mods, fonts and
	// the localization overlay.
	GameFiles bool
	// Cache removes downloaded archives.
	Cache bool
	// Settings removes config.json with the server profiles.
	Settings bool
}

// Empty reports whether the scope selects nothing.
func (u UninstallScope) Empty() bool {
	return !u.GameFiles && !u.Cache && !u.Settings
}

// UninstallResult reports what an uninstall actually removed.
type UninstallResult struct {
	// Removed lists the deleted directories in removal order.
	Removed []string
	// ConfigRemoved tells the shell that the running client has no
	// configuration left and should restart.
	ConfigRemoved bool
}

// Uninstall removes the selected client-owned directories. Only the work,
// cache and config directories are touched: the Morrowind installation, its
// Data Files and the game's own settings stay in place, and a directory that
// contains the Morrowind path, the user home or the config directory is
// refused instead of deleted.
func (s *Service) Uninstall(scope UninstallScope) (UninstallResult, error) {
	if s == nil {
		return UninstallResult{}, errors.New("app service is nil")
	}
	if scope.Empty() {
		return UninstallResult{}, ErrUninstallEmpty
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return UninstallResult{}, err
	}
	cacheDir, err := s.absoluteCacheDir()
	if err != nil {
		return UninstallResult{}, err
	}
	configDir := strings.TrimSpace(s.paths.ConfigDir)
	if configDir == "" {
		return UninstallResult{}, errors.New("config directory is empty")
	}

	result := UninstallResult{}
	targets := []uninstallTarget{
		{enabled: scope.GameFiles, path: workDir},
		{enabled: scope.Cache, path: cacheDir},
		{enabled: scope.Settings, path: configDir, settings: true},
	}

	s.closeLogForUninstall(targets)

	guards := []string{s.homeDir}
	morrowind := strings.TrimSpace(s.Config().MorrowindPath)
	for _, target := range targets {
		if !target.enabled {
			continue
		}
		if morrowind != "" && (pathWithin(target.path, morrowind) ||
			pathWithin(morrowind, target.path)) {
			err := fmt.Errorf(
				"refusing to remove %s: it overlaps the Morrowind folder %s",
				filepath.Clean(target.path), morrowind)
			s.LogError("uninstall", err)
			return result, err
		}
		targetGuards := append([]string(nil), guards...)
		if !samePath(target.path, configDir) {
			targetGuards = append(targetGuards, configDir)
		}
		removed, err := removeClientDir(target.path, targetGuards)
		if err != nil {
			s.LogError("uninstall", err)
			return result, err
		}
		if !removed {
			continue
		}
		result.Removed = append(result.Removed, filepath.Clean(target.path))
		if target.settings {
			result.ConfigRemoved = true
		}
	}
	s.LogInfof("uninstall removed %d directories (config=%t)",
		len(result.Removed), result.ConfigRemoved)
	return result, nil
}

// uninstallTarget is one client-owned directory selected for removal.
type uninstallTarget struct {
	enabled  bool
	path     string
	settings bool
}

// closeLogForUninstall closes the client log when its directory is about to be
// deleted; Windows refuses to remove an open file. Later log calls become
// no-ops for the rest of the session.
func (s *Service) closeLogForUninstall(targets []uninstallTarget) {
	if s.logFile == nil {
		return
	}
	logsDir := filepath.Clean(s.paths.LogsDir)
	for _, target := range targets {
		if target.enabled && pathWithin(logsDir, target.path) {
			_ = s.logFile.Close()
			return
		}
	}
}

// removeClientDir deletes one client-owned directory. It refuses filesystem
// roots, symlinks and any path that contains a guarded path such as the user
// home or the Morrowind installation.
func removeClientDir(path string, guards []string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, errors.New("uninstall path is empty")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return false, fmt.Errorf("refusing to remove %q: path is not absolute",
			path)
	}
	if filepath.Dir(clean) == clean {
		return false, fmt.Errorf("refusing to remove %q: filesystem root",
			path)
	}
	for _, guard := range guards {
		guard = strings.TrimSpace(guard)
		if guard == "" || !filepath.IsAbs(guard) {
			continue
		}
		if pathWithin(guard, clean) {
			return false, fmt.Errorf(
				"refusing to remove %s: it contains %s", clean, guard)
		}
	}
	info, err := os.Lstat(clean)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", clean, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("refusing to remove %s: not a directory",
			clean)
	}
	if err := os.RemoveAll(clean); err != nil {
		return false, fmt.Errorf("remove %s: %w", clean, err)
	}
	return true, nil
}

// pathWithin reports whether child is parent itself or lives below it.
func pathWithin(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if samePath(child, parent) {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
