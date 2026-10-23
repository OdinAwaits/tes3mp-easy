//go:build unix && !solaris

package retention

import (
	"fmt"
	"syscall"
)

func diskFree(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Bsize <= 0 {
		return 0, fmt.Errorf("invalid block size %d", stat.Bsize)
	}
	free := int64(stat.Bavail) * int64(stat.Bsize)
	if free < 0 {
		return 0, fmt.Errorf("free space does not fit in int64")
	}
	return free, nil
}
