package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderRedisContent renders Redis database information
func (m Model) renderRedisContent() string {
	// データベース情報を取得
	databases := m.cachedRedisDatabases
	if len(databases) == 0 {
		databases = monitor.GetRedisDatabases()
	}

	// Redis自体が動いているか確認
	cmd := monitor.CheckRedis()
	if strings.Contains(cmd, "停止中") {
		return "Redis: 停止中"
	}

	// 統計情報を生成
	summary := fmt.Sprintf(`統計情報:
  データベース数: %d個

データベース一覧:
`, len(databases))

	// データベースリストを生成
	databaseList := m.renderSelectableRedisContent()

	// 右パネルにフォーカスがある場合、選択されたデータベースの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "database" {
			// データベースの詳細情報を取得
			database := m.getSelectedRedisDatabase()
			if database != nil {
				details := m.renderRedisDatabaseDetails(database)
				return summary + databaseList + "\n" + details
			}
		}
	}

	return summary + databaseList
}

// renderRedisDatabaseDetails renders detailed information for a selected database
func (m Model) renderRedisDatabaseDetails(database *monitor.RedisDatabase) string {
	details := fmt.Sprintf(`
────────────────────────────────────────────────────
データベース詳細: %s
────────────────────────────────────────────────────
  キー数: %s`,
		database.Index,
		database.KeysNum,
	)

	return details
}

// renderSelectableRedisContent renders database list with selectable items highlighted
func (m Model) renderSelectableRedisContent() string {
	var newLines []string

	// キャッシュから取得
	databases := m.cachedRedisDatabases
	if len(databases) == 0 {
		databases = monitor.GetRedisDatabases()
	}

	// 各データベースを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "database" {
			continue
		}

		// データベースを検索
		var database *monitor.RedisDatabase
		for j := range databases {
			if databases[j].Index == item.Name {
				database = &databases[j]
				break
			}
		}

		if database == nil {
			continue
		}

		// データベース名とキー数
		databaseText := fmt.Sprintf("● %s", database.Index)
		keysText := fmt.Sprintf("  (%s)", database.KeysNum)

		// カーソル位置なら強調表示
		var line string
		if i == m.rightPanelCursor {
			line = HighlightStyle.Render("> "+databaseText) + CommentStyle.Render(keysText)
		} else {
			line = "  " + SuccessStyle.Render(databaseText) + CommentStyle.Render(keysText)
		}

		newLines = append(newLines, line)
	}

	if len(newLines) == 0 {
		return "  データベースがありません"
	}

	return strings.Join(newLines, "\n")
}
