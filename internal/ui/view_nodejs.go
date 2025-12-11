package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderNodejsContent renders Node.js process information
func (m Model) renderNodejsContent() string {
	// キャッシュから取得（Viewではブロッキング処理を行わない）
	processes := m.cachedNodeProcesses

	// キャッシュがない場合はローディング表示
	if len(processes) == 0 {
		return "データ取得中... (Node.js)\n\nプロセスが実行されていない可能性があります"
	}

	// 統計情報を生成
	summary := fmt.Sprintf(`統計情報:
  実行中のプロセス: %d個

プロセス一覧:
`, len(processes))

	// プロセスリストを生成
	processList := m.renderSelectableNodejsContent()

	// 右パネルにフォーカスがある場合、選択されたプロセスの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "process" {
			// プロセスの詳細情報を取得
			process := m.getSelectedNodeProcess()
			if process != nil {
				details := m.renderNodeProcessDetails(process)
				return summary + processList + "\n" + details
			}
		}
	}

	return summary + processList
}

// renderNodeProcessDetails renders detailed information for a selected process
func (m Model) renderNodeProcessDetails(process *monitor.NodeProcess) string {
	portText := process.Port
	if portText == "" {
		portText = "なし"
	} else {
		portText = ":" + portText
	}

	details := fmt.Sprintf(`
────────────────────────────────────────────────────
プロセス詳細: %s
────────────────────────────────────────────────────
  PID: %s
  プロジェクトディレクトリ: %s

  リソース使用状況:
    稼働時間: %s
    CPU使用率: %s
    メモリ使用: %s
    ポート: %s`,
		process.ProjectName,
		process.PID,
		process.ProjectDir,
		process.Uptime,
		process.CPUPerc,
		process.MemUsage,
		portText,
	)

	// URLを追加（ポートがある場合）
	if process.Port != "" {
		details += fmt.Sprintf(`

  アクセス情報:
    URL: http://localhost:%s`, process.Port)
	}

	return details
}

// renderSelectableNodejsContent renders process list with selectable items highlighted
func (m Model) renderSelectableNodejsContent() string {
	var newLines []string

	// キャッシュから取得（Viewではブロッキング処理を行わない）
	processes := m.cachedNodeProcesses
	if len(processes) == 0 {
		return "  データ取得中..."
	}

	// 各プロセスを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "process" {
			continue
		}

		// プロセスを検索
		var process *monitor.NodeProcess
		for j := range processes {
			if processes[j].PID == item.Name {
				process = &processes[j]
				break
			}
		}

		if process == nil {
			continue
		}

		// プロセス名とPID
		processText := fmt.Sprintf("● %s", process.ProjectName)
		pidText := fmt.Sprintf("  (PID: %s)", process.PID)

		// カーソル位置なら強調表示
		var line string
		if i == m.rightPanelCursor {
			line = HighlightStyle.Render("> "+processText) + CommentStyle.Render(pidText)
		} else {
			line = "  " + SuccessStyle.Render(processText) + CommentStyle.Render(pidText)
		}

		newLines = append(newLines, line)
	}

	if len(newLines) == 0 {
		return "  プロセスがありません"
	}

	return strings.Join(newLines, "\n")
}
