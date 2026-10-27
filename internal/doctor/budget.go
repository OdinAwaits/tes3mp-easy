package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/retention"
)

func checkBudgets(root string) Check {
	dirs := []struct {
		rel    string
		budget int64
		label  string
	}{
		{"backups/state", retention.DefaultStateBudget, "1.0 GiB"},
		{"backups/mods", retention.DefaultModsBudget, "512 MiB"},
		{"backups/snapshots", restorepoint.DefaultSnapshotBudget, "1.5 GiB"},
		{"backups/checkpoints", restorepoint.DefaultCheckpointBudget, "256 MiB"},
		{"backups/configs", 32 << 20, "32 MiB"},
	}

	problems := make([]string, 0)
	parts := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		path := filepath.Join(root, dir.rel)
		bytes, err := directorySizeOptional(path)
		if err != nil {
			return errorCheck("budgets", "cannot inspect %s: %v", path, err)
		}
		if bytes > dir.budget {
			problems = append(problems, fmt.Sprintf(
				"%s uses %s, over the %s budget",
				dir.rel, humanBytes(bytes), dir.label))
		}
		parts = append(parts, fmt.Sprintf("%s %s",
			strings.TrimPrefix(dir.rel, "backups/"), humanBytes(bytes)))
	}
	if len(problems) > 0 {
		return warnCheck("budgets", "%s", strings.Join(problems, "; "))
	}
	return okCheck("budgets", "%s", strings.Join(parts, ", "))
}

func directorySizeOptional(root string) (int64, error) {
	bytes, err := directorySize(root)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return bytes, err
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > 0 && total > math.MaxInt64-info.Size() {
			return fmt.Errorf("directory size overflows int64")
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}
