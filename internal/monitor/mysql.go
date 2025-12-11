package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// MySQLDatabase represents a MySQL database
type MySQLDatabase struct {
	Name string
	Size string
}

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

// GetMySQLDatabases returns list of MySQL databases
func GetMySQLDatabases() []MySQLDatabase {
	// データベース一覧を取得
	query := "SELECT table_schema as 'Database', ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) as 'Size (MB)' FROM information_schema.TABLES GROUP BY table_schema;"

	cmd := exec.Command("mysql", "-N", "-e", query)
	output, err := cmd.Output()

	if err != nil {
		return []MySQLDatabase{}
	}

	var databases []MySQLDatabase
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			dbName := parts[0]
			// システムデータベースはスキップ
			if dbName == "information_schema" || dbName == "performance_schema" || dbName == "mysql" || dbName == "sys" {
				continue
			}

			dbSize := parts[1] + " MB"
			databases = append(databases, MySQLDatabase{
				Name: dbName,
				Size: dbSize,
			})
		}
	}

	return databases
}
