// Package gameconfig reads and updates the TES3MP configuration files inside
// the data root without exposing raw file access to callers.
//
// Mutations always create a .bak-<timestamp> copy and replace the target
// atomically. Parsers only accept the documented whitelist for writes; unknown
// or non-scalar settings are either reported as read-only or rejected.
package gameconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// File identifies one supported configuration file.
type File string

const (
	FileServer  File = "server"
	FileConfig  File = "config"
	FileBanlist File = "banlist"

	maxConfigSize = 4 << 20
)

// ErrInvalid marks malformed config files and rejected values. Callers map it
// to exit code 9 (config_invalid).
var ErrInvalid = errors.New("invalid config")

// InvalidError carries the file and key that caused a config_invalid result.
// File and Key are optional for whole-file parse errors.
type InvalidError struct {
	File    File
	Key     string
	Message string
}

// Error implements error.
func (e *InvalidError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("%s: %s: %s", e.File, e.Key, e.Message)
	}
	if e.File != "" {
		return fmt.Sprintf("%s: %s", e.File, e.Message)
	}
	return e.Message
}

// Unwrap allows errors.Is(err, ErrInvalid).
func (e *InvalidError) Unwrap() error {
	return ErrInvalid
}

func invalidf(file File, key, format string, args ...any) error {
	return &InvalidError{
		File:    file,
		Key:     key,
		Message: fmt.Sprintf(format, args...),
	}
}

// ParseFile converts the --file argument to a supported file.
func ParseFile(raw string) (File, error) {
	switch File(raw) {
	case FileServer, FileConfig, FileBanlist:
		return File(raw), nil
	default:
		return "", fmt.Errorf("unsupported config file %q; want server, config or banlist", raw)
	}
}

// String returns the stable command-line name.
func (f File) String() string {
	return string(f)
}

// Snapshot is the structured config get result.
type Snapshot struct {
	File     File
	Values   map[string]any
	ReadOnly []string
}

// SetResult is the structured config set result.
type SetResult struct {
	File  File
	Key   string
	Value any
}

func (f File) path(root string) (string, error) {
	switch f {
	case FileServer:
		return filepath.Join(root, "configs", "tes3mp-server-default.cfg"), nil
	case FileConfig:
		return filepath.Join(root, "configs", "config.lua"), nil
	case FileBanlist:
		return filepath.Join(root, "configs", "banlist.json"), nil
	default:
		return "", invalidf(f, "", "unsupported config file")
	}
}

func readRegular(path string) ([]byte, os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, 0, invalidf("", "", "%s is not a regular file", path)
	}
	if info.Size() > maxConfigSize {
		return nil, 0, invalidf("", "", "%s is larger than %d bytes", path, maxConfigSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

// Read returns the documented values of one config file. It does not take the
// ops lock: config get is a read-only command.
func Read(root string, file File) (Snapshot, error) {
	path, err := file.path(root)
	if err != nil {
		return Snapshot{}, err
	}
	data, _, err := readRegular(path)
	if err != nil {
		return Snapshot{}, err
	}

	switch file {
	case FileServer:
		return readServer(path, data)
	case FileConfig:
		return readLua(path, data)
	case FileBanlist:
		return readBanlist(path, data)
	default:
		return Snapshot{}, invalidf(file, "", "unsupported config file")
	}
}

// Set validates and optionally writes one setting. The caller must hold the
// ops lock for a non-dry-run call. now is truncated to seconds and formatted
// in UTC for the backup name.
func Set(root string, file File, key, rawValue string, now time.Time,
	dryRun bool,
) (SetResult, error) {
	path, err := file.path(root)
	if err != nil {
		return SetResult{}, err
	}
	data, mode, err := readRegular(path)
	if err != nil {
		return SetResult{}, err
	}

	var updated []byte
	var value any
	switch file {
	case FileServer:
		updated, value, err = setServer(path, data, key, rawValue)
	case FileConfig:
		updated, value, err = setLua(path, data, key, rawValue)
	case FileBanlist:
		updated, value, err = setBanlist(path, data, key, rawValue)
	default:
		err = invalidf(file, key, "unsupported config file")
	}
	if err != nil {
		return SetResult{}, err
	}
	if dryRun {
		return SetResult{File: file, Key: canonicalKey(file, key), Value: value}, nil
	}

	if mode == 0 {
		mode = 0o600
	}
	backup, err := nextBackupPath(path, now)
	if err != nil {
		return SetResult{}, err
	}
	if err := atomicfile.WriteFile(backup, data, mode); err != nil {
		return SetResult{}, fmt.Errorf("write backup %s: %w", backup, err)
	}
	if err := atomicfile.WriteFile(path, updated, mode); err != nil {
		return SetResult{}, fmt.Errorf("write %s: %w", path, err)
	}
	return SetResult{File: file, Key: canonicalKey(file, key), Value: value}, nil
}

func canonicalKey(file File, key string) string {
	if file == FileServer {
		if spec, ok := serverSpecFor(key); ok {
			return spec.key
		}
	}
	return key
}

func nextBackupPath(path string, at time.Time) (string, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	base := path + ".bak-" + at.UTC().Format("2006-01-02_15-04-05")
	for collision := 0; collision <= 9999; collision++ {
		name := base
		if collision > 0 {
			name = fmt.Sprintf("%s-%02d", base, collision)
		}
		_, err := os.Lstat(name)
		switch {
		case err == nil:
			continue
		case errors.Is(err, fs.ErrNotExist):
			return name, nil
		default:
			return "", fmt.Errorf("stat backup %s: %w", name, err)
		}
	}
	return "", fmt.Errorf("no free backup name for %s", path)
}
