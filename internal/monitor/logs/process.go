package logs

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetProcessLogs returns the last N lines of process logs from project directory
func GetProcessLogs(projectDir string, lines int) (string, error) {
	if projectDir == "" {
		return "", fmt.Errorf("プロジェクトディレクトリが見つかりません")
	}

	// 一般的なログファイルのパスを試す
	logPaths := []string{
		projectDir + "/logs/*.log",
		projectDir + "/*.log",
		projectDir + "/.log/*.log",
		projectDir + "/log/*.log",
		projectDir + "/npm-debug.log",
		projectDir + "/yarn-error.log",
	}

	// 最初に見つかったログファイルを使用
	var logFile string
	for _, pattern := range logPaths {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("ls -t %s 2>/dev/null | head -1", pattern))
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			logFile = strings.TrimSpace(string(output))
			break
		}
	}

	if logFile == "" {
		return "", fmt.Errorf("ログファイルが見つかりません")
	}

	// tailコマンドで最後のN行を取得
	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), logFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("ログ取得失敗: %s", string(output))
	}

	return string(output), nil
}
