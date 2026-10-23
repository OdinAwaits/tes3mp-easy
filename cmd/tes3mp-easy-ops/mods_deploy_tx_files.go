package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// rollback restores the previous mods directories and metadata. It is
// idempotent and only touches paths that this transaction moved or wrote.
func (tx *modsDeployTx) rollback() error {
	var errs []error
	if tx.pluginsInstalled {
		errs = append(errs, os.RemoveAll(tx.pluginsDir))
	}
	if tx.pluginsMoved {
		errs = append(errs, os.Rename(tx.pluginsOld, tx.pluginsDir))
	}
	if tx.scriptsInstalled {
		errs = append(errs, os.RemoveAll(tx.scriptsDir))
	}
	if tx.scriptsMoved {
		errs = append(errs, os.Rename(tx.scriptsOld, tx.scriptsDir))
	}
	if tx.currentWritten {
		errs = append(errs, restoreOptionalFile(
			tx.currentPath, tx.oldCurrent, tx.currentExisted))
	}
	if tx.customWritten {
		errs = append(errs, restoreOptionalFile(
			tx.customPath, tx.oldCustom, tx.customExisted))
	}
	errs = append(errs, os.RemoveAll(tx.stagingDir))
	errs = append(errs, removePathIfExists(tx.currentPath+".part"))
	errs = append(errs, removePathIfExists(tx.customPath+".part"))
	errs = append(errs, atomicfile.SyncDir(tx.modsDir))
	errs = append(errs, atomicfile.SyncDir(tx.configsDir))
	return errors.Join(ignoreNotExist(errs...)...)
}

func (tx *modsDeployTx) commit() []string {
	var warnings []string
	if tx.pluginsMoved {
		if err := os.RemoveAll(tx.pluginsOld); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"remove %s: %v", tx.pluginsOld, err))
		}
	}
	if tx.scriptsMoved {
		if err := os.RemoveAll(tx.scriptsOld); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"remove %s: %v", tx.scriptsOld, err))
		}
	}
	if err := os.RemoveAll(tx.stagingDir); err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"remove %s: %v", tx.stagingDir, err))
	}
	if err := atomicfile.SyncDir(tx.modsDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("sync mods directory: %v", err))
	}
	if err := atomicfile.SyncDir(tx.stagingParent); err != nil {
		warnings = append(warnings, fmt.Sprintf("sync staging directory: %v", err))
	}
	return warnings
}

func optionalDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("%s: symlink is not allowed", path)
	case !info.IsDir():
		return fmt.Errorf("%s: not a directory", path)
	}
	return nil
}

func dirExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case info.Mode()&os.ModeSymlink != 0:
		return false, fmt.Errorf("%s: symlink is not allowed", path)
	case !info.IsDir():
		return false, fmt.Errorf("%s: not a directory", path)
	}
	return true, nil
}

func captureOptionalFile(path string, limit int64) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	case info.Mode()&os.ModeSymlink != 0:
		return nil, false, fmt.Errorf("%s: symlink is not allowed", path)
	case !info.Mode().IsRegular():
		return nil, false, fmt.Errorf("%s: not a regular file", path)
	case info.Size() > limit:
		return nil, false, fmt.Errorf("%s: file exceeds %d bytes", path, limit)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func restoreOptionalFile(path string, data []byte, existed bool) error {
	if existed {
		return atomicfile.WriteFile(path, data, 0o640)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func allPathsFree(paths ...string) (bool, error) {
	for _, path := range paths {
		_, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return false, err
		default:
			return false, nil
		}
	}
	return true, nil
}

func removePathIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func ignoreNotExist(errs ...error) []error {
	out := make([]error, 0, len(errs))
	for _, err := range errs {
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			continue
		}
		out = append(out, err)
	}
	return out
}
