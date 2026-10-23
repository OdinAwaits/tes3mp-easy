//go:build !unix

package doctor

import (
	"errors"
	"os"
)

type fileMetadata struct {
	UID  uint32
	GID  uint32
	Mode uint32
	Dev  uint64
}

func metadataOf(os.FileInfo) (fileMetadata, bool) {
	return fileMetadata{}, false
}

func deviceOfPath(string) (uint64, error) {
	return 0, errors.New("device inspection is supported only on unix")
}
