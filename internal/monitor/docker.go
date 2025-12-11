package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

// DockerContainer represents a Docker container
type DockerContainer struct {
	ID             string
	Name           string
	Status         string
	Image          string
	ComposeProject string // Composeプロジェクト名（空の場合は単体）
	ComposeService string // Composeサービス名
	ProjectDir     string // プロジェクトディレクトリ（Composeの場合はdocker-compose.ymlのあるディレクトリ）
	Port           string // 公開されているポート番号
}

// CheckDocker checks if Docker is running and counts containers
func CheckDocker() string {
	cmd := exec.Command("docker", "ps", "-q")
	output, err := cmd.Output()

	if err != nil {
		return "✗ Docker: 停止中"
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := len(lines)

	if count == 1 && lines[0] == "" {
		count = 0
	}

	if count == 0 {
		return "✓ Docker: 実行中（コンテナ0個）"
	}

	// コンテナ詳細情報を取得
	containers := getDockerContainerDetails()

	result := fmt.Sprintf("✓ Docker: %d個のコンテナ\n", count)
	for _, container := range containers {
		result += container
	}

	return result
}

// getDockerContainerDetails returns detailed info for each container
func getDockerContainerDetails() []string {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}|{{.Ports}}|{{.Status}}|{{.Image}}|{{.ID}}")
	output, err := cmd.Output()

	if err != nil {
		return []string{}
	}

	var containers []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 5 {
			name := strings.TrimSpace(parts[0])
			ports := strings.TrimSpace(parts[1])
			status := strings.TrimSpace(parts[2])
			image := strings.TrimSpace(parts[3])
			containerID := strings.TrimSpace(parts[4])

			// ポート情報を簡略化
			portInfo := extractMainPort(ports)

			// CPU・メモリ使用量取得
			stats := GetDockerContainerStats(containerID)

			// イメージサイズ取得
			imageSize := GetDockerImageSize(image)

			// 基本情報
			containerInfo := fmt.Sprintf("  - %s [%s] | %s", name, portInfo, status)
			
			// CPU・メモリ情報追加
			statsStr := formatDockerStatsString(stats)
			if statsStr != "" {
				containerInfo += fmt.Sprintf(" | %s", statsStr)
			}
			containerInfo += "\n"

			// イメージ名 + サイズ
			imageInfo := image
			if imageSize != "" {
				imageInfo += fmt.Sprintf(" (%s)", imageSize)
			}
			containerInfo += fmt.Sprintf("    └─ Image: %s\n", imageInfo)

			// WorkDir取得
			workDir := getContainerWorkDir(name)
			if workDir != "" {
				containerInfo += fmt.Sprintf("    └─ WorkDir: %s\n", workDir)
			}

			// マウント情報取得
			mounts := getContainerMounts(name)
			if len(mounts) > 0 {
				mainMount := mounts[0]
				containerInfo += fmt.Sprintf("    └─ Mount: %s\n", mainMount)
			}

			containers = append(containers, containerInfo)
		}
	}

	return containers
}

// GetDockerImageSize returns the size of a Docker image
func GetDockerImageSize(imageName string) string {
	cmd := exec.Command("docker", "images", imageName, "--format", "{{.Size}}")
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// DockerStats holds Docker container stats
type DockerStats struct {
	CPUPerc string
	MemUsage string
}

// GetDockerContainerStats returns CPU and memory stats for a container
func GetDockerContainerStats(containerID string) DockerStats {
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}|{{.MemUsage}}", containerID)
	output, err := cmd.Output()

	if err != nil {
		return DockerStats{}
	}

	line := strings.TrimSpace(string(output))
	parts := strings.Split(line, "|")

	if len(parts) >= 2 {
		return DockerStats{
			CPUPerc: strings.TrimSpace(parts[0]),
			MemUsage: strings.TrimSpace(parts[1]),
		}
	}

	return DockerStats{}
}

// formatDockerStatsString formats Docker stats to string
func formatDockerStatsString(stats DockerStats) string {
	if stats.CPUPerc == "" && stats.MemUsage == "" {
		return ""
	}
	
	result := ""
	if stats.CPUPerc != "" {
		result += fmt.Sprintf("CPU: %s", stats.CPUPerc)
	}
	if stats.MemUsage != "" {
		if result != "" {
			result += " | "
		}
		result += fmt.Sprintf("メモリ: %s", stats.MemUsage)
	}
	
	return result
}

// getContainerWorkDir returns the working directory of a container
func getContainerWorkDir(containerName string) string {
	cmd := exec.Command("docker", "inspect", "--format", "{{.Config.WorkingDir}}", containerName)
	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// getContainerMounts returns mount information for a container
func getContainerMounts(containerName string) []string {
	cmd := exec.Command("docker", "inspect", "--format", "{{range .Mounts}}{{.Source}} -> {{.Destination}}{{\"\\n\"}}{{end}}", containerName)
	output, err := cmd.Output()

	if err != nil {
		return []string{}
	}

	var mounts []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// /var/lib/docker/volumes/ で始まるボリュームは除外（内部ボリューム）
		if strings.HasPrefix(line, "/var/lib/docker/volumes/") {
			continue
		}

		// ホームディレクトリのパスのみ表示
		if strings.Contains(line, "/Users/") || strings.Contains(line, "/home/") {
			mounts = append(mounts, line)
		}
	}

	// 最初の1つだけ返す（メインのマウント）
	if len(mounts) > 0 {
		// 共通プレフィックスを見つける（プロジェクトルート）
		firstMount := mounts[0]
		parts := strings.Split(firstMount, " -> ")
		if len(parts) >= 1 {
			hostPath := parts[0]
			// プロジェクトルートを抽出（最後のディレクトリの親）
			pathParts := strings.Split(hostPath, "/")
			if len(pathParts) >= 2 {
				projectRoot := strings.Join(pathParts[:len(pathParts)-1], "/")
				return []string{projectRoot}
			}
		}
		return []string{firstMount}
	}

	return []string{}
}

// extractMainPort extracts the main exposed port from Docker ports string
func extractMainPort(ports string) string {
	if ports == "" {
		return "no ports"
	}

	if strings.Contains(ports, "->") {
		parts := strings.Split(ports, "->")
		if len(parts) >= 2 {
			portPart := strings.Split(parts[1], "/")[0]
			portPart = strings.Split(portPart, ",")[0]
			return ":" + strings.TrimSpace(portPart)
		}
	}

	if strings.Contains(ports, ":") {
		parts := strings.Split(ports, ":")
		if len(parts) >= 2 {
			port := strings.Split(parts[1], ",")[0]
			return ":" + strings.TrimSpace(port)
		}
	}

	return ports
}

// GetDockerContainers returns list of all Docker containers (simple version)
func GetDockerContainers() []DockerContainer {
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Status}}|{{.Image}}")
	output, err := cmd.Output()

	if err != nil {
		return []DockerContainer{}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var containers []DockerContainer

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		status := "exited"
		if strings.Contains(parts[2], "Up") {
			status = "running"
		}

		containerID := parts[0]

		// Compose情報を取得（軽量版）
		composeProject, composeService := getComposeInfo(containerID)

		// プロジェクトディレクトリを取得（Composeの場合）
		projectDir := ""
		if composeProject != "" {
			projectDir = getContainerProjectDir(containerID)
		}

		// ポート情報を取得
		port := getContainerPort(containerID)

		containers = append(containers, DockerContainer{
			ID:             containerID,
			Name:           parts[1],
			Status:         status,
			Image:          parts[3],
			ComposeProject: composeProject,
			ComposeService: composeService,
			ProjectDir:     projectDir,
			Port:           port,
		})
	}

	return containers
}

// getComposeInfo returns compose project and service for a container (lightweight)
func getComposeInfo(containerID string) (project, service string) {
	// Composeプロジェクト名を取得
	cmd := exec.Command("docker", "inspect", containerID, "--format", "{{index .Config.Labels \"com.docker.compose.project\"}}")
	output, err := cmd.Output()
	if err == nil {
		project = strings.TrimSpace(string(output))
		if project == "<no value>" {
			project = ""
		}
	}

	// Composeサービス名を取得
	cmd = exec.Command("docker", "inspect", containerID, "--format", "{{index .Config.Labels \"com.docker.compose.service\"}}")
	output, err = cmd.Output()
	if err == nil {
		service = strings.TrimSpace(string(output))
		if service == "<no value>" {
			service = ""
		}
	}

	return project, service
}

// IsComposeContainer checks if a container is part of a compose project
func IsComposeContainer(container DockerContainer) bool {
	return container.ComposeProject != ""
}

// getContainerProjectDir returns the project directory for a compose container
func getContainerProjectDir(containerID string) string {
	cmd := exec.Command("docker", "inspect", containerID, "--format", "{{index .Config.Labels \"com.docker.compose.project.working_dir\"}}")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	workDir := strings.TrimSpace(string(output))
	if workDir == "" || workDir == "<no value>" {
		return ""
	}

	return workDir
}

// getContainerPort returns the exposed port for a container
func getContainerPort(containerID string) string {
	cmd := exec.Command("docker", "port", containerID)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return ""
	}

	// 最初のポートマッピングを取得（例: "3000/tcp -> 0.0.0.0:3000"）
	firstLine := lines[0]
	if strings.Contains(firstLine, "->") {
		parts := strings.Split(firstLine, "->")
		if len(parts) >= 2 {
			portPart := strings.TrimSpace(parts[1])
			// "0.0.0.0:3000" から "3000" を抽出
			if strings.Contains(portPart, ":") {
				portNum := strings.Split(portPart, ":")[1]
				return portNum
			}
		}
	}

	return ""
}
