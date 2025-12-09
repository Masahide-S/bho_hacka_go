package monitor

import (
	"fmt"
	"os/exec"
)

// CheckRedis checks if Redis is running
func CheckRedis() string {
	cmd := exec.Command("pgrep", "redis-server")
	err := cmd.Run()

	if err != nil {
		return "✗ Redis: 停止中"
	}

	// ポート番号取得
	port := getPortByProcess("redis-server")
	portInfo := ""
	if port != "" {
		portInfo = " [:" + port + "]"
	}

	// CPU・メモリ使用量取得
	stats := getMultiProcessStats("redis-server")

	result := fmt.Sprintf("✓ Redis: 実行中%s", portInfo)

	// CPU・メモリ情報追加
	statsStr := formatStatsString(stats)
	if statsStr != "" {
		result += fmt.Sprintf(" | %s", statsStr)
	}

	return result
}
