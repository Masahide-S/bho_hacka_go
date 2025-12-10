package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleProcessKill handles process kill
func (m Model) handleProcessKill() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロセス以外は何もしない
	if selectedItem.Type != "process" {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "kill"
	m.confirmTarget = selectedItem.Name // PID
	m.confirmType = "process"

	return m, nil
}

// handleProcessForceKill handles process force kill
func (m Model) handleProcessForceKill() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロセス以外は何もしない
	if selectedItem.Type != "process" {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "force_kill"
	m.confirmTarget = selectedItem.Name // PID
	m.confirmType = "process"

	return m, nil
}
