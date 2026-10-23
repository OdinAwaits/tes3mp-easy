//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package lock

import "os"

// Ops targets Linux. Platforms without syscall.Flock compile, but refuse to
// pretend that an unlocked process is safe for mutations.
func tryLock(*os.File, Mode) (bool, error) {
	return false, ErrUnsupported
}

func unlockFile(*os.File) error {
	return ErrUnsupported
}
