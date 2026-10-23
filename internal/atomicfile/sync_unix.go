//go:build unix

package atomicfile

import (
	"errors"
	"os"
)

// syncDir makes the rename durable. Opening a directory for sync is supported
// on Unix and intentionally not best-effort here: without it a crash may lose
// the directory entry even though the file contents were synced.
func syncDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(syncErr, closeErr)
}
