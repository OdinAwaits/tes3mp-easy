package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type snapshotRestoreData struct {
	Archive             string  `json:"archive"`
	ModsArchive         *string `json:"mods_archive"`
	BaseSwitched        bool    `json:"base_switched"`
	PredeployCheckpoint string  `json:"predeploy_checkpoint"`
	Restarted           bool    `json:"restarted"`
}

func runSnapshotRestore(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	const command = "snapshot restore"
	restoreOpts, parseErr := parseRestoreArgs(command, args, false)
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

	target, targetErr := resolveSnapshotTarget(opts.root, restoreOpts)
	if targetErr != nil {
		return writeError(stdout, stderr, opts.json, targetErr)
	}
	now := time.Now().UTC()
	if _, spaceErr := checkRestorePointSpace(command, target.path); spaceErr != nil {
		return writeError(stdout, stderr, opts.json, spaceErr)
	}
	data := snapshotRestoreData{Archive: target.name}
	if opts.dryRun {
		predeploy, planErr := createCheckpointLocked(opts.root, now,
			"predeploy", "", true)
		if planErr != nil {
			return writeError(stdout, stderr, opts.json, planErr)
		}
		data.PredeployCheckpoint = predeploy.name
		return writeSnapshotRestore(opts, data, stdout, stderr)
	}

	staging, stagingErr := restoreStagingDir(opts.root, "snapshot")
	if stagingErr != nil {
		return writeError(stdout, stderr, opts.json, stagingErr)
	}
	defer os.RemoveAll(staging)

	if _, err := restorepoint.Extract(target.path, staging); err != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError(command+": extract", err))
	}
	wasActive, activeErr := deployIsActive(client, opts.commandTimeout)
	if activeErr != nil {
		return writeError(stdout, stderr, opts.json,
			ops.NewError(ops.ExitServiceUnavailable,
				fmt.Sprintf("%s: check %s: %v",
					command, systemd.UnitTES3MPService, activeErr)))
	}

	predeploy, preErr := createCheckpointLocked(opts.root, now, "predeploy", "", false)
	if preErr != nil {
		return writeError(stdout, stderr, opts.json, preErr)
	}
	data.PredeployCheckpoint = predeploy.name

	liveDigest, digestErr := restorepoint.LiveModsDigest(opts.root)
	if digestErr != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError(command+": scan mods", digestErr))
	}
	if liveDigest != target.manifest.ModsSHA256 {
		modsName, warnings, modsErr := installModsFromDir(
			opts.root, filepath.Join(staging, "mods"), now,
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
		data.BaseSwitched = true
		data.ModsArchive = &modsName
		for _, warning := range warnings {
			fmt.Fprintf(stderr, "tes3mp-easy-ops: %s: %s\n", command, warning)
		}
	}

	if wasActive && !data.BaseSwitched {
		if err := deployControl(client, systemd.ActionStop,
			opts.commandTimeout); err != nil {
			return writeError(stdout, stderr, opts.json,
				ops.NewError(ops.ExitServiceUnavailable,
					fmt.Sprintf("%s: stop %s: %v",
						command, systemd.UnitTES3MPService, err)))
		}
	}

	configRollback, configErr := restoreSnapshotConfigs(
		opts.root, filepath.Join(staging, "configs"), now)
	if configErr != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError(command+": restore configs", configErr))
	}

	tx, err := newStateDeployTx(opts.root, now)
	if err != nil {
		_ = configRollback()
		return writeError(stdout, stderr, opts.json,
			restorePointError(command+": prepare transaction", err))
	}
	fail := func(cause *ops.Error) int {
		if rollbackErr := tx.rollback(); rollbackErr != nil {
			cause.Message += fmt.Sprintf("; rollback files: %v", rollbackErr)
		}
		if rollbackErr := configRollback(); rollbackErr != nil {
			cause.Message += fmt.Sprintf("; rollback configs: %v", rollbackErr)
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
	if err := tx.stageDir(filepath.Join(staging, "state")); err != nil {
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
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "tes3mp-easy-ops: %s: %s\n", command, warning)
	}
	return writeSnapshotRestore(opts, data, stdout, stderr)
}

func restoreSnapshotConfigs(root, sourceDir string, now time.Time,
) (func() error, error) {
	names := []string{
		"config.lua", "tes3mp-server-default.cfg",
		"banlist.json", "customScripts.lua",
	}
	type oldFile struct {
		path    string
		data    []byte
		existed bool
	}
	old := make([]oldFile, 0, len(names))
	rollback := func() error {
		var errs []error
		for _, file := range old {
			if file.existed {
				errs = append(errs, atomicfile.WriteFile(file.path, file.data, 0o640))
			} else {
				err := os.Remove(file.path)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					errs = append(errs, err)
				}
			}
		}
		return errors.Join(errs...)
	}

	stamp := now.UTC().Format("2006-01-02_15-04-05")
	for _, name := range names {
		target := filepath.Join(root, name)
		data, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			_ = rollback()
			return nil, err
		}
		previous, readErr := os.ReadFile(target)
		existed := readErr == nil
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			_ = rollback()
			return nil, readErr
		}
		if existed {
			backup := target + ".bak-" + stamp
			for collision := 1; ; collision++ {
				if _, statErr := os.Lstat(backup); errors.Is(statErr, os.ErrNotExist) {
					break
				}
				backup = fmt.Sprintf("%s.bak-%s-%02d", target, stamp, collision)
			}
			if err := atomicfile.WriteFile(backup, previous, 0o640); err != nil {
				_ = rollback()
				return nil, err
			}
		}
		old = append(old, oldFile{path: target, data: previous, existed: existed})
		if err := atomicfile.WriteFile(target, data, 0o640); err != nil {
			_ = rollback()
			return nil, err
		}
	}
	return rollback, nil
}

func writeSnapshotRestore(opts options, data snapshotRestoreData,
	stdout, stderr io.Writer,
) int {
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	mods := "-"
	if data.ModsArchive != nil {
		mods = *data.ModsArchive
	}
	if _, err := fmt.Fprintf(stdout,
		"archive: %s\nmods_archive: %s\nbase_switched: %t\n"+
			"predeploy_checkpoint: %s\nrestarted: %t\n",
		data.Archive, mods, data.BaseSwitched,
		data.PredeployCheckpoint, data.Restarted); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}
