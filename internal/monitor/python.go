package monitor

import (
	"os/exec"
)

// CheckPython checks if Python process is running
func CheckPython() string {
	cmd := exec.Command("pgrep", "python")
	err := cmd.Run()

	if err == nil {
		return "✓ Python: 実行中"
	}
	return "✗ Python: 検出なし"
}
