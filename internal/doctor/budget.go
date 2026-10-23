package doctor

import (
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/retention"
)

func checkBudgets(root string) Check {
	stateDir := filepath.Join(root, "backups", "state")
	modsDir := filepath.Join(root, "backups", "mods")

	stateBytes, err := directorySize(stateDir)
	if err != nil {
		return errorCheck("budgets", "cannot inspect %s: %v", stateDir, err)
	}
	modsBytes, err := directorySize(modsDir)
	if err != nil {
		return errorCheck("budgets", "cannot inspect %s: %v", modsDir, err)
	}

	problems := make([]string, 0, 2)
	if stateBytes > retention.DefaultStateBudget {
		problems = append(problems, fmt.Sprintf(
			"backups/state uses %s, over the 1.0 GiB budget",
			humanBytes(stateBytes)))
	}
	if modsBytes > retention.DefaultModsBudget {
		problems = append(problems, fmt.Sprintf(
			"backups/mods uses %s, over the 512 MiB budget",
			humanBytes(modsBytes)))
	}
	if len(problems) > 0 {
		return warnCheck("budgets", "%s", strings.Join(problems, "; "))
	}
	return okCheck("budgets", "state %s, mods %s",
		humanBytes(stateBytes), humanBytes(modsBytes))
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
