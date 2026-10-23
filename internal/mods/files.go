package mods

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

const maxProfileSize int64 = 1 << 20

func validateServerID(id string) error {
	if id == "" || id != strings.TrimSpace(id) {
		return errors.New("server ID is empty or has surrounding whitespace")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("server ID %q is not a directory name", id)
	}
	if strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("server ID %q contains a path separator", id)
	}
	if err := pathguard.Base(id); err != nil {
		return fmt.Errorf("server ID: %w", err)
	}
	return nil
}

func ensureDirectory(path string) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create directory %s: %w", path, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("inspect %s: %w", path, err)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("%s is a symlink", path)
	case !info.IsDir():
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func removeOwnedTree(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func regularDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir()
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 &&
		info.Mode().IsRegular()
}

func cacheArchive(path string, current serverapi.Mods) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect mods cache: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("mods cache path %s is not a regular file", path)
	}
	if err := verifyArchiveFile(path, current); err != nil {
		if removeErr := removeRegularFile(path); removeErr != nil {
			return false, errors.Join(
				fmt.Errorf("discard invalid mods cache: %w", err), removeErr)
		}
		return false, nil
	}
	return true, nil
}

func verifyArchiveFile(path string, current serverapi.Mods) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect mods archive: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("mods archive %s is not a regular file", path)
	}
	if info.Size() != current.Size {
		return fmt.Errorf("mods archive is %d bytes, expected %d",
			info.Size(), current.Size)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, current.SHA256) {
		return fmt.Errorf("mods archive sha256 is %s, expected %s",
			sum, current.SHA256)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open mods archive: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash mods archive: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func removeRegularFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func readDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > 128 {
		return "", fmt.Errorf("%s is too large", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.ToLower(strings.TrimSpace(string(data)))
	if len(value) != 64 {
		return "", fmt.Errorf("%s has an invalid digest", path)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("%s has an invalid digest", path)
	}
	return value, nil
}

func readOptionalRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > maxProfileSize {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, maxProfileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func readRequirements(archiveDir string) ([]archive.RequiredFile, error) {
	path := filepath.Join(archiveDir, "plugins", metadataName)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > archive.MaxRequiredDataFilesSize {
		return nil, fmt.Errorf("%s exceeds %d bytes", path,
			archive.MaxRequiredDataFilesSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return archive.ParseRequiredDataFiles(data)
}

func checkMorrowindFiles(installation morrowind.Installation,
	required []archive.RequiredFile,
) error {
	present := make(map[string]string, len(morrowind.RequiredFiles))
	for _, name := range morrowind.RequiredFiles {
		present[strings.ToLower(name)] = name
	}
	found := make(map[string]bool, len(morrowind.RequiredFiles))
	for _, file := range required {
		if !isBaseESM(file.Name) {
			continue
		}
		lower := strings.ToLower(file.Name)
		found[lower] = true
		canonical, ok := present[lower]
		if !ok {
			return fmt.Errorf("requiredDataFiles.json contains unknown base ESM %q",
				file.Name)
		}
		path := installation.Files[canonical]
		if path == "" {
			return fmt.Errorf("Morrowind installation is missing %s", canonical)
		}
		if len(file.CRCs) == 0 {
			continue
		}
		actual, err := fileCRC32(path)
		if err != nil {
			return err
		}
		if !containsCRC(file.CRCs, actual) {
			return fmt.Errorf(
				"%s CRC mismatch: required %s, local 0x%08X",
				canonical, formatCRCs(file.CRCs), actual)
		}
	}
	for _, name := range morrowind.RequiredFiles {
		if !found[strings.ToLower(name)] {
			return fmt.Errorf("requiredDataFiles.json is missing base ESM %s", name)
		}
	}
	return nil
}

func fileCRC32(path string) (uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open Morrowind ESM: %w", err)
	}
	defer file.Close()
	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, fmt.Errorf("hash Morrowind ESM: %w", err)
	}
	return hash.Sum32(), nil
}

func containsCRC(values []uint32, actual uint32) bool {
	for _, value := range values {
		if value == actual {
			return true
		}
	}
	return false
}

func formatCRCs(values []uint32) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("0x%08X", value)
	}
	return strings.Join(parts, ", ")
}

func copyPlugins(source, destination string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read extracted plugins: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == metadataName {
			continue
		}
		if err := pathguard.Base(name); err != nil {
			return fmt.Errorf("plugin %q: %w", name, err)
		}
		if entry.IsDir() {
			return fmt.Errorf("plugin %q is a directory", name)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect plugin %q: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("plugin %q is not a regular file", name)
		}
		if err := copyRegular(filepath.Join(source, name),
			filepath.Join(destination, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyRegular(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy %s: %w", source, err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("sync %s: %w", destination, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destination, err)
	}
	return nil
}

func countPlugins(dataFiles string) (int, error) {
	entries, err := os.ReadDir(dataFiles)
	if err != nil {
		return 0, fmt.Errorf("read Data Files: %w", err)
	}
	count := 0
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return 0, fmt.Errorf("inspect Data Files entry: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return 0, fmt.Errorf("Data Files contains a symlink")
		}
		if info.Mode().IsRegular() {
			count++
		}
	}
	return count, nil
}
