package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// rollback restores the previous players/ and world/ contents. It is
// idempotent and only touches paths that this transaction moved or installed.
func (tx *stateDeployTx) rollback() error {
	var errs []error
	for _, name := range tx.installedPlayers {
		errs = append(errs,
			os.RemoveAll(filepath.Join(tx.playersDir, name)))
	}
	for _, name := range tx.installedWorld {
		errs = append(errs,
			os.RemoveAll(filepath.Join(tx.worldDir, name)))
	}
	if tx.playersMoved {
		_, err := moveDirEntries(tx.playersOld, tx.playersDir)
		errs = append(errs, err)
	}
	if tx.worldMoved {
		_, err := moveDirEntries(tx.worldOld, tx.worldDir)
		errs = append(errs, err)
	}
	errs = append(errs, os.RemoveAll(tx.stagingDir))
	errs = append(errs, atomicfile.SyncDir(tx.playersDir))
	errs = append(errs, atomicfile.SyncDir(tx.worldDir))
	errs = append(errs, atomicfile.SyncDir(tx.stagingParent))
	return errors.Join(ignoreNotExist(errs...)...)
}

func (tx *stateDeployTx) commit() []string {
	var warnings []string
	if err := os.RemoveAll(tx.stagingDir); err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"remove %s: %v", tx.stagingDir, err))
	}
	for _, dir := range []string{
		tx.playersDir,
		tx.worldDir,
		tx.stagingParent,
	} {
		if err := atomicfile.SyncDir(dir); err != nil {
			warnings = append(warnings, fmt.Sprintf("sync %s: %v", dir, err))
		}
	}
	return warnings
}
