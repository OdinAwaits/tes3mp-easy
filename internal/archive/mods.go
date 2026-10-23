package archive

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"path"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

const (
	// MaxRequiredDataFilesSize bounds the metadata file kept in memory. With
	// 50 000 entries and normal plugin names it stays well below this value.
	MaxRequiredDataFilesSize int64 = 8 << 20

	requiredDataFilesName = "requiredDataFiles.json"
)

var (
	baseESMs = map[string]struct{}{
		"morrowind.esm": {},
		"tribunal.esm":  {},
		"bloodmoon.esm": {},
	}

	pluginExtensions = map[string]struct{}{
		".esp":        {},
		".esm":        {},
		".omwaddon":   {},
		".omwscripts": {},
		".omwgame":    {},
	}
)

// RequiredFile is one entry of plugins/requiredDataFiles.json. CRCs is empty
// when the archive only requires the file to exist.
type RequiredFile struct {
	Name string
	CRCs []uint32
}

// ModsInfo describes a validated mods archive.
type ModsInfo struct {
	Plugins int
	Scripts int
}

// ValidateModsFile validates a mods tar.gz archive on disk.
func ValidateModsFile(name string) (ModsInfo, error) {
	info, _, err := ValidateModsFileWithSize(name)
	return info, err
}

// ValidateModsFileWithSize validates a mods tar.gz archive on disk and also
// returns the total uncompressed size of its files.
func ValidateModsFileWithSize(name string) (ModsInfo, int64, error) {
	reader, err := Open(name)
	if err != nil {
		return ModsInfo{}, 0, err
	}
	defer reader.Close()
	return ValidateModsWithSize(reader)
}

// ValidateModsReader validates a mods tar.gz archive from an io.Reader.
func ValidateModsReader(r io.Reader) (ModsInfo, error) {
	reader, err := NewReader(r)
	if err != nil {
		return ModsInfo{}, err
	}
	defer reader.Close()
	return ValidateMods(reader)
}

// ValidateMods checks the archive layout and metadata defined by
// docs/v2/03-contracts.md §6.1 and §6.3. It streams every entry through the
// reader, so plugin contents are never buffered in memory.
func ValidateMods(r *Reader) (ModsInfo, error) {
	info, _, err := ValidateModsWithSize(r)
	return info, err
}

// ValidateModsWithSize is ValidateMods plus the total uncompressed size of the
// archive's files. The mods deploy disk-space policy uses that number before
// extracting a pack.
func ValidateModsWithSize(r *Reader) (ModsInfo, int64, error) {
	if r == nil {
		return ModsInfo{}, 0, errors.New("archive: nil mods reader")
	}

	var (
		requirements []RequiredFile
		hasJSON      bool

		pluginCRCs   = make(map[string]uint32)
		pluginNames  []string
		scriptCount  int
		uncompressed int64

		buf = make([]byte, 32*1024)
	)

	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ModsInfo{}, 0, err
		}

		switch entry.Type {
		case TypeDir:
			switch {
			case entry.Name == "plugins", entry.Name == "scripts":
				continue
			case strings.HasPrefix(entry.Name, "plugins/"),
				strings.HasPrefix(entry.Name, "scripts/"):
				return ModsInfo{}, 0, archiveInvalidf(
					"%q: nested directories are not allowed", entry.Name)
			default:
				return ModsInfo{}, 0, archiveInvalidf(
					"%q: unexpected top-level directory", entry.Name)
			}
		case TypeFile:
			uncompressed += entry.Size
			// Continue below.
		default:
			return ModsInfo{}, 0, archiveInvalidf(
				"%q: unsupported entry type", entry.Name)
		}

		top, rest, hasDir := strings.Cut(entry.Name, "/")
		if !hasDir {
			if entry.Name == requiredDataFilesName {
				return ModsInfo{}, 0, archiveInvalidf(
					"%s must be inside plugins/, not at archive root", requiredDataFilesName)
			}
			return ModsInfo{}, 0, archiveInvalidf(
				"%q: unexpected top-level file", entry.Name)
		}

		switch top {
		case "plugins":
			if strings.Contains(rest, "/") {
				return ModsInfo{}, 0, archiveInvalidf(
					"%q: nested directories are not allowed", entry.Name)
			}
			if rest == requiredDataFilesName {
				if hasJSON {
					return ModsInfo{}, 0, archiveInvalidf(
						"%s: duplicate metadata file", requiredDataFilesName)
				}
				data, err := readRequiredDataFiles(r, entry.Size)
				if err != nil {
					return ModsInfo{}, 0, err
				}
				requirements, err = ParseRequiredDataFiles(data)
				if err != nil {
					return ModsInfo{}, 0, err
				}
				hasJSON = true
				continue
			}

			if err := validatePluginExtension(rest); err != nil {
				return ModsInfo{}, 0, fmt.Errorf("plugins/%s: %w", rest, err)
			}
			if isBaseESM(rest) {
				return ModsInfo{}, 0, archiveInvalidf(
					"plugins/%s: base ESM must not be archived", rest)
			}

			sum := crc32.NewIEEE()
			if _, err := io.CopyBuffer(sum, r, buf); err != nil {
				return ModsInfo{}, 0, err
			}
			pluginNames = append(pluginNames, rest)
			pluginCRCs[rest] = sum.Sum32()

		case "scripts":
			if strings.Contains(rest, "/") {
				return ModsInfo{}, 0, archiveInvalidf(
					"%q: nested directories are not allowed", entry.Name)
			}
			if err := validateScriptFilename(rest); err != nil {
				return ModsInfo{}, 0, fmt.Errorf("scripts/%s: %w", rest, err)
			}
			scriptCount++

		default:
			return ModsInfo{}, 0, archiveInvalidf(
				"%q: unexpected top-level entry", entry.Name)
		}
	}

	if !hasJSON {
		return ModsInfo{}, 0, archiveInvalidf(
			"%s is missing from plugins/", requiredDataFilesName)
	}
	if err := validateModsRequirements(requirements, pluginNames, pluginCRCs); err != nil {
		return ModsInfo{}, 0, err
	}

	return ModsInfo{Plugins: len(pluginNames), Scripts: scriptCount},
		uncompressed, nil
}

// ParseRequiredDataFiles parses the metadata format from
// docs/v2/03-contracts.md §6.3.
func ParseRequiredDataFiles(data []byte) ([]RequiredFile, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil {
		return nil, archiveInvalidf("invalid JSON: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil, archiveInvalidf("top-level value must be an array")
	}

	var (
		files    []RequiredFile
		seen     = make(map[string]string)
		seenMods bool
	)
	for dec.More() {
		if len(files) >= MaxEntries {
			return nil, archiveInvalidf("too many entries (limit %d)", MaxEntries)
		}

		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, archiveInvalidf("invalid JSON: %v", err)
		}
		obj, err := parseRequiredObject(raw)
		if err != nil {
			return nil, err
		}

		if err := pathguard.Base(obj.Name); err != nil {
			return nil, archiveInvalidf(
				"invalid plugin name %q: %v", obj.Name, err)
		}
		if err := validatePluginExtension(obj.Name); err != nil {
			return nil, archiveInvalidf(
				"invalid plugin name %q: %v", obj.Name, err)
		}

		key := strings.ToLower(obj.Name)
		if prev, ok := seen[key]; ok {
			return nil, archiveInvalidf(
				"duplicate plugin name %q (also %q)", obj.Name, prev)
		}

		if isBaseESM(obj.Name) {
			if seenMods {
				return nil, archiveInvalidf(
					"base ESM entries must come before mods")
			}
		} else {
			seenMods = true
		}

		crcs, err := parseCRCs(obj.CRCs)
		if err != nil {
			return nil, archiveInvalidf("%s: %v", obj.Name, err)
		}

		seen[key] = obj.Name
		files = append(files, RequiredFile{Name: obj.Name, CRCs: crcs})
	}

	tok, err = dec.Token()
	if err != nil {
		return nil, archiveInvalidf("invalid JSON: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != ']' {
		return nil, archiveInvalidf("top-level array is not closed")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, archiveInvalidf("trailing data after JSON array")
	}

	return files, nil
}

type requiredObject struct {
	Name string
	CRCs []string
}

func parseRequiredObject(data []byte) (requiredObject, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil {
		return requiredObject{}, archiveInvalidf("invalid entry: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return requiredObject{}, archiveInvalidf("entry must be an object")
	}
	if !dec.More() {
		return requiredObject{}, archiveInvalidf(
			"entry must contain exactly one key")
	}

	tok, err = dec.Token()
	if err != nil {
		return requiredObject{}, archiveInvalidf("invalid entry: %v", err)
	}
	name, ok := tok.(string)
	if !ok || name == "" {
		return requiredObject{}, archiveInvalidf(
			"entry key must be a non-empty string")
	}

	tok, err = dec.Token()
	if err != nil {
		return requiredObject{}, archiveInvalidf("invalid entry: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return requiredObject{}, archiveInvalidf(
			"%s: value must be an array of CRC strings", name)
	}

	var crcs []string
	for dec.More() {
		var crc string
		if err := dec.Decode(&crc); err != nil {
			return requiredObject{}, archiveInvalidf(
				"%s: CRC value must be a string", name)
		}
		crcs = append(crcs, crc)
	}

	tok, err = dec.Token()
	if err != nil {
		return requiredObject{}, archiveInvalidf("invalid entry: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != ']' {
		return requiredObject{}, archiveInvalidf(
			"%s: CRC array is not closed", name)
	}
	if dec.More() {
		return requiredObject{}, archiveInvalidf(
			"entry must contain exactly one key")
	}

	tok, err = dec.Token()
	if err != nil {
		return requiredObject{}, archiveInvalidf("invalid entry: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '}' {
		return requiredObject{}, archiveInvalidf("entry object is not closed")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return requiredObject{}, archiveInvalidf("trailing data in entry")
	}

	return requiredObject{Name: name, CRCs: crcs}, nil
}

func parseCRCs(raw []string) ([]uint32, error) {
	crcs := make([]uint32, 0, len(raw))
	for _, value := range raw {
		if len(value) != 10 || value[0:2] != "0x" {
			return nil, fmt.Errorf(
				"CRC %q must use 0x followed by 8 uppercase hex digits", value)
		}
		var sum uint32
		for i := 2; i < len(value); i++ {
			c := value[i]
			switch {
			case c >= '0' && c <= '9':
				sum = sum<<4 | uint32(c-'0')
			case c >= 'A' && c <= 'F':
				sum = sum<<4 | uint32(c-'A'+10)
			default:
				return nil, fmt.Errorf(
					"CRC %q must use 0x followed by 8 uppercase hex digits", value)
			}
		}
		crcs = append(crcs, sum)
	}
	return crcs, nil
}

func validateModsRequirements(
	requirements []RequiredFile,
	pluginNames []string,
	pluginCRCs map[string]uint32,
) error {
	required := make(map[string]RequiredFile, len(requirements))
	for _, file := range requirements {
		required[file.Name] = file
	}

	for _, file := range requirements {
		if isBaseESM(file.Name) {
			continue
		}
		actual, ok := pluginCRCs[file.Name]
		if !ok {
			return archiveInvalidf(
				"%s: plugin %q is missing from plugins/",
				requiredDataFilesName, file.Name)
		}
		if len(file.CRCs) > 0 && !containsCRC(file.CRCs, actual) {
			return archiveInvalidf(
				"%s: CRC mismatch for %s (expected %s, got 0x%08X)",
				requiredDataFilesName, file.Name,
				formatCRCs(file.CRCs), actual)
		}
	}

	for _, name := range pluginNames {
		if _, ok := required[name]; !ok {
			return archiveInvalidf(
				"plugins/%s: not listed in %s", name, requiredDataFilesName)
		}
	}
	return nil
}

func containsCRC(crcs []uint32, actual uint32) bool {
	for _, crc := range crcs {
		if crc == actual {
			return true
		}
	}
	return false
}

func formatCRCs(crcs []uint32) string {
	parts := make([]string, len(crcs))
	for i, crc := range crcs {
		parts[i] = fmt.Sprintf("0x%08X", crc)
	}
	return strings.Join(parts, ", ")
}

func readRequiredDataFiles(r *Reader, size int64) ([]byte, error) {
	return readLimitedEntry(r, requiredDataFilesName, size, MaxRequiredDataFilesSize)
}

// readLimitedEntry reads one metadata entry, enforcing the limit both on the
// tar header size and on the actual stream. Callers must invoke it directly
// after Next returned the matching regular file.
func readLimitedEntry(r *Reader, name string, size, limit int64) ([]byte, error) {
	if size > limit {
		return nil, archiveInvalidf(
			"%s: file exceeds %d bytes", name, limit)
	}

	data, err := io.ReadAll(&io.LimitedReader{R: r, N: limit + 1})
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, archiveInvalidf(
			"%s: file exceeds %d bytes", name, limit)
	}
	return data, nil
}

func validatePluginExtension(name string) error {
	ext := path.Ext(name)
	if _, ok := pluginExtensions[strings.ToLower(ext)]; !ok || len(name) <= len(ext) {
		return archiveInvalidf("unsupported plugin extension in %q", name)
	}
	return nil
}

func validateScriptFilename(name string) error {
	if path.Ext(name) != ".lua" {
		return archiveInvalidf("script %q must have the .lua extension", name)
	}
	stem := strings.TrimSuffix(name, ".lua")
	if stem == "" {
		return archiveInvalidf("script name must not be empty")
	}
	for i := 0; i < len(stem); i++ {
		c := stem[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' || c == '_' || c == '-' {
			continue
		}
		return archiveInvalidf(
			"script name %q may contain only letters, digits, '_' and '-'", stem)
	}
	return nil
}

func isBaseESM(name string) bool {
	_, ok := baseESMs[strings.ToLower(name)]
	return ok
}

// archiveInvalidf wraps ErrInvalid for every archive layout or metadata error.
func archiveInvalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
