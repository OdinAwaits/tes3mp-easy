package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

type stateDeployTx struct {
	stagingParent string
	stagingDir    string
	playersDir    string
	worldDir      string
	playersOld    string
	worldOld      string

	playersMoved     bool
	worldMoved       bool
	installedPlayers []string
	installedWorld   []string
}

func newStateDeployTx(root string, at time.Time) (*stateDeployTx, error) {
	rootDir := root
	playersDir := filepath.Join(rootDir, "players")
	worldDir := filepath.Join(rootDir, "world")
	stagingParent := filepath.Join(rootDir, ".staging")

	for _, dir := range []string{rootDir, playersDir, worldDir} {
		exists, err := dirExists(dir)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("%s: directory is missing", dir)
		}
	}
	if err := optionalDir(stagingParent); err != nil {
		return nil, err
	}

	tx := &stateDeployTx{
		stagingParent: stagingParent,
		playersDir:    playersDir,
		worldDir:      worldDir,
	}
	id := at.UTC().Format("2006-01-02_15-04-05")
	for collision := 0; collision <= 99; collision++ {
		candidate := id
		if collision > 0 {
			candidate = fmt.Sprintf("%s-%02d", id, collision)
		}
		tx.stagingDir = filepath.Join(stagingParent, "state-"+candidate)
		tx.playersOld = filepath.Join(tx.stagingDir, "old", "players")
		tx.worldOld = filepath.Join(tx.stagingDir, "old", "world")
		free, err := allPathsFree(tx.stagingDir)
		if err != nil {
			return nil, err
		}
		if free {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("no free state deploy transaction id for %s", id)
}

func (tx *stateDeployTx) stage(archivePath string) error {
	if err := os.MkdirAll(tx.stagingParent, 0o700); err != nil {
		return err
	}
	if err := optionalDir(tx.stagingParent); err != nil {
		return err
	}
	if err := os.Mkdir(tx.stagingDir, 0o700); err != nil {
		return err
	}
	if err := stateDeployExtractFunc(archivePath, tx.stagingDir); err != nil {
		return err
	}
	for _, dir := range []string{tx.playersOld, tx.worldOld} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := atomicfile.SyncDir(dir); err != nil {
			return err
		}
	}
	for _, dir := range []string{
		filepath.Join(tx.stagingDir, "players"),
		filepath.Join(tx.stagingDir, "world"),
		filepath.Join(tx.stagingDir, "old"),
		tx.stagingDir,
	} {
		if err := atomicfile.SyncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (tx *stateDeployTx) switchDirs() error {
	// The data root is root:root 0755, and ops runs as tes3mp. Renaming the
	// players/ and world/ directories themselves would need write permission on
	// the root. Move the directory contents instead: both the live dirs and the
	// staging transaction are writable by tes3mp.
	tx.playersMoved = true
	if _, err := moveDirEntries(tx.playersDir, tx.playersOld); err != nil {
		return fmt.Errorf("move old players: %w", err)
	}
	tx.worldMoved = true
	if _, err := moveDirEntries(tx.worldDir, tx.worldOld); err != nil {
		return fmt.Errorf("move old world: %w", err)
	}

	installed, err := moveDirEntries(
		filepath.Join(tx.stagingDir, "players"), tx.playersDir)
	tx.installedPlayers = installed
	if err != nil {
		return fmt.Errorf("install players: %w", err)
	}
	installed, err = moveDirEntries(
		filepath.Join(tx.stagingDir, "world"), tx.worldDir)
	tx.installedWorld = installed
	if err != nil {
		return fmt.Errorf("install world: %w", err)
	}

	return nil
}

// moveDirEntries renames every top-level entry from src to dst and returns the
// names that were moved. A partial failure returns the names moved before the
// error so rollback can restore them.
func moveDirEntries(src, dst string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	moved := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if err := stateDeployRenameFunc(
			filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
			return moved, fmt.Errorf("rename %s: %w", name, err)
		}
		moved = append(moved, name)
	}
	if err := atomicfile.SyncDir(src); err != nil {
		return moved, err
	}
	if err := atomicfile.SyncDir(dst); err != nil {
		return moved, err
	}
	return moved, nil
}
