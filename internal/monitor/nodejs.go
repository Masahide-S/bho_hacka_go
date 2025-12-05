package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckNodejs checks if Node.js process is running
func CheckNodejs() string {
	cmd := exec.Command("pgrep", "node")
	err := cmd.Run()

	if err != nil {
		return "✗ Node.js: 検出なし"
	}

	// ポート番号取得
	port := getPortByProcess("node")
	portInfo := ""
	if port != "" {
		portInfo = " [:" + port + "]"
	}

	// プロセス詳細情報取得
	details := getNodejsDetails()

	result := fmt.Sprintf("✓ Node.js: 実行中%s\n", portInfo)
	if details != "" {
		result += details
	}

	return result
}

// getNodejsDetails returns detailed info about Node.js processes
func getNodejsDetails() string {
	// 最初のNode.jsプロセスのPIDを取得
	cmd := exec.Command("pgrep", "node")
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(pids) == 0 {
		return ""
	}

	var result string

	// 各PIDについて情報取得（最大3つまで）
	for i, pid := range pids {
		if i >= 3 {
			break
		}

		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}

		// カレントディレクトリ取得
		cwdCmd := exec.Command("lsof", "-p", pid)
		cwdOutput, err := cwdCmd.Output()

		if err != nil {
			continue
		}

		lines := strings.Split(string(cwdOutput), "\n")
		var projectDir string

		for _, line := range lines {
			if strings.Contains(line, " cwd ") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					projectDir = fields[len(fields)-1]
					break
				}
			}
		}

		if projectDir == "" {
			continue
		}

		// 稼働時間取得
		uptime := getProcessUptime(pid)

		// CPU・メモリ使用量取得
		stats := getProcessStats(pid)

		// package.json からプロジェクト名取得
		projectName := getProjectNameFromPackageJson(projectDir)

		result += fmt.Sprintf("  └─ %s\n", projectDir)
		
		// プロジェクト名、稼働時間、CPU・メモリを表示
		infoLine := "     "
		if projectName != "" {
			infoLine += fmt.Sprintf("(package.json: %s)", projectName)
		}
		if uptime != "" {
			if projectName != "" {
				infoLine += " | "
			}
			infoLine += fmt.Sprintf("稼働: %s", uptime)
		}
		
		// CPU・メモリ情報追加
		statsStr := formatStatsString(stats)
		if statsStr != "" {
			infoLine += fmt.Sprintf(" | %s", statsStr)
		}
		
		result += infoLine + "\n"
	}

	return result
}

// getProcessUptime returns how long a process has been running
func getProcessUptime(pid string) string {
	cmd := exec.Command("ps", "-o", "etime=", "-p", pid)
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// PackageJson represents package.json structure
type PackageJson struct {
	Name string `json:"name"`
}

// getProjectNameFromPackageJson reads project name from package.json
func getProjectNameFromPackageJson(dir string) string {
	packageJsonPath := filepath.Join(dir, "package.json")

	data, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return ""
	}

	var pkg PackageJson
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	return pkg.Name
}
