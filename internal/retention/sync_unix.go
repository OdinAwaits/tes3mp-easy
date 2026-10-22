//go:build unix

package retention

import "os"

// syncDir makes successful deletions durable. Directory fsync is available on
// Unix and is intentionally best-effort at the Apply level.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
