//go:build unix

package doctor

import (
	"fmt"
	"os"
	"syscall"
)

// fileMetadata is the unix ownership and raw mode information doctor needs.
// The raw mode keeps setgid, which os.FileMode represents with a high bit.
type fileMetadata struct {
	UID  uint32
	GID  uint32
	Mode uint32
	Dev  uint64
}

func metadataOf(info os.FileInfo) (fileMetadata, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileMetadata{}, false
	}
	return fileMetadata{
		UID:  stat.Uid,
		GID:  stat.Gid,
		Mode: uint32(stat.Mode) & 0o7777,
		Dev:  uint64(stat.Dev),
	}, true
}

func deviceOfPath(path string) (uint64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	meta, ok := metadataOf(info)
	if !ok {
		return 0, fmt.Errorf("cannot inspect device for %s", path)
	}
	return meta.Dev, nil
}
