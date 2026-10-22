package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Apply runs Build and removes the planned archives. A deletion failure does
// not stop the run: it is reported in Warnings and the file is not listed in
// Pruned.
func Apply(cfg Config) (Result, error) {
	plan, err := Build(cfg)
	if err != nil {
		return Result{}, err
	}

	result := Result{Plan: plan}
	dirs := make(map[string]bool)
	for _, file := range plan.Prune {
		if err := os.Remove(file.Path); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("remove %s: %v", file.Path, err))
			switch {
			case file.Kind.IsState():
				result.StateBytesAfter = addBytes(result.StateBytesAfter, file.Size)
			case file.Kind.IsMods():
				result.ModsBytesAfter = addBytes(result.ModsBytesAfter, file.Size)
			}
			if result.FreeBytesAfter >= 0 {
				result.FreeBytesAfter -= file.Size
			}
			continue
		}
		result.Pruned = append(result.Pruned, file.Name)
		dirs[filepath.Dir(file.Path)] = true
	}

	dirList := make([]string, 0, len(dirs))
	for dir := range dirs {
		dirList = append(dirList, dir)
	}
	sort.Strings(dirList)
	for _, dir := range dirList {
		if err := syncDir(dir); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("sync %s: %v", dir, err))
		}
	}
	return result, nil
}
