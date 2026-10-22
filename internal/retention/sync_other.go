//go:build !unix && !windows

package retention

func syncDir(string) error {
	return nil
}
