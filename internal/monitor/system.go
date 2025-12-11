package monitor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// SystemResources holds system resource information
type SystemResources struct {
	// CPU情報
	CPUUsage    float64
	CPUCores    int

	// メモリ情報（Activity Monitor形式）
	MemoryTotal      int64   // 総メモリ (MB)
	MemoryUsed       int64   // 使用中 = App Memory + Wired + Compressed (MB)
	MemoryAppMemory  int64   // App Memory (Active) (MB)
	MemoryWired      int64   // Wired Memory (MB)
	MemoryCompressed int64   // Compressed (MB)
	MemoryCached     int64   // Cached Files (MB)
	MemoryAvailable  int64   // 使用可能 (MB)
	MemoryPerc       float64 // 使用率 (%)

	// ディスク情報
	DiskTotal int64   // 総容量 (GB)
	DiskUsed  int64   // 使用量 (GB)
	DiskFree  int64   // 空き容量 (GB)
	DiskPerc  float64 // 使用率 (%)

	// その他の情報
	ProcessCount int    // プロセス数
	Uptime       string // システム稼働時間
}

// GetSystemResources returns current system resource usage
func GetSystemResources() SystemResources {
	memStats := getDetailedMemoryStats()
	diskStats := getDiskStats()

	return SystemResources{
		// CPU
		CPUUsage: getCPUUsage(),
		CPUCores: getCPUCores(),

		// メモリ（Activity Monitor形式）
		MemoryTotal:      memStats.Total,
		MemoryUsed:       memStats.Used,
		MemoryAppMemory:  memStats.AppMemory,
		MemoryWired:      memStats.Wired,
		MemoryCompressed: memStats.Compressed,
		MemoryCached:     memStats.Cached,
		MemoryAvailable:  memStats.Available,
		MemoryPerc:       memStats.UsedPerc,

		// ディスク
		DiskTotal: diskStats.Total,
		DiskUsed:  diskStats.Used,
		DiskFree:  diskStats.Free,
		DiskPerc:  diskStats.UsedPerc,

		// その他
		ProcessCount: getProcessCount(),
		Uptime:       getSystemUptime(),
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

// getMemoryUsed returns used memory in MB using vm_stat parsing
func getMemoryUsed() int64 {
	// vm_stat の出力を取得
	cmd := exec.Command("vm_stat")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	pageSize := int64(4096) // macOSのページサイズは通常4KB

	var active, wired, compressed int64

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// 数値部分を抽出（末尾のピリオドを削除）
		valueStr := strings.TrimSuffix(fields[len(fields)-1], ".")
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			continue
		}

		if strings.Contains(line, "Pages active") {
			active = value
		} else if strings.Contains(line, "Pages wired down") {
			wired = value
		} else if strings.Contains(line, "Pages occupied by compressor") {
			compressed = value
		}
	}

	// 計算式: (Pages active + Pages wired down + Pages occupied by compressor) * 4096 / (1024 * 1024)
	usedMB := (active + wired + compressed) * pageSize / (1024 * 1024)

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
	return fmt.Sprintf("CPU: %.1f%% | メモリ: %.1fGB/%.1fGB (%.0f%%) | ディスク: %.0f%% (空き %dGB)",
		sr.CPUUsage,
		float64(sr.MemoryUsed)/1024.0,
		float64(sr.MemoryTotal)/1024.0,
		sr.MemoryPerc,
		sr.DiskPerc,
		sr.DiskFree,
	)
}

// MemoryStats holds detailed memory statistics
type MemoryStats struct {
	Total      int64
	Used       int64
	AppMemory  int64
	Wired      int64
	Compressed int64
	Cached     int64
	Available  int64
	UsedPerc   float64
}

// getDetailedMemoryStats returns detailed memory statistics (Activity Monitor style)
func getDetailedMemoryStats() MemoryStats {
	// vm_stat の出力を取得
	cmd := exec.Command("vm_stat")
	output, err := cmd.Output()
	if err != nil {
		return MemoryStats{}
	}

	lines := strings.Split(string(output), "\n")
	pageSize := int64(4096) // macOSのページサイズは通常4KB

	var active, wired, compressed, cached, free, inactive int64

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// 数値部分を抽出（末尾のピリオドを削除）
		valueStr := strings.TrimSuffix(fields[len(fields)-1], ".")
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			continue
		}

		if strings.Contains(line, "Pages active") {
			active = value
		} else if strings.Contains(line, "Pages wired down") {
			wired = value
		} else if strings.Contains(line, "Pages occupied by compressor") {
			compressed = value
		} else if strings.Contains(line, "File-backed pages") {
			cached = value
		} else if strings.Contains(line, "Pages free") {
			free = value
		} else if strings.Contains(line, "Pages inactive") {
			inactive = value
		}
	}

	// MB単位に変換
	totalMB := getMemoryTotal()
	appMemoryMB := (active * pageSize) / (1024 * 1024)
	wiredMB := (wired * pageSize) / (1024 * 1024)
	compressedMB := (compressed * pageSize) / (1024 * 1024)
	cachedMB := (cached * pageSize) / (1024 * 1024)
	freeMB := (free * pageSize) / (1024 * 1024)
	inactiveMB := (inactive * pageSize) / (1024 * 1024)

	// 使用中 = App Memory + Wired + Compressed (Activity Monitor形式)
	usedMB := appMemoryMB + wiredMB + compressedMB

	// 使用可能 = Free + Inactive
	availableMB := freeMB + inactiveMB

	// 使用率
	usedPerc := 0.0
	if totalMB > 0 {
		usedPerc = (float64(usedMB) / float64(totalMB)) * 100
	}

	return MemoryStats{
		Total:      totalMB,
		Used:       usedMB,
		AppMemory:  appMemoryMB,
		Wired:      wiredMB,
		Compressed: compressedMB,
		Cached:     cachedMB,
		Available:  availableMB,
		UsedPerc:   usedPerc,
	}
}

// DiskStats holds disk statistics
type DiskStats struct {
	Total    int64
	Used     int64
	Free     int64
	UsedPerc float64
}

// getDiskStats returns disk statistics
func getDiskStats() DiskStats {
	// df -h / でルートパーティションの情報を取得
	cmd := exec.Command("df", "-g", "/")
	output, err := cmd.Output()
	if err != nil {
		return DiskStats{}
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return DiskStats{}
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return DiskStats{}
	}

	// GB単位で取得
	total, _ := strconv.ParseInt(fields[1], 10, 64)
	used, _ := strconv.ParseInt(fields[2], 10, 64)
	free, _ := strconv.ParseInt(fields[3], 10, 64)

	usedPerc := 0.0
	if total > 0 {
		usedPerc = (float64(used) / float64(total)) * 100
	}

	return DiskStats{
		Total:    total,
		Used:     used,
		Free:     free,
		UsedPerc: usedPerc,
	}
}

// getCPUCores returns the number of CPU cores
func getCPUCores() int {
	cmd := exec.Command("sysctl", "-n", "hw.ncpu")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	cores, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	return cores
}

// getProcessCount returns the number of running processes
func getProcessCount() int {
	cmd := exec.Command("sh", "-c", "ps -A | wc -l")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	count, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	// ヘッダー行を除く
	return count - 1
}

// getSystemUptime returns system uptime
func getSystemUptime() string {
	cmd := exec.Command("uptime")
	output, err := cmd.Output()
	if err != nil {
		return "不明"
	}

	// uptime の出力から稼働時間部分を抽出
	uptimeStr := string(output)

	// "up" から "user" までの部分を抽出
	if strings.Contains(uptimeStr, "up") {
		parts := strings.Split(uptimeStr, "up")
		if len(parts) >= 2 {
			remaining := parts[1]
			if strings.Contains(remaining, "user") {
				uptimePart := strings.Split(remaining, "user")[0]
				return strings.TrimSpace(uptimePart)
			} else if strings.Contains(remaining, ",") {
				// "," の前までを取得
				uptimePart := strings.Split(remaining, ",")[0]
				return strings.TrimSpace(uptimePart)
			}
		}
	}

	return strings.TrimSpace(uptimeStr)
}

// getDiskUsage returns disk usage percentage and free space in GB using syscall.Statfs
func getDiskUsage() (float64, int64) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0.0, 0
	}

	// Total blocks と Available blocks から計算
	totalBlocks := stat.Blocks
	availableBlocks := stat.Bavail
	blockSize := uint64(stat.Bsize)

	// 使用済みブロック数 = Total - Available
	usedBlocks := totalBlocks - availableBlocks

	// 使用率（%）
	usagePerc := 0.0
	if totalBlocks > 0 {
		usagePerc = (float64(usedBlocks) / float64(totalBlocks)) * 100
	}

	// 空き容量（GB）
	freeGB := int64(availableBlocks * blockSize / (1024 * 1024 * 1024))

	return usagePerc, freeGB
}
