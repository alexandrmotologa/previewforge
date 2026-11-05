//go:build !windows

package orchestrator

import "golang.org/x/sys/unix"

func getPlatformDiskUsage(path string) DiskUsage {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		return DiskUsage{TotalGB: 40, FreeGB: 30, UsedGB: 10, Percent: 25}
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	totalGB := float64(totalBytes) / (1024 * 1024 * 1024)
	freeGB := float64(freeBytes) / (1024 * 1024 * 1024)
	usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
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
