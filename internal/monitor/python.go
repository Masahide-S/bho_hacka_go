package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckPython checks if Python process is running
func CheckPython() string {
	cmd := exec.Command("pgrep", "python")
	err := cmd.Run()

	if err != nil {
		return "✗ Python: 検出なし"
	}

	// ポート番号取得
	port := getPortByProcess("python")
	portInfo := ""
	if port != "" {
		portInfo = " [:" + port + "]"
	}

	// プロセス詳細情報取得
	details := getPythonDetails()

	result := fmt.Sprintf("✓ Python: 実行中%s\n", portInfo)
	if details != "" {
		result += details
	}

	return result
}

// getPythonDetails returns detailed info about Python processes
func getPythonDetails() string {
	// PythonプロセスのPIDを取得
	cmd := exec.Command("pgrep", "python")
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

		// プロセス種別判定（Jupyter/Flask/Django等）
		processType := detectPythonProcessType(pid, projectDir)

		result += fmt.Sprintf("  └─ %s\n", projectDir)
		
		// プロセス種別、稼働時間、CPU・メモリを表示
		infoLine := "     "
		if processType != "" {
			infoLine += fmt.Sprintf("(%s)", processType)
		}
		if uptime != "" {
			if processType != "" {
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

// detectPythonProcessType detects what type of Python process is running
func detectPythonProcessType(pid, projectDir string) string {
	// コマンドライン引数を取得
	cmd := exec.Command("ps", "-o", "command=", "-p", pid)
	output, err := cmd.Output()

	if err != nil {
		return "Python"
	}

	cmdLine := strings.ToLower(string(output))

	if strings.Contains(cmdLine, "jupyter") {
		return "Jupyter Notebook"
	}
	if strings.Contains(cmdLine, "flask") {
		return "Flask"
	}
	if strings.Contains(cmdLine, "django") {
		return "Django"
	}
	if strings.Contains(cmdLine, "uvicorn") || strings.Contains(cmdLine, "fastapi") {
		return "FastAPI"
	}
	if strings.Contains(cmdLine, "streamlit") {
		return "Streamlit"
	}

	return "Python"
}
