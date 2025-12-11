package monitor

import (
	"fmt"
	"strconv"
	"strings"
)

// ProcessStats holds CPU and memory usage
type ProcessStats struct {
	CPU    float64
	Memory int64 // KB
}

// getProcessStats returns CPU and memory usage for a single process
func getProcessStats(pid string) ProcessStats {
	// PIDのバリデーション
	if !IsValidPID(pid) {
		return ProcessStats{}
	}

	// タイムアウト付きでpsコマンドを実行
	output, err := RunCommandWithTimeout("ps", "-o", "%cpu,rss", "-p", pid)

	if err != nil {
		return ProcessStats{}
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return ProcessStats{}
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return ProcessStats{}
	}

	cpu, _ := strconv.ParseFloat(fields[0], 64)
	rss, _ := strconv.ParseInt(fields[1], 10, 64)

	return ProcessStats{
		CPU:    cpu,
		Memory: rss,
	}
}

// getMultiProcessStats returns total CPU and memory for multiple processes
func getMultiProcessStats(processName string) ProcessStats {
	// プロセス名のバリデーション
	if !IsValidIdentifier(processName) {
		return ProcessStats{}
	}

	// タイムアウト付きでpgrepを実行
	output, err := RunCommandWithTimeout("pgrep", processName)

	if err != nil {
		return ProcessStats{}
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(pids) == 0 {
		return ProcessStats{}
	}

	// 各PIDをバリデーション
	var validPids []string
	for _, pid := range pids {
		pid = strings.TrimSpace(pid)
		if IsValidPID(pid) {
			validPids = append(validPids, pid)
		}
	}
	if len(validPids) == 0 {
		return ProcessStats{}
	}

	// カンマ区切りのPIDリスト作成
	pidList := strings.Join(validPids, ",")

	// タイムアウト付きでpsを実行
	psOutput, err := RunCommandWithTimeout("ps", "-o", "%cpu,rss", "-p", pidList)

	if err != nil {
		return ProcessStats{}
	}

	var totalCPU float64
	var totalMemory int64

	lines := strings.Split(string(psOutput), "\n")
	for i, line := range lines {
		if i == 0 { // ヘッダースキップ
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		cpu, _ := strconv.ParseFloat(fields[0], 64)
		rss, _ := strconv.ParseInt(fields[1], 10, 64)

		totalCPU += cpu
		totalMemory += rss
	}

	return ProcessStats{
		CPU:    totalCPU,
		Memory: totalMemory,
	}
}

// formatMemory formats memory in KB to human-readable format
func formatMemory(kb int64) string {
	if kb < 1024 {
		return fmt.Sprintf("%d KB", kb)
	}
	mb := float64(kb) / 1024.0
	if mb < 1024 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	gb := mb / 1024.0
	return fmt.Sprintf("%.2f GB", gb)
}

// formatStatsString returns formatted "CPU: X% | メモリ: Y MB" string
func formatStatsString(stats ProcessStats) string {
	if stats.CPU == 0 && stats.Memory == 0 {
		return ""
	}
	return fmt.Sprintf("CPU: %.1f%% | メモリ: %s", stats.CPU, formatMemory(stats.Memory))
}
