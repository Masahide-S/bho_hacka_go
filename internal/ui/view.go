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

	mainView := m.render2ColumnLayout()

	// 確認ダイアログを重ねて表示
	if m.showConfirmDialog {
		return m.renderWithConfirmDialog(mainView)
	}

	return mainView
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
			issueText := ""
			if m.aiIssueCount > 0 {
				issueText = WarningStyle.Render(fmt.Sprintf(" [%d件]", m.aiIssueCount))
			}
			
			line := cursor + item.Name + issueText
			
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

	// 左パネルにフォーカスがある場合は枠線色を変更
	isFocused := m.focusedPanel == "left"
	boxBorderColor := borderColor
	if isFocused {
		boxBorderColor = accentColor
	}

	// ボックスで囲む
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(boxBorderColor).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(menuContent)

	return m.embedTitleInBorder(box, "メニュー", isFocused)
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

		// Dockerの場合は特別処理
		if selectedItem.Name == "Docker" {
			content = m.renderDockerContent()
		} else {
			// キャッシュから取得（即座に表示）
			if cache, exists := m.serviceCache[selectedItem.Name]; exists {
				baseContent := cache.Data

				// 右パネルにフォーカスがあり、選択可能な項目がある場合、強調表示
				if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
					content = m.renderSelectableContent(baseContent)
				} else {
					content = baseContent
				}

				// 更新中の表示（データが空の場合のみ）
				if cache.Updating && cache.Data == "" {
					ageSeconds := int(time.Since(cache.UpdatedAt).Seconds())
					content = fmt.Sprintf("データ取得中... (%d秒経過)", ageSeconds)
				}
			} else {
				// キャッシュがない場合
				content = "データ取得中..."
			}
		}

	default:
		title = "選択してください"
		content = "左のメニューから項目を選択してください"
	}

	// 右パネルにフォーカスがある場合は枠線色を変更
	isFocused := m.focusedPanel == "right"
	return m.createBox(title, content, width, height, isFocused)
}

// renderDockerContent renders Docker container information
func (m Model) renderDockerContent() string {
	// キャッシュから取得（高速化）
	containers := m.cachedContainers
	if len(containers) == 0 {
		// キャッシュがない場合は取得
		containers = monitor.GetDockerContainers()
	}

	// 統計情報を生成
	totalContainers := len(containers)
	runningContainers := 0
	for _, c := range containers {
		if c.Status == "running" {
			runningContainers++
		}
	}

	// イメージ数を計算
	imageSet := make(map[string]bool)
	for _, c := range containers {
		imageSet[c.Image] = true
	}
	totalImages := len(imageSet)

	// 統計サマリー
	summary := fmt.Sprintf(`統計情報:
  コンテナ: %d個 (稼働中: %d個)
  イメージ: %d種類

`, totalContainers, runningContainers, totalImages)

	// 階層構造のコンテナリストを生成
	containerList := m.renderSelectableContent("")

	// 右パネルにフォーカスがある場合、選択されたアイテムの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "container" {
			// コンテナの詳細情報を取得
			container := m.getSelectedContainer()
			if container != nil {
				details := m.renderContainerDetails(container)
				return summary + containerList + "\n" + details
			}
		} else if selectedItem.Type == "project" {
			// プロジェクトの詳細情報を取得
			details := m.renderProjectDetails(selectedItem.Name)
			return summary + containerList + "\n" + details
		}
	}

	return summary + containerList
}

// renderProjectDetails renders detailed information for a selected project
func (m Model) renderProjectDetails(projectName string) string {
	// キャッシュから取得
	containers := m.cachedContainers
	if len(containers) == 0 {
		containers = monitor.GetDockerContainers()
	}

	// プロジェクト内のコンテナを集計
	var projectContainers []monitor.DockerContainer
	for _, c := range containers {
		if c.ComposeProject == projectName {
			projectContainers = append(projectContainers, c)
		}
	}

	if len(projectContainers) == 0 {
		return ""
	}

	// 稼働中のコンテナ数
	runningCount := 0
	for _, c := range projectContainers {
		if c.Status == "running" {
			runningCount++
		}
	}

	details := fmt.Sprintf(`
────────────────────────────────────────────────────
プロジェクト詳細: %s
────────────────────────────────────────────────────
  種類: Docker Compose
  コンテナ数: %d個 (稼働中: %d個)

  含まれるサービス:`,
		projectName,
		len(projectContainers),
		runningCount,
	)

	// 各サービスを表示
	for _, c := range projectContainers {
		statusIcon := "○"
		if c.Status == "running" {
			statusIcon = "●"
		}
		details += fmt.Sprintf("\n    %s %s (%s)", statusIcon, c.ComposeService, c.Image)
	}

	return details
}

// renderContainerDetails renders detailed information for a selected container
func (m Model) renderContainerDetails(container *monitor.DockerContainer) string {
	// キャッシュから取得
	var stats monitor.DockerStats
	var imageSize string

	if cache, exists := m.containerStatsCache[container.ID]; exists {
		stats = cache.Stats
		imageSize = cache.ImageSize
	}

	// 値が空の場合のデフォルト表示
	if imageSize == "" {
		imageSize = "取得中..."
	}
	cpuPerc := stats.CPUPerc
	if cpuPerc == "" {
		cpuPerc = "取得中..."
	}
	memUsage := stats.MemUsage
	if memUsage == "" {
		memUsage = "取得中..."
	}

	details := fmt.Sprintf(`
────────────────────────────────────────────────────
コンテナ詳細: %s
────────────────────────────────────────────────────
  ステータス: %s

  イメージ情報:
    名前: %s
    サイズ: %s

  リソース使用状況:
    CPU使用率: %s
    メモリ使用量: %s`,
		container.Name,
		container.Status,
		container.Image,
		imageSize,
		cpuPerc,
		memUsage,
	)

	// Compose情報を追加
	if container.ComposeProject != "" {
		details += fmt.Sprintf(`

  Compose情報:
    プロジェクト: %s
    サービス名: %s`, container.ComposeProject, container.ComposeService)
	}

	return details
}

// renderAIAnalysis renders AI analysis result
func (m Model) renderAIAnalysis() string {
	if m.aiIssueCount == 0 {
		return `AI Assistant

✓ すべて正常です

監視状況:
  ✓ 全サービス正常稼働
  ✓ ポート衝突なし
  ✓ リソース使用量: 正常範囲

[a] 環境全体を分析`
	}

	return `AI Assistant

[!] 検知された問題 (2件):

1. Docker メモリ使用率
   512MB / 7.66GB (6.7%)
   
   原因: 長時間稼働による蓄積
   
   推奨対応:
   - docker restart vit-viz-app
   - メモリ制限の設定を確認

2. Node.js 長時間稼働
   稼働: 37日23時間
   
   推奨対応:
   - 定期的な再起動
   - pm2 restart all

全体の健全性: 70%

[a] 再分析`
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
func (m Model) createBox(title, content string, width, height int, isFocused bool) string {
	// コンテンツをスタイリング
	styledContent := styleContent(content)

	// 内容を高さに合わせて調整
	contentLines := strings.Split(styledContent, "\n")
	maxContentLines := height - 4

	// スクロール処理（右パネルにフォーカスがある場合のみ）
	startLine := 0
	if isFocused && m.focusedPanel == "right" {
		startLine = m.detailScroll
	}

	// スクロール位置から表示
	if startLine < len(contentLines) {
		contentLines = contentLines[startLine:]
	}

	showScrollIndicator := false
	if len(contentLines) > maxContentLines {
		contentLines = contentLines[:maxContentLines]
		showScrollIndicator = true
	}

	// 足りない行を空行で埋める
	for len(contentLines) < maxContentLines {
		contentLines = append(contentLines, "")
	}

	// スクロールインジケーターを追加
	if showScrollIndicator {
		if len(contentLines) > 0 {
			contentLines[len(contentLines)-1] = CommentStyle.Render("... (Ctrl+D/U: スクロール)")
		}
	}

	adjustedContent := strings.Join(contentLines, "\n")

	// フォーカス時の枠線色を変更
	boxBorderColor := borderColor
	if isFocused {
		boxBorderColor = accentColor
	}

	// ボックス作成
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(boxBorderColor).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(adjustedContent)

	// タイトルを上部ボーダーに埋め込む
	return m.embedTitleInBorder(box, title, isFocused)
}

// embedTitleInBorder embeds title into the top border
func (m Model) embedTitleInBorder(box, title string, isFocused bool) string {
	lines := strings.Split(box, "\n")
	if len(lines) < 1 {
		return box
	}

	formattedTitle := SectionTitleStyle.Render(title)
	topBorder := lines[0]
	actualWidth := lipgloss.Width(topBorder)
	titleWidth := lipgloss.Width(formattedTitle)

	if titleWidth < actualWidth-4 {
		// フォーカス時は枠線色を変更
		boxBorderColor := borderColor
		if isFocused {
			boxBorderColor = accentColor
		}

		borderStyle := lipgloss.NewStyle().Foreground(boxBorderColor)

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

// renderSelectableContent renders content with selectable items highlighted
func (m Model) renderSelectableContent(baseContent string) string {
	var newLines []string

	// キャッシュから取得（高速化）
	containers := m.cachedContainers
	if len(containers) == 0 {
		containers = monitor.GetDockerContainers()
	}

	containerMap := make(map[string]*monitor.DockerContainer)
	for i := range containers {
		containerMap[containers[i].ID] = &containers[i]
	}

	// 表示するアイテムのインデックスを管理
	displayIndex := 0

	// 階層構造で表示
	for i, item := range m.rightPanelItems {
		var line string
		shouldDisplay := true

		if item.Type == "project" {
			// プロジェクト名を表示
			icon := "▶"
			if item.IsExpanded {
				icon = "▼"
			}

			// プロジェクト配下のコンテナ数とステータスを取得
			runningCount := 0
			totalCount := 0
			for _, c := range containers {
				if c.ComposeProject == item.Name {
					totalCount++
					if c.Status == "running" {
						runningCount++
					}
				}
			}

			statusText := fmt.Sprintf("[%d/%d稼働]", runningCount, totalCount)
			statusStyle := CommentStyle
			if runningCount > 0 && runningCount == totalCount {
				statusStyle = SuccessStyle
			} else if runningCount > 0 {
				statusStyle = WarningStyle
			} else {
				statusStyle = ErrorStyle
			}

			projectText := fmt.Sprintf("%s %s (Compose) %s", icon, item.Name, statusText)

			// カーソル位置なら強調表示
			if i == m.rightPanelCursor {
				line = HighlightStyle.Render("> " + projectText)
			} else {
				line = "  " + statusStyle.Render(projectText)
			}
		} else {
			// コンテナを表示
			container := containerMap[item.ContainerID]
			if container == nil {
				shouldDisplay = false
			} else {
				// 展開されていないプロジェクトのコンテナはスキップ
				if item.ProjectName != "" {
					// 親プロジェクトが展開されているか確認
					parentExpanded := false
					for _, pItem := range m.rightPanelItems {
						if pItem.Type == "project" && pItem.Name == item.ProjectName {
							parentExpanded = pItem.IsExpanded
							break
						}
					}
					if !parentExpanded {
						shouldDisplay = false
					}
				}

				if shouldDisplay {
					// インデント
					indent := ""
					if item.ProjectName != "" {
						indent = "    "
					}

					// ステータスアイコン
					statusIcon := "●"
					statusColor := ErrorStyle
					if container.Status == "running" {
						statusIcon = "●"
						statusColor = SuccessStyle
					} else {
						statusIcon = "○"
						statusColor = CommentStyle
					}

					// コンテナ名とイメージ
					containerText := fmt.Sprintf("%s%s %s", indent, statusIcon, container.Name)
					imageText := fmt.Sprintf("  (%s)", container.Image)

					// カーソル位置なら強調表示
					if i == m.rightPanelCursor {
						line = HighlightStyle.Render("> " + containerText) + CommentStyle.Render(imageText)
					} else {
						line = "  " + statusColor.Render(containerText) + CommentStyle.Render(imageText)
					}
				}
			}
		}

		if shouldDisplay {
			newLines = append(newLines, line)
			displayIndex++
		}
	}

	// 元のコンテンツは無視して、新しい階層構造を表示
	if len(newLines) == 0 {
		return baseContent
	}

	return strings.Join(newLines, "\n")
}

// renderFooter renders the footer
func (m Model) renderFooter() string {
	if m.showConfirmDialog {
		return HelpStyle.Render("Y: はい | N: いいえ")
	}

	if m.focusedPanel == "left" {
		return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | l/→: 詳細へ")
	} else {
		if len(m.rightPanelItems) > 0 {
			// Dockerの場合、選択されたコンテナがComposeかどうかを判定
			selectedItem := m.menuItems[m.selectedItem]
			if selectedItem.Name == "Docker" {
				isCompose := m.isSelectedContainerCompose()

				if isCompose {
					// Composeコンテナ: すべてのコマンドが使える
					return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Space: トグル | Ctrl+D/U: スクロール | s: 起動/停止 | r: 再起動 | b: リビルド")
				} else {
					// 単体コンテナ: リビルドは使えない
					return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Space: トグル | Ctrl+D/U: スクロール | s: 起動/停止 | r: 再起動")
				}
			}
			return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール")
		} else {
			return HelpStyle.Render("q: 終了 | h/←: 戻る")
		}
	}
}

// renderWithConfirmDialog renders main view with confirmation dialog
func (m Model) renderWithConfirmDialog(mainView string) string {
	var dialogContent string

	if m.confirmType == "project" {
		// プロジェクト全体の操作
		actionJP := ""
		switch m.confirmAction {
		case "toggle_project":
			actionJP = "起動/停止"
		case "restart_project":
			actionJP = "再起動"
		case "rebuild_project":
			actionJP = "リビルド"
		}

		dialogContent = fmt.Sprintf(`プロジェクト全体を %s しますか？

プロジェクト: %s (Compose)

[Y] はい
[N] いいえ`, actionJP, m.confirmTarget)
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

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Width(60).
		Render(dialogContent)

	// メインビューを行に分割
	mainLines := strings.Split(mainView, "\n")
	dialogLines := strings.Split(dialog, "\n")

	// ダイアログのサイズ
	dialogHeight := len(dialogLines)
	dialogWidth := lipgloss.Width(dialog)

	// 中央に配置する位置を計算
	startY := (m.height - dialogHeight) / 2
	startX := (m.width - dialogWidth) / 2

	// ダイアログを重ねる
	for i, dialogLine := range dialogLines {
		lineY := startY + i
		if lineY >= 0 && lineY < len(mainLines) {
			mainLine := mainLines[lineY]

			// 左側部分
			leftPart := ""
			if startX > 0 && len(mainLine) > startX {
				leftPart = mainLine[:startX]
			}

			// 右側部分
			rightPart := ""
			rightStart := startX + dialogWidth
			if rightStart < len(mainLine) {
				rightPart = mainLine[rightStart:]
			}

			mainLines[lineY] = leftPart + dialogLine + rightPart
		}
	}

	return strings.Join(mainLines, "\n")
}

// wrapWithHeaderFooter adds header, footer, and outer border
func (m Model) wrapWithHeaderFooter(content string) string {
	header := m.renderHeader()
	footer := m.renderFooter()

	// コマンド実行結果がある場合は表示
	var commandResult string
	if m.lastCommandResult != "" {
		if strings.Contains(m.lastCommandResult, "成功") || strings.Contains(m.lastCommandResult, "しました") {
			commandResult = SuccessStyle.Render("✓ " + m.lastCommandResult)
		} else {
			commandResult = ErrorStyle.Render("✗ " + m.lastCommandResult)
		}
	}

	innerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		content,
		"",
		commandResult,
		footer,
	)

	// 全体を外枠で囲む
	return OuterBorderStyle.Render(innerContent)
}
