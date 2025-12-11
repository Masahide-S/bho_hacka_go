package logs

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// GetContainerLogs returns the last N lines of container logs (optimized)
func GetContainerLogs(containerID string, lines int) (string, error) {
	// タイムアウト付きコンテキスト（3秒）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// docker logs <container_id> --tail <lines> --since 1h (最近1時間のみ)
	// --sinceオプションで大量のログがある場合の検索時間を短縮
	cmd := exec.CommandContext(ctx, "docker", "logs", containerID, "--tail", fmt.Sprintf("%d", lines), "--since", "1h")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// タイムアウトの場合
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("ログ取得タイムアウト（3秒）")
		}
		return "", fmt.Errorf("ログ取得失敗: %s", string(output))
	}

	return string(output), nil
}
