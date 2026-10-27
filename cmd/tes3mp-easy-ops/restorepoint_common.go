package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
	"github.com/Saagilat/tes3mp-easy/internal/retention"
	"github.com/Saagilat/tes3mp-easy/internal/version"
)

type snapshotCreateResult struct {
	name     string
	size     int64
	sha256   string
	manifest restorepoint.Manifest
	mods     string
	warnings []string
}

type checkpointCreateResult struct {
	name     string
	size     int64
	sha256   string
	manifest restorepoint.Manifest
	baseName string
	baseSHA  string
	warnings []string
}

func restorePointError(command string, err error) *ops.Error {
	exit := ops.ExitInternal
	switch {
	case errors.Is(err, syscall.ENOSPC):
		exit = ops.ExitNoSpace
	case errors.Is(err, restorepoint.ErrLimit), errors.Is(err, restorepoint.ErrInvalid),
		errors.Is(err, archive.ErrInvalid), errors.Is(err, archive.ErrLimit):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, pathguard.ErrUnsafePath):
		exit = ops.ExitUnsafePath
	}
	return ops.NewError(exit, fmt.Sprintf("%s: %v", command, err))
}

func checkRestorePointSpace(command, path string) ([]string, *ops.Error) {
	free, err := backupDiskFree(path)
	if err != nil {
		return nil, ops.NewError(ops.ExitInternal,
			fmt.Sprintf("%s: free space: %v", command, err))
	}
	switch {
	case free < retention.DefaultMinFree:
		return nil, ops.NewError(ops.ExitNoSpace,
			fmt.Sprintf("%s: free space %d is below %d bytes",
				command, free, retention.DefaultMinFree))
	case free < backupWarnFree:
		return []string{fmt.Sprintf(
			"free space %d is below %d bytes", free, backupWarnFree)}, nil
	default:
		return nil, nil
	}
}

func ensureRestorePointDir(root string, kind restorepoint.Kind, dryRun bool) (string, *ops.Error) {
	dir := restorepoint.Dir(root, kind)
	if dryRun {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", ops.NewError(ops.ExitInternal,
			fmt.Sprintf("create %s: %v", dir, err))
	}
	return dir, nil
}

// createSnapshotLocked writes a snapshot without taking the ops lock. It is
// shared by `snapshot create`, `checkpoint create` (lazy base) and restores.
func createSnapshotLocked(root string, now time.Time, reason, label string,
	dryRun bool,
) (snapshotCreateResult, *ops.Error) {
	const command = "snapshot create"
	dir, dirErr := ensureRestorePointDir(root, restorepoint.KindSnapshot, dryRun)
	if dirErr != nil {
		return snapshotCreateResult{}, dirErr
	}
	spacePath := dir
	if dryRun {
		spacePath = root
	}
	warnings, spaceErr := checkRestorePointSpace(command, spacePath)
	if spaceErr != nil {
		return snapshotCreateResult{}, spaceErr
	}

	name, err := restorepoint.NextName(dir, restorepoint.KindSnapshot, now)
	if err != nil {
		return snapshotCreateResult{}, restorePointError(command, err)
	}
	target := filepath.Join(dir, name)
	counter := &countingWriter{}

	var destination io.Writer = io.Discard
	var writer *atomicfile.Writer
	if !dryRun {
		writer, err = atomicfile.New(target, 0o640)
		if err != nil {
			return snapshotCreateResult{}, restorePointError(
				fmt.Sprintf("%s: open %s", command, target), err)
		}
		destination = writer
	}

	manifest, writeErr := restorepoint.WriteSnapshot(
		io.MultiWriter(destination, counter), restorepoint.SnapshotOptions{
			Root:         root,
			Now:          now,
			Reason:       reason,
			Label:        label,
			PanelVersion: version.Version,
		})
	if writeErr != nil {
		if writer != nil {
			writeErr = errors.Join(writeErr, writer.Abort())
		}
		return snapshotCreateResult{}, restorePointError(
			fmt.Sprintf("%s: write archive", command), writeErr)
	}
	limit := restorepoint.Limits(restorepoint.KindSnapshot)
	if counter.n > limit.CompressedSize {
		err := fmt.Errorf("%w: compressed snapshot exceeds %d bytes",
			restorepoint.ErrLimit, limit.CompressedSize)
		if writer != nil {
			err = errors.Join(err, writer.Abort())
		}
		return snapshotCreateResult{}, restorePointError(
			fmt.Sprintf("%s: write archive", command), err)
	}
	if writer != nil {
		if err := writer.Commit(); err != nil {
			return snapshotCreateResult{}, restorePointError(
				fmt.Sprintf("%s: commit %s", command, target), err)
		}
	}

	result := snapshotCreateResult{
		name:     name,
		size:     counter.n,
		manifest: manifest,
		warnings: append([]string{}, warnings...),
	}
	if !dryRun {
		sum, err := backupSHA256(target)
		if err != nil {
			return snapshotCreateResult{}, restorePointError(
				fmt.Sprintf("%s: sha256 %s", command, target), err)
		}
		result.sha256 = sum
	}
	if current, err := archive.ReadCurrentFile(
		filepath.Join(root, "backups", "mods", backupsCurrentFile)); err == nil {
		result.mods = current.Name
	} else {
		result.warnings = append(result.warnings,
			fmt.Sprintf("mods archive: %v", err))
	}
	return result, nil
}
