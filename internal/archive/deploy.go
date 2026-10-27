package archive

import (
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

// ValidateModsDir validates the live mods tree that a deploy produces:
// plugins/ with requiredDataFiles.json and at least one plugin, plus scripts/
// with zero or more .lua files. Nested directories, symlinks and ambiguous
// case-insensitive names are rejected.
func ValidateModsDir(modsDir string) (ModsInfo, error) {
	if modsDir == "" {
		return ModsInfo{}, errors.New("archive: mods directory is required")
	}
	if err := requireModsDir(modsDir); err != nil {
		return ModsInfo{}, err
	}

	pluginsDir := filepath.Join(modsDir, "plugins")
	scriptsDir := filepath.Join(modsDir, "scripts")
	if err := requireModsDir(pluginsDir); err != nil {
		return ModsInfo{}, err
	}
	if err := requireModsDir(scriptsDir); err != nil {
		return ModsInfo{}, err
	}

	metadataPath := filepath.Join(pluginsDir, requiredDataFilesName)
	metadataInfo, err := lstatRegular(metadataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModsInfo{}, archiveInvalidf(
				"%s: required metadata file is missing", metadataPath)
		}
		return ModsInfo{}, err
	}
	if metadataInfo.Size() > MaxRequiredDataFilesSize {
		return ModsInfo{}, fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, metadataPath, MaxRequiredDataFilesSize)
	}
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		return ModsInfo{}, err
	}
	if int64(len(metadata)) > MaxRequiredDataFilesSize {
		return ModsInfo{}, fmt.Errorf("%w: %s exceeds %d bytes",
			ErrLimit, metadataPath, MaxRequiredDataFilesSize)
	}
	requirements, err := ParseRequiredDataFiles(metadata)
	if err != nil {
		return ModsInfo{}, fmt.Errorf("%s: %w", metadataPath, err)
	}

	pluginNames := make([]string, 0)
	pluginCRCs := make(map[string]uint32)
	var uncompressed int64
	buf := make([]byte, 32*1024)

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return ModsInfo{}, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if err := pathguard.Base(name); err != nil {
			return ModsInfo{}, err
		}
		if name == requiredDataFilesName {
			continue
		}
		if err := validatePluginExtension(name); err != nil {
			return ModsInfo{}, fmt.Errorf("%s: %w", filepath.Join(pluginsDir, name), err)
		}
		if isBaseESM(name) {
			return ModsInfo{}, archiveInvalidf(
				"%s: base ESM must not be stored with mods", name)
		}

		filePath := filepath.Join(pluginsDir, name)
		info, err := lstatRegular(filePath)
		if err != nil {
			return ModsInfo{}, err
		}
		uncompressed += info.Size()
		if uncompressed > MaxUncompressedSize {
			return ModsInfo{}, fmt.Errorf(
				"%w: live mods exceed %d uncompressed bytes",
				ErrLimit, MaxUncompressedSize)
		}

		sum, err := fileCRC32(filePath, buf)
		if err != nil {
			return ModsInfo{}, err
		}
		pluginNames = append(pluginNames, name)
		pluginCRCs[name] = sum
	}

	scriptEntries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return ModsInfo{}, err
	}
	scriptNames := make([]string, 0, len(scriptEntries))
	for _, entry := range scriptEntries {
		name := entry.Name()
		if err := pathguard.Base(name); err != nil {
			return ModsInfo{}, err
		}
		if err := validateScriptFilename(name); err != nil {
			return ModsInfo{}, fmt.Errorf(
				"%s: %w", filepath.Join(scriptsDir, name), err)
		}
		info, err := lstatRegular(filepath.Join(scriptsDir, name))
		if err != nil {
			return ModsInfo{}, err
		}
		uncompressed += info.Size()
		if uncompressed > MaxUncompressedSize {
			return ModsInfo{}, fmt.Errorf(
				"%w: live mods exceed %d uncompressed bytes",
				ErrLimit, MaxUncompressedSize)
		}
		scriptNames = append(scriptNames, name)
	}
	sort.Strings(scriptNames)

	if err := validateModsRequirements(requirements, pluginNames, pluginCRCs); err != nil {
		return ModsInfo{}, err
	}
	return ModsInfo{Plugins: len(pluginNames), Scripts: len(scriptNames)}, nil
}

// ExtractModsFile extracts a previously validated mods archive into dest. dest
// is created if missing and must be an empty directory tree. The archive reader
// has already enforced the entry, count and uncompressed-size limits and has
// rejected absolute or traversal paths.
func ExtractModsFile(name, dest string) error {
	reader, err := Open(name)
	if err != nil {
		return err
	}
	defer reader.Close()
	return ExtractMods(reader, dest)
}

// ExtractMods extracts a mods archive from r into dest.
func ExtractMods(r *Reader, dest string) error {
	if r == nil {
		return errors.New("archive: nil mods reader")
	}
	if dest == "" {
		return errors.New("archive: mods destination is required")
	}
	if err := ensureExtractDir(dest); err != nil {
		return err
	}
	for _, dir := range []string{"plugins", "scripts"} {
		if err := ensureExtractDir(filepath.Join(dest, dir)); err != nil {
			return err
		}
	}

	buf := make([]byte, 32*1024)
	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		switch entry.Type {
		case TypeDir:
			if entry.Name != "plugins" && entry.Name != "scripts" {
				return archiveInvalidf(
					"%q: unexpected directory in mods archive", entry.Name)
			}
			continue
		case TypeFile:
			// Continue below.
		default:
			return archiveInvalidf(
				"%q: unsupported entry type", entry.Name)
		}

		top, rest, hasDir := strings.Cut(entry.Name, "/")
		if !hasDir || (top != "plugins" && top != "scripts") ||
			rest == "" || strings.Contains(rest, "/") {
			return archiveInvalidf(
				"%q: unexpected file in mods archive", entry.Name)
		}
		switch top {
		case "plugins":
			if rest != requiredDataFilesName {
				if err := validatePluginExtension(rest); err != nil {
					return fmt.Errorf("plugins/%s: %w", rest, err)
				}
				if isBaseESM(rest) {
					return archiveInvalidf(
						"plugins/%s: base ESM must not be archived", rest)
				}
			}
		case "scripts":
			if err := validateScriptFilename(rest); err != nil {
				return fmt.Errorf("scripts/%s: %w", rest, err)
			}
		}

		target := filepath.Join(dest, top, rest)
		if err := writeExtractedFile(target, r, buf); err != nil {
			return err
		}
	}
}

func requireModsDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return archiveInvalidf("%s: mods directory is missing", path)
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0:
		return archiveInvalidf("%s: symlink is not allowed", path)
	case !info.IsDir():
		return archiveInvalidf("%s: not a directory", path)
	}
	return nil
}

func lstatRegular(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return nil, archiveInvalidf("%s: symlink is not allowed", path)
	case info.IsDir():
		return nil, archiveInvalidf("%s: nested directories are not allowed", path)
	case !info.Mode().IsRegular():
		return nil, archiveInvalidf("%s: not a regular file", path)
	}
	return info, nil
}

func fileCRC32(path string, buf []byte) (uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	sum := crc32.NewIEEE()
	if _, err := io.CopyBuffer(sum, file, buf); err != nil {
		return 0, err
	}
	return sum.Sum32(), nil
}

func ensureExtractDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(path, 0o750); err != nil {
			return err
		}
		return nil
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0:
		return archiveInvalidf("%s: symlink is not allowed", path)
	case !info.IsDir():
		return archiveInvalidf("%s: not a directory", path)
	}
	return nil
}

func writeExtractedFile(path string, r io.Reader, buf []byte) error {
	return writeExtractedFileMode(path, r, buf, 0o640)
}

func writeExtractedFileMode(path string, r io.Reader, buf []byte,
	mode os.FileMode,
) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.CopyBuffer(file, r, buf); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
