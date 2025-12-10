package ui

import "github.com/charmbracelet/lipgloss"

// Tokyo Night カラーパレット（明るめに調整）
var (
	bgColor        = lipgloss.Color("#1a1b26")
	fgColor        = lipgloss.Color("#c0caf5")
	borderColor    = lipgloss.Color("#7aa2f7") // ← 明るく変更（青系）
	accentColor    = lipgloss.Color("#ff9e64") // ← オレンジに変更
	successColor   = lipgloss.Color("#9ece6a")
	errorColor     = lipgloss.Color("#f7768e")
	warningColor   = lipgloss.Color("#e0af68")
	commentColor   = lipgloss.Color("#787c99") // ← 少し明るく
	highlightColor = lipgloss.Color("#bb9af7")
)

// スタイル定義
var (
	// タイトル
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(bgColor)

	// セクションタイトル（枠線上に表示）
	SectionTitleStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Background(bgColor). // 背景色を指定
				Bold(true).
				Padding(0, 1) // 左右にスペース

	// 成功（実行中）
	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	// エラー（停止中）
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	// 情報
	InfoStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// タイムスタンプ
	TimestampStyle = lipgloss.NewStyle().
			Foreground(accentColor). // ← 明るく変更
			Italic(true)

	// ヘルプ
	HelpStyle = lipgloss.NewStyle().
			Foreground(accentColor) // ← 明るく変更

	// 警告
	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// コメント
	CommentStyle = lipgloss.NewStyle().
			Foreground(commentColor)

	// ボーダー
	BorderStyle = lipgloss.NewStyle().
			Foreground(borderColor)

	// 外枠スタイル（全体を囲む）
	OuterBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Padding(1, 2)
	// 既存のスタイルに追加

	// ハイライト（選択中の項目）
	HighlightStyle = lipgloss.NewStyle().
    	Foreground(accentColor).
    	Bold(true).
    	Background(lipgloss.Color("#283457"))  // 薄い青背景
)

// CreateBox creates a styled box with border
func CreateBox(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 1).
		Width(width).
		Height(height)
}
