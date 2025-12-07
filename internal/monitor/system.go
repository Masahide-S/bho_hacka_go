package monitor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// SystemResources holds system resource information
type SystemResources struct {
	CPUUsage    float64
	MemoryUsed  int64 // MB
	MemoryTotal int64 // MB
	MemoryPerc  float64
}

// GetSystemResources returns current system resource usage
func GetSystemResources() SystemResources {
	return SystemResources{
		CPUUsage:    getCPUUsage(),
		MemoryUsed:  getMemoryUsed(),
		MemoryTotal: getMemoryTotal(),
		MemoryPerc:  getMemoryPercentage(),
	}
}

// getCPUUsage returns current CPU usage percentage
func getCPUUsage() float64 {
	// macOS: top コマンドで取得
	cmd := exec.Command("sh", "-c", "top -l 1 | grep 'CPU usage' | awk '{print $3}' | sed 's/%//'")
	output, err := cmd.Output()

	if err != nil {
		return 0.0
	}

	usage, _ := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return usage
}

// getMemoryUsed returns used memory in MB
func getMemoryUsed() int64 {
	// macOS: vm_stat コマンドで取得
	cmd := exec.Command("sh", "-c", "vm_stat | grep 'Pages active' | awk '{print $3}' | sed 's/\\.//'")
	output, err := cmd.Output()

	if err != nil {
		return 0
	}

	pages, _ := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	// ページサイズは通常4096バイト
	usedMB := (pages * 4096) / (1024 * 1024)
	return usedMB
}

// getMemoryTotal returns total memory in MB
func getMemoryTotal() int64 {
	// macOS: sysctl で取得
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()

	if err != nil {
		return 0
	}

	bytes, _ := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	totalMB := bytes / (1024 * 1024)
	return totalMB
}

// getMemoryPercentage returns memory usage percentage
func getMemoryPercentage() float64 {
	used := getMemoryUsed()
	total := getMemoryTotal()

	if total == 0 {
		return 0.0
	}

	return (float64(used) / float64(total)) * 100
}

// FormatSystemResources formats system resources for display
func FormatSystemResources(sr SystemResources) string {
	return fmt.Sprintf("CPU: %.1f%% | メモリ: %.1fGB/%.1fGB (%.0f%%)",
		sr.CPUUsage,
		float64(sr.MemoryUsed)/1024.0,
		float64(sr.MemoryTotal)/1024.0,
		sr.MemoryPerc,
	)
}
