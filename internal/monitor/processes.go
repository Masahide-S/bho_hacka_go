package monitor

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// ProcessInfo holds process information
type ProcessInfo struct {
	Name      string
	PID       string
	CPU       float64
	Memory    int64 // MB
	IsDevTool bool  // 開発ツールかどうか
}

// GetTopProcesses returns top N processes by CPU/Memory
func GetTopProcesses(n int) []ProcessInfo {
	// ps で全プロセスの情報取得
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()

	if err != nil {
		return []ProcessInfo{}
	}

	lines := strings.Split(string(output), "\n")
	var processes []ProcessInfo

	// ヘッダー行をスキップ
	for i, line := range lines {
		if i == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		// CPU使用率（3番目）
		cpu, _ := strconv.ParseFloat(fields[2], 64)

		// メモリ使用量（4番目、KB単位）
		memKB, _ := strconv.ParseInt(fields[5], 10, 64)
		memMB := memKB / 1024

		// プロセス名（11番目以降）
		name := strings.Join(fields[10:], " ")

		// プロセス名を短縮
		name = getShortProcessName(name)

		// 開発ツールかチェック
		isDevTool := isDevProcess(name)

		processes = append(processes, ProcessInfo{
			Name:      name,
			PID:       fields[1],
			CPU:       cpu,
			Memory:    memMB,
			IsDevTool: isDevTool,
		})
	}

	// CPU使用率でソート
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPU > processes[j].CPU
	})

	// 上位N件
	if len(processes) > n {
		processes = processes[:n]
	}

	return processes
}

// GetDevProcesses returns development-related processes
func GetDevProcesses() []ProcessInfo {
	allProcesses := GetTopProcesses(100) // 上位100件から探す

	var devProcesses []ProcessInfo
	for _, p := range allProcesses {
		if p.IsDevTool {
			devProcesses = append(devProcesses, p)
		}
	}

	return devProcesses
}

// getShortProcessName shortens process name
func getShortProcessName(fullName string) string {
	// パスを削除
	parts := strings.Split(fullName, "/")
	name := parts[len(parts)-1]

	// 引数を削除
	name = strings.Split(name, " ")[0]

	// よくある名前の変換
	if strings.Contains(name, "Docker") {
		return "Docker Desktop"
	}
	if strings.Contains(name, "Google Chrome") || strings.Contains(name, "chrome") {
		return "Chrome"
	}
	if strings.Contains(name, "node") {
		return "Node.js"
	}
	if strings.Contains(name, "python") {
		return "Python"
	}
	if strings.Contains(name, "postgres") {
		return "PostgreSQL"
	}

	return name
}

// isDevProcess checks if process is development-related
func isDevProcess(name string) bool {
	devKeywords := []string{
		"docker", "Docker",
		"node", "Node",
		"python", "Python",
		"postgres", "PostgreSQL",
		"mysql", "MySQL",
		"redis", "Redis",
		"nginx", "Nginx",
		"code", "VSCode",
	}

	nameLower := strings.ToLower(name)
	for _, keyword := range devKeywords {
		if strings.Contains(nameLower, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}
// FormatTopProcesses formats top processes for display
func FormatTopProcesses(processes []ProcessInfo) string {
	var result strings.Builder

	for i, p := range processes {
		// 番号を固定幅（右寄せ）、名前を固定幅（左寄せ）
		result.WriteString(fmt.Sprintf("  %2d. %-30s  %5.1f%% CPU | %dMB\n",
			i+1, p.Name, p.CPU, p.Memory))
	}

	return result.String()
}

// FormatDevProcesses formats dev processes for display
func FormatDevProcesses(processes []ProcessInfo) string {
	if len(processes) == 0 {
		return "  開発プロセスが見つかりません\n"
	}

	var result strings.Builder

	for _, p := range processes {
		result.WriteString(fmt.Sprintf("  ✓ %-30s  %5.1f%% CPU | %dMB\n",
			p.Name, p.CPU, p.Memory))
	}

	return result.String()
}
