package mods

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

func recoverInstallation(targetDF, targetProfile string) error {
	if err := ensureDirectory(filepath.Dir(targetDF)); err != nil {
		return err
	}
	if err := recoverTarget(targetDF+".old", targetDF, true); err != nil {
		return err
	}
	return recoverTarget(targetProfile+".old", targetProfile, false)
}

func recoverTarget(backup, target string, wantDir bool) error {
	targetInfo, targetErr := os.Lstat(target)
	backupInfo, backupErr := os.Lstat(backup)
	switch {
	case errors.Is(backupErr, os.ErrNotExist):
	case backupErr != nil:
		return fmt.Errorf("inspect mods backup %s: %w", backup, backupErr)
	case backupInfo.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("mods backup %s is a symlink", backup)
	case wantDir && !backupInfo.IsDir():
		return fmt.Errorf("mods backup %s is not a directory", backup)
	case !wantDir && !backupInfo.Mode().IsRegular():
		return fmt.Errorf("mods backup %s is not a regular file", backup)
	}

	if errors.Is(targetErr, os.ErrNotExist) {
		if backupErr == nil {
			if err := os.Rename(backup, target); err != nil {
				return fmt.Errorf("restore previous mods file %s: %w", target, err)
			}
			targetErr = nil
			targetInfo, targetErr = os.Lstat(target)
		}
	} else if targetErr != nil {
		return fmt.Errorf("inspect mods target %s: %w", target, targetErr)
	} else if backupErr == nil {
		// A crash after activation left a complete new target and an old backup.
		if err := removeOwnedTree(backup); err != nil {
			return err
		}
	}

	if targetErr == nil {
		switch {
		case targetInfo.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("mods target %s is a symlink", target)
		case wantDir && !targetInfo.IsDir():
			return fmt.Errorf("mods target %s is not a directory", target)
		case !wantDir && !targetInfo.Mode().IsRegular():
			return fmt.Errorf("mods target %s is not a regular file", target)
		}
	}
	return nil
}

// swapInstallation activates the staged Data Files and openmw.cfg. It moves the
// previous versions aside, restores them on any failure and writes the digest
// last, so a crash never leaves a new tree marked as installed with an old
// digest.
func swapInstallation(newDF, newProfile, targetDF, targetProfile,
	digestPath, digest string,
) ([]string, error) {
	backupDF := targetDF + ".old"
	backupProfile := targetProfile + ".old"
	var (
		movedDF      bool
		movedProfile bool
		activeDF     bool
		activeProf   bool
	)

	rollback := func() error {
		var errs []error
		if activeProf {
			if err := removeOwnedTree(targetProfile); err != nil {
				errs = append(errs, err)
			}
			activeProf = false
		}
		if movedProfile {
			if err := os.Rename(backupProfile, targetProfile); err != nil {
				errs = append(errs,
					fmt.Errorf("restore previous openmw.cfg: %w", err))
			}
			movedProfile = false
		}
		if activeDF {
			if err := removeOwnedTree(targetDF); err != nil {
				errs = append(errs, err)
			}
			activeDF = false
		}
		if movedDF {
			if err := os.Rename(backupDF, targetDF); err != nil {
				errs = append(errs,
					fmt.Errorf("restore previous Data Files: %w", err))
			}
			movedDF = false
		}
		return errors.Join(errs...)
	}

	if err := ensureDirectory(filepath.Dir(targetDF)); err != nil {
		return nil, err
	}
	if err := ensureDirectory(filepath.Dir(targetProfile)); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(targetDF); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("mods target %s is not a directory", targetDF)
		}
		if err := os.Rename(targetDF, backupDF); err != nil {
			return nil, fmt.Errorf("move old Data Files aside: %w", err)
		}
		movedDF = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect Data Files target: %w", err)
	}
	if info, err := os.Lstat(targetProfile); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("mods target %s is not a regular file",
				targetProfile)
		}
		if err := os.Rename(targetProfile, backupProfile); err != nil {
			return nil, errors.Join(
				fmt.Errorf("move old openmw.cfg aside: %w", err), rollback())
		}
		movedProfile = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(
			fmt.Errorf("inspect openmw.cfg target: %w", err), rollback())
	}

	if err := os.Rename(newDF, targetDF); err != nil {
		return nil, errors.Join(
			fmt.Errorf("activate new Data Files: %w", err), rollback())
	}
	activeDF = true
	if err := os.Rename(newProfile, targetProfile); err != nil {
		return nil, errors.Join(
			fmt.Errorf("activate new openmw.cfg: %w", err), rollback())
	}
	activeProf = true
	if err := atomicfile.WriteFile(digestPath, []byte(digest+"\n"), 0o600); err != nil {
		return nil, errors.Join(
			fmt.Errorf("write mods digest: %w", err), rollback())
	}

	var warnings []string
	if movedDF {
		if err := removeOwnedTree(backupDF); err != nil {
			warnings = append(warnings,
				fmt.Sprintf("cannot remove old Data Files %s: %v", backupDF, err))
		}
	}
	if movedProfile {
		if err := removeOwnedTree(backupProfile); err != nil {
			warnings = append(warnings,
				fmt.Sprintf("cannot remove old openmw.cfg %s: %v",
					backupProfile, err))
		}
	}
	return warnings, nil
}
