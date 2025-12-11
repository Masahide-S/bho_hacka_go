package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleTopProcessKill handles killing a top process by PID
func (m Model) handleTopProcessKill() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]
	if selectedItem.Type != "process_item" {
		return m, nil
	}

	// プロセス情報を取得
	process := m.getSelectedTopProcess()
	if process == nil {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "kill_top_process"
	m.confirmTarget = process.PID
	m.confirmType = "top_process"

	return m, nil
}

// handleTopProcessForceKill handles force killing a top process by PID
func (m Model) handleTopProcessForceKill() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]
	if selectedItem.Type != "process_item" {
		return m, nil
	}

	// プロセス情報を取得
	process := m.getSelectedTopProcess()
	if process == nil {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "force_kill_top_process"
	m.confirmTarget = process.PID
	m.confirmType = "top_process"

	return m, nil
}
