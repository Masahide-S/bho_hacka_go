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

// clearCommandResultMsg is sent to clear command result message
type clearCommandResultMsg struct{}

// containerStatsMsg is sent when container stats are fetched
type containerStatsMsg struct {
	Containers     map[string]*ContainerStatsCache // コンテナID -> キャッシュ
	ContainersList []monitor.DockerContainer       // コンテナリスト
}

// MenuItem represents an item in the left menu
type MenuItem struct {
	Name     string
	Type     string
	Status   string
	HasIssue bool
}

// RightPanelItem represents an item in the right panel
type RightPanelItem struct {
	Type        string // "project" or "container"
	Name        string
	ProjectName string // プロジェクト名（コンテナの場合）
	ContainerID string // コンテナの場合のID
	IsExpanded  bool   // プロジェクトが展開されているか
}

// ServiceCache holds cached service data
type ServiceCache struct {
	Data      string
	UpdatedAt time.Time
	Updating  bool
}

// ContainerStatsCache holds cached container stats
type ContainerStatsCache struct {
	Stats     monitor.DockerStats
	ImageSize string
	UpdatedAt time.Time
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
	serviceCache            map[string]*ServiceCache
	containerStatsCache     map[string]*ContainerStatsCache // コンテナID -> 統計キャッシュ
	cachedContainers        []monitor.DockerContainer       // コンテナリストのキャッシュ
	cachedPostgresDatabases []monitor.PostgresDatabase      // PostgreSQLデータベースのキャッシュ
	cachedNodeProcesses     []monitor.NodeProcess           // Node.jsプロセスのキャッシュ
	tickCount               int

	// Right panel navigation
	focusedPanel     string            // "left" or "right"
	rightPanelCursor int               // 右パネルのカーソル位置
	rightPanelItems  []RightPanelItem  // 右パネルの選択可能な項目
	detailScroll     int               // 詳細情報のスクロール位置

	// Command execution
	showConfirmDialog bool
	confirmAction     string
	confirmTarget     string // コンテナIDまたはプロジェクト名
	confirmType       string // "container" or "project"
	lastCommandResult string // 最後のコマンド実行結果
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
		aiIssueCount:        0,
		systemResources:         monitor.GetSystemResources(),
		serviceCache:            make(map[string]*ServiceCache),
		containerStatsCache:     make(map[string]*ContainerStatsCache),
		cachedContainers:        []monitor.DockerContainer{},
		cachedPostgresDatabases: []monitor.PostgresDatabase{},
		cachedNodeProcesses:     []monitor.NodeProcess{},
		tickCount:               0,
		focusedPanel:        "left",
		rightPanelCursor:    0,
		rightPanelItems:     []RightPanelItem{},
		detailScroll:        0,
		showConfirmDialog: false,
		confirmAction:     "",
		confirmTarget:     "",
		confirmType:       "",
		lastCommandResult: "",
	}
}


// Init initializes the model
func (m Model) Init() tea.Cmd {
	// ログ初期化
	logger.InitLogger()

	return tea.Batch(
		tick(),
		m.fetchAllServicesCmd(),
		m.fetchContainerStatsCmd(),
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

				// 最初の表示可能なアイテムにカーソルを移動
				for m.rightPanelCursor < len(m.rightPanelItems) && !m.isItemVisible(m.rightPanelCursor) {
					m.rightPanelCursor++
				}
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
				// 右パネルのカーソル移動（表示されていないアイテムをスキップ）
				if m.rightPanelCursor > 0 {
					m.rightPanelCursor--
					// 展開されていないコンテナをスキップ
					for m.rightPanelCursor >= 0 && !m.isItemVisible(m.rightPanelCursor) {
						m.rightPanelCursor--
					}
					if m.rightPanelCursor < 0 {
						m.rightPanelCursor = 0
					}
					// スクロール位置をリセット
					m.detailScroll = 0
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
				// 右パネルのカーソル移動（表示されていないアイテムをスキップ）
				if m.rightPanelCursor < len(m.rightPanelItems)-1 {
					m.rightPanelCursor++
					// 展開されていないコンテナをスキップ
					for m.rightPanelCursor < len(m.rightPanelItems) && !m.isItemVisible(m.rightPanelCursor) {
						m.rightPanelCursor++
					}
					if m.rightPanelCursor >= len(m.rightPanelItems) {
						m.rightPanelCursor = len(m.rightPanelItems) - 1
					}
					// スクロール位置をリセット
					m.detailScroll = 0
				}
				return m, nil
			}

		// スペースキー: プロジェクトのトグル開閉
		case " ":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				return m.handleProjectToggle()
			}

		// コマンド実行キー（右パネルでコンテナ選択時のみ）
		case "s":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				return m.handleContainerToggle()
			}

		case "r":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				return m.handleContainerRestart()
			}

		case "b":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				// Composeコンテナの場合のみ
				if m.isSelectedContainerCompose() {
					return m.handleContainerRebuild()
				}
			}

		case "d":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "Docker" {
					return m.handleContainerRemove()
				} else if selectedItem.Name == "PostgreSQL" {
					return m.handleDatabaseDrop()
				}
			}

		case "x":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "Node.js" {
					return m.handleProcessKill()
				}
			}

		case "X":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "Node.js" {
					return m.handleProcessForceKill()
				}
			}

		case "v":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "PostgreSQL" {
					return m.handleDatabaseVacuum()
				}
			}

		case "a":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "PostgreSQL" {
					return m.handleDatabaseAnalyze()
				}
			}

		// スクロール（右パネルで詳細表示時のみ）
		case "ctrl+d":
			if m.focusedPanel == "right" {
				m.detailScroll += 5
				return m, nil
			}

		case "ctrl+u":
			if m.focusedPanel == "right" {
				m.detailScroll -= 5
				if m.detailScroll < 0 {
					m.detailScroll = 0
				}
				return m, nil
			}

		// 確認ダイアログの応答
		case "y", "Y":
			if m.showConfirmDialog {
				return m.executeCommand()
			}

		case "n", "N", "esc":
			if m.showConfirmDialog {
				m.showConfirmDialog = false
				m.confirmAction = ""
				m.confirmTarget = ""
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

		// 5秒ごと: Docker統計のキャッシュ更新
		if m.tickCount%5 == 0 {
			selectedItem := m.menuItems[m.selectedItem]
			if selectedItem.Name == "Docker" {
				cmds = append(cmds, m.fetchContainerStatsCmd())
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

	case executeCommandMsg:
		// コマンド実行結果を保存
		m.lastCommandResult = msg.message

		// 選択中のサービスに応じて更新
		selectedItem := m.menuItems[m.selectedItem]
		var updateCmds []tea.Cmd

		if selectedItem.Name == "Docker" {
			// Dockerの場合: コンテナ統計とリストを更新
			updateCmds = append(updateCmds, m.fetchContainerStatsCmd())
		} else if selectedItem.Name == "PostgreSQL" {
			// PostgreSQLの場合: 右パネルを更新
			m = m.updateRightPanelItems()
		} else if selectedItem.Name == "Node.js" {
			// Node.jsの場合: 右パネルを更新
			m = m.updateRightPanelItems()
		}

		updateCmds = append(updateCmds,
			m.fetchSelectedServiceCmd(),
			tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return clearCommandResultMsg{}
			}),
		)

		return m, tea.Batch(updateCmds...)

	case clearCommandResultMsg:
		m.lastCommandResult = ""
		return m, nil

	case containerStatsMsg:
		// コンテナ統計キャッシュを一括更新
		for containerID, cache := range msg.Containers {
			m.containerStatsCache[containerID] = cache
		}
		// コンテナリストのキャッシュも更新
		m.cachedContainers = msg.ContainersList

		// Dockerパネルが選択されている場合のみ右パネルを更新
		selectedItem := m.menuItems[m.selectedItem]
		if selectedItem.Name == "Docker" {
			m = m.updateRightPanelItems()
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

	// 現在選択中のコンテナIDを保存
	var currentSelectedContainerID string
	var currentSelectedProjectName string
	if m.rightPanelCursor < len(m.rightPanelItems) {
		currentItem := m.rightPanelItems[m.rightPanelCursor]
		if currentItem.Type == "container" {
			currentSelectedContainerID = currentItem.ContainerID
		} else if currentItem.Type == "project" {
			currentSelectedProjectName = currentItem.Name
		}
	}

	// 既存のトグル状態を保存
	expandedState := make(map[string]bool)
	for _, item := range m.rightPanelItems {
		if item.Type == "project" {
			expandedState[item.Name] = item.IsExpanded
		}
	}

	m.rightPanelItems = []RightPanelItem{}

	switch selectedItem.Name {
	case "Docker":
		// Dockerコンテナ一覧を取得
		containers := monitor.GetDockerContainers()

		// プロジェクトごとにグループ化
		projects := make(map[string][]monitor.DockerContainer)
		var standaloneContainers []monitor.DockerContainer

		for _, c := range containers {
			if c.ComposeProject != "" {
				projects[c.ComposeProject] = append(projects[c.ComposeProject], c)
			} else {
				standaloneContainers = append(standaloneContainers, c)
			}
		}

		// プロジェクトを追加
		for projectName, containers := range projects {
			// 既存の展開状態を取得、なければデフォルトでfalse（閉じる）
			isExpanded, exists := expandedState[projectName]
			if !exists {
				isExpanded = false
			}

			// プロジェクト自体を追加
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type:        "project",
				Name:        projectName,
				ProjectName: projectName,
				IsExpanded:  isExpanded,
			})

			// プロジェクト内のコンテナを追加
			for _, c := range containers {
				m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
					Type:        "container",
					Name:        c.Name,
					ProjectName: c.ComposeProject,
					ContainerID: c.ID,
				})
			}
		}

		// 単体コンテナを追加
		for _, c := range standaloneContainers {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type:        "container",
				Name:        c.Name,
				ContainerID: c.ID,
			})
		}

	case "PostgreSQL":
		// PostgreSQLデータベース一覧を取得
		databases := monitor.GetPostgresDatabases()
		m.cachedPostgresDatabases = databases

		// データベースを追加
		for _, db := range databases {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type: "database",
				Name: db.Name,
			})
		}

	case "Node.js":
		// Node.jsプロセス一覧を取得
		processes := monitor.GetNodeProcesses()
		m.cachedNodeProcesses = processes

		// プロセスを追加
		for _, proc := range processes {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type: "process",
				Name: proc.PID,
			})
		}

	default:
		// その他は選択不可
		m.rightPanelItems = []RightPanelItem{}
	}

	// カーソル位置を復元
	if currentSelectedContainerID != "" || currentSelectedProjectName != "" {
		for i, item := range m.rightPanelItems {
			if item.Type == "container" && item.ContainerID == currentSelectedContainerID {
				m.rightPanelCursor = i
				break
			} else if item.Type == "project" && item.Name == currentSelectedProjectName {
				m.rightPanelCursor = i
				break
			}
		}
	}

	// カーソル位置が範囲外の場合は調整
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		m.rightPanelCursor = len(m.rightPanelItems) - 1
	}
	if m.rightPanelCursor < 0 {
		m.rightPanelCursor = 0
	}

	return m
}

// isItemVisible checks if an item should be visible (not hidden by collapsed parent)
func (m Model) isItemVisible(index int) bool {
	if index < 0 || index >= len(m.rightPanelItems) {
		return false
	}

	item := m.rightPanelItems[index]

	// プロジェクトは常に表示
	if item.Type == "project" {
		return true
	}

	// コンテナの場合、親プロジェクトが展開されているか確認
	if item.ProjectName != "" {
		for _, pItem := range m.rightPanelItems {
			if pItem.Type == "project" && pItem.Name == item.ProjectName {
				return pItem.IsExpanded
			}
		}
	}

	// 単体コンテナは常に表示
	return true
}

// isSelectedContainerCompose checks if the selected container is a compose container
func (m Model) isSelectedContainerCompose() bool {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return false
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロジェクト自体はCompose
	if selectedItem.Type == "project" {
		return true
	}

	// コンテナの場合、ProjectNameがあればCompose
	if selectedItem.Type == "container" && selectedItem.ProjectName != "" {
		return true
	}

	return false
}

// getSelectedContainer returns the currently selected container
func (m Model) getSelectedContainer() *monitor.DockerContainer {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロジェクトの場合はnil
	if selectedItem.Type == "project" {
		return nil
	}

	// コンテナIDから検索
	containers := monitor.GetDockerContainers()
	for i := range containers {
		if containers[i].ID == selectedItem.ContainerID {
			return &containers[i]
		}
	}

	return nil
}

// getSelectedDatabase returns the currently selected database
func (m Model) getSelectedDatabase() *monitor.PostgresDatabase {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// データベース以外はnil
	if selectedItem.Type != "database" {
		return nil
	}

	// データベース名から検索
	for i := range m.cachedPostgresDatabases {
		if m.cachedPostgresDatabases[i].Name == selectedItem.Name {
			return &m.cachedPostgresDatabases[i]
		}
	}

	return nil
}

// getSelectedNodeProcess returns the currently selected Node.js process
func (m Model) getSelectedNodeProcess() *monitor.NodeProcess {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロセス以外はnil
	if selectedItem.Type != "process" {
		return nil
	}

	// PIDから検索
	for i := range m.cachedNodeProcesses {
		if m.cachedNodeProcesses[i].PID == selectedItem.Name {
			return &m.cachedNodeProcesses[i]
		}
	}

	return nil
}

// executeCommand executes the confirmed command
func (m Model) executeCommand() (Model, tea.Cmd) {
	// アクションとターゲットを保存
	target := m.confirmTarget
	action := m.confirmAction
	targetType := m.confirmType

	// ダイアログを閉じる
	m.showConfirmDialog = false
	m.confirmAction = ""
	m.confirmTarget = ""
	m.confirmType = ""

	// コマンドを非同期で実行
	return m, executeCommandCmd(target, action, targetType)
}

// executeCommandMsg is sent when command execution completes
type executeCommandMsg struct {
	success bool
	message string
}

// executeCommandCmd executes a command asynchronously
func executeCommandCmd(target, action, targetType string) tea.Cmd {
	return func() tea.Msg {
		var result monitor.CommandResult

		if targetType == "database" {
			result = monitor.ExecutePostgresCommand(target, action)
		} else if targetType == "process" {
			result = monitor.ExecuteNodeCommand(target, action)
		} else {
			result = monitor.ExecuteDockerCommand(target, action, targetType)
		}

		return executeCommandMsg{
			success: result.Success,
			message: result.Message,
		}
	}
}

// fetchContainerStatsCmd fetches container stats for all running containers
func (m Model) fetchContainerStatsCmd() tea.Cmd {
	return func() tea.Msg {
		containers := monitor.GetDockerContainers()

		// 並列で統計を取得
		type statsResult struct {
			containerID string
			stats       monitor.DockerStats
			imageSize   string
		}

		results := make(chan statsResult, len(containers))

		for _, c := range containers {
			go func(container monitor.DockerContainer) {
				stats := monitor.GetDockerContainerStats(container.ID)
				imageSize := monitor.GetDockerImageSize(container.Image)

				results <- statsResult{
					containerID: container.ID,
					stats:       stats,
					imageSize:   imageSize,
				}
			}(c)
		}

		// 全ての結果を収集
		cacheMap := make(map[string]*ContainerStatsCache)
		for i := 0; i < len(containers); i++ {
			result := <-results
			cacheMap[result.containerID] = &ContainerStatsCache{
				Stats:     result.stats,
				ImageSize: result.imageSize,
				UpdatedAt: time.Now(),
			}
		}

		close(results)

		return containerStatsMsg{
			Containers:     cacheMap,
			ContainersList: containers,
		}
	}
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
