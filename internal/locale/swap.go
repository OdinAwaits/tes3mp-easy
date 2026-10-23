package locale

import (
	"errors"
	"fmt"
	"os"
)

// swapInstallation activates staging as dir and keeps the previous overlay in
// dir.old until the rename succeeds. A failure restores the previous overlay.
func swapInstallation(staging, dir string) ([]string, error) {
	backup := dir + ".old"
	info, err := os.Lstat(dir)
	hasOld := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect existing localization: %w", err)
	}
	if !hasOld {
		if backupInfo, backupErr := os.Lstat(backup); backupErr == nil {
			if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.IsDir() {
				return nil, fmt.Errorf(
					"localization backup path %q is not a directory", backup)
			}
			if err := os.Rename(backup, dir); err != nil {
				return nil, fmt.Errorf("restore previous localization: %w", err)
			}
			info, err = os.Lstat(dir)
			hasOld = err == nil
		}
	}
	if hasOld {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("localization path %q is not a directory", dir)
		}
		if err := os.RemoveAll(backup); err != nil {
			return nil, fmt.Errorf("remove stale localization backup: %w", err)
		}
		if err := os.Rename(dir, backup); err != nil {
			return nil, fmt.Errorf("move old localization aside: %w", err)
		}
	}
	if err := os.Rename(staging, dir); err != nil {
		var restoreErr error
		if hasOld {
			restoreErr = os.Rename(backup, dir)
		}
		return nil, errors.Join(
			fmt.Errorf("activate new localization: %w", err),
			wrapRestore(restoreErr),
		)
	}
	var warnings []string
	if hasOld {
		if err := os.RemoveAll(backup); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"cannot remove old localization backup %s: %v", backup, err))
		}
	}
	return warnings, nil
}

func wrapRestore(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("restore previous localization: %w", err)
}
