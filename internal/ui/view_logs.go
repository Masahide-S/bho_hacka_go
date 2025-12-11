package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderWithLogView renders main view with log viewer overlay
func (m Model) renderWithLogView(mainView string) string {
	// ログ内容を行に分割
	logLines := strings.Split(m.logContent, "\n")

	// ログビューのサイズ設定（画面の80%）
	logWidth := int(float64(m.width) * 0.8)
	logHeight := int(float64(m.height) * 0.8)

	// 最小サイズ制限
	if logWidth < 60 {
		logWidth = 60
	}
	if logHeight < 20 {
		logHeight = 20
	}

	// タイトルとヘルプメッセージ
	title := fmt.Sprintf("📋 ログ: %s", m.logTargetName)
	helpMsg := "[Ctrl+D/U: スクロール | ESC: 閉じる]"

	// 表示可能なログ行数（タイトル、ヘルプ、パディングを除く）
	contentHeight := logHeight - 6

	// スクロール位置の調整
	maxScroll := len(logLines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	// 実際のスクロール位置を計算
	scrollPos := m.logScroll
	if scrollPos > maxScroll {
		scrollPos = maxScroll
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// 表示するログ行を抽出
	startLine := scrollPos
	endLine := startLine + contentHeight
	if endLine > len(logLines) {
		endLine = len(logLines)
	}

	visibleLines := logLines[startLine:endLine]

	// ログ内容を構築
	var logContent strings.Builder
	logContent.WriteString(TitleStyle.Width(logWidth - 4).Render(title))
	logContent.WriteString("\n\n")

	// ログ行を表示（各行を幅に合わせてトリミング）
	for _, line := range visibleLines {
		// 幅を超える行はトリミング
		runes := []rune(line)
		if len(runes) > logWidth-4 {
			line = string(runes[:logWidth-4])
		}
		logContent.WriteString(line)
		logContent.WriteString("\n")
	}

	// スクロール情報
	scrollInfo := fmt.Sprintf("\n[%d-%d / %d行]", startLine+1, endLine, len(logLines))
	logContent.WriteString(CommentStyle.Render(scrollInfo))
	logContent.WriteString("\n")
	logContent.WriteString(CommentStyle.Render(helpMsg))

	// ログビューのスタイル
	logStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(logWidth).
		Height(logHeight).
		Align(lipgloss.Left)

	logView := logStyle.Render(logContent.String())

	// メインビューを行に分割
	mainLines := strings.Split(mainView, "\n")
	logViewLines := strings.Split(logView, "\n")

	// ログビューのサイズ
	logViewHeight := len(logViewLines)
	actualLogWidth := lipgloss.Width(logView)

	// 中央に配置する位置を計算
	startY := (m.height - logViewHeight) / 2
	startX := (m.width - actualLogWidth) / 2

	// ログビューを重ねる（背景を空白で覆う）
	for i, logLine := range logViewLines {
		lineY := startY + i
		if lineY >= 0 && lineY < len(mainLines) {
			// ログビュー行の実際の表示幅
			logDisplayWidth := lipgloss.Width(logLine)

			// 左側の余白（空白で埋める）
			leftPadding := strings.Repeat(" ", startX)

			// 右側の余白（空白で埋める）
			rightPadding := ""
			if startX+logDisplayWidth < m.width {
				rightPadding = strings.Repeat(" ", m.width-startX-logDisplayWidth)
			}

			// 組み立て（ログビュー部分は空白で背景を完全に覆う）
			mainLines[lineY] = leftPadding + logLine + rightPadding
		}
	}

	return strings.Join(mainLines, "\n")
}
