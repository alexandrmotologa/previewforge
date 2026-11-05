//go:build windows

package orchestrator

import (
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func getPlatformDiskUsage(path string) DiskUsage {
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes int64

	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return DiskUsage{TotalGB: 100, FreeGB: 50, UsedGB: 50, Percent: 50}
	}

	r1, _, _ := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if r1 == 0 {
		return DiskUsage{TotalGB: 100, FreeGB: 50, UsedGB: 50, Percent: 50}
	}

	totalGB := float64(totalNumberOfBytes) / (1024 * 1024 * 1024)
	freeGB := float64(totalNumberOfFreeBytes) / (1024 * 1024 * 1024)
	usedGB := totalGB - freeGB
	var percent float64
	if totalGB > 0 {
		percent = (usedGB / totalGB) * 100
	}

	return DiskUsage{
		TotalGB: totalGB,
		FreeGB:  freeGB,
		UsedGB:  usedGB,
		Percent: percent,
	}
}
