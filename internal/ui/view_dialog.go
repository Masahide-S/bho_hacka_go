package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderWithConfirmDialog renders main view with confirmation dialog
func (m Model) renderWithConfirmDialog(mainView string) string {
	var dialogContent string

	if m.confirmType == "python_process" {
		// Pythonプロセスの操作
		process := m.getSelectedPythonProcess()
		if process == nil {
			return mainView
		}

		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "kill":
			actionJP = "停止"
			actionDetail = "このプロセスを停止します"
		case "force_kill":
			actionJP = "強制停止"
			actionDetail = "⚠ このプロセスを強制停止します（SIGKILL）"
		}

		dialogContent = fmt.Sprintf(`プロセスを %s しますか？

%s

プロジェクト: %s
PID: %s

[Y] はい
[N] いいえ`, actionJP, actionDetail, process.ProcessType, process.PID)
	} else if m.confirmType == "process" {
		// Node.jsプロセスの操作
		process := m.getSelectedNodeProcess()
		if process == nil {
			return mainView
		}

		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "kill":
			actionJP = "停止"
			actionDetail = "このプロセスを停止します"
		case "force_kill":
			actionJP = "強制停止"
			actionDetail = "⚠ このプロセスを強制停止します（SIGKILL）"
		}

		dialogContent = fmt.Sprintf(`プロセスを %s しますか？

%s

プロジェクト: %s
PID: %s

[Y] はい
[N] いいえ`, actionJP, actionDetail, process.ProjectName, process.PID)
	} else if m.confirmType == "mysql_database" {
		// MySQLデータベースの操作
		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "drop":
			actionJP = "削除"
			actionDetail = "⚠ このデータベースを削除します（データは復元できません）"
		case "optimize":
			actionJP = "最適化"
			actionDetail = "このデータベースを最適化します"
		}

		dialogContent = fmt.Sprintf(`データベースを %s しますか？

%s

データベース名: %s

[Y] はい
[N] いいえ`, actionJP, actionDetail, m.confirmTarget)
	} else if m.confirmType == "redis_database" {
		// Redisデータベースの操作
		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "flushdb":
			actionJP = "クリア"
			actionDetail = "⚠ このデータベースの全キーを削除します（データは復元できません）"
		}

		dialogContent = fmt.Sprintf(`データベースを %s しますか？

%s

データベース: %s

[Y] はい
[N] いいえ`, actionJP, actionDetail, m.confirmTarget)
	} else if m.confirmType == "port" {
		// ポートの操作（プロセス停止）
		port := m.getSelectedPort()
		if port == nil {
			return mainView
		}

		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "kill_port":
			actionJP = "停止"
			actionDetail = "このプロセスを停止します"
		case "force_kill_port":
			actionJP = "強制停止"
			actionDetail = "⚠ このプロセスを強制停止します（SIGKILL）"
		}

		projectName := port.ProjectName
		if projectName == "" {
			projectName = port.Process
		}

		dialogContent = fmt.Sprintf(`プロセスを %s しますか？

%s

プロジェクト: %s
PID: %s
ポート: :%s

[Y] はい
[N] いいえ`, actionJP, actionDetail, projectName, port.PID, port.Port)
	} else if m.confirmType == "top_process" {
		// Top 10 プロセスの操作（プロセス停止）
		process := m.getSelectedTopProcess()
		if process == nil {
			return mainView
		}

		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "kill_top_process":
			actionJP = "停止"
			actionDetail = "このプロセスを停止します"
		case "force_kill_top_process":
			actionJP = "強制停止"
			actionDetail = "⚠ このプロセスを強制停止します（SIGKILL）"
		}

		processType := getProcessTypeText(process.IsDevTool)

		dialogContent = fmt.Sprintf(`プロセスを %s しますか？

%s

プロセス名: %s
PID: %s
CPU: %.1f%%
メモリ: %dMB
種類: %s

[Y] はい
[N] いいえ`, actionJP, actionDetail, process.Name, process.PID, process.CPU, process.Memory, processType)
	} else if m.confirmType == "database" {
		// PostgreSQLデータベースの操作
		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "drop":
			actionJP = "削除"
			actionDetail = "⚠ このデータベースを削除します（データは復元できません）"
		case "vacuum":
			actionJP = "VACUUM実行"
			actionDetail = "このデータベースを最適化します"
		case "analyze":
			actionJP = "ANALYZE実行"
			actionDetail = "このデータベースの統計情報を更新します"
		}

		dialogContent = fmt.Sprintf(`データベースを %s しますか？

%s

データベース名: %s

[Y] はい
[N] いいえ`, actionJP, actionDetail, m.confirmTarget)
	} else if m.confirmType == "docker_system" {
		// Dockerシステム操作
		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "clean_dangling":
			actionJP = "ダングリングイメージを削除"
			actionDetail = "⚠ 使用されていないイメージを削除します"
		}

		dialogContent = fmt.Sprintf(`%s しますか？

%s

[Y] はい
[N] いいえ`, actionJP, actionDetail)
	} else if m.confirmType == "project" {
		// プロジェクト全体の操作
		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "start_project":
			actionJP = "起動"
			actionDetail = "このプロジェクトの全コンテナを起動します"
		case "stop_project":
			actionJP = "停止"
			actionDetail = "このプロジェクトの全コンテナを停止します"
		case "delete_project":
			actionJP = "削除"
			actionDetail = "⚠ このプロジェクトの全コンテナを削除します（ボリュームは保持）"
		case "restart_project":
			actionJP = "再起動"
			actionDetail = "このプロジェクトの全コンテナを再起動します"
		case "rebuild_project":
			actionJP = "リビルド"
			actionDetail = "このプロジェクトの全コンテナをリビルドします"
		}

		dialogContent = fmt.Sprintf(`プロジェクト全体を %s しますか？

%s

プロジェクト: %s (Compose)

[Y] はい
[N] いいえ`, actionJP, actionDetail, m.confirmTarget)
	} else {
		// 個別コンテナの操作
		container := m.getSelectedContainer()
		if container == nil {
			return mainView
		}

		actionJP := ""
		actionDetail := ""
		switch m.confirmAction {
		case "start":
			actionJP = "起動"
			actionDetail = "このコンテナを起動します"
		case "stop":
			actionJP = "停止"
			actionDetail = "このコンテナを停止します"
		case "restart":
			actionJP = "再起動"
			actionDetail = "このコンテナを再起動します"
		case "rebuild":
			actionJP = "リビルド"
			actionDetail = "このコンテナをリビルドします"
		case "remove":
			actionJP = "削除"
			actionDetail = "⚠ このコンテナを削除します（データは削除されません）"
		}

		containerType := "単体コンテナ"
		if container.ComposeProject != "" {
			containerType = fmt.Sprintf("Compose: %s / %s", container.ComposeProject, container.ComposeService)
		}

		statusInfo := ""
		if container.Status == "running" {
			statusInfo = "ステータス: 稼働中"
		} else {
			statusInfo = "ステータス: 停止中"
		}

		dialogContent = fmt.Sprintf(`コンテナを %s しますか？

%s

名前: %s
イメージ: %s
種類: %s
%s

[Y] はい
[N] いいえ`, actionJP, actionDetail, container.Name, container.Image, containerType, statusInfo)
	}

	// ダイアログの幅を計算（コンテンツに合わせて調整）
	lines := strings.Split(dialogContent, "\n")
	maxLineWidth := 0
	for _, line := range lines {
		lineWidth := len([]rune(line))
		if lineWidth > maxLineWidth {
			maxLineWidth = lineWidth
		}
	}

	// 最小幅50、最大幅80に制限
	dialogWidth := maxLineWidth
	if dialogWidth < 50 {
		dialogWidth = 50
	}
	if dialogWidth > 80 {
		dialogWidth = 80
	}

	// ダイアログのスタイル
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Width(dialogWidth).
		Align(lipgloss.Left)

	dialog := dialogStyle.Render(dialogContent)

	// メインビューを行に分割
	mainLines := strings.Split(mainView, "\n")
	dialogLines := strings.Split(dialog, "\n")

	// ダイアログのサイズ
	dialogHeight := len(dialogLines)
	actualDialogWidth := lipgloss.Width(dialog)

	// 中央に配置する位置を計算
	startY := (m.height - dialogHeight) / 2
	startX := (m.width - actualDialogWidth) / 2

	// ダイアログを重ねる（背景を空白で覆う - 安定版）
	for i, dialogLine := range dialogLines {
		lineY := startY + i
		if lineY >= 0 && lineY < len(mainLines) {
			// ダイアログ行の実際の表示幅
			dialogDisplayWidth := lipgloss.Width(dialogLine)

			// 左側の余白（空白で埋める）
			leftPadding := strings.Repeat(" ", startX)

			// 右側の余白（空白で埋める）
			rightPadding := ""
			if startX+dialogDisplayWidth < m.width {
				rightPadding = strings.Repeat(" ", m.width-startX-dialogDisplayWidth)
			}

			// 組み立て（ダイアログ部分は空白で背景を完全に覆う）
			mainLines[lineY] = leftPadding + dialogLine + rightPadding
		}
	}

	return strings.Join(mainLines, "\n")
}
