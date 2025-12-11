package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleDatabaseDrop handles database drop
func (m Model) handleDatabaseDrop() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// データベース以外は何もしない
	if selectedItem.Type != "database" {
		return m, nil
	}

	// システムデータベースは削除不可
	systemDatabases := []string{"postgres", "template0", "template1"}
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

// handleDatabaseVacuum handles database vacuum
func (m Model) handleDatabaseVacuum() (Model, tea.Cmd) {
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
	m.confirmAction = "vacuum"
	m.confirmTarget = selectedItem.Name
	m.confirmType = "database"

	return m, nil
}

// handleDatabaseAnalyze handles database analyze
func (m Model) handleDatabaseAnalyze() (Model, tea.Cmd) {
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
	m.confirmAction = "analyze"
	m.confirmTarget = selectedItem.Name
	m.confirmType = "database"

	return m, nil
}
