package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

func (m Model) renderGraphView() string {
	var title string
	var color asciigraph.AnsiColor

	if m.currentView == viewGraphRealtime {
		title = "Realtime CPU Usage (Last few mins)"
		color = asciigraph.Red
	} else {
		title = "Long-term CPU Trend (Last 3 Days / Hourly Avg)"
		color = asciigraph.Blue
	}

	if len(m.graphData) < 2 {
		return fmt.Sprintf("\n  %s\n\n  Waiting for data... (Needs at least 2 points)\n  %s", title, m.message)
	}

	// グラフのサイズ設定
	width := m.width - 10
	height := m.height - 10
	if width < 10 {
		width = 10
	}
	if height < 5 {
		height = 5
	}

	graph := asciigraph.Plot(m.graphData,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(title),
		asciigraph.SeriesColors(color),
	)

	// スタイリング
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2)

	footer := "\n [ESC] Back to Monitor   [g] Realtime View   [h] 3-Day History"

	content := lipgloss.JoinVertical(lipgloss.Left,
		graph,
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(footer),
	)

	return style.Render(content)
}
