package locale

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

const (
	appliedDataFiles = "data/Data Files"
	backupDirName    = ".locale-backup"
)

type appliedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	// Backup is a path below servers/<id>/.locale-backup for a file that the
	// profile already had before the localization was applied. The original is
	// restored when the overlay is removed.
	Backup string `json:"backup,omitempty"`
}

type appliedManifest struct {
	DataFiles string        `json:"data_files"`
	Files     []appliedFile `json:"files"`
}

// Applied describes one overlay application.
type Applied struct {
	DataFiles string
	Files     int
	Warnings  []string
}

type treeEntry struct {
	path string
	dir  bool
}

type overlayFile struct {
	path   string
	source string
}

// Apply copies the installed localization layers into servers/<id>/data/Data
// Files. It first removes a previous application recorded in the manifest and
// then records every copied file for a later RemoveOverlay.
func Apply(workDir, serverID string) (Applied, error) {
	serverDir, err := validateProfile(workDir, serverID)
	if err != nil {
		return Applied{}, err
	}
	state, err := Inspect(workDir, serverID)
	if err != nil {
		return Applied{}, err
	}
	if !state.Installed {
		return Applied{}, errors.New("Russian localization is not installed")
	}
	target := filepath.Join(serverDir, filepath.FromSlash(appliedDataFiles))
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return Applied{}, errors.New("profile Data Files is not installed")
	}
	if err != nil {
		return Applied{}, fmt.Errorf("inspect profile Data Files: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Applied{}, errors.New("profile Data Files is not a directory")
	}
	previousWarnings, err := RemoveOverlay(workDir, serverID)
	if err != nil {
		return Applied{}, err
	}

	targetEntries, err := targetTree(target)
	if err != nil {
		return Applied{}, err
	}
	files, err := overlayFiles(workDir, serverID, targetEntries)
	if err != nil {
		return Applied{}, err
	}
	if len(files) == 0 {
		return Applied{}, errors.New("localization overlay is empty")
	}
	manifest := appliedManifest{DataFiles: appliedDataFiles}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	for _, file := range files {
		destination := filepath.Join(target, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
			return Applied{}, fmt.Errorf("create overlay directory: %w", err)
		}
		backup, err := backupExisting(serverDir, file.path, destination)
		if err != nil {
			return Applied{}, err
		}
		digest, err := writeOverlayFile(file.source, destination)
		if err != nil {
			return Applied{}, fmt.Errorf("overlay %s: %w", file.path, err)
		}
		manifest.Files = append(manifest.Files,
			appliedFile{Path: file.path, SHA256: digest, Backup: backup})
	}
	if err := writeAppliedManifest(serverDir, manifest); err != nil {
		return Applied{}, err
	}
	return Applied{
		DataFiles: target,
		Files:     len(manifest.Files),
		Warnings:  previousWarnings,
	}, nil
}

func targetTree(root string) (map[string]treeEntry, error) {
	entries := make(map[string]treeEntry)
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("profile Data Files contains a symlink: %s", current)
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		key := strings.ToLower(relative)
		if previous, ok := entries[key]; ok {
			return fmt.Errorf(
				"profile Data Files has case-conflicting paths %q and %q",
				previous.path, relative)
		}
		entries[key] = treeEntry{path: relative, dir: info.IsDir()}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect profile Data Files: %w", err)
	}
	return entries, nil
}

// overlayFiles merges layers in OverlayDirs order. Directory and file names are
// canonicalized case-insensitively against the existing Data Files and earlier
// layers, matching the legacy installer's duplicate merge. A later layer wins
// for the same path, so voice files win over the main archive.
func overlayFiles(workDir, serverID string,
	target map[string]treeEntry,
) ([]overlayFile, error) {
	dirs := make(map[string]string)
	for key, entry := range target {
		if entry.dir {
			dirs[key] = entry.path
		}
	}
	byPath := make(map[string]overlayFile)
	for _, root := range OverlayDirs(workDir, serverID) {
		err := filepath.WalkDir(root, func(current string, entry os.DirEntry,
			walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}
			if current == root {
				return nil
			}
			info, err := os.Lstat(current)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("localization contains a symlink: %s", current)
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			parts := strings.Split(relative, "/")
			actualDir := ""
			for index := 0; index < len(parts)-1; index++ {
				key := strings.ToLower(strings.Join(parts[:index+1], "/"))
				if existing, ok := target[key]; ok && !existing.dir {
					return fmt.Errorf("localization path %q is below file %q",
						relative, existing.path)
				}
				if canonical, ok := dirs[key]; ok {
					actualDir = canonical
					continue
				}
				actualDir = path.Join(actualDir, parts[index])
				dirs[key] = actualDir
			}
			fullKey := strings.ToLower(relative)
			actual := path.Join(actualDir, parts[len(parts)-1])
			if existing, ok := target[fullKey]; ok {
				if existing.dir {
					return fmt.Errorf("localization file %q collides with a directory",
						relative)
				}
				actual = existing.path
			}
			if previous, ok := byPath[fullKey]; ok {
				actual = previous.path
			}
			byPath[fullKey] = overlayFile{path: actual, source: current}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("inspect localization overlay: %w", err)
		}
	}
	files := make([]overlayFile, 0, len(byPath))
	for _, file := range byPath {
		files = append(files, file)
	}
	return files, nil
}

// RemoveOverlay removes the files recorded by the last Apply call. Files that
// were changed after Apply are left in place and reported as warnings.
func RemoveOverlay(workDir, serverID string) ([]string, error) {
	serverDir, err := validateProfile(workDir, serverID)
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(serverDir, AppliedName)
	data, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read applied localization manifest: %w", err)
	}
	var manifest appliedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse applied localization manifest: %w", err)
	}
	if filepath.ToSlash(filepath.Clean(manifest.DataFiles)) != appliedDataFiles {
		return nil, errors.New("applied localization manifest has an invalid target")
	}
	target := filepath.Join(serverDir, filepath.FromSlash(appliedDataFiles))
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, removeManifest(manifestPath)
	}
	if err != nil {
		return nil, fmt.Errorf("inspect profile Data Files: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("profile Data Files is not a directory")
	}

	var warnings []string
	for _, file := range manifest.Files {
		relative, err := pathFromSlash(file.Path)
		if err != nil {
			return nil, fmt.Errorf("applied localization path: %w", err)
		}
		destination := filepath.Join(target, relative)
		if !within(target, destination) {
			return nil, fmt.Errorf("applied localization path %q escapes Data Files",
				file.Path)
		}
		backup, err := appliedBackup(serverDir, file.Backup)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(destination)
		if errors.Is(err, os.ErrNotExist) {
			if backup != "" {
				if err := restoreBackup(backup, destination); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect applied localization %q: %w",
				file.Path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			warnings = append(warnings, fmt.Sprintf(
				"localization file %s was changed; keeping it", file.Path))
			continue
		}
		digest, _, err := digestFile(destination)
		if err != nil {
			return nil, fmt.Errorf("verify applied localization %q: %w",
				file.Path, err)
		}
		if !strings.EqualFold(digest, file.SHA256) {
			warnings = append(warnings, fmt.Sprintf(
				"localization file %s was modified; keeping it", file.Path))
			continue
		}
		if err := os.Remove(destination); err != nil {
			return nil, fmt.Errorf("remove applied localization %q: %w",
				file.Path, err)
		}
		if backup != "" {
			if err := restoreBackup(backup, destination); err != nil {
				return nil, err
			}
		}
	}
	removeEmptyDirs(target)
	backupRoot := filepath.Join(serverDir, backupDirName)
	removeEmptyDirs(backupRoot)
	_ = os.Remove(backupRoot)
	if err := removeManifest(manifestPath); err != nil {
		return nil, err
	}
	return warnings, nil
}

func appliedBackup(serverDir, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	relative, err := pathFromSlash(name)
	if err != nil {
		return "", fmt.Errorf("applied localization backup: %w", err)
	}
	root := filepath.Join(serverDir, backupDirName)
	backup := filepath.Join(root, relative)
	if !within(root, backup) {
		return "", fmt.Errorf("applied localization backup %q escapes its directory",
			name)
	}
	info, err := os.Lstat(backup)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect applied localization backup %q: %w",
			name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("applied localization backup %q is not a regular file",
			name)
	}
	return backup, nil
}

func restoreBackup(backup, destination string) error {
	if _, err := writeOverlayFile(backup, destination); err != nil {
		return fmt.Errorf("restore profile file %s: %w", destination, err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("remove localization backup %s: %w", backup, err)
	}
	return nil
}

func backupExisting(serverDir, path, destination string) (string, error) {
	info, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect existing overlay file %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("profile file %q is not a regular file", path)
	}
	relative, err := pathFromSlash(path)
	if err != nil {
		return "", fmt.Errorf("backup overlay path: %w", err)
	}
	backup := filepath.Join(serverDir, backupDirName, relative)
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		return "", fmt.Errorf("create localization backup directory: %w", err)
	}
	if _, err := writeOverlayFile(destination, backup); err != nil {
		return "", fmt.Errorf("backup profile file %q: %w", path, err)
	}
	return filepath.ToSlash(relative), nil
}

func writeOverlayFile(source, target string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("source is not a regular file")
	}
	writer, err := atomicfile.New(target, 0o640)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(writer, hash), in); err != nil {
		return "", errors.Join(err, writer.Abort())
	}
	if err := writer.Commit(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func digestFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func writeAppliedManifest(serverDir string, manifest appliedManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode applied localization manifest: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(serverDir, AppliedName)
	if err := atomicfile.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write applied localization manifest: %w", err)
	}
	return nil
}

func removeManifest(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove applied localization manifest: %w", err)
	}
	if err := atomicfile.SyncDir(filepath.Dir(path)); err != nil {
		return err
	}
	return nil
}

func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(current string, entry os.DirEntry,
		walkErr error,
	) error {
		if walkErr == nil && entry.IsDir() && current != root {
			dirs = append(dirs, current)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		_ = os.Remove(dir)
	}
}

func within(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
