package monitor

import (
	"sort"
	"strconv"
	"strings"
)

// PortInfo holds information about a listening port
type PortInfo struct {
	Port        string
	Process     string
	PID         string
	BindAddress string // バインドアドレス（127.0.0.1, *, ::1など）
	ProjectName string // プロジェクト名（Dockerコンテナの場合）
	URL         string // アクセス可能なURL
}

// GetListeningPorts returns all listening ports
func GetListeningPorts() []PortInfo {
	// タイムアウト付きで実行（lsofはハングしやすいため）
	output, err := RunCommandWithTimeout("lsof", "-i", "-P", "-n")

	if err != nil {
		return []PortInfo{}
	}
	
	var ports []PortInfo
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		if !strings.Contains(line, "LISTEN") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		portInfo := fields[8]
		if strings.Contains(portInfo, ":") {
			parts := strings.Split(portInfo, ":")
			port := parts[len(parts)-1]

			// バインドアドレスを取得
			bindAddress := "*"
			if len(parts) >= 2 {
				bindAddress = strings.Join(parts[:len(parts)-1], ":")
			}

			processName := fields[0]
			pid := fields[1]

			// プロジェクト名を取得（Dockerの場合）
			projectName := processName
			if strings.HasPrefix(processName, "com.docke") {
				// Dockerコンテナの場合、コンテナ名またはプロジェクト名を取得
				projectName = getDockerProjectNameByPID(pid)
			}

			// URLを生成
			url := generateURL(bindAddress, port)

			ports = append(ports, PortInfo{
				Port:        port,
				Process:     processName,
				PID:         pid,
				BindAddress: bindAddress,
				ProjectName: projectName,
				URL:         url,
			})
		}
	}

	// ポート番号でソート（昇順）
	sort.Slice(ports, func(i, j int) bool {
		portI, errI := strconv.Atoi(ports[i].Port)
		portJ, errJ := strconv.Atoi(ports[j].Port)

		// 数値変換に失敗した場合は文字列比較
		if errI != nil || errJ != nil {
			return ports[i].Port < ports[j].Port
		}

		return portI < portJ
	})

	return ports
}

// ListAllPorts displays all listening ports
func ListAllPorts() string {
	ports := GetListeningPorts()

	if len(ports) == 0 {
		return "ポート: 検出なし"
	}

	result := "使用中のポート:\n"
	for _, p := range ports {
		displayName := p.ProjectName
		if displayName == "" {
			displayName = p.Process
		}

		result += "  :" + p.Port + " - " + displayName
		if p.URL != "" {
			result += " (" + p.URL + ")"
		}
		result += "\n"
	}

	return result
}

// getDockerProjectNameByPID returns the Docker project/container name by PID
func getDockerProjectNameByPID(pid string) string {
	// PIDのバリデーション
	if !IsValidPID(pid) {
		return "Docker"
	}

	// タイムアウト付きでpsコマンドを実行
	output, err := RunCommandWithTimeout("ps", "-p", pid, "-o", "command=")
	if err != nil {
		return "Docker"
	}

	cmdLine := string(output)

	// Dockerコンテナのプロセスの場合、docker psでコンテナ情報を取得
	containers := GetDockerContainers()
	for _, container := range containers {
		// コンテナIDやプロセス情報から一致するものを探す
		// 簡易的な実装として、実行中のコンテナのポート情報と照合
		if container.ComposeProject != "" {
			return container.ComposeProject
		}
	}

	// コンテナ名が取得できない場合はDocker
	if strings.Contains(cmdLine, "docker") {
		return "Docker"
	}

	return "Docker"
}

// generateURL generates an accessible URL based on bind address and port
func generateURL(bindAddress, port string) string {
	// バインドアドレスに応じてURLを生成
	if bindAddress == "*" || bindAddress == "0.0.0.0" {
		return "http://localhost:" + port
	} else if bindAddress == "127.0.0.1" || bindAddress == "localhost" {
		return "http://localhost:" + port
	} else if bindAddress == "::1" {
		return "http://[::1]:" + port
	} else if bindAddress == "::" || bindAddress == "[::]" {
		return "http://localhost:" + port
	} else {
		// 特定のIPアドレスにバインドされている場合
		return "http://" + bindAddress + ":" + port
	}
}
