//go:build !unix

package doctor

// writableDir has no portable equivalent outside unix. Ops targets Linux, so
// the check is skipped rather than turning every non-unix run into an error.
func writableDir(string) error {
	return nil
}
