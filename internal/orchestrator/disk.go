package orchestrator

import (
	"os"
	"path/filepath"
)

// DiskUsage holds host disk capacity metrics.
type DiskUsage struct {
	TotalGB float64
	FreeGB  float64
	UsedGB  float64
	Percent float64
}

// GetDiskUsage estimates available disk space on the volume hosting dataDir.
func GetDiskUsage(dataDir string) DiskUsage {
	absDir, err := filepath.Abs(dataDir)
	if err != nil {
		absDir = "."
	}

	// Create directory if not exists to check stat
	_ = os.MkdirAll(absDir, 0755)

	return getPlatformDiskUsage(absDir)
}
