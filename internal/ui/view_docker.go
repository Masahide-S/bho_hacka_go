package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderDockerContent renders Docker container information
func (m Model) renderDockerContent() string {
	// キャッシュから取得（高速化）
	containers := m.cachedContainers

	// キャッシュがない場合はローディング表示（Viewではブロッキング処理を行わない）
	if len(containers) == 0 {
		return "データ取得中... (Docker)"
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

	// ダングリングイメージの数とサイズを取得
	danglingCount := monitor.GetDanglingImagesCount()
	danglingSize := monitor.GetDanglingImagesSize()

	// 統計サマリー
	summary := fmt.Sprintf(`統計情報:
  コンテナ: %d個 (稼働中: %d個)
  イメージ: %d種類
  Dangling Images: %d個 (%s)

`, totalContainers, runningContainers, totalImages, danglingCount, danglingSize)

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
	// キャッシュから取得（Viewではブロッキング処理を行わない）
	containers := m.cachedContainers
	if len(containers) == 0 {
		return "データ取得中..."
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

		// プロジェクトディレクトリを追加
		if container.ProjectDir != "" {
			details += fmt.Sprintf(`
    プロジェクトディレクトリ: %s`, container.ProjectDir)
		}
	}

	// ポート情報とURLを追加
	if container.Port != "" {
		details += fmt.Sprintf(`

  アクセス情報:
    ポート: %s
    URL: http://localhost:%s`, container.Port, container.Port)
	}

	return details
}

// renderSelectableContent renders content with selectable items highlighted
func (m Model) renderSelectableContent(baseContent string) string {
	var newLines []string

	// キャッシュから取得（Viewではブロッキング処理を行わない）
	containers := m.cachedContainers
	if len(containers) == 0 {
		return "データ取得中..."
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
