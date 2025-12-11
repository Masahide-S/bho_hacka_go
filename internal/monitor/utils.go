package monitor

import (
	"context"
	"os/exec"
	"regexp"
	"time"
)

// DefaultTimeout はデフォルトのタイムアウト時間
const DefaultTimeout = 3 * time.Second

// RunCommandWithTimeout はタイムアウト付きでコマンドを実行します
func RunCommandWithTimeout(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// RunCommandWithTimeoutCombined はタイムアウト付きでコマンドを実行し、stdout/stderrを結合して返します
func RunCommandWithTimeoutCombined(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// RunCommandWithCustomTimeout はカスタムタイムアウトでコマンドを実行します
func RunCommandWithCustomTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// IsValidIdentifier は識別子（データベース名など）が安全かチェックします
// 英数字、アンダースコア、ハイフンのみ許可
func IsValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", s)
	return matched
}

// IsValidPID はPIDが安全かチェックします
// 数字のみ許可
func IsValidPID(s string) bool {
	if s == "" {
		return false
	}
	matched, _ := regexp.MatchString("^[0-9]+$", s)
	return matched
}

// IsValidContainerID はコンテナIDが安全かチェックします
// 16進数のみ許可（短縮ID・完全ID両対応）
func IsValidContainerID(s string) bool {
	if s == "" {
		return false
	}
	matched, _ := regexp.MatchString("^[a-f0-9]+$", s)
	return matched
}

// IsServiceRunning はタイムアウト付きでサービスが起動しているかチェックします
func IsServiceRunning(processName string) bool {
	_, err := RunCommandWithTimeout("pgrep", processName)
	return err == nil
}
