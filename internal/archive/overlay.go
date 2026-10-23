package archive

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

// Mapper maps an archive entry to a relative destination path. Returning
// include=false skips the entry. An empty target is only valid for a directory
// and means the destination root itself.
type Mapper func(name string, directory bool) (string, bool, error)

// ExtractOverlayFile extracts a previously validated archive into dest using
// mapper. Unlike ExtractMods it accepts arbitrary relative directory trees,
// which is what the localization overlay needs. Entries the mapper skips are
// read and discarded, so a mapper may filter a whole archive.
func ExtractOverlayFile(name, dest string, mapper Mapper) error {
	// Release assets are occasionally served as tar.gz; accepting both keeps
	// the installer independent of the release packaging.
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	var magic [2]byte
	_, readErr := io.ReadFull(file, magic[:])
	closeErr := file.Close()
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	var reader *Reader
	if readErr == nil && magic == [2]byte{0x1f, 0x8b} {
		reader, err = Open(name)
	} else {
		reader, err = OpenTar(name)
	}
	if err != nil {
		return err
	}
	defer reader.Close()
	return ExtractOverlay(reader, dest, mapper)
}

// ExtractOverlay extracts r into dest using mapper. The archive reader has
// already rejected absolute paths, traversal, duplicate names and unsafe entry
// types; this function additionally validates every mapped destination.
func ExtractOverlay(r *Reader, dest string, mapper Mapper) error {
	if r == nil {
		return errors.New("archive: nil overlay reader")
	}
	if dest == "" {
		return errors.New("archive: overlay destination is required")
	}
	if mapper == nil {
		mapper = func(name string, _ bool) (string, bool, error) {
			return name, true, nil
		}
	}
	if err := ensureExtractDir(dest); err != nil {
		return err
	}

	buf := make([]byte, 64*1024)
	seen := make(map[string]string)
	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		directory := entry.Type == TypeDir
		target, include, err := mapper(entry.Name, directory)
		if err != nil {
			return fmt.Errorf("map %q: %w", entry.Name, err)
		}
		if !include {
			continue
		}
		if target == "" {
			if !directory {
				return fmt.Errorf("%w: %q maps to the destination root",
					ErrInvalid, entry.Name)
			}
			continue
		}
		target, err = pathguard.Path(target)
		if err != nil {
			return fmt.Errorf("map %q: %w", entry.Name, err)
		}
		key := strings.ToLower(target)
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("%w: entries %q and %q map to the same path",
				ErrInvalid, previous, entry.Name)
		}
		seen[key] = entry.Name

		path := filepath.Join(dest, filepath.FromSlash(target))
		switch entry.Type {
		case TypeDir:
			if err := ensureExtractDir(path); err != nil {
				return err
			}
		case TypeFile:
			if err := ensureExtractDir(filepath.Dir(path)); err != nil {
				return err
			}
			if err := writeExtractedFile(path, r, buf); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %q: unsupported entry type", ErrInvalid,
				entry.Name)
		}
	}
}

// CopyTree copies a regular directory tree into dest. It never follows
// symlinks and rejects anything other than directories and regular files.
// Existing files are replaced, so a later overlay layer can win.
func CopyTree(source, dest string) error {
	if source == "" || dest == "" {
		return errors.New("archive: copy source and destination are required")
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("archive: %s is not a directory", source)
	}
	if err := ensureExtractDir(dest); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(dest, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("%w: %s: symlink is not allowed", ErrInvalid,
				path)
		case info.IsDir():
			return ensureExtractDir(target)
		case info.Mode().IsRegular():
			if err := ensureExtractDir(filepath.Dir(target)); err != nil {
				return err
			}
			return copyRegular(path, target)
		default:
			return fmt.Errorf("%w: %s: unsupported file type", ErrInvalid,
				path)
		}
	})
}

func copyRegular(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
