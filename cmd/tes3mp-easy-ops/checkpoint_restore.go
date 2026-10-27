package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type checkpointRestoreData struct {
	Archive             string              `json:"archive"`
	Base                *checkpointBaseData `json:"base"`
	BaseDeployed        bool                `json:"base_deployed"`
	BaseMissing         bool                `json:"base_missing"`
	ModsArchive         *string             `json:"mods_archive"`
	MissingPlugins      []string            `json:"missing_plugins"`
	PredeployCheckpoint string              `json:"predeploy_checkpoint"`
	Restarted           bool                `json:"restarted"`
}

func runCheckpointRestore(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	const command = "checkpoint restore"
	restoreOpts, parseErr := parseRestoreArgs(command, args, true)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	mode := lock.Exclusive
	if opts.dryRun {
		mode = lock.Shared
	}
	held, lockErr := acquireOpsLock(opts.root, command, mode,
		opts.lockTimeout, opts.dryRun)
	if lockErr != nil {
		return writeError(stdout, stderr, opts.json, lockErr)
	}
	if held != nil {
		defer func() { _ = held.Close() }()
	}

	target, targetErr := resolveCheckpointTarget(opts.root, restoreOpts)
	if targetErr != nil {
		return writeError(stdout, stderr, opts.json, targetErr)
	}
	baseName := target.manifest.Base.Name
	basePath := filepath.Join(restorepoint.Dir(opts.root, restorepoint.KindSnapshot),
		baseName)
	baseExists, err := regularFileExists(basePath)
	if err != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError(command, err))
	}
	data := checkpointRestoreData{
		Archive:        target.name,
		Base:           &checkpointBaseData{Name: baseName, SHA256: target.manifest.Base.SHA256, ModsSHA256: target.manifest.Base.ModsSHA256},
		BaseMissing:    !baseExists,
		MissingPlugins: []string{},
	}
	if !baseExists {
		data.Base = nil
		if !restoreOpts.acceptMismatch {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitNotFound,
					fmt.Sprintf("%s: base snapshot %s is missing", command, baseName)))
		}
	}

	now := time.Now().UTC()
	spaceWarnings, spaceErr := checkRestorePointSpace(command, target.path)
	if spaceErr != nil {
		return writeError(stdout, stderr, opts.json, spaceErr)
	}
	data.MissingPlugins = append(data.MissingPlugins, spaceWarnings...)
	if opts.dryRun {
		predeploy, planErr := createCheckpointLocked(opts.root, now,
			"predeploy", "", true)
		if planErr != nil {
			return writeError(stdout, stderr, opts.json, planErr)
		}
		data.PredeployCheckpoint = predeploy.name
		return writeCheckpointRestore(opts, data, stdout, stderr)
	}

	staging, stagingErr := restoreStagingDir(opts.root, "checkpoint")
	if stagingErr != nil {
		return writeError(stdout, stderr, opts.json, stagingErr)
	}
	defer os.RemoveAll(staging)

	if _, err := restorepoint.Extract(target.path, staging); err != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError(command+": extract", err))
	}
	if baseExists {
		sum, err := backupSHA256(basePath)
		if err != nil {
			return writeError(stdout, stderr, opts.json,
				restorePointError(command, err))
		}
		if sum != target.manifest.Base.SHA256 {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitArchiveInvalid,
					fmt.Sprintf("%s: base snapshot sha256 mismatch", command)))
		}
	}

	wasActive, activeErr := deployIsActive(client, opts.commandTimeout)
	if activeErr != nil {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitServiceUnavailable,
				fmt.Sprintf("%s: check %s: %v",
					command, systemd.UnitTES3MPService, activeErr)))
	}

	if !baseExists {
		missing, missingErr := missingPlugins(staging,
			filepath.Join(opts.root, "mods"))
		if missingErr != nil {
			return writeError(stdout, stderr, opts.json, missingErr)
		}
		data.MissingPlugins = missing
	} else {
		liveDigest, digestErr := restorepoint.LiveModsDigest(opts.root)
		if digestErr != nil {
			return writeError(stdout, stderr, opts.json,
				restorePointError(command+": scan mods", digestErr))
		}
		if liveDigest != target.manifest.Base.ModsSHA256 {
			baseStaging, baseErr := restoreStagingDir(opts.root, "base")
			if baseErr != nil {
				return writeError(stdout, stderr, opts.json, baseErr)
			}
			defer os.RemoveAll(baseStaging)
			if _, err := restorepoint.Extract(basePath, baseStaging); err != nil {
				return writeError(stdout, stderr, opts.json,
					restorePointError(command+": extract base", err))
			}
			modsName, modsWarnings, modsErr := installModsFromDir(
				opts.root, filepath.Join(baseStaging, "mods"), now,
				opts.commandTimeout, client)
			if modsErr != nil {
				if wasActive && !restoreOpts.noRestart {
					if startErr := deployControl(client, systemd.ActionStart,
						opts.commandTimeout); startErr != nil {
						modsErr.Message += fmt.Sprintf("; restart %s: %v",
							systemd.UnitTES3MPService, startErr)
					}
				}
				return writeError(stdout, stderr, opts.json, modsErr)
			}
			data.BaseDeployed = true
			data.ModsArchive = &modsName
			data.MissingPlugins = append(data.MissingPlugins, modsWarnings...)
		}
	}

	predeploy, preErr := createCheckpointLocked(opts.root, now, "predeploy", "", false)
	if preErr != nil {
		return writeError(stdout, stderr, opts.json, preErr)
	}
	data.PredeployCheckpoint = predeploy.name

	if wasActive && !data.BaseDeployed {
		if err := deployControl(client, systemd.ActionStop,
			opts.commandTimeout); err != nil {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitServiceUnavailable,
					fmt.Sprintf("%s: stop %s: %v",
						command, systemd.UnitTES3MPService, err)))
		}
	}

	tx, err := newStateDeployTx(opts.root, now)
	if err != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError(command+": prepare transaction", err))
	}
	fail := func(cause *ops.Error) int {
		if rollbackErr := tx.rollback(); rollbackErr != nil {
			cause.Message += fmt.Sprintf("; rollback files: %v", rollbackErr)
		}
		if wasActive && !restoreOpts.noRestart {
			if startErr := deployControl(client, systemd.ActionStart,
				opts.commandTimeout); startErr != nil {
				cause.Message += fmt.Sprintf("; restart %s: %v",
					systemd.UnitTES3MPService, startErr)
			}
		}
		return writeError(stdout, stderr, opts.json, cause)
	}
	if err := tx.stageDir(staging); err != nil {
		return fail(restorePointError(command+": stage state", err))
	}
	if err := tx.switchDirs(); err != nil {
		return fail(restorePointError(command+": install state", err))
	}

	if wasActive && !restoreOpts.noRestart {
		if err := deployControl(client, systemd.ActionStart,
			opts.commandTimeout); err != nil {
			return fail(ops.NewError(ops.ExitServiceUnavailable,
				fmt.Sprintf("%s: start %s: %v",
					command, systemd.UnitTES3MPService, err)))
		}
		data.Restarted = true
	}

	warnings := tx.commit()
	data.MissingPlugins = append(data.MissingPlugins, warnings...)
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: %s: %s\n", command, warning)
	}
	return writeCheckpointRestore(opts, data, stdout, stderr)
}

func writeCheckpointRestore(opts options, data checkpointRestoreData,
	stdout, stderr io.Writer,
) int {
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	base := "-"
	if data.Base != nil {
		base = data.Base.Name
	}
	mods := "-"
	if data.ModsArchive != nil {
		mods = *data.ModsArchive
	}
	if _, err := fmt.Fprintf(stdout,
		"archive: %s\nbase: %s\nbase_deployed: %t\nbase_missing: %t\n"+
			"mods_archive: %s\npredeploy_checkpoint: %s\nrestarted: %t\n",
		data.Archive, base, data.BaseDeployed, data.BaseMissing, mods,
		data.PredeployCheckpoint, data.Restarted); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}
