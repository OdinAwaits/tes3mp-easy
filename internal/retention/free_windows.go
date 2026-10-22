//go:build windows

package retention

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func diskFree(path string) (int64, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var available, total, totalFree uint64
	ok, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if ok == 0 {
		if callErr != syscall.Errno(0) {
			return 0, callErr
		}
		return 0, fmt.Errorf("GetDiskFreeSpaceExW failed")
	}
	if available > math.MaxInt64 {
		return 0, fmt.Errorf("free space does not fit in int64")
	}
	return int64(available), nil
}
