package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverProton searches the Steam Proton and GE-Proton locations used by the
// legacy client. The newest-looking directory wins (lexicographic descending).
func DiscoverProton(home string) (string, error) {
	roots := []string{
		filepath.Join(home, ".steam", "steam", "steamapps", "common"),
		filepath.Join(home, ".local", "share", "Steam", "steamapps", "common"),
		filepath.Join(home, ".steam", "steam", "compatibilitytools.d"),
		filepath.Join(home, ".local", "share", "Steam",
			"compatibilitytools.d"),
	}
	var found []string
	seen := make(map[string]struct{})
	for _, root := range roots {
		for _, pattern := range []string{"Proton*", "GE-Proton*"} {
			matches, err := filepath.Glob(filepath.Join(root, pattern))
			if err != nil {
				return "", fmt.Errorf("search Proton in %s: %w", root, err)
			}
			for _, match := range matches {
				info, err := os.Lstat(match)
				if err != nil || info.Mode()&os.ModeSymlink != 0 ||
					!info.IsDir() {
					continue
				}
				if !executableFile(filepath.Join(match, "proton")) {
					continue
				}
				if _, ok := seen[match]; ok {
					continue
				}
				seen[match] = struct{}{}
				found = append(found, match)
			}
		}
	}
	if len(found) == 0 {
		return "", fmt.Errorf(
			"Proton was not found in %s; install Proton through Steam or set the Proton folder in Settings",
			strings.Join(roots, ", "))
	}
	sort.Slice(found, func(i, j int) bool {
		left, right := filepath.Base(found[i]), filepath.Base(found[j])
		if left != right {
			return left > right
		}
		return found[i] > found[j]
	})
	return found[0], nil
}

func protonDir(raw, home string) (string, error) {
	path, err := expandHome(strings.TrimSpace(raw), home)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("Proton folder %q: %w", raw, err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("Proton folder %q: %w", absolute, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("Proton folder %q is not a directory", absolute)
	}
	proton := filepath.Join(absolute, "proton")
	info, err = os.Lstat(proton)
	if err != nil {
		return "", fmt.Errorf("Proton executable %s: %w", proton, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Proton executable %s is not a regular file", proton)
	}
	return absolute, nil
}

func steamRoot(protonDir, home string) string {
	normalized := filepath.ToSlash(protonDir)
	for _, marker := range []string{"/compatibilitytools.d/", "/steamapps/"} {
		if index := strings.Index(normalized, marker); index >= 0 {
			root := normalized[:index]
			if root != "" {
				return filepath.FromSlash(root)
			}
		}
	}
	for _, candidate := range []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join(home, ".steam", "steam")
}

func expandHome(raw, home string) (string, error) {
	if raw == "" {
		return "", errors.New("path is empty")
	}
	if raw == "~" {
		return home, nil
	}
	if strings.HasPrefix(raw, "~/") {
		return filepath.Join(home, strings.TrimPrefix(raw, "~/")), nil
	}
	return raw, nil
}

func absoluteDirectory(path, label string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("%s is empty", label)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s %q is not absolute", label, path)
	}
	return filepath.Clean(path), nil
}

func executableFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 &&
		info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
