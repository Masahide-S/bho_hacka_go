package monitor

import (
	"fmt"
	"os/exec"
)

// CheckMySQL checks if MySQL is running
func CheckMySQL() string {
	cmd := exec.Command("pgrep", "mysqld")
	err := cmd.Run()

	if err != nil {
		return "✗ MySQL: 停止中"
	}

	// ポート番号取得
	port := getPortByProcess("mysqld")
	portInfo := ""
	if port != "" {
		portInfo = " [:" + port + "]"
	}

	// CPU・メモリ使用量取得
	stats := getMultiProcessStats("mysqld")

	result := fmt.Sprintf("✓ MySQL: 実行中%s", portInfo)

	// CPU・メモリ情報追加
	statsStr := formatStatsString(stats)
	if statsStr != "" {
		result += fmt.Sprintf(" | %s", statsStr)
	}

	return result
}
