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
			stats := getDockerContainerStats(containerID)

			// イメージサイズ取得
			imageSize := getDockerImageSize(image)

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

// getDockerImageSize returns the size of a Docker image
func getDockerImageSize(imageName string) string {
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

// getDockerContainerStats returns CPU and memory stats for a container
func getDockerContainerStats(containerID string) DockerStats {
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
