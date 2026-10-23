//go:build !unix

package atomicfile

// Directory fsync is not portable outside Unix. Ops targets Linux, where the
// real implementation is used; other platforms still get file fsync + rename.
func syncDir(string) error {
	return nil
}
