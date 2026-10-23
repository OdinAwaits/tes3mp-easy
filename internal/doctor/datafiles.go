package doctor

import (
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

const requiredDataFilesName = "requiredDataFiles.json"

var baseESMs = map[string]struct{}{
	"morrowind.esm": {},
	"tribunal.esm":  {},
	"bloodmoon.esm": {},
}

var pluginExtensions = map[string]struct{}{
	".esp":        {},
	".esm":        {},
	".omwaddon":   {},
	".omwscripts": {},
	".omwgame":    {},
}

func checkDataFiles(root string) Check {
	dir := filepath.Join(root, "mods", "plugins")
	metadata := filepath.Join(dir, requiredDataFilesName)
	data, err := readRegularFile(metadata, archive.MaxRequiredDataFilesSize)
	if err != nil {
		return errorCheck("datafiles", "%v", err)
	}
	required, err := archive.ParseRequiredDataFiles(data)
	if err != nil {
		return errorCheck("datafiles", "%s: %v", metadata, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return errorCheck("datafiles", "cannot read %s: %v", dir, err)
	}

	problems := make([]string, 0)
	plugins := make(map[string]string)
	for _, entry := range entries {
		if entry.Name() == requiredDataFilesName {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		case info.Mode()&os.ModeSymlink != 0:
			problems = append(problems, fmt.Sprintf("%s is a symlink", entry.Name()))
			continue
		case !info.Mode().IsRegular():
			problems = append(problems, fmt.Sprintf("%s is not a regular file", entry.Name()))
			continue
		}
		if _, ok := pluginExtensions[strings.ToLower(filepath.Ext(entry.Name()))]; !ok {
			problems = append(problems, fmt.Sprintf("%s is not a plugin file", entry.Name()))
			continue
		}
		if _, ok := baseESMs[strings.ToLower(entry.Name())]; ok {
			problems = append(problems, fmt.Sprintf(
				"base ESM %s must not be in mods/plugins", entry.Name()))
			continue
		}
		plugins[entry.Name()] = path
	}

	listed := make(map[string]bool, len(required))
	baseSeen := make(map[string]bool, len(baseESMs))
	crcs := make(map[string]uint32)
	for _, file := range required {
		if _, base := baseESMs[strings.ToLower(file.Name)]; base {
			baseSeen[strings.ToLower(file.Name)] = true
			continue
		}
		listed[file.Name] = true
		path, ok := plugins[file.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%s is listed but missing from plugins/", file.Name))
			continue
		}
		if len(file.CRCs) == 0 {
			continue
		}
		sum, ok := crcs[file.Name]
		if !ok {
			sum, err = crc32File(path)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", file.Name, err))
				continue
			}
			crcs[file.Name] = sum
		}
		matched := false
		for _, want := range file.CRCs {
			if sum == want {
				matched = true
				break
			}
		}
		if !matched {
			problems = append(problems, fmt.Sprintf(
				"%s CRC is 0x%08X, not in %s", file.Name, sum,
				formatCRCs(file.CRCs)))
		}
	}
	for _, name := range []string{"Morrowind.esm", "Tribunal.esm", "Bloodmoon.esm"} {
		if !baseSeen[strings.ToLower(name)] {
			problems = append(problems, fmt.Sprintf(
				"%s is missing from %s", name, requiredDataFilesName))
		}
	}
	for name := range plugins {
		if !listed[name] {
			problems = append(problems, fmt.Sprintf(
				"%s is present but not listed in %s", name, requiredDataFilesName))
		}
	}

	if len(problems) > 0 {
		return errorCheck("datafiles", "%s", joinProblems(problems))
	}
	return okCheck("datafiles", "%d plugins", len(plugins))
}

func readRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("%s does not exist", path)
	case err != nil:
		return nil, fmt.Errorf("%s: %w", path, err)
	case info.Mode()&os.ModeSymlink != 0:
		return nil, fmt.Errorf("%s is a symlink", path)
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%s is not a regular file", path)
	case info.Size() > limit:
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	return data, nil
}

func crc32File(path string) (uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	sum := crc32.NewIEEE()
	if _, err := io.CopyBuffer(sum, file, make([]byte, 32*1024)); err != nil {
		return 0, err
	}
	return sum.Sum32(), nil
}

func formatCRCs(crcs []uint32) string {
	parts := make([]string, len(crcs))
	for i, crc := range crcs {
		parts[i] = fmt.Sprintf("0x%08X", crc)
	}
	return strings.Join(parts, ", ")
}
