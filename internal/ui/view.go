package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return SuccessStyle.Render("監視を終了しました\n")
	}

	if m.width == 0 || m.height == 0 {
		return "初期化中..."
	}

	return m.render2ColumnLayout()
}

// render2ColumnLayout renders the 2-column layout with menu
func (m Model) render2ColumnLayout() string {
	// 利用可能な領域を計算（ヘッダーが1行増えたので調整）
	contentWidth := m.width - 8
	contentHeight := m.height - 10  // ← -8 から -10 に変更

	// 2カラムの幅（25% vs 75%）
	leftBoxWidth := (contentWidth / 4) - 2
	rightBoxWidth := (contentWidth * 3 / 4) - 2
	boxHeight := contentHeight - 3

	// 左側: メニューリスト
	leftColumn := m.renderLeftMenu(leftBoxWidth, boxHeight)

	// 右側: 選択されたアイテムの詳細
	rightColumn := m.renderRightDetail(rightBoxWidth, boxHeight)

	// 横並び
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn)

	return m.wrapWithHeaderFooter(content)
}

// renderLeftMenu renders the left menu list
func (m Model) renderLeftMenu(width, height int) string {
	var menuLines []string

	for i, item := range m.menuItems {
		// セパレーターはそのまま表示
		if item.Type == "separator" {
			menuLines = append(menuLines, CommentStyle.Render(item.Name))
			continue
		}

		// 選択カーソル
		cursor := "  "
		if i == m.selectedItem {
			cursor = "> "
		}

		// ステータスアイコン
		status := ""
		if item.Status != "" {
			status = " " + item.Status
		}

		// AI項目の特別表示
		if item.Type == "ai" {
			// Ollama接続状態のマーク
			statusMark := " ●" // デフォルト（未確認）
			statusStyle := InfoStyle

			if m.ollamaAvailable {
				statusMark = " ✓"
				statusStyle = SuccessStyle
			} else {
				statusMark = " ✗"
				statusStyle = ErrorStyle
			}

			issueText := ""
			if m.aiIssueCount > 0 {
				issueText = WarningStyle.Render(fmt.Sprintf(" [%d件]", m.aiIssueCount))
			}

			line := cursor + item.Name + statusStyle.Render(statusMark) + issueText

			if i == m.selectedItem {
				line = HighlightStyle.Render(line)
			}
			menuLines = append(menuLines, line)
			continue
		}

		// 通常項目
		line := cursor + item.Name + status

		// スタイル適用
		if i == m.selectedItem {
			line = HighlightStyle.Render(line)
		} else if item.Status == "✓" {
			line = SuccessStyle.Render(line)
		} else if item.Status == "⚠" {
			line = WarningStyle.Render(line)
		} else if item.Status == "✗" {
			line = ErrorStyle.Render(line)
		}

		menuLines = append(menuLines, line)
	}

	// 高さに合わせて調整
	for len(menuLines) < height-4 {
		menuLines = append(menuLines, "")
	}

	menuContent := strings.Join(menuLines, "\n")

	// ボックスで囲む
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(menuContent)

	return m.embedTitleInBorder(box, "メニュー")
}

// renderRightDetail renders the right detail panel
func (m Model) renderRightDetail(width, height int) string {
	selectedItem := m.menuItems[m.selectedItem]

	var content string
	var title string

	// 選択されたアイテムに応じて内容を変更
	switch selectedItem.Type {
	case "ai":
		title = "環境分析結果"
		content = m.renderAIAnalysis()

	case "service", "info":
		title = selectedItem.Name

		// キャッシュから取得（即座に表示）
		if cache, exists := m.serviceCache[selectedItem.Name]; exists {
			content = cache.Data

			// 更新中の表示
			if cache.Updating {
				ageSeconds := int(time.Since(cache.UpdatedAt).Seconds())
				content += fmt.Sprintf("\n\n更新中... (最終更新: %d秒前)", ageSeconds)
			}
		} else {
			// キャッシュがない場合
			content = "データ取得中..."
		}

	default:
		title = "選択してください"
		content = "左のメニューから項目を選択してください"
	}

	return m.createBox(title, content, width, height)
}

// renderAIAnalysis renders AI analysis result
func (m Model) renderAIAnalysis() string {
	// ヘッダー部分：モデル情報と接続状態
	header := m.renderAIHeader()

	switch m.aiState {
	case aiStateLoading:
		// ストリーミング中は既に受信した内容を表示
		if m.aiResponse != "" {
			return header + "\n\n" + m.aiResponse + "\n\n" + InfoStyle.Render("生成中...")
		}
		return header + "\n\n" + `環境を分析中...

Ollamaが環境情報を読み取っています。
しばらくお待ちください。`

	case aiStateSuccess:
		baseContent := header + "\n\n" + m.aiResponse

		// コマンド実行待ちの場合のプロンプト表示
		if m.aiPendingCmd != "" {
			prompt := fmt.Sprintf(`

────────────────────────────────────────
🤖 AIがアクションを提案しています:

  $ %s

[Enter] 実行する    [Esc] キャンセル
────────────────────────────────────────`, m.aiPendingCmd)
			baseContent += WarningStyle.Render(prompt)
		}

		// 実行結果の表示
		if m.aiCmdResult != "" {
			resultStyle := InfoStyle
			if len(m.aiCmdResult) > 0 && m.aiCmdResult[0] == 226 { // '✗' のUTF-8先頭バイト
				resultStyle = ErrorStyle
			} else if len(m.aiCmdResult) > 0 && m.aiCmdResult[0] == 226 { // '✓' のUTF-8先頭バイト
				resultStyle = SuccessStyle
			}
			// 文字列で判定
			if strings.HasPrefix(m.aiCmdResult, "✗") {
				resultStyle = ErrorStyle
			} else if strings.HasPrefix(m.aiCmdResult, "✓") {
				resultStyle = SuccessStyle
			}

			baseContent += "\n\n" + resultStyle.Render(m.aiCmdResult)
		}

		baseContent += "\n\n[a] 再分析"
		return baseContent

	case aiStateError:
		return header + fmt.Sprintf(`

エラーが発生しました:
%s

[a] 再試行`, m.aiResponse)

	default: // aiStateIdle
		return header + `

環境分析の準備ができています。

[a] キーを押して環境全体を分析`
	}
}

// renderAIHeader はAI画面のヘッダー情報を生成します
func (m Model) renderAIHeader() string {
	// 接続状態
	statusText := ""
	if m.ollamaAvailable {
		statusText = SuccessStyle.Render("● 接続中")
	} else {
		statusText = ErrorStyle.Render("● 未接続")
	}

	// モデル情報
	modelText := ""
	if len(m.availableModels) > 0 {
		modelText = fmt.Sprintf("Model: %s", m.aiService.GetModel())
		if len(m.availableModels) > 1 {
			modelText += CommentStyle.Render(fmt.Sprintf(" (Tab: %d個利用可能)", len(m.availableModels)))
		}
	} else {
		modelText = fmt.Sprintf("Model: %s", m.aiService.GetModel())
	}

	return fmt.Sprintf("AI Assistant  %s\n%s", statusText, modelText)
}

// renderServiceDetail renders service detail
func (m Model) renderServiceDetail(serviceName string) string {
	switch serviceName {
	case "PostgreSQL":
		return monitor.CheckPostgres()
	case "MySQL":
		return monitor.CheckMySQL()  // 🆕 追加
	case "Redis":
		return monitor.CheckRedis()  // 🆕 追加
	case "Docker":
		return monitor.CheckDocker()
	case "Node.js":
		return monitor.CheckNodejs()
	case "Python":
		return monitor.CheckPython()
	default:
		return serviceName + " の詳細情報"
	}
}

// renderInfoPanel renders info panel
func (m Model) renderInfoPanel(panelName string) string {
	switch panelName {
	case "ポート一覧":
		return monitor.ListAllPorts()
	default:
		return panelName
	}
}

// renderSystemResources renders system resource info (detailed)
func (m Model) renderSystemResources() string {
	// キャッシュから取得（重い処理なので）
	if cache, exists := m.serviceCache["システムリソース"]; exists {
		return cache.Data
	}

	return "データ取得中..."
}

// createBox creates a box with title embedded in border
func (m Model) createBox(title, content string, width, height int) string {
	// コンテンツをスタイリング
	styledContent := styleContent(content)

	// 内容を高さに合わせて調整
	contentLines := strings.Split(styledContent, "\n")
	maxContentLines := height - 4

	if len(contentLines) > maxContentLines {
		contentLines = contentLines[:maxContentLines]
		contentLines = append(contentLines, CommentStyle.Render("... (続く)"))
	}

	// 足りない行を空行で埋める
	for len(contentLines) < maxContentLines {
		contentLines = append(contentLines, "")
	}

	adjustedContent := strings.Join(contentLines, "\n")

	// ボックス作成
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(adjustedContent)

	// タイトルを上部ボーダーに埋め込む
	return m.embedTitleInBorder(box, title)
}

// embedTitleInBorder embeds title into the top border
func (m Model) embedTitleInBorder(box, title string) string {
	lines := strings.Split(box, "\n")
	if len(lines) < 1 {
		return box
	}

	formattedTitle := SectionTitleStyle.Render(title)
	topBorder := lines[0]
	actualWidth := lipgloss.Width(topBorder)
	titleWidth := lipgloss.Width(formattedTitle)

	if titleWidth < actualWidth-4 {
		borderStyle := lipgloss.NewStyle().Foreground(borderColor)

		leftCorner := borderStyle.Render("╭")
		rightCorner := borderStyle.Render("╮")
		dash := borderStyle.Render("─")

		remainingWidth := actualWidth - titleWidth - 2
		leftDashes := remainingWidth / 2
		rightDashes := remainingWidth - leftDashes

		newTopBorder := leftCorner +
			strings.Repeat(dash, leftDashes-1) +
			" " + formattedTitle + " " +
			strings.Repeat(dash, rightDashes-1) +
			rightCorner

		lines[0] = newTopBorder
	}

	return strings.Join(lines, "\n")
}

// renderHeader renders the header
func (m Model) renderHeader() string {
	title := TitleStyle.Render("Local Development Monitor")
	
	timestamp := TimestampStyle.Render(fmt.Sprintf(
		"最終更新: %s",
		m.lastUpdate.Format("2006-01-02 15:04:05"),
	))
	
	// システムリソース情報
	sysResources := InfoStyle.Render(monitor.FormatSystemResources(m.systemResources))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		timestamp,
		sysResources,  // 🆕 追加
	)
}

// renderFooter renders the footer
func (m Model) renderFooter() string {
	// AI分析選択中の場合、追加のヘルプを表示
	selectedItem := m.menuItems[m.selectedItem]
	if selectedItem.Type == "ai" && len(m.availableModels) > 1 {
		return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | a: AI分析実行 | Tab: モデル切替")
	}
	return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | a: AI分析実行")
}

// wrapWithHeaderFooter adds header, footer, and outer border
func (m Model) wrapWithHeaderFooter(content string) string {
	header := m.renderHeader()
	footer := m.renderFooter()

	innerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		content,
		"",
		footer,
	)

	// 全体を外枠で囲む
	return OuterBorderStyle.Render(innerContent)
}


