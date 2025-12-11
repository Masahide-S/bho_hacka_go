package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleMySQLDatabaseDrop handles database drop
func (m Model) handleMySQLDatabaseDrop() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// データベース以外は何もしない
	if selectedItem.Type != "database" {
		return m, nil
	}

	// システムデータベースは削除不可
	systemDatabases := []string{"information_schema", "performance_schema", "mysql", "sys"}
	for _, sysDB := range systemDatabases {
		if selectedItem.Name == sysDB {
			return m, nil
		}
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "drop"
	m.confirmTarget = selectedItem.Name
	m.confirmType = "database"

	return m, nil
}

// handleMySQLDatabaseOptimize handles database optimize
func (m Model) handleMySQLDatabaseOptimize() (Model, tea.Cmd) {
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
	m.confirmAction = "optimize"
	m.confirmTarget = selectedItem.Name
	m.confirmType = "database"

	return m, nil
}
