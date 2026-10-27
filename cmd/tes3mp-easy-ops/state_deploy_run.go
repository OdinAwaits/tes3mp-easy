package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

func deployStateLocked(req stateDeployRequest,
	client *systemd.Client,
) (stateDeployResult, *ops.Error) {
	if req.root == "" {
		return stateDeployResult{}, ops.NewError(ops.ExitInternal,
			"state deploy: root is empty")
	}
	now := req.now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	selection, selectionErr := resolveStateDeployTarget(req)
	if selectionErr != nil {
		return stateDeployResult{}, selectionErr
	}

	stateDir := filepath.Join(req.root, "backups", "state")
	stateEstimate, err := estimateStateBackupSize(stateDir)
	if err != nil {
		return stateDeployResult{}, stateDeployError("estimate state backup", err)
	}
	if spaceErr := checkDeploySpace(
		"state deploy", stateDir, selection.uncompressed+stateEstimate); spaceErr != nil {
		return stateDeployResult{}, spaceErr
	}

	predeploy, backupErr := createStateBackupLocked(stateBackupOptions{
		root:    req.root,
		kind:    archive.KindStatePredeploy,
		now:     now,
		dryRun:  req.dryRun,
		protect: []string{selection.name},
	})
	if backupErr != nil {
		return stateDeployResult{}, backupErr
	}
	result := stateDeployResult{
		archive:   selection.name,
		mods:      selection.modsArchive,
		predeploy: predeploy.archive,
		warnings:  append([]string{}, predeploy.warnings...),
	}
	if req.dryRun {
		return result, nil
	}

	tx, err := newStateDeployTx(req.root, now)
	if err != nil {
		return stateDeployResult{}, stateDeployError("prepare transaction", err)
	}

	wasActive, err := deployIsActive(client, req.timeout)
	if err != nil {
		return stateDeployResult{}, ops.NewError(ops.ExitServiceUnavailable,
			fmt.Sprintf("state deploy: check %s: %v",
				systemd.UnitTES3MPService, err))
	}
	if wasActive {
		if err := deployControl(client, systemd.ActionStop, req.timeout); err != nil {
			return stateDeployResult{}, ops.NewError(ops.ExitServiceUnavailable,
				fmt.Sprintf("state deploy: stop %s: %v",
					systemd.UnitTES3MPService, err))
		}
	}

	fail := func(original *ops.Error) *ops.Error {
		var problems []string
		if rollbackErr := tx.rollback(); rollbackErr != nil {
			problems = append(problems,
				fmt.Sprintf("rollback files: %v", rollbackErr))
		}
		if wasActive && !req.noRestart {
			if err := deployControl(client, systemd.ActionStart,
				req.timeout); err != nil {
				problems = append(problems,
					fmt.Sprintf("restart %s: %v",
						systemd.UnitTES3MPService, err))
			}
		}
		if len(problems) > 0 {
			original.Message += "; " + strings.Join(problems, "; ")
		}
		return original
	}

	if err := tx.stage(selection.path); err != nil {
		return stateDeployResult{}, fail(stateDeployError("extract archive", err))
	}
	if err := tx.switchDirs(); err != nil {
		return stateDeployResult{}, fail(stateDeployError("install state", err))
	}

	if wasActive && !req.noRestart {
		if err := deployControl(client, systemd.ActionStart, req.timeout); err != nil {
			return stateDeployResult{}, fail(ops.NewError(
				ops.ExitServiceUnavailable,
				fmt.Sprintf("state deploy: start %s: %v",
					systemd.UnitTES3MPService, err)))
		}
		result.restarted = true
	}

	result.warnings = append(result.warnings, tx.commit()...)
	return result, nil
}
