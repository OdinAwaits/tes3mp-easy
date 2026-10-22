//go:build windows

package retention

// Windows has no portable directory fsync; deletions reported by os.Remove are
// already committed by the filesystem.
func syncDir(string) error {
	return nil
}
