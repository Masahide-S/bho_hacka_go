package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderPortsContent renders port list information
func (m Model) renderPortsContent() string {
	// キャッシュから取得（高速化）
	ports := m.cachedPorts
	if len(ports) == 0 {
		// キャッシュがない場合は取得
		ports = monitor.GetListeningPorts()
	}

	// 統計情報を生成
	totalPorts := len(ports)

	// 統計サマリー
	summary := fmt.Sprintf(`統計情報:
  使用中のポート: %d個

`, totalPorts)

	// ポートリストを生成
	portList := m.renderSelectablePortsContent()

	// 右パネルにフォーカスがある場合、選択されたアイテムの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "port" {
			// ポートの詳細情報を取得
			port := m.getSelectedPort()
			if port != nil {
				details := m.renderPortDetails(port)
				return summary + portList + "\n" + details
			}
		}
	}

	return summary + portList
}

// renderPortDetails renders detailed information for a selected port
func (m Model) renderPortDetails(port *monitor.PortInfo) string {
	bindAddrText := port.BindAddress
	if bindAddrText == "" {
		bindAddrText = "不明"
	}

	projectNameText := port.ProjectName
	if projectNameText == "" {
		projectNameText = port.Process
	}

	details := fmt.Sprintf(`
────────────────────────────────────────────────────
ポート詳細: :%s
────────────────────────────────────────────────────
  PID: %s
  プロセス: %s
  プロジェクト名: %s
  バインドアドレス: %s`,
		port.Port,
		port.PID,
		port.Process,
		projectNameText,
		bindAddrText,
	)

	// URLを追加
	if port.URL != "" {
		details += fmt.Sprintf(`

  アクセス情報:
    URL: %s`, port.URL)
	}

	return details
}

// renderSelectablePortsContent renders port list with selectable items highlighted
func (m Model) renderSelectablePortsContent() string {
	var newLines []string

	// キャッシュから取得
	ports := m.cachedPorts
	if len(ports) == 0 {
		ports = monitor.GetListeningPorts()
	}

	// 各ポートを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "port" {
			continue
		}

		// ポートを検索
		var port *monitor.PortInfo
		for j := range ports {
			if ports[j].Port == item.Name {
				port = &ports[j]
				break
			}
		}

		if port == nil {
			continue
		}

		// プロジェクト名またはプロセス名
		displayName := port.ProjectName
		if displayName == "" {
			displayName = port.Process
		}

		// ポート番号とプロジェクト名
		portText := fmt.Sprintf("● :%s - %s", port.Port, displayName)
		urlText := ""
		if port.URL != "" {
			urlText = fmt.Sprintf("  (%s)", port.URL)
		}

		// カーソル位置なら強調表示
		var line string
		if i == m.rightPanelCursor {
			line = HighlightStyle.Render("> "+portText) + CommentStyle.Render(urlText)
		} else {
			line = "  " + SuccessStyle.Render(portText) + CommentStyle.Render(urlText)
		}

		newLines = append(newLines, line)
	}

	if len(newLines) == 0 {
		return "ポートが検出されませんでした"
	}

	return strings.Join(newLines, "\n")
}
