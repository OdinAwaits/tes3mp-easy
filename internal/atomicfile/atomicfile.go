// Package atomicfile writes a file without exposing a partially written
// replacement.
//
// A writer always creates <target>.part in the target directory, writes and
// fsyncs it, renames it over the target and fsyncs the directory. The old file
// remains readable until the rename. A crash before the rename leaves the
// deterministic .part file for doctor and the operator to find.
//
// Callers that can run concurrently must serialize writers with internal/lock.
// The deterministic .part name intentionally does not allow two uncoordinated
// writers to target the same file.
package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DefaultMode is used when New receives mode zero.
const DefaultMode os.FileMode = 0o600

// ErrClosed reports a write or commit on a writer that already finished.
var ErrClosed = errors.New("atomic writer is closed")

type hooks struct {
	syncFile func(*os.File) error
	rename   func(string, string) error
	syncDir  func(string) error
}

func defaultHooks() hooks {
	return hooks{
		syncFile: func(file *os.File) error { return file.Sync() },
		rename:   os.Rename,
		syncDir:  syncDir,
	}
}

// Writer is a pending atomic replacement. Create it with New, then either
// Commit or Abort. A Writer is not safe for concurrent use.
type Writer struct {
	target string
	part   string
	file   *os.File
	done   bool
	hooks  hooks
}

// New creates the .part file next to target. An existing .part is truncated:
// ops serializes mutations with the ops lock, so a leftover file is from a
// crashed process and its contents are not a valid committed version.
//
// Mode zero selects DefaultMode. Only permission bits are accepted because the
// temporary file is regular by construction.
func New(target string, mode os.FileMode) (*Writer, error) {
	switch {
	case target == "":
		return nil, errors.New("atomic target is empty")
	case mode == 0:
		mode = DefaultMode
	case mode&os.ModePerm != mode:
		return nil, fmt.Errorf("atomic mode %v is not permission bits", mode)
	}

	part := target + ".part"
	file, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create temp file %s: %w", part, err)
	}
	if err := file.Chmod(mode); err != nil {
		closeErr := file.Close()
		removeErr := os.Remove(part)
		return nil, errors.Join(
			fmt.Errorf("chmod temp file %s: %w", part, err),
			closeErr, removeErr,
		)
	}

	return &Writer{
		target: target,
		part:   part,
		file:   file,
		hooks:  defaultHooks(),
	}, nil
}

// Write appends to the temporary file.
func (w *Writer) Write(p []byte) (int, error) {
	if !w.open() {
		return 0, ErrClosed
	}
	n, err := w.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("write temp file %s: %w", w.part, err)
	}
	return n, nil
}

func (w *Writer) open() bool {
	return w != nil && w.file != nil && !w.done
}

// Commit fsyncs the temporary file, renames it over the target and fsyncs the
// directory. The target is untouched if the file sync, close or rename fails.
// A directory sync error is returned after the rename; the new target is
// already visible in that case.
func (w *Writer) Commit() error {
	if !w.open() {
		return ErrClosed
	}
	w.done = true

	if err := w.hooks.syncFile(w.file); err != nil {
		return errors.Join(
			fmt.Errorf("fsync temp file %s: %w", w.part, err),
			w.discard(),
		)
	}
	closeErr := w.file.Close()
	w.file = nil
	if closeErr != nil {
		return errors.Join(
			fmt.Errorf("close temp file %s: %w", w.part, closeErr),
			w.discard(),
		)
	}

	if err := w.hooks.rename(w.part, w.target); err != nil {
		return errors.Join(
			fmt.Errorf("rename %s to %s: %w", w.part, w.target, err),
			w.discard(),
		)
	}
	if err := w.hooks.syncDir(filepath.Dir(w.target)); err != nil {
		return fmt.Errorf("fsync target directory: %w", err)
	}
	return nil
}

// Abort closes the temporary file and removes it. The target is not touched.
// Abort is idempotent and does nothing after a successful Commit.
func (w *Writer) Abort() error {
	if w == nil || w.done {
		return nil
	}
	w.done = true
	return w.discard()
}

func (w *Writer) discard() error {
	var errs []error
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close temp file %s: %w", w.part, err))
		}
		w.file = nil
	}
	if err := os.Remove(w.part); err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove temp file %s: %w", w.part, err))
	}
	return errors.Join(errs...)
}

// SyncDir fsyncs a directory. Callers that rename or remove entries directly
// use it to make the directory change durable; on platforms without directory
// fsync it is a no-op.
func SyncDir(path string) error {
	return syncDir(path)
}

// WriteFile atomically replaces target with data.
func WriteFile(target string, data []byte, mode os.FileMode) error {
	writer, err := New(target, mode)
	if err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return errors.Join(err, writer.Abort())
	}
	return writer.Commit()
}
