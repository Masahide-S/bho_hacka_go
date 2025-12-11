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

	// ログビューを重ねて表示
	if m.showLogView {
		return m.renderWithLogView(mainView)
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
		} else if selectedItem.Name == "PostgreSQL" {
			// PostgreSQLの場合は特別処理
			content = m.renderPostgresContent()
		} else if selectedItem.Name == "MySQL" {
			// MySQLの場合は特別処理
			content = m.renderMySQLContent()
		} else if selectedItem.Name == "Redis" {
			// Redisの場合は特別処理
			content = m.renderRedisContent()
		} else if selectedItem.Name == "Node.js" {
			// Node.jsの場合は特別処理
			content = m.renderNodejsContent()
		} else if selectedItem.Name == "Python" {
			// Pythonの場合は特別処理
			content = m.renderPythonContent()
		} else if selectedItem.Name == "ポート一覧" {
			// ポート一覧の場合は特別処理
			content = m.renderPortsContent()
		} else if selectedItem.Name == "Top 10 プロセス" {
			// Top 10 プロセスの場合は特別処理
			content = m.renderTopProcessesContent()
		} else if selectedItem.Name == "システムリソース" {
			// システムリソースの場合は特別処理
			content = m.renderSystemResourcesDetail()
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


// renderFooter renders the footer
func (m Model) renderFooter() string {
	if m.showConfirmDialog {
		return HelpStyle.Render("Y: はい | N: いいえ")
	}

	if m.focusedPanel == "left" {
		return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | l/→: 詳細へ")
	} else {
		if len(m.rightPanelItems) > 0 {
			// 選択されたサービスに応じてヘルプメッセージを変更
			selectedItem := m.menuItems[m.selectedItem]
			if selectedItem.Name == "Docker" {
				isCompose := m.isSelectedContainerCompose()

				// 起動/停止のラベルを動的に決定
				startStopText := "s: 起動/停止"
				if m.rightPanelCursor < len(m.rightPanelItems) {
					item := m.rightPanelItems[m.rightPanelCursor]
					if item.Type == "project" {
						// プロジェクト全体の場合、コンテナの状態を確認
						containers := m.cachedContainers
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
						if runningCount > 0 && runningCount == totalCount {
							startStopText = "s: 停止"
						} else {
							startStopText = "s: 起動"
						}
					} else if item.Type == "container" {
						// 個別コンテナの場合
						container := m.getSelectedContainer()
						if container != nil {
							if container.Status == "running" {
								startStopText = "s: 停止"
							} else {
								startStopText = "s: 起動"
							}
						}
					}
				}

				if isCompose {
					// Composeコンテナ: すべてのコマンドが使える
					return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Space: トグル | Ctrl+D/U: スクロール | " + startStopText + " | r: 再起動 | b: リビルド | d: 削除 | c: クリーン | L: ログ | o: VSCodeで開く")
				} else {
					// 単体コンテナ: リビルドは使えない
					return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Space: トグル | Ctrl+D/U: スクロール | " + startStopText + " | r: 再起動 | d: 削除 | c: クリーン | L: ログ | o: VSCodeで開く")
				}
			} else if selectedItem.Name == "PostgreSQL" {
				// PostgreSQLの場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | d: 削除 | v: VACUUM | a: ANALYZE")
			} else if selectedItem.Name == "Node.js" {
				// Node.jsの場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | x: 停止 | X: 強制停止 | L: ログ | o: VSCodeで開く")
			} else if selectedItem.Name == "MySQL" {
				// MySQLの場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | d: 削除 | o: 最適化")
			} else if selectedItem.Name == "Redis" {
				// Redisの場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | f: FLUSHDB")
			} else if selectedItem.Name == "Python" {
				// Pythonの場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | x: 停止 | X: 強制停止 | L: ログ | o: VSCodeで開く")
			} else if selectedItem.Name == "ポート一覧" {
				// ポート一覧の場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | x: 停止 | X: 強制停止")
			} else if selectedItem.Name == "Top 10 プロセス" {
				// Top 10 プロセスの場合
				return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール | x: 停止 | X: 強制停止")
			}
			return HelpStyle.Render("q: 終了 | ↑↓/j/k: 選択 | h/←: 戻る | Ctrl+D/U: スクロール")
		} else {
			return HelpStyle.Render("q: 終了 | h/←: 戻る")
		}
	}
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
