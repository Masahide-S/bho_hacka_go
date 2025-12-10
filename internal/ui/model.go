package ui

import (
	"os/exec"
	"time"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
	"github.com/Masahide-S/bho_hacka_go/internal/logger"
)


// tickMsg is sent every second to trigger updates
type tickMsg time.Time

// serviceDataMsg is sent when service data is fetched
type serviceDataMsg struct {
	ServiceName string
	Data        string
	UpdatedAt   time.Time
}

// MenuItem represents an item in the left menu
type MenuItem struct {
	Name     string
	Type     string
	Status   string
	HasIssue bool
}

// ServiceCache holds cached service data
type ServiceCache struct {
	Data      string
	UpdatedAt time.Time
	Updating  bool
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
	systemResources monitor.SystemResources

	// Cache
	serviceCache map[string]*ServiceCache
	tickCount    int

	// Right panel navigation
	focusedPanel     string   // "left" or "right"
	rightPanelCursor int      // 右パネルのカーソル位置
	rightPanelItems  []string // 右パネルの選択可能な項目
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
			{Name: "MySQL", Type: "service", Status: "✗"},
			{Name: "Redis", Type: "service", Status: "✗"},
			{Name: "Docker", Type: "service", Status: "✗"},
			{Name: "Node.js", Type: "service", Status: "✗"},
			{Name: "Python", Type: "service", Status: "✗"},
			{Name: "────────────", Type: "separator", Status: ""},
			{Name: "ポート一覧", Type: "info", Status: ""},
			{Name: "システムリソース", Type: "info", Status: ""},
		},
		aiIssueCount:    0,
		systemResources: monitor.GetSystemResources(),
		serviceCache:    make(map[string]*ServiceCache),
		tickCount:       0,
		focusedPanel:    "left",
		rightPanelCursor: 0,
		rightPanelItems: []string{},
	}
}


// Init initializes the model
func (m Model) Init() tea.Cmd {
	// ログ初期化
	logger.InitLogger()

	return tea.Batch(
		tick(),
		m.fetchAllServicesCmd(),
	)
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

		// h/←: 左パネルへ移動
		case "h", "left":
			if m.focusedPanel == "right" {
				m.focusedPanel = "left"
			}
			return m, nil

		// l/→: 右パネルへ移動
		case "l", "right":
			if m.focusedPanel == "left" {
				m.focusedPanel = "right"
				m.rightPanelCursor = 0
				m = m.updateRightPanelItems()
			}
			return m, nil

		case "up", "k":
			if m.focusedPanel == "left" {
				// 左パネルのカーソル移動
				m.selectedItem--
				if m.selectedItem >= 0 && m.menuItems[m.selectedItem].Type == "separator" {
					m.selectedItem--
				}
				if m.selectedItem < 0 {
					m.selectedItem = len(m.menuItems) - 1
				}
				return m, m.fetchSelectedServiceCmd()
			} else {
				// 右パネルのカーソル移動
				if m.rightPanelCursor > 0 {
					m.rightPanelCursor--
				}
				return m, nil
			}

		case "down", "j":
			if m.focusedPanel == "left" {
				// 左パネルのカーソル移動
				m.selectedItem++
				if m.selectedItem < len(m.menuItems) && m.menuItems[m.selectedItem].Type == "separator" {
					m.selectedItem++
				}
				if m.selectedItem >= len(m.menuItems) {
					m.selectedItem = 0
				}
				return m, m.fetchSelectedServiceCmd()
			} else {
				// 右パネルのカーソル移動
				if m.rightPanelCursor < len(m.rightPanelItems)-1 {
					m.rightPanelCursor++
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		m.tickCount++

		var cmds []tea.Cmd
		cmds = append(cmds, tick())

		// 毎秒: サービス起動/停止チェック（並列化済みで高速）
		m = m.updateServiceStatus()

		// 2秒ごと: システムリソース更新
		if m.tickCount%2 == 0 {
			m.systemResources = monitor.GetSystemResources()
		}

		// 選択中のサービスを優先更新
		selectedItem := m.menuItems[m.selectedItem]
		
		if selectedItem.Type == "service" {
			// サービス詳細: 3秒ごと（選択中）
			if m.tickCount%3 == 0 {
				cmds = append(cmds, m.fetchSelectedServiceCmd())
			}
		} else if selectedItem.Type == "info" {
			// ポート一覧など: 5秒ごと（選択中）
			if m.tickCount%5 == 0 {
				cmds = append(cmds, m.fetchSelectedServiceCmd())
			}
		}

		// 10秒ごと: 選択されていないサービスをバックグラウンド更新
		if m.tickCount%10 == 0 {
			cmds = append(cmds, m.fetchNonSelectedServicesCmd())
		}

		if m.tickCount%10 == 0 {
			logger.LogSystemResources(
				m.systemResources.CPUUsage,
				m.systemResources.MemoryUsed,
				m.systemResources.MemoryTotal,
			)
		}

		return m, tea.Batch(cmds...)

	case serviceDataMsg:
		// キャッシュ更新
		m.serviceCache[msg.ServiceName] = &ServiceCache{
			Data:      msg.Data,
			UpdatedAt: msg.UpdatedAt,
			Updating:  false,
		}
		return m, nil
	}

	return m, nil
}

// fetchSelectedServiceCmd fetches the currently selected service data
func (m Model) fetchSelectedServiceCmd() tea.Cmd {
	selectedItem := m.menuItems[m.selectedItem]

	// サービス以外は取得しない
	if selectedItem.Type != "service" && selectedItem.Type != "info" {
		return nil
	}

	serviceName := selectedItem.Name

	// キャッシュの有効期限を種類別に設定
	var cacheValidDuration time.Duration
	
	if selectedItem.Type == "service" {
		cacheValidDuration = 3 * time.Second  // サービス: 3秒
	} else if selectedItem.Type == "info" {
		cacheValidDuration = 5 * time.Second  // 情報: 5秒
	}

	// キャッシュが新しければスキップ
	if cache, exists := m.serviceCache[serviceName]; exists {
		if time.Since(cache.UpdatedAt) < cacheValidDuration {
			return nil
		}
	}

	// 更新中フラグチェック
	if cache, exists := m.serviceCache[serviceName]; exists && cache.Updating {
		return nil
	}

	// 更新中フラグを立てる（既存のデータを保持）
	if cache, exists := m.serviceCache[serviceName]; exists {
		cache.Updating = true
		m.serviceCache[serviceName] = cache
	} else {
		m.serviceCache[serviceName] = &ServiceCache{
			Data:      "",
			UpdatedAt: time.Time{},
			Updating:  true,
		}
	}

	return fetchServiceDataCmd(serviceName)
}

// fetchAllServicesCmd fetches all services data in background
func (m Model) fetchAllServicesCmd() tea.Cmd {
	var cmds []tea.Cmd

	for _, item := range m.menuItems {
		if item.Type == "service" || item.Type == "info" {
			cmds = append(cmds, fetchServiceDataCmd(item.Name))
		}
	}

	return tea.Batch(cmds...)
}

// fetchServiceDataCmd fetches service data asynchronously
func fetchServiceDataCmd(serviceName string) tea.Cmd {
	return func() tea.Msg {
		var data string

		switch serviceName {
		case "PostgreSQL":
			data = monitor.CheckPostgres()
		case "MySQL":
			data = monitor.CheckMySQL()
		case "Redis":
			data = monitor.CheckRedis()
		case "Docker":
			data = monitor.CheckDocker()
		case "Node.js":
			data = monitor.CheckNodejs()
		case "Python":
			data = monitor.CheckPython()
		case "ポート一覧":
			data = monitor.ListAllPorts()
		case "システムリソース":
			// 詳細なシステムリソース情報
			sr := monitor.GetSystemResources()
			topProcs := monitor.GetTopProcesses(5)  // TOP5
			devProcs := monitor.GetDevProcesses()

			data = fmt.Sprintf(`システムリソース

全体:
  CPU: %.1f%%
  メモリ: %.1fGB / %.1fGB (%.0f%%)
  			
TOP5 リソース使用:
%s
開発プロセス:
%s`,
					sr.CPUUsage,
					float64(sr.MemoryUsed)/1024.0,
					float64(sr.MemoryTotal)/1024.0,
					sr.MemoryPerc,
					monitor.FormatTopProcesses(topProcs),
					monitor.FormatDevProcesses(devProcs),
				)
			default:
				data = serviceName + " のデータ"
			}

		return serviceDataMsg{
			ServiceName: serviceName,
			Data:        data,
			UpdatedAt:   time.Now(),
		}
	}
}

// fetchNonSelectedServicesCmd fetches non-selected services in background
func (m Model) fetchNonSelectedServicesCmd() tea.Cmd {
	var cmds []tea.Cmd

	selectedName := m.menuItems[m.selectedItem].Name

	for _, item := range m.menuItems {
		// 選択中のものはスキップ（別途更新される）
		if item.Name == selectedName {
			continue
		}

		if item.Type == "service" || item.Type == "info" {
			cmds = append(cmds, fetchServiceDataCmd(item.Name))
		}
	}

	return tea.Batch(cmds...)
}

// updateServiceStatus updates the status of services (parallel version)
func (m Model) updateServiceStatus() Model {
	// チャネルで結果を受け取る
	type statusResult struct {
		index  int
		status string
	}

	results := make(chan statusResult, len(m.menuItems))

	// 並列でチェック
	activeCount := 0
	for i, item := range m.menuItems {
		if item.Type != "service" {
			continue
		}

		activeCount++
		go func(index int, serviceName string) {
			var processName string
			switch serviceName {
			case "PostgreSQL":
				processName = "postgres"
			case "MySQL":
				processName = "mysqld"
			case "Redis":
				processName = "redis-server"
			case "Docker":
				processName = "docker"
			case "Node.js":
				processName = "node"
			case "Python":
				processName = "python"
			}

			status := "✗"
			if isServiceRunning(processName) {
				status = "✓"
			}

			results <- statusResult{index: index, status: status}
		}(i, item.Name)
	}

	// 結果を収集
	for i := 0; i < activeCount; i++ {
		result := <-results
		m.menuItems[result.index].Status = result.status
	}

	close(results)

	return m
}

// isServiceRunning checks if a service is running
func isServiceRunning(processName string) bool {
	cmd := exec.Command("pgrep", processName)
	err := cmd.Run()
	return err == nil
}

// updateRightPanelItems updates the right panel items based on selected service
func (m Model) updateRightPanelItems() Model {
	selectedItem := m.menuItems[m.selectedItem]
	m.rightPanelItems = []string{}

	switch selectedItem.Name {
	case "Docker":
		// Dockerコンテナ一覧を取得
		containers := monitor.GetDockerContainers()
		for _, c := range containers {
			m.rightPanelItems = append(m.rightPanelItems, c.Name)
		}

	default:
		// その他は選択不可
		m.rightPanelItems = []string{}
	}

	return m
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
