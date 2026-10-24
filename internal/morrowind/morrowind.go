// Package morrowind discovers a Morrowind installation and verifies the three
// base ESM files. It only reads the original game directory.
package morrowind

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// EncodingWin1251 marks Morrowind ESM files whose Russian text is encoded in
// CP1251. OpenMW needs encoding=win1251 for such data files.
const EncodingWin1251 = "win1251"

const encodingProbeSize = 1 << 20

// cp1251Markers are common Russian strings from Morrowind.esm encoded in
// CP1251. They are absent from English and UTF-8 data files.
var cp1251Markers = [][]byte{
	{0xC7, 0xE0, 0xE3, 0xF0, 0xF3, 0xE7, 0xEA, 0xE0},       // Загрузка
	{0xD1, 0xF2, 0xE0, 0xF0, 0xF2, 0xEE, 0xE2, 0xE0, 0xFF}, // Стартовая
}

// RequiredFiles are the base game files TES3MP cannot run without.
var RequiredFiles = []string{
	"Morrowind.esm",
	"Tribunal.esm",
	"Bloodmoon.esm",
}

var (
	// ErrNotFound marks a failed automatic search.
	ErrNotFound = errors.New("Morrowind installation not found")
	// ErrIncomplete marks an existing Morrowind folder with missing files.
	ErrIncomplete = errors.New("Morrowind installation is incomplete")
)

// Installation is a verified Morrowind root. Root is the folder that contains
// Data Files, not Data Files itself.
type Installation struct {
	Root      string
	DataFiles string
	Files     map[string]string
	// Encoding is EncodingWin1251 when Morrowind.esm contains CP1251 text,
	// otherwise it is empty.
	Encoding string
}

// IncompleteError reports which required files are missing.
type IncompleteError struct {
	Root    string
	Missing []string
}

// Error implements error.
func (e *IncompleteError) Error() string {
	if len(e.Missing) == 0 {
		return fmt.Sprintf("Morrowind folder %q is incomplete", e.Root)
	}
	return fmt.Sprintf("Morrowind folder %q is missing %s",
		e.Root, strings.Join(e.Missing, ", "))
}

// Unwrap allows errors.Is(err, ErrIncomplete).
func (e *IncompleteError) Unwrap() error {
	return ErrIncomplete
}

// NotFoundError lists the locations checked by Discover.
type NotFoundError struct {
	Candidates []string
	Incomplete []*IncompleteError
}

// Error implements error.
func (e *NotFoundError) Error() string {
	if len(e.Incomplete) > 0 {
		first := e.Incomplete[0]
		return fmt.Sprintf("%s; missing %s", first.Root,
			strings.Join(first.Missing, ", "))
	}
	if len(e.Candidates) == 0 {
		return "Morrowind was not found in the standard locations"
	}
	const maxShown = 6
	shown := e.Candidates
	if len(shown) > maxShown {
		shown = shown[:maxShown]
	}
	suffix := ""
	if len(e.Candidates) > len(shown) {
		suffix = fmt.Sprintf(" and %d more", len(e.Candidates)-len(shown))
	}
	return fmt.Sprintf("Morrowind was not found; checked %s%s",
		strings.Join(shown, ", "), suffix)
}

// Unwrap allows errors.Is(err, ErrNotFound).
func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// Inspect verifies root/Data Files and all required ESM files. The comparison
// is case-insensitive after an exact match fails, as required on Windows and
// useful for Linux installs copied from Windows.
func Inspect(root string) (Installation, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Installation{}, errors.New("Morrowind folder is empty")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Installation{}, fmt.Errorf("Morrowind folder %q: %w", root, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Installation{}, fmt.Errorf("%w: Morrowind folder %q: %v",
				ErrNotFound, root, err)
		}
		return Installation{}, fmt.Errorf("Morrowind folder %q: %w", root, err)
	}
	if !info.IsDir() {
		return Installation{}, fmt.Errorf("Morrowind folder %q is not a directory: %w",
			root, ErrIncomplete)
	}

	dataFiles, err := childDir(absolute, "Data Files")
	if err != nil {
		return Installation{}, err
	}
	files := make(map[string]string, len(RequiredFiles))
	missing := make([]string, 0, len(RequiredFiles))
	for _, name := range RequiredFiles {
		path, err := childFile(dataFiles, name)
		if err != nil {
			return Installation{}, err
		}
		if path == "" {
			missing = append(missing, "Data Files/"+name)
			continue
		}
		files[name] = path
	}
	if len(missing) > 0 {
		return Installation{}, &IncompleteError{Root: absolute, Missing: missing}
	}
	return Installation{
		Root:      absolute,
		DataFiles: dataFiles,
		Files:     files,
		Encoding:  detectEncoding(files["Morrowind.esm"]),
	}, nil
}

func detectEncoding(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, encodingProbeSize))
	if err != nil {
		return ""
	}
	for _, marker := range cp1251Markers {
		if bytes.Contains(data, marker) {
			return EncodingWin1251
		}
	}
	return ""
}

func childDir(root, name string) (string, error) {
	exact := filepath.Join(root, name)
	info, err := os.Stat(exact)
	if err == nil {
		if info.IsDir() {
			return exact, nil
		}
		return "", &IncompleteError{Root: root, Missing: []string{name}}
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("inspect %s: %w", exact, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", root, err)
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			path := filepath.Join(root, entry.Name())
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				return path, nil
			}
		}
	}
	return "", &IncompleteError{Root: root, Missing: []string{name}}
}

func childFile(dir, name string) (string, error) {
	exact := filepath.Join(dir, name)
	info, err := os.Stat(exact)
	if err == nil {
		if info.Mode().IsRegular() {
			return exact, nil
		}
		return "", nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("inspect %s: %w", exact, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), name) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	return "", nil
}
