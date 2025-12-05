package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckDocker checks if Docker is running and counts containers
func CheckDocker() string {
	cmd := exec.Command("docker", "ps", "-q")
	output, err := cmd.Output()

	if err != nil {
		return "✗ Docker: 停止中"
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := len(lines)

	// 空の場合は0個
	if count == 1 && lines[0] == "" {
		count = 0
	}

	if count == 0 {
		return "✓ Docker: 実行中（コンテナ0個）"
	}

	return fmt.Sprintf("✓ Docker: %d個のコンテナ", count)
}
