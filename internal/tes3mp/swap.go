package tes3mp

import (
	"errors"
	"fmt"
	"os"
)

// swapInstallation moves staging to dir and preserves the previous directory
// as dir.old until the new one is in place. A failed rename restores the old
// directory. The returned warnings describe cleanup problems that do not
// invalidate the successful installation.
func swapInstallation(staging, dir string) ([]string, error) {
	backup := dir + ".old"
	info, err := os.Lstat(dir)
	hasOld := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect existing TES3MP directory: %w", err)
	}
	if !hasOld {
		// Recover from a crash after the old directory was moved aside but
		// before staging was activated. Never follow a planted backup link.
		if backupInfo, backupErr := os.Lstat(backup); backupErr == nil {
			if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.IsDir() {
				return nil, fmt.Errorf("TES3MP backup path %q is not a directory",
					backup)
			}
			if err := os.Rename(backup, dir); err != nil {
				return nil, fmt.Errorf("restore previous TES3MP installation: %w",
					err)
			}
			info, err = os.Lstat(dir)
			hasOld = err == nil
		}
	}
	if hasOld {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("TES3MP path %q is not a directory", dir)
		}
		if err := os.RemoveAll(backup); err != nil {
			return nil, fmt.Errorf("remove stale TES3MP backup: %w", err)
		}
		if err := os.Rename(dir, backup); err != nil {
			return nil, fmt.Errorf("move old TES3MP installation aside: %w", err)
		}
	}

	if err := os.Rename(staging, dir); err != nil {
		var restoreErr error
		if hasOld {
			restoreErr = os.Rename(backup, dir)
		}
		return nil, errors.Join(
			fmt.Errorf("activate new TES3MP installation: %w", err),
			wrapRestore(restoreErr),
		)
	}

	var warnings []string
	if hasOld {
		if err := os.RemoveAll(backup); err != nil {
			warnings = append(warnings,
				fmt.Sprintf("cannot remove old TES3MP backup %s: %v", backup, err))
		}
	}
	return warnings, nil
}

func wrapRestore(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("restore previous TES3MP installation: %w", err)
}
