package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderMySQLContent renders MySQL database information
func (m Model) renderMySQLContent() string {
	// キャッシュから取得（Viewではブロッキング処理を行わない）
	databases := m.cachedMySQLDatabases

	// キャッシュがない場合はローディング表示
	if len(databases) == 0 {
		return "データ取得中... (MySQL)\n\nMySQLが停止中の可能性があります"
	}

	// 統計情報を生成
	summary := fmt.Sprintf(`統計情報:
  データベース数: %d個

データベース一覧:
`, len(databases))

	// データベースリストを生成
	databaseList := m.renderSelectableMySQLContent()

	// 右パネルにフォーカスがある場合、選択されたデータベースの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "database" {
			// データベースの詳細情報を取得
			database := m.getSelectedMySQLDatabase()
			if database != nil {
				details := m.renderMySQLDatabaseDetails(database)
				return summary + databaseList + "\n" + details
			}
		}
	}

	return summary + databaseList
}

// renderMySQLDatabaseDetails renders detailed information for a selected database
func (m Model) renderMySQLDatabaseDetails(database *monitor.MySQLDatabase) string {
	details := fmt.Sprintf(`
────────────────────────────────────────────────────
データベース詳細: %s
────────────────────────────────────────────────────
  サイズ: %s`,
		database.Name,
		database.Size,
	)

	return details
}

// renderSelectableMySQLContent renders database list with selectable items highlighted
func (m Model) renderSelectableMySQLContent() string {
	var newLines []string

	// キャッシュから取得（Viewではブロッキング処理を行わない）
	databases := m.cachedMySQLDatabases
	if len(databases) == 0 {
		return "  データ取得中..."
	}

	// 各データベースを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "database" {
			continue
		}

		// データベースを検索
		var database *monitor.MySQLDatabase
		for j := range databases {
			if databases[j].Name == item.Name {
				database = &databases[j]
				break
			}
		}

		if database == nil {
			continue
		}

		// データベース名とサイズ
		databaseText := fmt.Sprintf("● %s", database.Name)
		sizeText := fmt.Sprintf("  (%s)", database.Size)

		// カーソル位置なら強調表示
		var line string
		if i == m.rightPanelCursor {
			line = HighlightStyle.Render("> "+databaseText) + CommentStyle.Render(sizeText)
		} else {
			line = "  " + SuccessStyle.Render(databaseText) + CommentStyle.Render(sizeText)
		}

		newLines = append(newLines, line)
	}

	if len(newLines) == 0 {
		return "  データベースがありません"
	}

	return strings.Join(newLines, "\n")
}
