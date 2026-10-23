package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

type stateDeployTx struct {
	rootDir       string
	stagingParent string
	stagingDir    string
	playersDir    string
	worldDir      string
	playersOld    string
	worldOld      string

	playersMoved     bool
	worldMoved       bool
	playersInstalled bool
	worldInstalled   bool
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
		rootDir:       rootDir,
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
		tx.playersOld = playersDir + ".old-" + candidate
		tx.worldOld = worldDir + ".old-" + candidate
		free, err := allPathsFree(tx.stagingDir, tx.playersOld, tx.worldOld)
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
	for _, dir := range []string{
		filepath.Join(tx.stagingDir, "players"),
		filepath.Join(tx.stagingDir, "world"),
		tx.stagingDir,
	} {
		if err := atomicfile.SyncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (tx *stateDeployTx) switchDirs() error {
	if err := stateDeployRenameFunc(tx.playersDir, tx.playersOld); err != nil {
		return fmt.Errorf("move old players: %w", err)
	}
	tx.playersMoved = true
	if err := stateDeployRenameFunc(tx.worldDir, tx.worldOld); err != nil {
		return fmt.Errorf("move old world: %w", err)
	}
	tx.worldMoved = true

	if err := stateDeployRenameFunc(
		filepath.Join(tx.stagingDir, "players"), tx.playersDir); err != nil {
		return fmt.Errorf("install players: %w", err)
	}
	tx.playersInstalled = true
	if err := stateDeployRenameFunc(
		filepath.Join(tx.stagingDir, "world"), tx.worldDir); err != nil {
		return fmt.Errorf("install world: %w", err)
	}
	tx.worldInstalled = true

	if err := atomicfile.SyncDir(tx.rootDir); err != nil {
		return fmt.Errorf("sync root directory: %w", err)
	}
	return nil
}
