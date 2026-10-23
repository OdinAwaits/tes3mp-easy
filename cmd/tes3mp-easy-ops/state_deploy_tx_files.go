package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// rollback restores the previous players/ and world/ directories. It is
// idempotent and only touches paths that this transaction moved or installed.
func (tx *stateDeployTx) rollback() error {
	var errs []error
	if tx.playersInstalled {
		errs = append(errs, os.RemoveAll(tx.playersDir))
	}
	if tx.playersMoved {
		errs = append(errs, stateDeployRenameFunc(tx.playersOld, tx.playersDir))
	}
	if tx.worldInstalled {
		errs = append(errs, os.RemoveAll(tx.worldDir))
	}
	if tx.worldMoved {
		errs = append(errs, stateDeployRenameFunc(tx.worldOld, tx.worldDir))
	}
	errs = append(errs, os.RemoveAll(tx.stagingDir))
	errs = append(errs, atomicfile.SyncDir(tx.rootDir))
	errs = append(errs, atomicfile.SyncDir(tx.stagingParent))
	return errors.Join(ignoreNotExist(errs...)...)
}

func (tx *stateDeployTx) commit() []string {
	var warnings []string
	if tx.playersMoved {
		if err := os.RemoveAll(tx.playersOld); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"remove %s: %v", tx.playersOld, err))
		}
	}
	if tx.worldMoved {
		if err := os.RemoveAll(tx.worldOld); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"remove %s: %v", tx.worldOld, err))
		}
	}
	if err := os.RemoveAll(tx.stagingDir); err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"remove %s: %v", tx.stagingDir, err))
	}
	if err := atomicfile.SyncDir(tx.rootDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("sync root directory: %v", err))
	}
	if err := atomicfile.SyncDir(tx.stagingParent); err != nil {
		warnings = append(warnings, fmt.Sprintf("sync staging directory: %v", err))
	}
	return warnings
}
