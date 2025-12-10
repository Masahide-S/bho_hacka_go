package ui

import (
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
		m.showConfirmDialog = true
		m.confirmAction = "toggle_project"
		m.confirmTarget = selectedItem.ProjectName
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

	// プロジェクトの削除は危険なので未対応
	if selectedItem.Type == "project" {
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
