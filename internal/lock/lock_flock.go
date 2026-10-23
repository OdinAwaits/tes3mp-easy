//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package lock

import (
	"errors"
	"os"
	"syscall"
)

func tryLock(file *os.File, mode Mode) (bool, error) {
	how := syscall.LOCK_EX | syscall.LOCK_NB
	if mode == Shared {
		how = syscall.LOCK_SH | syscall.LOCK_NB
	}

	for {
		err := syscall.Flock(int(file.Fd()), how)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EWOULDBLOCK), errors.Is(err, syscall.EAGAIN):
			return false, nil
		default:
			return false, err
		}
	}
}

func unlockFile(file *os.File) error {
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return err
	}
}
