package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

// StateSource names the live files copied into a state archive. The metadata
// files are read and validated before the first tar entry is written, so an
// invalid current.txt never produces a partially valid archive.
//
// ConfigsDir is required. The state config file must exist and be a regular
// file; server-only files such as tes3mp-server-default.cfg and banlist.json
// are deliberately excluded.
type StateSource struct {
	PlayersDir        string
	WorldDir          string
	ConfigsDir        string
	RequiredDataFiles string
	CurrentFile       string
}

// stateConfigFiles is the fixed configuration set stored in a state archive.
// customScripts.lua is generated from mods/scripts and deliberately excluded.
// Server-only files (tes3mp-server-default.cfg with the server password and
// banlist.json) are not state data and never enter the public archive.
var stateConfigFiles = []string{
	"config.lua",
}

var stateConfigFileSet = map[string]struct{}{
	"config.lua": {},
}

// StateStats describes a written state archive. Uncompressed follows the tar
// stream size, including headers, padding and the two end blocks.
type StateStats struct {
	Entries      int
	Files        int
	Uncompressed int64
}

type stateFileSource struct {
	name string
	path string
}

type stateDirSource struct {
	name  string
	info  fs.FileInfo
	files []stateFileSource
}

// WriteState writes a state tar.gz archive from the live players/, world/ and
// configs/ trees. It validates the nested layout from docs/v2/03-contracts.md
// §6.2 and writes entries in a deterministic order:
//
//	current.txt, configs/, configs/config.lua, requiredDataFiles.json,
//	players/, world/, then the five world subdirectories.
//
// current.txt is first so retention can read the mods reference without
// scanning the whole archive. Files inside players/ and each world directory
// are sorted by name. Entries never follow symlinks and nested directories are
// rejected. The caller owns the destination file and its fsync/rename.
func WriteState(w io.Writer, src StateSource) (StateStats, error) {
	if w == nil {
		return StateStats{}, errors.New("archive: nil state destination")
	}
	if err := validateStateSource(src); err != nil {
		return StateStats{}, err
	}

	requiredData, requiredInfo, err := readStateMetadata(
		src.RequiredDataFiles, MaxRequiredDataFilesSize)
	if err != nil {
		return StateStats{}, err
	}
	if _, err := ParseRequiredDataFiles(requiredData); err != nil {
		return StateStats{}, fmt.Errorf("%s: %w", src.RequiredDataFiles, err)
	}

	currentData, currentInfo, err := readStateMetadata(
		src.CurrentFile, MaxCurrentFileSize)
	if err != nil {
		return StateStats{}, err
	}
	current, err := ParseCurrent(currentData)
	if err != nil {
		return StateStats{}, fmt.Errorf("%s: %w", src.CurrentFile, err)
	}
	currentData = FormatCurrent(current)

	configs, err := collectStateConfigs(src.ConfigsDir)
	if err != nil {
		return StateStats{}, err
	}
	dirs, err := collectStateDirectories(src)
	if err != nil {
		return StateStats{}, err
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	stats := StateStats{}

	fail := func(writeErr error) (StateStats, error) {
		_ = tw.Close()
		_ = gz.Close()
		return StateStats{}, writeErr
	}

	// current.txt is intentionally the first entry.
	if err := addStateEntry(&stats, int64(len(currentData))); err != nil {
		return fail(err)
	}
	if err := writeStateBytes(tw, currentFileName, currentData, currentInfo); err != nil {
		return fail(fmt.Errorf("write %s: %w", currentFileName, err))
	}
	stats.Files++

	if err := addStateEntry(&stats, 0); err != nil {
		return fail(err)
	}
	if err := writeStateDir(tw, "configs/", configs.info); err != nil {
		return fail(fmt.Errorf("write configs/: %w", err))
	}
	for _, file := range configs.files {
		if err := writeStateFile(tw, file.name, file.path, &stats); err != nil {
			return fail(err)
		}
	}

	if err := addStateEntry(&stats, int64(len(requiredData))); err != nil {
		return fail(err)
	}
	if err := writeStateBytes(tw, requiredDataFilesName, requiredData,
		requiredInfo); err != nil {
		return fail(fmt.Errorf("write %s: %w", requiredDataFilesName, err))
	}
	stats.Files++

	for _, dir := range dirs {
		if err := addStateEntry(&stats, 0); err != nil {
			return fail(err)
		}
		if err := writeStateDir(tw, dir.name, dir.info); err != nil {
			return fail(fmt.Errorf("write %s: %w", dir.name, err))
		}
		for _, file := range dir.files {
			if err := writeStateFile(tw, file.name, file.path, &stats); err != nil {
				return fail(err)
			}
		}
	}

	// tar.Writer.Close writes the two zero blocks that terminate the stream.
	stats.Uncompressed += 2 * tarBlockSize
	if stats.Uncompressed > MaxUncompressedSize {
		return fail(fmt.Errorf("%w: uncompressed state exceeds %d bytes",
			ErrLimit, MaxUncompressedSize))
	}
	if err := tw.Close(); err != nil {
		return fail(fmt.Errorf("close state tar: %w", err))
	}
	if err := gz.Close(); err != nil {
		return StateStats{}, fmt.Errorf("close state gzip: %w", err)
	}
	return stats, nil
}

const tarBlockSize int64 = 512

func validateStateSource(src StateSource) error {
	switch {
	case src.PlayersDir == "":
		return errors.New("archive: players directory is required")
	case src.WorldDir == "":
		return errors.New("archive: world directory is required")
	case src.ConfigsDir == "":
		return errors.New("archive: configs directory is required")
	case src.RequiredDataFiles == "":
		return errors.New("archive: requiredDataFiles.json is required")
	case src.CurrentFile == "":
		return errors.New("archive: current.txt is required")
	default:
		return nil
	}
}

type stateConfigSource struct {
	info  fs.FileInfo
	files []stateFileSource
}

func collectStateConfigs(dir string) (*stateConfigSource, error) {
	info, err := readStateDir(dir)
	if err != nil {
		return nil, err
	}
	configs := &stateConfigSource{info: info}
	for _, name := range stateConfigFiles {
		path := filepath.Join(dir, name)
		fileInfo, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrInvalid, path, err)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: %s: symlink is not allowed", ErrInvalid, path)
		}
		if !fileInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: %s: not a regular file", ErrInvalid, path)
		}
		configs.files = append(configs.files, stateFileSource{
			name: "configs/" + name,
			path: path,
		})
	}
	return configs, nil
}

func readStateMetadata(path string, limit int64) ([]byte, fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%s: symlink is not allowed", path)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s: not a regular file", path)
	}
	if info.Size() > limit {
		return nil, nil, fmt.Errorf("%s: file exceeds %d bytes", path, limit)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) > limit {
		return nil, nil, fmt.Errorf("%s: file exceeds %d bytes", path, limit)
	}
	return data, info, nil
}

func collectStateDirectories(src StateSource) ([]stateDirSource, error) {
	playersInfo, err := readStateDir(src.PlayersDir)
	if err != nil {
		return nil, err
	}
	worldInfo, err := readStateDir(src.WorldDir)
	if err != nil {
		return nil, err
	}

	worldEntries, err := os.ReadDir(src.WorldDir)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(stateWorldDirs))
	for _, name := range stateWorldDirs {
		known[name] = true
	}
	for _, entry := range worldEntries {
		path := filepath.Join(src.WorldDir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("%s: only the five state world directories are allowed", path)
		}
		if !known[entry.Name()] {
			return nil, fmt.Errorf("%s: unexpected world directory", path)
		}
	}

	dirs := make([]stateDirSource, 0, 2+len(stateWorldDirs))
	playersFiles, err := readStateFiles(src.PlayersDir, "players/")
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, stateDirSource{
		name: "players/", info: playersInfo, files: playersFiles,
	})
	dirs = append(dirs, stateDirSource{
		name: "world/", info: worldInfo,
	})
	for _, name := range stateWorldDirs {
		path := filepath.Join(src.WorldDir, name)
		info, err := readStateDir(path)
		if err != nil {
			return nil, err
		}
		files, err := readStateFiles(path, "world/"+name+"/")
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, stateDirSource{
			name: "world/" + name + "/", info: info, files: files,
		})
	}
	return dirs, nil
}

func readStateDir(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s: symlink is not allowed", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", path)
	}
	return info, nil
}

func readStateFiles(dir, prefix string) ([]stateFileSource, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	files := make([]stateFileSource, 0, len(entries))
	for _, entry := range entries {
		if err := pathguard.Base(entry.Name()); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return nil, fmt.Errorf("%s: symlink is not allowed", path)
		case info.IsDir():
			return nil, fmt.Errorf("%s: nested directories are not allowed", path)
		case !info.Mode().IsRegular():
			return nil, fmt.Errorf("%s: not a regular file", path)
		}
		files = append(files, stateFileSource{
			name: prefix + entry.Name(),
			path: path,
		})
	}
	return files, nil
}

func writeStateDir(tw *tar.Writer, name string, info fs.FileInfo) error {
	return tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     int64(info.Mode().Perm()),
		ModTime:  info.ModTime().UTC(),
		Typeflag: tar.TypeDir,
	})
}

func writeStateFile(tw *tar.Writer, name, path string, stats *StateStats) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	current, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !current.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file", path)
	}
	if err := addStateEntry(stats, current.Size()); err != nil {
		return err
	}

	header := &tar.Header{
		Name:     name,
		Mode:     int64(current.Mode().Perm()),
		Size:     current.Size(),
		ModTime:  current.ModTime().UTC(),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write header %s: %w", name, err)
	}
	written, err := io.CopyN(tw, file, current.Size())
	if err != nil {
		return fmt.Errorf("copy %s: %w", path, err)
	}
	if written != current.Size() {
		return fmt.Errorf("copy %s: wrote %d of %d bytes",
			path, written, current.Size())
	}
	stats.Files++
	return nil
}

func writeStateBytes(tw *tar.Writer, name string, data []byte, info fs.FileInfo) error {
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     int64(info.Mode().Perm()),
		Size:     int64(len(data)),
		ModTime:  info.ModTime().UTC(),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	written, err := tw.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func addStateEntry(stats *StateStats, size int64) error {
	stats.Entries++
	if stats.Entries > MaxEntries {
		return fmt.Errorf("%w: state entries exceed %d", ErrLimit, MaxEntries)
	}

	padded := size
	if remainder := size % tarBlockSize; remainder != 0 {
		padded += tarBlockSize - remainder
	}
	stats.Uncompressed += tarBlockSize + padded
	if stats.Uncompressed > MaxUncompressedSize {
		return fmt.Errorf("%w: uncompressed state exceeds %d bytes",
			ErrLimit, MaxUncompressedSize)
	}
	return nil
}
