package monitor

import (
	"os/exec"
)

// CheckPostgres checks if PostgreSQL is running
func CheckPostgres() string {
	cmd := exec.Command("pgrep", "postgres")
	err := cmd.Run()

	if err == nil {
		return "✓ PostgreSQL: 実行中"
	}
	return "✗ PostgreSQL: 停止中"
}
