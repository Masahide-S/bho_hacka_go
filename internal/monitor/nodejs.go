package monitor

import (
	"os/exec"
)

// CheckNodejs checks if Node.js process is running
func CheckNodejs() string {
	cmd := exec.Command("pgrep", "node")
	err := cmd.Run()

	if err == nil {
		return "✓ Node.js: 実行中"
	}
	return "✗ Node.js: 検出なし"
}
