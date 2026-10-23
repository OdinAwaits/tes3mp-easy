package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func checkCleanup(root string) Check {
	problems := make([]string, 0)

	staging := filepath.Join(root, ".staging")
	entries, err := os.ReadDir(staging)
	switch {
	case err != nil:
		problems = append(problems, fmt.Sprintf(".staging/: %v", err))
	case len(entries) > 0:
		problems = append(problems,
			fmt.Sprintf(".staging/ contains %d leftover entries", len(entries)))
	}

	locks := filepath.Join(root, ".locks")
	entries, err = os.ReadDir(locks)
	switch {
	case err != nil:
		problems = append(problems, fmt.Sprintf(".locks/: %v", err))
	default:
		for _, entry := range entries {
			path := filepath.Join(locks, entry.Name())
			info, lstatErr := os.Lstat(path)
			if lstatErr != nil {
				problems = append(problems, fmt.Sprintf(".locks/%s: %v", entry.Name(), lstatErr))
				continue
			}
			if entry.Name() != "ops.lock" {
				problems = append(problems,
					fmt.Sprintf(".locks/%s is a stale lock file", entry.Name()))
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				problems = append(problems, ".locks/ops.lock is not a regular file")
			}
		}
	}

	if err := walkOrphans(filepath.Join(root, "backups"), "backups", &problems); err != nil {
		problems = append(problems, err.Error())
	}

	for _, dir := range []string{"", "mods", "mods/plugins", "panel-data/uploads"} {
		path := root
		label := ""
		if dir != "" {
			path = filepath.Join(root, dir)
			label = dir + "/"
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", label, err))
			continue
		}
		for _, entry := range entries {
			if orphanName(entry.Name()) {
				problems = append(problems, fmt.Sprintf("%s%s is leftover",
					label, entry.Name()))
			}
		}
	}

	if len(problems) > 0 {
		return errorCheck("cleanup", "%s", joinProblems(problems))
	}
	return Check{Name: "cleanup", Status: StatusOK}
}

func walkOrphans(root, label string, problems *[]string) error {
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if path == root {
			return nil
		}
		if orphanName(entry.Name()) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			*problems = append(*problems,
				fmt.Sprintf("%s/%s is leftover", label, filepath.ToSlash(rel)))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func orphanName(name string) bool {
	return strings.HasSuffix(name, ".part") || strings.Contains(name, ".old-")
}
