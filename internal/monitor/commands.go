package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// CommandResult holds the result of command execution
type CommandResult struct {
	Success bool
	Message string
}

// ExecuteDockerCommand executes a Docker command
func ExecuteDockerCommand(target, action, targetType string) CommandResult {
	if targetType == "project" {
		return executeComposeProjectCommand(target, action)
	} else {
		return executeDockerContainerCommand(target, action)
	}
}

// executeDockerContainerCommand executes command on a specific container
func executeDockerContainerCommand(containerID, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "start":
		cmd = exec.Command("docker", "start", containerID)
	case "stop":
		cmd = exec.Command("docker", "stop", containerID)
	case "restart":
		cmd = exec.Command("docker", "restart", containerID)
	case "rebuild":
		// 個別コンテナのリビルドはdocker-compose経由で実行
		return executeContainerRebuild(containerID)
	case "remove":
		// コンテナを削除（-vオプションで関連ボリュームも削除）
		cmd = exec.Command("docker", "rm", "-f", containerID)
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("コンテナ操作失敗: %s", string(output)),
		}
	}

	actionJP := getActionJapanese(action)
	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("コンテナを%sしました", actionJP),
	}
}

// executeComposeProjectCommand executes command on entire compose project
func executeComposeProjectCommand(projectName, action string) CommandResult {
	// プロジェクトのコンテナを取得してワーキングディレクトリを見つける
	workDir := findComposeWorkDir(projectName)
	if workDir == "" {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("プロジェクト %s の作業ディレクトリが見つかりません", projectName),
		}
	}

	// docker-composeコマンドを判定
	composeCmd := getComposeCommand()
	if composeCmd == "" {
		return CommandResult{
			Success: false,
			Message: "docker-compose または docker compose コマンドが見つかりません",
		}
	}

	var cmd *exec.Cmd

	// docker-compose v1 or v2
	if strings.HasPrefix(composeCmd, "docker-compose") {
		switch action {
		case "start_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "up", "-d")
		case "stop_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "stop")
		case "delete_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "down")
		case "restart_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "up", "-d")
		case "rebuild_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "up", "-d", "--build")
		default:
			return CommandResult{Success: false, Message: "不明なアクション"}
		}
	} else {
		// docker compose v2
		switch action {
		case "start_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "up", "-d")
		case "stop_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "stop")
		case "delete_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "down")
		case "restart_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "up", "-d")
		case "rebuild_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "up", "-d", "--build")
		default:
			return CommandResult{Success: false, Message: "不明なアクション"}
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("Compose操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "start_project":
		actionJP = "起動"
	case "stop_project":
		actionJP = "停止"
	case "delete_project":
		actionJP = "削除"
	case "restart_project":
		actionJP = "再起動"
	case "rebuild_project":
		actionJP = "リビルド"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("プロジェクト %s を%sしました", projectName, actionJP),
	}
}

// executeContainerRebuild rebuilds a specific container in compose
func executeContainerRebuild(containerID string) CommandResult {
	// コンテナ情報を取得
	containers := GetDockerContainers()
	var targetContainer *DockerContainer
	for i := range containers {
		if containers[i].ID == containerID {
			targetContainer = &containers[i]
			break
		}
	}

	if targetContainer == nil || targetContainer.ComposeProject == "" {
		return CommandResult{Success: false, Message: "Composeコンテナではありません"}
	}

	workDir := findComposeWorkDir(targetContainer.ComposeProject)
	if workDir == "" {
		return CommandResult{Success: false, Message: "作業ディレクトリが見つかりません"}
	}

	composeCmd := getComposeCommand()
	if composeCmd == "" {
		return CommandResult{Success: false, Message: "docker-composeコマンドが見つかりません"}
	}

	var cmd *exec.Cmd
	if strings.HasPrefix(composeCmd, "docker-compose") {
		cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "up", "-d", "--build", targetContainer.ComposeService)
	} else {
		cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "up", "-d", "--build", targetContainer.ComposeService)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("リビルド失敗: %s", string(output)),
		}
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("サービス %s をリビルドしました", targetContainer.ComposeService),
	}
}

// findComposeWorkDir finds the working directory for a compose project
func findComposeWorkDir(projectName string) string {
	containers := GetDockerContainers()
	for _, c := range containers {
		if c.ComposeProject == projectName {
			// docker inspectでworking dirを取得
			cmd := exec.Command("docker", "inspect", c.ID, "--format", "{{index .Config.Labels \"com.docker.compose.project.working_dir\"}}")
			output, err := cmd.Output()
			if err == nil {
				workDir := strings.TrimSpace(string(output))
				if workDir != "" && workDir != "<no value>" {
					return workDir
				}
			}
		}
	}
	return ""
}

// getComposeCommand returns the available docker-compose command
func getComposeCommand() string {
	// docker-compose (v1) をチェック
	cmd := exec.Command("which", "docker-compose")
	if err := cmd.Run(); err == nil {
		return "docker-compose"
	}

	// docker compose (v2) をチェック
	cmd = exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		return "docker compose"
	}

	return ""
}

// getActionJapanese converts action to Japanese
func getActionJapanese(action string) string {
	switch action {
	case "start":
		return "起動"
	case "stop":
		return "停止"
	case "restart":
		return "再起動"
	case "rebuild":
		return "リビルド"
	case "remove":
		return "削除"
	default:
		return action
	}
}

// ExecutePostgresCommand executes a PostgreSQL command
func ExecutePostgresCommand(databaseName, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "drop":
		// データベースを削除
		cmd = exec.Command("dropdb", databaseName)
	case "vacuum":
		// VACUUM実行
		cmd = exec.Command("psql", "-d", databaseName, "-c", "VACUUM;")
	case "analyze":
		// ANALYZE実行
		cmd = exec.Command("psql", "-d", databaseName, "-c", "ANALYZE;")
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("データベース操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "drop":
		actionJP = "削除"
	case "vacuum":
		actionJP = "VACUUM実行"
	case "analyze":
		actionJP = "ANALYZE実行"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("データベース %s を%sしました", databaseName, actionJP),
	}
}

// ExecuteNodeCommand executes a Node.js process command
func ExecuteNodeCommand(pid, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "kill":
		// プロセスを停止
		cmd = exec.Command("kill", pid)
	case "force_kill":
		// プロセスを強制停止
		cmd = exec.Command("kill", "-9", pid)
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("プロセス操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "kill":
		actionJP = "停止"
	case "force_kill":
		actionJP = "強制停止"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("Node.jsプロセス (PID: %s) を%sしました", pid, actionJP),
	}
}

// ExecuteMySQLCommand executes a MySQL command
func ExecuteMySQLCommand(databaseName, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "drop":
		// データベースを削除
		cmd = exec.Command("mysql", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", databaseName))
	case "optimize":
		// データベースを最適化
		cmd = exec.Command("mysql", "-e", fmt.Sprintf("OPTIMIZE TABLE %s.*;", databaseName))
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("データベース操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "drop":
		actionJP = "削除"
	case "optimize":
		actionJP = "最適化"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("データベース %s を%sしました", databaseName, actionJP),
	}
}

// ExecuteRedisCommand executes a Redis command
func ExecuteRedisCommand(dbIndex, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "flushdb":
		// データベースをクリア
		dbNum := strings.TrimPrefix(dbIndex, "db")
		cmd = exec.Command("redis-cli", "-n", dbNum, "FLUSHDB")
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("Redis操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "flushdb":
		actionJP = "クリア"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("Redis %s を%sしました", dbIndex, actionJP),
	}
}

// ExecutePythonCommand executes a Python process command
func ExecutePythonCommand(pid, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "kill":
		// プロセスを停止
		cmd = exec.Command("kill", pid)
	case "force_kill":
		// プロセスを強制停止
		cmd = exec.Command("kill", "-9", pid)
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("プロセス操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "kill":
		actionJP = "停止"
	case "force_kill":
		actionJP = "強制停止"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("Pythonプロセス (PID: %s) を%sしました", pid, actionJP),
	}
}

// ExecutePortCommand executes a command on a port (process)
func ExecutePortCommand(pid, action string) CommandResult {
	var cmd *exec.Cmd

	switch action {
	case "kill_port":
		// プロセスを停止
		cmd = exec.Command("kill", pid)
	case "force_kill_port":
		// プロセスを強制停止
		cmd = exec.Command("kill", "-9", pid)
	default:
		return CommandResult{Success: false, Message: "不明なアクション"}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("プロセス操作失敗: %s", string(output)),
		}
	}

	actionJP := ""
	switch action {
	case "kill_port":
		actionJP = "停止"
	case "force_kill_port":
		actionJP = "強制停止"
	}

	return CommandResult{
		Success: true,
		Message: fmt.Sprintf("プロセス (PID: %s) を%sしました", pid, actionJP),
	}
}

// CleanDanglingImages removes all dangling images
func CleanDanglingImages() CommandResult {
	cmd := exec.Command("docker", "image", "prune", "-f")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return CommandResult{
			Success: false,
			Message: fmt.Sprintf("ダングリングイメージの削除失敗: %s", string(output)),
		}
	}

	return CommandResult{
		Success: true,
		Message: "ダングリングイメージを削除しました",
	}
}
