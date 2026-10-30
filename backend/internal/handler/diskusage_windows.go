//go:build windows

package handler

import (
	"syscall"
	"unsafe"
)

var (
	kernel32Disk            = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = kernel32Disk.NewProc("GetDiskFreeSpaceExW")
)

// diskUsage 返回 path 所在卷的可用 / 总字节数（Windows 实现，GetDiskFreeSpaceExW）
func diskUsage(path string) (free, total uint64) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	var freeToCaller, totalBytes, totalFree uint64
	r, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeToCaller)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		return 0, 0
	}
	return freeToCaller, totalBytes
}
