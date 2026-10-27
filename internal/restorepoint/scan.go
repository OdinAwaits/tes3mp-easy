package restorepoint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

const requiredDataFilesName = "requiredDataFiles.json"

var stateWorldDirs = []string{"cell", "world", "map", "recordstore", "custom"}

var configFiles = []string{
	"config.lua",
	"tes3mp-server-default.cfg",
	"banlist.json",
	"customScripts.lua",
}

type layerFile struct {
	File
	source string
}

type snapshotPlan struct {
	state   []layerFile
	mods    []layerFile
	configs []layerFile
	players int
}

type checkpointPlan struct {
	state   []layerFile
	players int
}

func scanState(root string) ([]layerFile, int, error) {
	players, err := scanTopLevelFiles(filepath.Join(root, "players"), "players/")
	if err != nil {
		return nil, 0, err
	}
	world, err := scanWorld(filepath.Join(root, "world"))
	if err != nil {
		return nil, 0, err
	}
	requiredPath := filepath.Join(root, "mods", "plugins", requiredDataFilesName)
	required, err := scanMetadataFile(requiredPath, requiredDataFilesName)
	if err != nil {
		return nil, 0, err
	}

	files := make([]layerFile, 0, len(players)+len(world)+1)
	files = append(files, players...)
	files = append(files, world...)
	files = append(files, required)
	return files, len(players), nil
}

func scanWorld(worldDir string) ([]layerFile, error) {
	if err := requireDir(worldDir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(worldDir)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(stateWorldDirs))
	for _, name := range stateWorldDirs {
		known[name] = true
	}
	for _, entry := range entries {
		if !known[entry.Name()] {
			return nil, invalidf("%s: unexpected world directory", entry.Name())
		}
	}
	for _, name := range stateWorldDirs {
		if err := requireDir(filepath.Join(worldDir, name)); err != nil {
			return nil, err
		}
	}

	var files []layerFile
	for _, name := range stateWorldDirs {
		layer, err := scanTopLevelFiles(filepath.Join(worldDir, name), "world/"+name+"/")
		if err != nil {
			return nil, err
		}
		files = append(files, layer...)
	}
	return files, nil
}

func scanTopLevelFiles(dir, prefix string) ([]layerFile, error) {
	if err := requireDir(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	files := make([]layerFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if err := pathguard.Base(name); err != nil {
			return nil, err
		}
		source := filepath.Join(dir, name)
		info, err := os.Lstat(source)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, invalidf("%s: symlink is not allowed", source)
		}
		if info.IsDir() {
			return nil, invalidf("%s: nested directories are not allowed", source)
		}
		if !info.Mode().IsRegular() {
			return nil, invalidf("%s: not a regular file", source)
		}
		file, err := hashFile(prefix+name, source, info.Size())
		if err != nil {
			return nil, err
		}
		files = append(files, layerFile{File: file, source: source})
	}
	return files, nil
}

func scanMetadataFile(source, archivePath string) (layerFile, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return layerFile{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return layerFile{}, invalidf("%s: not a regular file", source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return layerFile{}, err
	}
	if _, err := archive.ParseRequiredDataFiles(data); err != nil {
		return layerFile{}, fmt.Errorf("%s: %w", source, err)
	}
	file, err := hashFile(archivePath, source, info.Size())
	if err != nil {
		return layerFile{}, err
	}
	return layerFile{File: file, source: source}, nil
}

func scanMods(root string) ([]layerFile, error) {
	modsDir := filepath.Join(root, "mods")
	if _, err := archive.ValidateModsDir(modsDir); err != nil {
		return nil, err
	}
	var files []layerFile
	for _, dir := range []string{"plugins", "scripts"} {
		layer, err := scanTopLevelFiles(filepath.Join(modsDir, dir), dir+"/")
		if err != nil {
			return nil, err
		}
		files = append(files, layer...)
	}
	return files, nil
}

func scanConfigs(root string) ([]layerFile, error) {
	files := make([]layerFile, 0, len(configFiles))
	for _, name := range configFiles {
		source := filepath.Join(root, name)
		info, err := os.Lstat(source)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, invalidf("%s: not a regular file", source)
		}
		file, err := hashFile(name, source, info.Size())
		if err != nil {
			return nil, err
		}
		files = append(files, layerFile{File: file, source: source})
	}
	return files, nil
}

func hashFile(archivePath, source string, size int64) (File, error) {
	file, err := os.Open(source)
	if err != nil {
		return File{}, err
	}
	defer file.Close()

	sum := sha256.New()
	written, err := io.CopyBuffer(sum, file, make([]byte, 32*1024))
	if err != nil {
		return File{}, err
	}
	if written != size {
		return File{}, invalidf("%s: size changed while scanning", source)
	}
	return File{
		Path:   archivePath,
		Size:   written,
		SHA256: hex.EncodeToString(sum.Sum(nil)),
	}, nil
}

func requireDir(dir string) error {
	info, err := os.Lstat(dir)
	switch {
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0:
		return invalidf("%s: symlink is not allowed", dir)
	case !info.IsDir():
		return invalidf("%s: not a directory", dir)
	default:
		return nil
	}
}

// LiveModsDigest hashes the current root/mods tree using the same layer digest
// as a snapshot manifest. checkpoint create compares it with current.txt.
func LiveModsDigest(root string) (string, error) {
	mods, err := scanMods(root)
	if err != nil {
		return "", err
	}
	return layerDigest("mods", mods), nil
}

func layerDigest(layer string, files []layerFile) string {
	plain := make([]File, len(files))
	for i := range files {
		plain[i] = files[i].File
	}
	return Digest(layer, plain)
}

func totalSize(files []layerFile) int64 {
	var total int64
	for _, file := range files {
		total += file.Size
	}
	return total
}

func validateLayerLimits(kind Kind, layers ...[]layerFile) error {
	var total int64
	var entries int
	for _, layer := range layers {
		total += totalSize(layer)
		entries += len(layer)
	}
	limits := Limits(kind)
	if total > limits.UncompressedSize {
		return fmt.Errorf("%w: uncompressed size %d exceeds %d",
			ErrLimit, total, limits.UncompressedSize)
	}
	if entries+10 > limits.Entries {
		return fmt.Errorf("%w: %d entries exceed %d",
			ErrLimit, entries+10, limits.Entries)
	}
	return nil
}
