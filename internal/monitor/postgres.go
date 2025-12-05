package monitor

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CheckPostgres checks if PostgreSQL is running
func CheckPostgres() string {
	cmd := exec.Command("pgrep", "postgres")
	err := cmd.Run()

	if err != nil {
		return "✗ PostgreSQL: 停止中"
	}

	// ポート番号取得
	port := getPortByProcess("postgres")
	
	// ポート情報を含める
	portInfo := ""
	if port != "" {
		portInfo = " [:" + port + "]"
	}
	
	// 稼働時間取得
	uptime := getPostgresUptime()
	
	// CPU・メモリ使用量取得
	stats := getMultiProcessStats("postgres")
	
	// データベース詳細情報取得
	databases := getPostgresDatabaseDetails()
	
	result := fmt.Sprintf("✓ PostgreSQL: 実行中%s", portInfo)
	if uptime != "" {
		result += fmt.Sprintf(" | 稼働: %s", uptime)
	}
	
	// CPU・メモリ情報追加
	statsStr := formatStatsString(stats)
	if statsStr != "" {
		result += fmt.Sprintf(" | %s", statsStr)
	}
	
	result += "\n"
	
	// データベース情報
	if len(databases) > 0 {
		for _, db := range databases {
			result += fmt.Sprintf("  - %s\n", db)
		}
	}
	
	return result
}

// getPostgresDatabaseDetails returns detailed info for each database
func getPostgresDatabaseDetails() []string {
	// データベース名、サイズ、作成日時を取得
	sizeQuery := `
		SELECT 
			d.datname,
			pg_size_pretty(pg_database_size(d.datname)) as size,
			(pg_stat_file('base/'||d.oid ||'/PG_VERSION')).modification as created
		FROM pg_database d
		WHERE d.datistemplate = false
		ORDER BY d.datname;
	`
	
	cmd := exec.Command("psql", "-d", "postgres", "-c", sizeQuery, "-t", "-A", "-F", "|")
	output, err := cmd.Output()
	
	if err != nil {
		return getPostgresDatabasesBasic()
	}
	
	// 最終接続時刻を別途取得
	accessQuery := `SELECT datname, stats_reset FROM pg_stat_database WHERE datname NOT IN ('template0', 'template1');`
	accessCmd := exec.Command("psql", "-d", "postgres", "-c", accessQuery, "-t", "-A", "-F", "|")
	accessOutput, _ := accessCmd.Output()
	
	// 最終接続時刻をマップに格納
	accessMap := make(map[string]string)
	if accessOutput != nil {
		accessLines := strings.Split(string(accessOutput), "\n")
		for _, line := range accessLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 2 {
				dbName := strings.TrimSpace(parts[0])
				accessTime := strings.TrimSpace(parts[1])
				accessMap[dbName] = accessTime
			}
		}
	}
	
	var databases []string
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			dbName := strings.TrimSpace(parts[0])
			dbSize := strings.TrimSpace(parts[1])
			created := strings.TrimSpace(parts[2])
			
			// 日時フォーマット整形
			createdStr := formatTimestamp(created)
			
			// 最終接続時刻を取得
			lastAccess := accessMap[dbName]
			lastAccessStr := formatTimeAgo(lastAccess)
			
			dbInfo := fmt.Sprintf("%s (%s) | 作成: %s | 最終接続: %s", 
				dbName, dbSize, createdStr, lastAccessStr)
			databases = append(databases, dbInfo)
		}
	}
	
	return databases
}

// formatTimestamp formats timestamp to readable format
func formatTimestamp(timestamp string) string {
	if timestamp == "" {
		return "不明"
	}
	
	// PostgreSQLのタイムスタンプをパース
	t, err := time.Parse("2006-01-02 15:04:05-07", timestamp)
	if err != nil {
		return timestamp
	}
	
	return t.Format("2006-01-02")
}

// formatTimeAgo converts timestamp to "X分前" format
func formatTimeAgo(timestamp string) string {
	if timestamp == "" {
		return "不明"
	}
	
	// PostgreSQLのタイムスタンプをパース（マイクロ秒対応）
	t, err := time.Parse("2006-01-02 15:04:05.999999-07", timestamp)
	if err != nil {
		// マイクロ秒なしも試す
		t, err = time.Parse("2006-01-02 15:04:05-07", timestamp)
		if err != nil {
			return "不明"
		}
	}
	
	duration := time.Since(t)
	
	if duration < time.Minute {
		return "1分以内"
	} else if duration < time.Hour {
		return fmt.Sprintf("%.0f分前", duration.Minutes())
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%.0f時間前", duration.Hours())
	} else {
		return fmt.Sprintf("%.0f日前", duration.Hours()/24)
	}
}

// getPostgresDatabasesBasic returns basic database list (fallback)
func getPostgresDatabasesBasic() []string {
	cmd := exec.Command("psql", "-d", "postgres", "-l", "-t")
	output, err := cmd.Output()
	
	if err != nil {
		return []string{}
	}
	
	var databases []string
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "(") {
			continue
		}
		
		fields := strings.Split(line, "|")
		if len(fields) > 0 {
			dbName := strings.TrimSpace(fields[0])
			if dbName != "" && dbName != "template0" && dbName != "template1" {
				databases = append(databases, dbName)
			}
		}
	}
	
	return databases
}

// getPortByProcess finds the port for a given process name
func getPortByProcess(processName string) string {
	cmd := exec.Command("lsof", "-i", "-P", "-n")
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		if !strings.Contains(line, processName) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		portInfo := fields[8]
		if strings.Contains(portInfo, ":") {
			parts := strings.Split(portInfo, ":")
			return parts[len(parts)-1]
		}
	}

	return ""
}
// getPostgresUptime returns how long PostgreSQL has been running
func getPostgresUptime() string {
	cmd := exec.Command("sh", "-c", "ps -o etime= -p $(pgrep postgres | head -1)")
	output, err := cmd.Output()
	
	if err != nil {
		return ""
	}
	
	uptime := strings.TrimSpace(string(output))
	return uptime
}
