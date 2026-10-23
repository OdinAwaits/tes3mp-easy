//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// deviceOfPath returns the filesystem device containing path. Symlinks are
// followed: the caller validates the final archive directory separately.
func deviceOfPath(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("cannot inspect device for %s", path)
	}
	return uint64(stat.Dev), nil
}
