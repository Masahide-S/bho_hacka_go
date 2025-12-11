package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderTopProcessesContent renders top 10 processes information
func (m Model) renderTopProcessesContent() string {
	// キャッシュから取得（高速化）
	processes := m.cachedTopProcesses
	if len(processes) == 0 {
		// キャッシュがない場合は取得
		processes = monitor.GetTopProcesses(10)
	}

	// 統計情報を生成
	totalProcesses := len(processes)

	// 統計サマリー
	summary := fmt.Sprintf(`統計情報:
  Top %d プロセス（CPU使用率順）

`, totalProcesses)

	// プロセスリストを生成
	processList := m.renderSelectableProcessesContent()

	// 右パネルにフォーカスがある場合、選択されたアイテムの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "process_item" {
			// プロセスの詳細情報を取得
			process := m.getSelectedTopProcess()
			if process != nil {
				details := m.renderProcessDetails(process)
				return summary + processList + "\n" + details
			}
		}
	}

	return summary + processList
}

// renderProcessDetails renders detailed information for a selected process
func (m Model) renderProcessDetails(process *monitor.ProcessInfo) string {
	details := fmt.Sprintf(`
────────────────────────────────────────────────────
プロセス詳細: %s
────────────────────────────────────────────────────
  PID: %s
  CPU使用率: %.1f%%
  メモリ使用量: %dMB
  種類: %s`,
		process.Name,
		process.PID,
		process.CPU,
		process.Memory,
		getProcessTypeText(process.IsDevTool),
	)

	return details
}

// getProcessTypeText returns process type text
func getProcessTypeText(isDevTool bool) string {
	if isDevTool {
		return "開発ツール"
	}
	return "システムプロセス"
}

// renderSelectableProcessesContent renders process list with selectable items highlighted
func (m Model) renderSelectableProcessesContent() string {
	var newLines []string

	// キャッシュから取得
	processes := m.cachedTopProcesses
	if len(processes) == 0 {
		processes = monitor.GetTopProcesses(10)
	}

	// 各プロセスを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "process_item" {
			continue
		}

		// プロセスを検索
		var process *monitor.ProcessInfo
		for j := range processes {
			if processes[j].PID == item.ProcessPID {
				process = &processes[j]
				break
			}
		}

		if process == nil {
			continue
		}

		// プロセス情報のテキスト
		processText := fmt.Sprintf("● %s (PID: %s)", process.Name, process.PID)
		statsText := fmt.Sprintf("  CPU: %.1f%% | Mem: %dMB", process.CPU, process.Memory)

		// カーソル位置なら強調表示
		var line string
		if i == m.rightPanelCursor {
			line = HighlightStyle.Render("> "+processText) + CommentStyle.Render(statsText)
		} else {
			// 開発ツールは緑、それ以外は通常色
			if process.IsDevTool {
				line = "  " + SuccessStyle.Render(processText) + CommentStyle.Render(statsText)
			} else {
				line = "  " + processText + CommentStyle.Render(statsText)
			}
		}

		newLines = append(newLines, line)
	}

	if len(newLines) == 0 {
		return "プロセスが検出されませんでした"
	}

	return strings.Join(newLines, "\n")
}

// getSelectedTopProcess returns the currently selected process
func (m Model) getSelectedTopProcess() *monitor.ProcessInfo {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]
	if selectedItem.Type != "process_item" {
		return nil
	}

	// キャッシュから検索
	for i := range m.cachedTopProcesses {
		if m.cachedTopProcesses[i].PID == selectedItem.ProcessPID {
			return &m.cachedTopProcesses[i]
		}
	}

	return nil
}
