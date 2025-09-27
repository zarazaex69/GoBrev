package utils

import (
	"fmt"
	"runtime"
	"time"
)

// FormatUptime formats uptime duration
func FormatUptime(startTime time.Time) string {
	uptime := time.Since(startTime)
	
	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%dд %dч %dм %dс", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dч %dм %dс", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dм %dс", minutes, seconds)
	}
	return fmt.Sprintf("%dс", seconds)
}

// FormatMemory formats memory usage
func FormatMemory() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Convert bytes to MB
	allocMB := float64(m.Alloc) / 1024 / 1024
	totalAllocMB := float64(m.TotalAlloc) / 1024 / 1024
	sysMB := float64(m.Sys) / 1024 / 1024
	
	return fmt.Sprintf("Alloc: %.2f MB\nTotalAlloc: %.2f MB\nSys: %.2f MB", 
		allocMB, totalAllocMB, sysMB)
}

// FormatSystemInfo formats system information
func FormatSystemInfo() string {
	return fmt.Sprintf("Go Version: %s\nOS: %s\nArch: %s\nGoroutines: %d",
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		runtime.NumGoroutine())
}
