package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

const maxCustomScriptsSize int64 = 4 << 20

type modsDeployTx struct {
	modsDir       string
	stagingParent string
	stagingDir    string
	pluginsDir    string
	scriptsDir    string
	pluginsOld    string
	scriptsOld    string
	configsDir    string
	customPath    string
	currentPath   string

	pluginsExisted bool
	scriptsExisted bool
	oldCurrent     []byte
	currentExisted bool
	oldCustom      []byte
	customExisted  bool

	pluginsMoved     bool
	scriptsMoved     bool
	pluginsInstalled bool
	scriptsInstalled bool
	customWritten    bool
	currentWritten   bool
}

func newModsDeployTx(root string, at time.Time) (*modsDeployTx, error) {
	modsDir := filepath.Join(root, "mods")
	pluginsDir := filepath.Join(modsDir, "plugins")
	scriptsDir := filepath.Join(modsDir, "scripts")
	stagingParent := filepath.Join(root, ".staging")
	configsDir := filepath.Join(root, "configs")

	if err := optionalDir(modsDir); err != nil {
		return nil, err
	}
	var err error
	tx := &modsDeployTx{
		modsDir:       modsDir,
		stagingParent: stagingParent,
		pluginsDir:    pluginsDir,
		scriptsDir:    scriptsDir,
		configsDir:    configsDir,
		customPath:    filepath.Join(configsDir, "customScripts.lua"),
		currentPath:   filepath.Join(root, "backups", "mods", backupsCurrentFile),
	}
	if tx.pluginsExisted, err = dirExists(pluginsDir); err != nil {
		return nil, err
	}
	if tx.scriptsExisted, err = dirExists(scriptsDir); err != nil {
		return nil, err
	}
	if err := optionalDir(stagingParent); err != nil {
		return nil, err
	}
	if err := optionalDir(configsDir); err != nil {
		return nil, err
	}
	if tx.oldCurrent, tx.currentExisted, err = captureOptionalFile(
		tx.currentPath, archive.MaxCurrentFileSize); err != nil {
		return nil, err
	}
	if tx.oldCustom, tx.customExisted, err = captureOptionalFile(
		tx.customPath, maxCustomScriptsSize); err != nil {
		return nil, err
	}

	id := at.UTC().Format("2006-01-02_15-04-05")
	for collision := 0; collision <= 99; collision++ {
		candidate := id
		if collision > 0 {
			candidate = fmt.Sprintf("%s-%02d", id, collision)
		}
		tx.stagingDir = filepath.Join(stagingParent, "mods-"+candidate)
		tx.pluginsOld = pluginsDir + ".old-" + candidate
		tx.scriptsOld = scriptsDir + ".old-" + candidate
		free, err := allPathsFree(tx.stagingDir, tx.pluginsOld, tx.scriptsOld)
		if err != nil {
			return nil, err
		}
		if free {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("no free mods deploy transaction id for %s", id)
}

func (tx *modsDeployTx) stage(archivePath string) error {
	if err := os.MkdirAll(tx.stagingParent, 0o700); err != nil {
		return err
	}
	if err := optionalDir(tx.stagingParent); err != nil {
		return err
	}
	if err := os.Mkdir(tx.stagingDir, 0o700); err != nil {
		return err
	}
	if err := archive.ExtractModsFile(archivePath, tx.stagingDir); err != nil {
		return err
	}
	if _, err := archive.ValidateModsDir(tx.stagingDir); err != nil {
		return err
	}
	for _, dir := range []string{
		filepath.Join(tx.stagingDir, "plugins"),
		filepath.Join(tx.stagingDir, "scripts"),
		tx.stagingDir,
	} {
		if err := atomicfile.SyncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (tx *modsDeployTx) switchDirs() error {
	stagePlugins := filepath.Join(tx.stagingDir, "plugins")
	stageScripts := filepath.Join(tx.stagingDir, "scripts")

	if tx.pluginsExisted {
		if err := os.Rename(tx.pluginsDir, tx.pluginsOld); err != nil {
			return fmt.Errorf("move old mods/plugins: %w", err)
		}
		tx.pluginsMoved = true
	}
	if err := os.Rename(stagePlugins, tx.pluginsDir); err != nil {
		return fmt.Errorf("install mods/plugins: %w", err)
	}
	tx.pluginsInstalled = true

	if tx.scriptsExisted {
		if err := os.Rename(tx.scriptsDir, tx.scriptsOld); err != nil {
			return fmt.Errorf("move old mods/scripts: %w", err)
		}
		tx.scriptsMoved = true
	}
	if err := os.Rename(stageScripts, tx.scriptsDir); err != nil {
		return fmt.Errorf("install mods/scripts: %w", err)
	}
	tx.scriptsInstalled = true

	if err := atomicfile.SyncDir(tx.modsDir); err != nil {
		return fmt.Errorf("sync mods directory: %w", err)
	}
	return nil
}

func (tx *modsDeployTx) writeCustomScripts() error {
	lines, err := generatedScriptLines(tx.scriptsDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(tx.configsDir, 0o750); err != nil {
		return err
	}
	if err := optionalDir(tx.configsDir); err != nil {
		return err
	}
	data := []byte(strings.Join(lines, "\n"))
	if len(lines) > 0 {
		data = append(data, '\n')
	}
	tx.customWritten = true
	return modsDeployWriteFileFunc(tx.customPath, data, 0o640)
}

func (tx *modsDeployTx) writeCurrent(name, sha string) error {
	tx.currentWritten = true
	return modsDeployWriteFileFunc(tx.currentPath, archive.FormatCurrent(
		archive.Current{SHA256: sha, Name: name}), 0o640)
}

func generatedScriptLines(scriptsDir string) ([]string, error) {
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".lua") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		stem := strings.TrimSuffix(name, ".lua")
		lines = append(lines, fmt.Sprintf("require(\"custom.%s\")", stem))
	}
	return lines, nil
}
