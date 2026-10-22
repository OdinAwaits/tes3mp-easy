//go:build unix

package doctor

import "syscall"

// accessWrite is W_OK from access(2). syscall does not export W_OK on every
// unix platform, but the value is fixed by POSIX.
const accessWrite = 2

func writableDir(path string) error {
	return syscall.Access(path, accessWrite)
}
