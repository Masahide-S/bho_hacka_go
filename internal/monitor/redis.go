package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// RedisDatabase represents a Redis database
type RedisDatabase struct {
	Index   string
	KeysNum string
}

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

// GetRedisDatabases returns list of Redis databases
func GetRedisDatabases() []RedisDatabase {
	// Redis INFOコマンドでデータベース情報を取得
	cmd := exec.Command("redis-cli", "INFO", "keyspace")
	output, err := cmd.Output()

	if err != nil {
		return []RedisDatabase{}
	}

	var databases []RedisDatabase
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "db") {
			continue
		}

		// db0:keys=100,expires=0,avg_ttl=0 のような形式
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		dbIndex := parts[0]
		info := parts[1]

		// keys=100 を抽出
		keysNum := "0"
		infoParts := strings.Split(info, ",")
		for _, part := range infoParts {
			if strings.HasPrefix(part, "keys=") {
				keysNum = strings.TrimPrefix(part, "keys=")
				break
			}
		}

		databases = append(databases, RedisDatabase{
			Index:   dbIndex,
			KeysNum: keysNum + " keys",
		})
	}

	return databases
}
