package ui

import (
	"os/exec"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor/logs"
	tea "github.com/charmbracelet/bubbletea"
)

// handleProjectToggle toggles project expand/collapse
func (m Model) handleProjectToggle() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロジェクトの場合のみトグル
	if selectedItem.Type == "project" {
		// IsExpandedを反転
		m.rightPanelItems[m.rightPanelCursor].IsExpanded = !m.rightPanelItems[m.rightPanelCursor].IsExpanded

		// 表示を再構築
		m = m.updateRightPanelItems()
	}

	return m, nil
}

// handleContainerToggle handles start/stop toggle
func (m Model) handleContainerToggle() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	if selectedItem.Type == "project" {
		// プロジェクト全体の操作
		// プロジェクト内のコンテナの状態をチェック
		containers := m.cachedContainers
		runningCount := 0
		totalCount := 0
		for _, c := range containers {
			if c.ComposeProject == selectedItem.Name {
				totalCount++
				if c.Status == "running" {
					runningCount++
				}
			}
		}

		// すべて起動中なら停止、それ以外なら起動
		action := "start_project"
		if runningCount > 0 && runningCount == totalCount {
			action = "stop_project"
		}

		m.showConfirmDialog = true
		m.confirmAction = action
		m.confirmTarget = selectedItem.Name
		m.confirmType = "project"
	} else {
		// 個別コンテナの操作
		container := m.getSelectedContainer()
		if container == nil {
			return m, nil
		}

		action := "start"
		if container.Status == "running" {
			action = "stop"
		}

		m.showConfirmDialog = true
		m.confirmAction = action
		m.confirmTarget = container.ID
		m.confirmType = "container"
	}

	return m, nil
}

// handleContainerRestart handles container restart
func (m Model) handleContainerRestart() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	if selectedItem.Type == "project" {
		// プロジェクト全体の再起動
		m.showConfirmDialog = true
		m.confirmAction = "restart_project"
		m.confirmTarget = selectedItem.ProjectName
		m.confirmType = "project"
	} else {
		// 個別コンテナの再起動
		container := m.getSelectedContainer()
		if container == nil {
			return m, nil
		}

		m.showConfirmDialog = true
		m.confirmAction = "restart"
		m.confirmTarget = container.ID
		m.confirmType = "container"
	}

	return m, nil
}

// handleContainerRebuild handles container rebuild (Compose only)
func (m Model) handleContainerRebuild() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	if selectedItem.Type == "project" {
		// プロジェクト全体のリビルド
		m.showConfirmDialog = true
		m.confirmAction = "rebuild_project"
		m.confirmTarget = selectedItem.ProjectName
		m.confirmType = "project"
	} else if selectedItem.ProjectName != "" {
		// Composeコンテナのリビルド
		m.showConfirmDialog = true
		m.confirmAction = "rebuild"
		m.confirmTarget = selectedItem.ContainerID
		m.confirmType = "container"
	}

	return m, nil
}

// handleContainerRemove handles container removal
func (m Model) handleContainerRemove() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	if selectedItem.Type == "project" {
		// プロジェクト全体の削除
		m.showConfirmDialog = true
		m.confirmAction = "delete_project"
		m.confirmTarget = selectedItem.Name
		m.confirmType = "project"
		return m, nil
	}

	// 個別コンテナの削除
	container := m.getSelectedContainer()
	if container == nil {
		return m, nil
	}

	m.showConfirmDialog = true
	m.confirmAction = "remove"
	m.confirmTarget = container.ID
	m.confirmType = "container"

	return m, nil
}

// handleOpenInVSCode opens the project directory in VSCode
func (m Model) handleOpenInVSCode() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	var directory string
	selectedMenuItem := m.menuItems[m.selectedItem]

	// 選択されているサービスに応じてディレクトリを取得
	switch selectedMenuItem.Name {
	case "Docker":
		// Dockerコンテナの場合
		container := m.getSelectedContainer()
		if container != nil {
			directory = container.ProjectDir
		}

	case "Node.js":
		// Node.jsプロセスの場合
		process := m.getSelectedNodeProcess()
		if process != nil {
			directory = process.ProjectDir
		}

	case "Python":
		// Pythonプロセスの場合
		process := m.getSelectedPythonProcess()
		if process != nil {
			directory = process.ProjectDir
		}
	}

	// ディレクトリが取得できた場合、VSCodeで開く
	if directory != "" {
		cmd := exec.Command("code", directory)
		err := cmd.Start()

		if err != nil {
			m.lastCommandResult = "VSCodeで開けませんでした: " + err.Error()
		} else {
			m.lastCommandResult = "VSCodeで開きました: " + directory
		}
	} else {
		m.lastCommandResult = "ディレクトリ情報が見つかりません"
	}

	return m, nil
}

// handleViewContainerLogs handles viewing container logs
func (m Model) handleViewContainerLogs() (Model, tea.Cmd) {
	container := m.getSelectedContainer()
	if container == nil {
		return m, nil
	}

	return m, fetchContainerLogsCmd(container.ID, container.Name)
}

// containerLogsMsg is sent when container logs are fetched
type containerLogsMsg struct {
	content    string
	targetName string
	err        error
}

// fetchContainerLogsCmd fetches container logs asynchronously
func fetchContainerLogsCmd(containerID, containerName string) tea.Cmd {
	return func() tea.Msg {
		logContent, err := logs.GetContainerLogs(containerID, 100)
		return containerLogsMsg{
			content:    logContent,
			targetName: containerName,
			err:        err,
		}
	}
}

// handleViewNodeProcessLogs handles viewing Node.js process logs
func (m Model) handleViewNodeProcessLogs() (Model, tea.Cmd) {
	process := m.getSelectedNodeProcess()
	if process == nil {
		return m, nil
	}

	return m, fetchProcessLogsCmd(process.ProjectDir, process.ProjectName)
}

// handleViewPythonProcessLogs handles viewing Python process logs
func (m Model) handleViewPythonProcessLogs() (Model, tea.Cmd) {
	process := m.getSelectedPythonProcess()
	if process == nil {
		return m, nil
	}

	return m, fetchProcessLogsCmd(process.ProjectDir, process.ProcessType)
}

// processLogsMsg is sent when process logs are fetched
type processLogsMsg struct {
	content    string
	targetName string
	err        error
}

// fetchProcessLogsCmd fetches process logs asynchronously
func fetchProcessLogsCmd(projectDir, processName string) tea.Cmd {
	return func() tea.Msg {
		logContent, err := logs.GetProcessLogs(projectDir, 100)
		return processLogsMsg{
			content:    logContent,
			targetName: processName,
			err:        err,
		}
	}
}

// handleCleanDanglingImages handles cleaning dangling images
func (m Model) handleCleanDanglingImages() (Model, tea.Cmd) {
	m.showConfirmDialog = true
	m.confirmAction = "clean_dangling"
	m.confirmType = "docker_system"

	return m, nil
}
