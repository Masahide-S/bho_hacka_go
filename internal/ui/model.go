package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FFFF")).
		MarginBottom(1)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00"))

	timestampStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		MarginTop(1)
)

// tickMsg is sent every second to trigger updates
type tickMsg time.Time

// Model holds the TUI state
type Model struct {
	lastUpdate time.Time
	quitting   bool
}

// InitialModel returns the initial model
func InitialModel() Model {
	return Model{
		lastUpdate: time.Now(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tick()
}

// tick returns a command that sends a tickMsg every second
func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		return m, tick()
	}

	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return "👋 監視を終了しました\n"
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("=== Local Development Monitor ==="))
	b.WriteString("\n\n")

	// Timestamp
	timestamp := m.lastUpdate.Format("2006-01-02 15:04:05")
	b.WriteString(timestampStyle.Render(fmt.Sprintf("最終更新: %s", timestamp)))
	b.WriteString("\n\n")

	// PostgreSQL Status
	b.WriteString(formatSection("PostgreSQL", monitor.CheckPostgres()))
	b.WriteString("\n")

	// Docker Status
	b.WriteString(formatSection("Docker", monitor.CheckDocker()))
	b.WriteString("\n")

	// Node.js Status
	b.WriteString(formatSection("Node.js", monitor.CheckNodejs()))
	b.WriteString("\n")

	// Python Status
	b.WriteString(formatSection("Python", monitor.CheckPython()))
	b.WriteString("\n")

	// Ports
	b.WriteString(formatSection("使用中のポート", monitor.ListAllPorts()))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("q: 終了"))
	b.WriteString("\n")

	return b.String()
}

// formatSection applies color based on status
func formatSection(title, content string) string {
	if strings.Contains(content, "✓") {
		return successStyle.Render(content)
	} else if strings.Contains(content, "✗") {
		return errorStyle.Render(content)
	}
	return infoStyle.Render(content)
}

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(InitialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}