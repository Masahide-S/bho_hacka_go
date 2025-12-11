package ui

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// handlePortKill handles killing a process by PID
func (m Model) handlePortKill() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]
	if selectedItem.Type != "port" {
		return m, nil
	}

	// ポート情報を取得
	port := m.getSelectedPort()
	if port == nil {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "kill_port"
	m.confirmTarget = port.PID
	m.confirmType = "port"

	return m, nil
}

// handlePortForceKill handles force killing a process by PID
func (m Model) handlePortForceKill() (Model, tea.Cmd) {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return m, nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]
	if selectedItem.Type != "port" {
		return m, nil
	}

	// ポート情報を取得
	port := m.getSelectedPort()
	if port == nil {
		return m, nil
	}

	// 確認ダイアログを表示
	m.showConfirmDialog = true
	m.confirmAction = "force_kill_port"
	m.confirmTarget = port.PID
	m.confirmType = "port"

	return m, nil
}

// executePortCommand executes a command on a port (actually on the process)
func (m Model) executePortCommand() tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd

		switch m.confirmAction {
		case "kill_port":
			// プロセスを停止
			cmd = exec.Command("kill", m.confirmTarget)
		case "force_kill_port":
			// プロセスを強制停止
			cmd = exec.Command("kill", "-9", m.confirmTarget)
		default:
			return executeCommandMsg{success: false, message: "不明なアクション"}
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return executeCommandMsg{
				success: false,
				message: "プロセス操作失敗: " + string(output),
			}
		}

		actionJP := ""
		switch m.confirmAction {
		case "kill_port":
			actionJP = "停止"
		case "force_kill_port":
			actionJP = "強制停止"
		}

		return executeCommandMsg{
			success: true,
			message: "プロセス (PID: " + m.confirmTarget + ") を" + actionJP + "しました",
		}
	}
}
