package ui

import (
	"fmt"
	"strings"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// renderPostgresContent renders PostgreSQL database information
func (m Model) renderPostgresContent() string {
	// キャッシュから接続情報を取得（Viewでのブロッキング処理を排除）
	conn := m.cachedPostgresConnection

	if !conn.IsRunning {
		return "PostgreSQL: 停止中"
	}

	// 統計情報を生成
	portInfo := ""
	if conn.Port != "" {
		portInfo = fmt.Sprintf(" [:%s]", conn.Port)
	}

	summary := fmt.Sprintf(`統計情報:
  ステータス: 稼働中%s
  稼働時間: %s
  CPU使用率: %s
  メモリ使用: %s

データベース一覧:
`, portInfo, conn.Uptime, conn.CPUPerc, conn.MemUsage)

	// データベースリストを生成
	databaseList := m.renderSelectablePostgresContent()

	// 右パネルにフォーカスがある場合、選択されたデータベースの詳細情報を追加
	if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 && m.rightPanelCursor < len(m.rightPanelItems) {
		selectedItem := m.rightPanelItems[m.rightPanelCursor]

		if selectedItem.Type == "database" {
			// データベースの詳細情報を取得
			database := m.getSelectedDatabase()
			if database != nil {
				details := m.renderDatabaseDetails(database)
				return summary + databaseList + "\n" + details
			}
		}
	}

	return summary + databaseList
}

// renderDatabaseDetails renders detailed information for a selected database
func (m Model) renderDatabaseDetails(database *monitor.PostgresDatabase) string {
	lastAccessText := database.LastAccess
	if lastAccessText == "" {
		lastAccessText = "不明"
	}

	details := fmt.Sprintf(`
────────────────────────────────────────────────────
データベース詳細: %s
────────────────────────────────────────────────────
  所有者: %s
  サイズ: %s

  設定情報:
    エンコーディング: %s
    照合順序: %s

  アクセス情報:
    最終接続: %s`,
		database.Name,
		database.Owner,
		database.Size,
		database.Encoding,
		database.Collation,
		lastAccessText,
	)

	return details
}

// renderSelectablePostgresContent renders database list with selectable items highlighted
func (m Model) renderSelectablePostgresContent() string {
	var newLines []string

	// キャッシュから取得（Viewではブロッキング処理を行わない）
	databases := m.cachedPostgresDatabases
	if len(databases) == 0 {
		return "  データ取得中..."
	}

	// 各データベースを表示
	for i, item := range m.rightPanelItems {
		if item.Type != "database" {
			continue
		}

		// データベースを検索
		var database *monitor.PostgresDatabase
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
