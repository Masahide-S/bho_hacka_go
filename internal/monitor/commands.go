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
		case "toggle_project":
			// 起動/停止はupとdownで実現
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "up", "-d")
		case "restart_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "restart")
		case "rebuild_project":
			cmd = exec.Command("docker-compose", "-f", workDir+"/docker-compose.yml", "up", "-d", "--build")
		default:
			return CommandResult{Success: false, Message: "不明なアクション"}
		}
	} else {
		// docker compose v2
		switch action {
		case "toggle_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "up", "-d")
		case "restart_project":
			cmd = exec.Command("docker", "compose", "-f", workDir+"/docker-compose.yml", "restart")
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
	case "toggle_project":
		actionJP = "起動"
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
	default:
		return action
	}
}
