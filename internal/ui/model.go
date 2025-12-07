package ui

import (
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// tickMsg is sent every second to trigger updates
type tickMsg time.Time

// MenuItem represents an item in the left menu
type MenuItem struct {
	Name     string
	Type     string
	Status   string
	HasIssue bool
}

// Model holds the TUI state
type Model struct {
	lastUpdate time.Time
	quitting   bool
	width      int
	height     int

	// Menu navigation
	menuItems    []MenuItem
	selectedItem int

	// AI Analysis
	aiIssueCount int

	// System Resources
	systemResources monitor.SystemResources  // 🆕 追加
}

// InitialModel returns the initial model
func InitialModel() Model {
	return Model{
		lastUpdate:   time.Now(),
		selectedItem: 0,
		menuItems: []MenuItem{
			{Name: "AI分析", Type: "ai", Status: ""},
			{Name: "────────────", Type: "separator", Status: ""},
			{Name: "PostgreSQL", Type: "service", Status: "✗"},
			{Name: "MySQL", Type: "service", Status: "✗"},        // 🆕 追加
			{Name: "Redis", Type: "service", Status: "✗"},        // 🆕 追加
			{Name: "Docker", Type: "service", Status: "✗"},
			{Name: "Node.js", Type: "service", Status: "✗"},
			{Name: "Python", Type: "service", Status: "✗"},
			{Name: "────────────", Type: "separator", Status: ""},
			{Name: "ポート一覧", Type: "info", Status: ""},
		},
		aiIssueCount:    0,
		systemResources: monitor.GetSystemResources(),
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

		case "up", "k":
			m.selectedItem--
			if m.selectedItem >= 0 && m.menuItems[m.selectedItem].Type == "separator" {
				m.selectedItem--
			}
			if m.selectedItem < 0 {
				m.selectedItem = len(m.menuItems) - 1
			}

		case "down", "j":
			m.selectedItem++
			if m.selectedItem < len(m.menuItems) && m.menuItems[m.selectedItem].Type == "separator" {
				m.selectedItem++
			}
			if m.selectedItem >= len(m.menuItems) {
				m.selectedItem = 0
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		m.systemResources = monitor.GetSystemResources()  // 🆕 毎秒更新
		m = m.updateServiceStatus()
		return m, tick()
	}

	return m, nil
}

// updateServiceStatus updates the status of services
func (m Model) updateServiceStatus() Model {
	for i, item := range m.menuItems {
		if item.Type != "service" {
			continue
		}

		// 各サービスの状態をチェック
		switch item.Name {
		case "PostgreSQL":
			if isServiceRunning("postgres") {
				m.menuItems[i].Status = "✓"
			} else {
				m.menuItems[i].Status = "✗"
			}

		case "MySQL":
			if isServiceRunning("mysqld") {
				m.menuItems[i].Status = "✓"
			} else {
				m.menuItems[i].Status = "✗"
			}

		case "Redis":
			if isServiceRunning("redis-server") {
				m.menuItems[i].Status = "✓"
			} else {
				m.menuItems[i].Status = "✗"
			}

		case "Docker":
			if isServiceRunning("docker") {
				m.menuItems[i].Status = "✓"
			} else {
				m.menuItems[i].Status = "✗"
			}

		case "Node.js":
			if isServiceRunning("node") {
				m.menuItems[i].Status = "✓"
			} else {
				m.menuItems[i].Status = "✗"
			}

		case "Python":
			if isServiceRunning("python") {
				m.menuItems[i].Status = "✓"
			} else {
				m.menuItems[i].Status = "✗"
			}
		}
	}

	return m
}

// isServiceRunning checks if a service is running
func isServiceRunning(processName string) bool {
	cmd := exec.Command("pgrep", processName)
	err := cmd.Run()
	return err == nil
}

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(
		InitialModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
