package archive

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

// ExtractStateFile extracts a previously validated state archive into dest.
// dest is created if missing and must be an empty directory tree.
func ExtractStateFile(name, dest string) error {
	reader, err := Open(name)
	if err != nil {
		return err
	}
	defer reader.Close()
	return ExtractState(reader, dest)
}

// ExtractState extracts a state archive from r into dest. It accepts only the
// nested layout from docs/v2/03-contracts.md §6.2 and validates the metadata
// files while streaming them to disk.
func ExtractState(r *Reader, dest string) error {
	if r == nil {
		return errors.New("archive: nil state reader")
	}
	if dest == "" {
		return errors.New("archive: state destination is required")
	}
	if err := ensureExtractDir(dest); err != nil {
		return err
	}
	for _, dir := range stateRequiredDirs() {
		if err := ensureExtractDir(filepath.Join(dest, dir)); err != nil {
			return err
		}
	}

	var (
		seen       = make(map[string]bool)
		hasJSON    bool
		hasCurrent bool
		buf        = make([]byte, 32*1024)
	)

	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		switch entry.Type {
		case TypeDir:
			if err := validateStateDir(entry.Name, seen); err != nil {
				return err
			}
			continue
		case TypeFile:
			// Continue below.
		default:
			return archiveInvalidf(
				"%q: unsupported entry type", entry.Name)
		}

		switch entry.Name {
		case requiredDataFilesName:
			if hasJSON {
				return archiveInvalidf(
					"%s: duplicate metadata file", requiredDataFilesName)
			}
			data, err := readLimitedEntry(r, requiredDataFilesName,
				entry.Size, MaxRequiredDataFilesSize)
			if err != nil {
				return err
			}
			if _, err := ParseRequiredDataFiles(data); err != nil {
				return err
			}
			if err := writeExtractedFile(filepath.Join(dest, entry.Name),
				bytes.NewReader(data), buf); err != nil {
				return err
			}
			hasJSON = true
			continue

		case currentFileName:
			if hasCurrent {
				return archiveInvalidf(
					"%s: duplicate metadata file", currentFileName)
			}
			data, err := readLimitedEntry(r, currentFileName,
				entry.Size, MaxCurrentFileSize)
			if err != nil {
				return err
			}
			if _, err := ParseCurrent(data); err != nil {
				return err
			}
			if err := writeExtractedFile(filepath.Join(dest, entry.Name),
				bytes.NewReader(data), buf); err != nil {
				return err
			}
			hasCurrent = true
			continue
		}

		top, rest, ok := strings.Cut(entry.Name, "/")
		if !ok || rest == "" {
			return archiveInvalidf(
				"%q: unexpected top-level file", entry.Name)
		}

		switch top {
		case "players":
			if strings.Contains(rest, "/") {
				return archiveInvalidf(
					"%q: nested files are not allowed", entry.Name)
			}
			if err := writeExtractedFile(filepath.Join(dest, top, rest),
				r, buf); err != nil {
				return err
			}
			seen["players"] = true

		case "world":
			sub, file, ok := strings.Cut(rest, "/")
			if !ok || file == "" {
				return archiveInvalidf(
					"%q: expected world/<name>/<file>", entry.Name)
			}
			if strings.Contains(file, "/") {
				return archiveInvalidf(
					"%q: nested files are not allowed", entry.Name)
			}
			if _, ok := stateWorldDirSet[sub]; !ok {
				return archiveInvalidf(
					"%q: unexpected world directory", entry.Name)
			}
			if err := writeExtractedFile(
				filepath.Join(dest, top, sub, file), r, buf); err != nil {
				return err
			}
			seen["world"] = true
			seen["world/"+sub] = true

		default:
			return archiveInvalidf(
				"%q: unexpected top-level file", entry.Name)
		}
	}

	for _, name := range stateRequiredDirs() {
		if !seen[name] {
			return archiveInvalidf(
				"state archive is missing %s/", name)
		}
	}
	if !hasJSON {
		return archiveInvalidf(
			"state archive is missing %s", requiredDataFilesName)
	}
	if !hasCurrent {
		return archiveInvalidf(
			"state archive is missing %s", currentFileName)
	}
	return nil
}
