package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleRedisFlushDB handles database flush
func (m Model) handleRedisFlushDB() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// データベース以外は何もしない
	if selectedItem.Type != "database" {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "flushdb"
	m.confirmTarget = selectedItem.Name
	m.confirmType = "database"

	return m, nil
}
