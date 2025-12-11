package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderPythonContent renders Python process information
func (m Model) renderPythonContent() string {
	// プロセス情報を取得
	processes := m.cachedPythonProcesses
	if len(processes) == 0 {
		processes = monitor.GetPythonProcesses()
	}

	if len(processes) == 0 {
		return "Python: プロセスが実行されていません"
	}

	// 統計情報を生成
	summary := fmt.Sprintf(`統計情報:
  実行中のプロセス: %d個

プロセス一覧:
`, len(processes))

	// プロセスリストを生成
	processList := m.renderSelectablePythonContent()

	// 右パネルにフォーカスがある場合、選択されたプロセスの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "process" {
			// プロセスの詳細情報を取得
			process := m.getSelectedPythonProcess()
			if process != nil {
				details := m.renderPythonProcessDetails(process)
				return summary + processList + "\n" + details
			}
		}
	}

	return summary + processList
}

// renderPythonProcessDetails renders detailed information for a selected process
func (m Model) renderPythonProcessDetails(process *monitor.PythonProcess) string {
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
		process.ProcessType,
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

// renderSelectablePythonContent renders process list with selectable items highlighted
func (m Model) renderSelectablePythonContent() string {
	var newLines []string

	// キャッシュから取得
	processes := m.cachedPythonProcesses
	if len(processes) == 0 {
		processes = monitor.GetPythonProcesses()
	}

	// 各プロセスを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "process" {
			continue
		}

		// プロセスを検索
		var process *monitor.PythonProcess
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
		processText := fmt.Sprintf("● %s", process.ProcessType)
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
