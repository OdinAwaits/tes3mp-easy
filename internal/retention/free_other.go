//go:build !unix && !windows

package retention

import "fmt"

func diskFree(path string) (int64, error) {
	return 0, fmt.Errorf("free space is not supported on this platform")
}
