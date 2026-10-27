package archive

import (
	"errors"
	"io"
	"strings"
)

// StateInfo describes a validated state archive. RequiredDataFiles is the
// parsed root metadata file, and Current is the parsed root current.txt.
// ConfigFiles is always 1: only configs/config.lua belongs to the state format.
type StateInfo struct {
	Current           Current
	RequiredDataFiles []RequiredFile
	Players           int
	WorldFiles        int
	ConfigFiles       int
}

// stateWorldDirs is the fixed set of world subdirectories from
// docs/v2/03-contracts.md §6.2. The slice preserves a deterministic order for
// error messages and missing-entry checks.
var stateWorldDirs = []string{"cell", "world", "map", "recordstore", "custom"}

var stateWorldDirSet = map[string]struct{}{
	"cell":        {},
	"world":       {},
	"map":         {},
	"recordstore": {},
	"custom":      {},
}

// ValidateStateFile validates a state tar.gz archive on disk, including the
// referenced mods archive under modsDir (normally <root>/backups/mods).
func ValidateStateFile(name, modsDir string) (StateInfo, error) {
	info, _, err := ValidateStateFileWithSize(name, modsDir)
	return info, err
}

// ValidateStateLayoutFile validates a state archive on disk without checking
// the referenced mods package. Public downloads use it to reject archives that
// do not match the current layout, including old archives that still contain
// tes3mp-server-default.cfg or banlist.json.
func ValidateStateLayoutFile(name string) (StateInfo, error) {
	reader, err := Open(name)
	if err != nil {
		return StateInfo{}, err
	}
	defer reader.Close()
	return ValidateState(reader)
}

// ValidateStateFileWithSize is ValidateStateFile plus the total uncompressed
// size of the state archive's files. State deploy uses that number before
// extracting a backup.
func ValidateStateFileWithSize(name, modsDir string) (StateInfo, int64, error) {
	reader, err := Open(name)
	if err != nil {
		return StateInfo{}, 0, err
	}
	defer reader.Close()

	info, size, err := ValidateStateWithSize(reader)
	if err != nil {
		return StateInfo{}, 0, err
	}
	if err := VerifyStateMods(info, modsDir); err != nil {
		return StateInfo{}, 0, err
	}
	return info, size, nil
}

// ValidateStateReader validates a state tar.gz archive from an io.Reader and
// verifies its current.txt against modsDir. See ValidateStateFile.
func ValidateStateReader(r io.Reader, modsDir string) (StateInfo, error) {
	reader, err := NewReader(r)
	if err != nil {
		return StateInfo{}, err
	}
	defer reader.Close()

	info, err := ValidateState(reader)
	if err != nil {
		return StateInfo{}, err
	}
	if err := VerifyStateMods(info, modsDir); err != nil {
		return StateInfo{}, err
	}
	return info, nil
}

// ValidateState checks the nested archive layout and metadata from
// docs/v2/03-contracts.md §6.2 and §6.4. It does not inspect backups/mods;
// callers that need the full deploy-time check use ValidateStateFile,
// ValidateStateReader or VerifyStateMods.
func ValidateState(r *Reader) (StateInfo, error) {
	info, _, err := ValidateStateWithSize(r)
	return info, err
}

// ValidateStateWithSize is ValidateState plus the total uncompressed size of
// the archive's files.
func ValidateStateWithSize(r *Reader) (StateInfo, int64, error) {
	if r == nil {
		return StateInfo{}, 0, errors.New("archive: nil state reader")
	}

	var (
		info         StateInfo
		seen         = make(map[string]bool)
		hasJSON      bool
		hasCurrent   bool
		uncompressed int64
	)

	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return StateInfo{}, 0, err
		}

		switch entry.Type {
		case TypeDir:
			if err := validateStateDir(entry.Name, seen); err != nil {
				return StateInfo{}, 0, err
			}
		case TypeFile:
			if err := validateStateEntry(r, entry, seen, &info, &hasJSON, &hasCurrent); err != nil {
				return StateInfo{}, 0, err
			}
			uncompressed += entry.Size
		default:
			return StateInfo{}, 0, archiveInvalidf(
				"%q: unsupported entry type", entry.Name)
		}
	}

	for _, name := range stateRequiredDirs() {
		if !seen[name] {
			return StateInfo{}, 0, archiveInvalidf(
				"state archive is missing %s/", name)
		}
	}
	if err := validateStateConfigs(seen); err != nil {
		return StateInfo{}, 0, err
	}
	if !hasJSON {
		return StateInfo{}, 0, archiveInvalidf(
			"state archive is missing %s", requiredDataFilesName)
	}
	if !hasCurrent {
		return StateInfo{}, 0, archiveInvalidf(
			"state archive is missing %s", currentFileName)
	}

	return info, uncompressed, nil
}

// ReadStateCurrent reads current.txt from a state archive without validating
// the rest of the archive. WriteState stores current.txt first, so retention
// can read the mods reference without scanning player and world files.
func ReadStateCurrent(name string) (Current, error) {
	reader, err := Open(name)
	if err != nil {
		return Current{}, err
	}
	defer reader.Close()

	for {
		entry, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Current{}, err
		}
		if entry.Type != TypeFile || entry.Name != currentFileName {
			continue
		}
		data, err := readLimitedEntry(reader, currentFileName,
			entry.Size, MaxCurrentFileSize)
		if err != nil {
			return Current{}, err
		}
		return ParseCurrent(data)
	}
	return Current{}, archiveInvalidf(
		"state archive is missing %s", currentFileName)
}

func stateRequiredDirs() []string {
	required := make([]string, 0, 2+len(stateWorldDirs))
	required = append(required, "players", "world")
	for _, name := range stateWorldDirs {
		required = append(required, "world/"+name)
	}
	return required
}

func validateStateConfigs(seen map[string]bool) error {
	for _, name := range stateConfigFiles {
		if !seen["configs/"+name] {
			return archiveInvalidf(
				"state archive is missing configs/%s", name)
		}
	}
	return nil
}

func validateStateDir(name string, seen map[string]bool) error {
	switch {
	case name == "players", name == "world", name == "configs":
		seen[name] = true
		return nil
	case strings.HasPrefix(name, "players/"):
		return archiveInvalidf(
			"%q: nested directories are not allowed", name)
	case strings.HasPrefix(name, "configs/"):
		return archiveInvalidf(
			"%q: nested directories are not allowed", name)
	case strings.HasPrefix(name, "world/"):
		sub := strings.TrimPrefix(name, "world/")
		if strings.Contains(sub, "/") {
			return archiveInvalidf(
				"%q: nested directories are not allowed", name)
		}
		if _, ok := stateWorldDirSet[sub]; !ok {
			return archiveInvalidf(
				"%q: unexpected world directory", name)
		}
		seen[name] = true
		seen["world"] = true
		return nil
	default:
		return archiveInvalidf(
			"%q: unexpected top-level directory", name)
	}
}

func validateStateEntry(
	r *Reader,
	entry *Entry,
	seen map[string]bool,
	info *StateInfo,
	hasJSON, hasCurrent *bool,
) error {
	switch entry.Name {
	case requiredDataFilesName:
		if *hasJSON {
			return archiveInvalidf(
				"%s: duplicate metadata file", requiredDataFilesName)
		}
		data, err := readLimitedEntry(
			r, requiredDataFilesName, entry.Size, MaxRequiredDataFilesSize)
		if err != nil {
			return err
		}
		requirements, err := ParseRequiredDataFiles(data)
		if err != nil {
			return err
		}
		info.RequiredDataFiles = requirements
		*hasJSON = true
		return nil

	case currentFileName:
		if *hasCurrent {
			return archiveInvalidf(
				"%s: duplicate metadata file", currentFileName)
		}
		data, err := readLimitedEntry(
			r, currentFileName, entry.Size, MaxCurrentFileSize)
		if err != nil {
			return err
		}
		current, err := ParseCurrent(data)
		if err != nil {
			return err
		}
		info.Current = current
		*hasCurrent = true
		return nil
	}

	top, rest, ok := strings.Cut(entry.Name, "/")
	if !ok {
		return archiveInvalidf(
			"%q: unexpected top-level file", entry.Name)
	}

	switch top {
	case "players":
		if strings.Contains(rest, "/") {
			return archiveInvalidf(
				"%q: nested files are not allowed", entry.Name)
		}
		info.Players++
		seen["players"] = true
		return nil

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
		info.WorldFiles++
		seen["world"] = true
		seen["world/"+sub] = true
		return nil

	case "configs":
		if rest == "" || strings.Contains(rest, "/") {
			return archiveInvalidf(
				"%q: expected configs/<file>", entry.Name)
		}
		if _, ok := stateConfigFileSet[rest]; !ok {
			return archiveInvalidf(
				"%q: unexpected config file", entry.Name)
		}
		info.ConfigFiles++
		seen["configs"] = true
		seen[entry.Name] = true
		return nil

	default:
		return archiveInvalidf(
			"%q: unexpected top-level file", entry.Name)
	}
}
