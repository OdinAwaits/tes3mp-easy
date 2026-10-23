package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
)

func checkCurrent(root string) Check {
	dir := filepath.Join(root, "backups", "mods")
	path := filepath.Join(dir, "current.txt")

	current, err := archive.ReadCurrentFile(path)
	if err != nil {
		return errorCheck("current", "%v", err)
	}

	target := filepath.Join(dir, current.Name)
	info, err := os.Lstat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return errorCheck("current", "%s points to missing archive %s", path, current.Name)
	case err != nil:
		return errorCheck("current", "%s: %v", target, err)
	case info.Mode()&os.ModeSymlink != 0:
		return errorCheck("current", "%s is a symlink", target)
	case !info.Mode().IsRegular():
		return errorCheck("current", "%s is not a regular file", target)
	case info.Size() > archive.MaxCompressedSize:
		return errorCheck("current", "%s exceeds %d bytes",
			target, archive.MaxCompressedSize)
	}

	sum, err := sha256File(target)
	if err != nil {
		return errorCheck("current", "%s: %v", target, err)
	}
	if sum != current.SHA256 {
		return errorCheck("current", "%s sha256 is %s, want %s",
			target, sum, current.SHA256)
	}
	return okCheck("current", "%s (%s)", current.Name, current.SHA256)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	sum := sha256.New()
	if _, err := io.CopyBuffer(sum, file, make([]byte, 32*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
