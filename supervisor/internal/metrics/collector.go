package metrics

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ProcessMetrics holds collected process metrics
type ProcessMetrics struct {
	MemoryMB   int64
	CPUPercent float64
}

// CollectProcessMetrics gathers memory and CPU metrics for a given PID
// Reads from /proc filesystem which is Linux-specific
func CollectProcessMetrics(pid int) (*ProcessMetrics, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid PID: %d", pid)
	}

	metrics := &ProcessMetrics{}

	// Read memory from /proc/[pid]/status
	// Look for VmRSS (Resident Set Size)
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proc status: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			// Parse "VmRSS:    12345 kB"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					metrics.MemoryMB = kb / 1024
				}
			}
			break
		}
	}

	// CPU usage requires sampling over time, which is complex
	// For now, we'll return 0 and rely on K8s metrics for CPU
	// A proper implementation would read /proc/[pid]/stat multiple times
	// and calculate the delta
	metrics.CPUPercent = 0.0

	return metrics, nil
}

// GetMemoryUsageMB returns memory usage in MB for a PID
// Returns 0 if unable to read
func GetMemoryUsageMB(pid int) int64 {
	metrics, err := CollectProcessMetrics(pid)
	if err != nil {
		return 0
	}
	return metrics.MemoryMB
}
