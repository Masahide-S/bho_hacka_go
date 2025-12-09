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
	// 軽量版: ps コマンドで全プロセスのCPU使用率を合計
	// top -l 1 (1秒) → ps (50ms) に変更
	cmd := exec.Command("sh", "-c", "ps -A -o %cpu | awk '{s+=$1} END {print s}'")
	output, err := cmd.Output()

	if err != nil {
		return 0.0
	}

	usage, _ := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return usage
}

// getMemoryUsed returns used memory in MB (lightweight version)
func getMemoryUsed() int64 {
	// vm_stat の代わりに sysctl を使用（より軽量）
	cmd := exec.Command("sh", "-c", "sysctl -n hw.memsize")
	totalBytes, err := cmd.Output()
	if err != nil {
		return 0
	}

	total, _ := strconv.ParseInt(strings.TrimSpace(string(totalBytes)), 10, 64)

	// 空きメモリ取得
	cmd = exec.Command("sh", "-c", "vm_stat | grep 'Pages free' | awk '{print $3}' | sed 's/\\.//'")
	freePages, err := cmd.Output()
	if err != nil {
		return 0
	}

	free, _ := strconv.ParseInt(strings.TrimSpace(string(freePages)), 10, 64)

	// 使用中 = 全体 - 空き
	totalMB := total / (1024 * 1024)
	freeMB := (free * 4096) / (1024 * 1024)
	usedMB := totalMB - freeMB

	return usedMB
}

// getMemoryTotal を簡略化（キャッシュ）
var cachedMemoryTotal int64 = 0

func getMemoryTotal() int64 {
	// 総メモリは変わらないのでキャッシュ
	if cachedMemoryTotal > 0 {
		return cachedMemoryTotal
	}

	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()

	if err != nil {
		return 0
	}

	bytes, _ := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	cachedMemoryTotal = bytes / (1024 * 1024)

	return cachedMemoryTotal
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
