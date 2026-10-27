package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

type stateDeployRequest struct {
	root      string
	archive   string
	latest    bool
	noRestart bool
	now       time.Time
	dryRun    bool
	timeout   time.Duration
}

type stateDeployResult struct {
	archive   string
	mods      string
	predeploy string
	restarted bool
	warnings  []string
}

// These hooks keep state deploy tests independent of the wall clock, a small
// test filesystem and extraction failure injection.
var (
	stateDeployNowFunc     = func() time.Time { return time.Now().UTC() }
	stateDeployExtractFunc = archive.ExtractStateFile
	stateDeployRenameFunc  = os.Rename
)

type stateDeploySelection struct {
	name         string
	path         string
	modsArchive  string
	uncompressed int64
}

func resolveStateDeployTarget(req stateDeployRequest) (stateDeploySelection, *ops.Error) {
	stateDir := filepath.Join(req.root, "backups", "state")
	modsDir := filepath.Join(req.root, "backups", "mods")

	name := req.archive
	switch {
	case req.archive != "":
		if err := pathguard.Base(req.archive); err != nil {
			return stateDeploySelection{}, ops.NewError(ops.ExitUnsafePath,
				fmt.Sprintf("state deploy: archive %q: %v", req.archive, err))
		}
		if err := archive.ValidateStateArchiveName(req.archive); err != nil {
			return stateDeploySelection{}, stateDeployError(
				"validate archive name", err)
		}
	case req.latest:
		latest, err := latestStateArchive(stateDir)
		if err != nil {
			return stateDeploySelection{}, err
		}
		name = latest
	}

	target := filepath.Join(stateDir, name)
	info, size, err := inspectStateDeployArchive(target, modsDir)
	if err != nil {
		return stateDeploySelection{}, err
	}
	return stateDeploySelection{
		name:         name,
		path:         target,
		modsArchive:  info.Current.Name,
		uncompressed: size,
	}, nil
}

func inspectStateDeployArchive(name, modsDir string) (archive.StateInfo, int64, *ops.Error) {
	info, err := os.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return archive.StateInfo{}, 0, ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("state deploy: archive %s does not exist", name))
	case err != nil:
		return archive.StateInfo{}, 0, stateDeployError("stat archive", err)
	case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
		return archive.StateInfo{}, 0, ops.NewError(ops.ExitArchiveInvalid,
			fmt.Sprintf("state deploy: %s is not a regular archive file", name))
	case info.Size() > archive.MaxCompressedSize:
		return archive.StateInfo{}, 0, ops.NewError(ops.ExitArchiveInvalid,
			fmt.Sprintf("state deploy: %s exceeds %d bytes",
				name, archive.MaxCompressedSize))
	}

	state, uncompressed, err := archive.ValidateStateFileWithSize(name, modsDir)
	if err != nil {
		return archive.StateInfo{}, 0, stateDeployError("validate archive", err)
	}
	return state, uncompressed, nil
}

func latestStateArchive(stateDir string) (string, *ops.Error) {
	entries, err := os.ReadDir(stateDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", ops.NewError(ops.ExitNotFound,
			fmt.Sprintf("state deploy: %s does not exist", stateDir))
	case err != nil:
		return "", stateDeployError("read state directory", err)
	}

	type candidate struct {
		name   string
		parsed archive.ArchiveName
	}
	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		parsed, err := archive.ParseArchiveName(name)
		if err != nil ||
			(parsed.Kind != archive.KindStateAuto &&
				parsed.Kind != archive.KindStateManual) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", stateDeployError("stat archive "+name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, candidate{name: name, parsed: parsed})
	}
	if len(candidates) == 0 {
		return "", ops.NewError(ops.ExitNotFound,
			"state deploy: no state archive is available for --latest")
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i].parsed, candidates[j].parsed
		if !left.Time.Equal(right.Time) {
			return left.Time.After(right.Time)
		}
		if left.Collision != right.Collision {
			return left.Collision > right.Collision
		}
		return candidates[i].name > candidates[j].name
	})
	return candidates[0].name, nil
}

func stateDeployError(operation string, err error) *ops.Error {
	exit := ops.ExitInternal
	switch {
	case errors.Is(err, archive.ErrInvalid), errors.Is(err, archive.ErrLimit):
		exit = ops.ExitArchiveInvalid
	case errors.Is(err, pathguard.ErrUnsafePath):
		exit = ops.ExitUnsafePath
	case errors.Is(err, syscall.ENOSPC):
		exit = ops.ExitNoSpace
	case errors.Is(err, fs.ErrNotExist):
		exit = ops.ExitNotFound
	}
	return ops.NewError(exit,
		fmt.Sprintf("state deploy: %s: %v", operation, err))
}
