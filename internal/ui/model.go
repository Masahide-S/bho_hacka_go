package ui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Masahide-S/bho_hacka_go/internal/ai"
	"github.com/Masahide-S/bho_hacka_go/internal/llm"
	"github.com/Masahide-S/bho_hacka_go/internal/logger"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
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

// portsDataMsg is sent when port data is fetched
type portsDataMsg struct {
	Ports     []monitor.PortInfo
	UpdatedAt time.Time
}

// topProcessesDataMsg is sent when top processes data is fetched
type topProcessesDataMsg struct {
	Processes []monitor.ProcessInfo
	UpdatedAt time.Time
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
	Type        string // "project", "container", "port", "process_item"
	Name        string
	ProjectName string // プロジェクト名（コンテナの場合）
	ContainerID string // コンテナの場合のID
	ProcessPID  string // プロセスの場合のPID
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
	cachedMySQLDatabases    []monitor.MySQLDatabase         // MySQLデータベースのキャッシュ
	cachedRedisDatabases    []monitor.RedisDatabase         // Redisデータベースのキャッシュ
	cachedNodeProcesses     []monitor.NodeProcess           // Node.jsプロセスのキャッシュ
	cachedPythonProcesses   []monitor.PythonProcess         // Pythonプロセスのキャッシュ
	cachedPorts             []monitor.PortInfo              // ポート一覧のキャッシュ
	cachedPortsUpdatedAt    time.Time                       // ポート一覧の最終更新時刻
	cachedTopProcesses      []monitor.ProcessInfo           // Top 10プロセスのキャッシュ
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

	// Log viewing
	showLogView   bool
	logContent    string
	logScroll     int
	logTargetName string // ログ表示対象の名前


	// AI関連フィールド
	aiService    *ai.Service
	aiState      int
	aiResponse   string
	aiPendingCmd string // 実行待ちのコマンド
	aiCmdResult  string // コマンド実行結果

	// ストリーミング用フィールド
	currentStream <-chan llm.GenerateResponseStream

	// Ollama接続状態
	ollamaAvailable bool
	availableModels []string
	selectedModel   int // モデル選択インデックス
}




// AIの状態を表す定数
const (
	aiStateIdle = iota
	aiStateLoading
	aiStateSuccess
	aiStateError
)

// aiAnalysisMsg はAI分析結果を運ぶメッセージ
type aiAnalysisMsg struct {
	Result string
	Err    error
}

// cmdExecMsg はコマンド実行結果を運ぶメッセージ
type cmdExecMsg struct {
	Result string

}

// ストリーミング開始を通知するメッセージ
type aiStreamStartMsg <-chan llm.GenerateResponseStream

// ストリーミングの各パケットを運ぶメッセージ
type aiStreamMsg struct {
	Response string
	Done     bool
	Err      error
}

// Ollamaヘルスチェック結果を運ぶメッセージ
type aiHealthMsg struct {
	Err error
}

// モデル一覧取得結果を運ぶメッセージ
type aiModelsMsg struct {
	Models []string
	Err    error
}

// コマンド抽出用の正規表現
var cmdRegex = regexp.MustCompile(`<cmd>(.*?)</cmd>`)


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
			{Name: "Top 10 プロセス", Type: "info", Status: ""},
			{Name: "システムリソース", Type: "info", Status: ""},
		},
		aiIssueCount:        0,
		systemResources:         monitor.GetSystemResources(),
		serviceCache:            make(map[string]*ServiceCache),
		containerStatsCache:     make(map[string]*ContainerStatsCache),
		cachedContainers:        []monitor.DockerContainer{},
		cachedPostgresDatabases: []monitor.PostgresDatabase{},
		cachedMySQLDatabases:    []monitor.MySQLDatabase{},
		cachedRedisDatabases:    []monitor.RedisDatabase{},
		cachedNodeProcesses:     []monitor.NodeProcess{},
		cachedPythonProcesses:   []monitor.PythonProcess{},
		cachedTopProcesses:      []monitor.ProcessInfo{},
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
		showLogView:       false,
		logContent:        "",
		logScroll:         0,
		logTargetName:     "",
    aiService:       ai.NewService(),
		aiState:         aiStateIdle,
		aiPendingCmd:    "",
		aiCmdResult:     "",
		ollamaAvailable: false,
		availableModels: []string{},
		selectedModel:   0,

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

// checkHealthCmd はOllamaサーバーの接続確認を行うコマンド
func (m Model) checkHealthCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.aiService.CheckHealth(context.Background())
		return aiHealthMsg{Err: err}
	}
}

// fetchModelsCmd は利用可能なモデル一覧を取得するコマンド
func (m Model) fetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		models, err := m.aiService.ListModels(context.Background())
		return aiModelsMsg{Models: models, Err: err}
	}
}

// waitForStreamResponse はストリーミングチャネルから次のデータを待つコマンド
func waitForStreamResponse(sub <-chan llm.GenerateResponseStream) tea.Cmd {
	return func() tea.Msg {
		data, ok := <-sub
		if !ok {
			// チャネルが閉じられた場合は完了とみなす
			return aiStreamMsg{Done: true}
		}
		return aiStreamMsg{
			Response: data.Response,
			Done:     data.Done,
			Err:      data.Err,
		}
	}
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
		// AIのコマンド実行待ち状態の時のキー操作
		if m.aiPendingCmd != "" {
			switch msg.String() {
			case "enter":
				// コマンド実行
				cmdStr := m.aiPendingCmd
				m.aiPendingCmd = ""
				m.aiCmdResult = fmt.Sprintf("実行中: %s...", cmdStr)
				return m, executePendingCmd(cmdStr)

			case "esc", "n":
				// キャンセル
				m.aiPendingCmd = ""
				m.aiCmdResult = "コマンド実行をキャンセルしました。"
				return m, nil

			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
			// コマンド待ちの時は他の操作をブロック
			return m, nil
		}

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
				} else if selectedItem.Name == "MySQL" {
					return m.handleMySQLDatabaseDrop()
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
				} else if selectedItem.Name == "Python" {
					return m.handlePythonProcessKill()
				} else if selectedItem.Name == "ポート一覧" {
					return m.handlePortKill()
				} else if selectedItem.Name == "Top 10 プロセス" {
					return m.handleTopProcessKill()
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
				} else if selectedItem.Name == "Python" {
					return m.handlePythonProcessForceKill()
				} else if selectedItem.Name == "ポート一覧" {
					return m.handlePortForceKill()
				} else if selectedItem.Name == "Top 10 プロセス" {
					return m.handleTopProcessForceKill()
				}
			}

		case "o":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "MySQL" {
					return m.handleMySQLDatabaseOptimize()
				} else if selectedItem.Name == "Docker" || selectedItem.Name == "Node.js" || selectedItem.Name == "Python" {
					return m.handleOpenInVSCode()
				}
			}

		case "f":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "Redis" {
					return m.handleRedisFlushDB()
				}
			}

		case "c":
			if m.showConfirmDialog {
				return m, nil
			}
			selectedItem := m.menuItems[m.selectedItem]
			if selectedItem.Name == "Docker" {
				return m.handleCleanDanglingImages()
			}

		case "L":
			if m.showConfirmDialog {
				return m, nil
			}
			if m.focusedPanel == "right" && len(m.rightPanelItems) > 0 {
				selectedItem := m.menuItems[m.selectedItem]
				if selectedItem.Name == "Docker" {
					return m.handleViewContainerLogs()
				} else if selectedItem.Name == "Node.js" {
					return m.handleViewNodeProcessLogs()
				} else if selectedItem.Name == "Python" {
					return m.handleViewPythonProcessLogs()
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

		// スクロール（右パネルで詳細表示時のみ）
		case "ctrl+d":
			if m.showLogView {
				m.logScroll += 5
				return m, nil
			}
			if m.focusedPanel == "right" {
				m.detailScroll += 5
				return m, nil
			}

		case "ctrl+u":
			if m.showLogView {
				m.logScroll -= 5
				if m.logScroll < 0 {
					m.logScroll = 0
				}
				return m, nil
			}
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
			if m.showLogView {
				m.showLogView = false
				m.logContent = ""
				m.logScroll = 0
				m.logTargetName = ""
				return m, nil
			}
		}

	case containerLogsMsg:
		// コンテナログの取得結果を処理
		if msg.err != nil {
			m.lastCommandResult = fmt.Sprintf("ログ取得失敗: %v", msg.err)
			return m, nil

		// [a] キーでAI分析開始（AI分析メニュー選択時のみ）
		case "a":
			selectedItem := m.menuItems[m.selectedItem]
			if selectedItem.Type == "ai" && m.aiState != aiStateLoading {
				if !m.ollamaAvailable {
					m.aiState = aiStateError
					m.aiResponse = "Ollamaサーバーに接続できません。\nOllamaが起動しているか確認してください。"
					return m, nil
				}
				m.aiState = aiStateLoading
				m.aiResponse = ""
				m.aiPendingCmd = "" // リセット
				m.aiCmdResult = ""  // リセット
				return m, m.runAIAnalysisCmd()
			}

		// [tab] キーでモデル切り替え（AI分析メニュー選択時のみ）
		case "tab":
			selectedItem := m.menuItems[m.selectedItem]
			if selectedItem.Type == "ai" && len(m.availableModels) > 0 {
				m.selectedModel = (m.selectedModel + 1) % len(m.availableModels)
				m.aiService.SetModel(m.availableModels[m.selectedModel])
			}
		}

		m.showLogView = true
		m.logContent = msg.content
		m.logScroll = 999999 // 一番下から表示（view_logs.goで自動調整される）
		m.logTargetName = msg.targetName

		return m, nil

	case processLogsMsg:
		// プロセスログの取得結果を処理
		if msg.err != nil {
			m.lastCommandResult = fmt.Sprintf("ログ取得失敗: %v", msg.err)
			return m, nil
		}

		m.showLogView = true
		m.logContent = msg.content
		m.logScroll = 999999 // 一番下から表示（view_logs.goで自動調整される）
		m.logTargetName = msg.targetName

		return m, nil

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
			// ポート一覧: 3秒ごと（選択中、高速更新）
			if selectedItem.Name == "ポート一覧" && m.tickCount%3 == 0 {
				cmds = append(cmds, m.fetchPortsDataCmd())
			} else if selectedItem.Name == "Top 10 プロセス" && m.tickCount%3 == 0 {
				// Top 10 プロセス: 3秒ごと（選択中、高速更新）
				cmds = append(cmds, m.fetchTopProcessesDataCmd())
			} else if m.tickCount%5 == 0 {
				// その他のinfo: 5秒ごと
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
		} else if selectedItem.Name == "PostgreSQL" || selectedItem.Name == "MySQL" || selectedItem.Name == "Redis" {
			// データベースの場合: 右パネルを更新
			m = m.updateRightPanelItems()
		} else if selectedItem.Name == "Node.js" || selectedItem.Name == "Python" {
			// プロセスの場合: 右パネルを更新
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

	case portsDataMsg:
		// ポート一覧のキャッシュを更新
		m.cachedPorts = msg.Ports
		m.cachedPortsUpdatedAt = msg.UpdatedAt

		// ポート一覧パネルが選択されている場合のみ右パネルを更新
		selectedItem := m.menuItems[m.selectedItem]
		if selectedItem.Name == "ポート一覧" {
			m = m.updateRightPanelItems()
		}

		return m, nil

	case topProcessesDataMsg:
		// Top 10 プロセスのキャッシュを更新
		m.cachedTopProcesses = msg.Processes

		// Top 10 プロセスパネルが選択されている場合のみ右パネルを更新
		selectedItem := m.menuItems[m.selectedItem]
		if selectedItem.Name == "Top 10 プロセス" {
			m = m.updateRightPanelItems()
		}

		// AI分析結果の受信
	case aiAnalysisMsg:
		if msg.Err != nil {
			m.aiState = aiStateError
			m.aiResponse = "エラーが発生しました:\n" + msg.Err.Error()
		} else {
			m.aiState = aiStateSuccess
			m.aiResponse = msg.Result

			// コマンドが含まれているかチェック
			matches := cmdRegex.FindStringSubmatch(msg.Result)
			if len(matches) > 1 {
				m.aiPendingCmd = matches[1] // コマンド部分を抽出して保存
			} else {
				m.aiPendingCmd = ""
			}
		}
		return m, nil

	// コマンド実行結果の受信
	case cmdExecMsg:
		m.aiCmdResult = msg.Result
		// 実行後に最新の状態を反映するため、全サービス再取得をトリガー
		return m, m.fetchAllServicesCmd()

	// ストリーミング開始の受信
	case aiStreamStartMsg:
		m.currentStream = msg
		return m, waitForStreamResponse(m.currentStream)

	// ストリーミングデータの受信
	case aiStreamMsg:
		if msg.Err != nil {
			m.aiState = aiStateError
			m.aiResponse += "\n\nエラーが発生しました:\n" + msg.Err.Error()
			m.currentStream = nil
			return m, nil
		}

		// 応答を追記
		m.aiResponse += msg.Response

		if msg.Done {
			m.aiState = aiStateSuccess
			// コマンド解析は完了後に実行
			matches := cmdRegex.FindStringSubmatch(m.aiResponse)
			if len(matches) > 1 {
				m.aiPendingCmd = matches[1]
			}
			m.currentStream = nil
			return m, nil
		}

		// まだ終わっていない場合、次のデータを待つ
		return m, waitForStreamResponse(m.currentStream)

	// Ollamaヘルスチェック結果の受信
	case aiHealthMsg:
		if msg.Err == nil {
			m.ollamaAvailable = true
		} else {
			m.ollamaAvailable = false
		}
		return m, nil

	// モデル一覧取得結果の受信
	case aiModelsMsg:
		if msg.Err == nil && len(msg.Models) > 0 {
			m.availableModels = msg.Models
			// デフォルトモデルがリストにあるか確認
			currentModel := m.aiService.GetModel()
			for i, model := range m.availableModels {
				if model == currentModel {
					m.selectedModel = i
					break
				}
			}
		}

		return m, nil
	}

	return m, nil
}

// runAIAnalysisCmd は非同期でAI分析を実行（ストリーミングモード）
func (m Model) runAIAnalysisCmd() tea.Cmd {
	return func() tea.Msg {
		// コンテキスト構築（RAG）
		prompt := m.aiService.BuildSystemContext()

		// ストリーミングモードで推論実行
		stream, err := m.aiService.AnalyzeStream(context.Background(), prompt)
		if err != nil {
			return aiAnalysisMsg{Err: err}
		}

		// ストリームチャネルをメッセージとして返す
		return aiStreamStartMsg(stream)
	}
}

// executePendingCmd はシェル経由でコマンドを実行
func executePendingCmd(command string) tea.Cmd {
	return func() tea.Msg {
		// sh -c を使うことでパイプやリダイレクトを含むコマンドも実行可能
		cmd := exec.Command("sh", "-c", command)
		output, err := cmd.CombinedOutput()

		result := ""
		if err != nil {
			result = fmt.Sprintf("✗ 実行エラー: %v\n%s", err, string(output))
		} else {
			result = fmt.Sprintf("✓ 実行成功:\n%s", string(output))
		}

		return cmdExecMsg{Result: result}
	}
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

	case "MySQL":
		// MySQLデータベース一覧を取得
		databases := monitor.GetMySQLDatabases()
		m.cachedMySQLDatabases = databases

		// データベースを追加
		for _, db := range databases {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type: "database",
				Name: db.Name,
			})
		}

	case "Redis":
		// Redisデータベース一覧を取得
		databases := monitor.GetRedisDatabases()
		m.cachedRedisDatabases = databases

		// データベースを追加
		for _, db := range databases {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type: "database",
				Name: db.Index,
			})
		}

	case "Python":
		// Pythonプロセス一覧を取得
		processes := monitor.GetPythonProcesses()
		m.cachedPythonProcesses = processes

		// プロセスを追加
		for _, proc := range processes {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type: "process",
				Name: proc.PID,
			})
		}

	case "ポート一覧":
		// ポート一覧を取得
		ports := monitor.GetListeningPorts()
		m.cachedPorts = ports

		// ポートを追加
		for _, port := range ports {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type: "port",
				Name: port.Port,
			})
		}

	case "Top 10 プロセス":
		// Top 10 プロセスを取得
		processes := monitor.GetTopProcesses(10)
		m.cachedTopProcesses = processes

		// プロセスを追加
		for _, proc := range processes {
			m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
				Type:       "process_item",
				Name:       proc.Name,
				ProcessPID: proc.PID,
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

// getSelectedMySQLDatabase returns the currently selected MySQL database
func (m Model) getSelectedMySQLDatabase() *monitor.MySQLDatabase {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// データベース以外はnil
	if selectedItem.Type != "database" {
		return nil
	}

	// データベース名から検索
	for i := range m.cachedMySQLDatabases {
		if m.cachedMySQLDatabases[i].Name == selectedItem.Name {
			return &m.cachedMySQLDatabases[i]
		}
	}

	return nil
}

// getSelectedRedisDatabase returns the currently selected Redis database
func (m Model) getSelectedRedisDatabase() *monitor.RedisDatabase {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// データベース以外はnil
	if selectedItem.Type != "database" {
		return nil
	}

	// データベースインデックスから検索
	for i := range m.cachedRedisDatabases {
		if m.cachedRedisDatabases[i].Index == selectedItem.Name {
			return &m.cachedRedisDatabases[i]
		}
	}

	return nil
}

// getSelectedPythonProcess returns the currently selected Python process
func (m Model) getSelectedPythonProcess() *monitor.PythonProcess {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]

	// プロセス以外はnil
	if selectedItem.Type != "process" {
		return nil
	}

	// PIDから検索
	for i := range m.cachedPythonProcesses {
		if m.cachedPythonProcesses[i].PID == selectedItem.Name {
			return &m.cachedPythonProcesses[i]
		}
	}

	return nil
}

// getSelectedPort returns the selected port
func (m Model) getSelectedPort() *monitor.PortInfo {
	if m.rightPanelCursor >= len(m.rightPanelItems) {
		return nil
	}

	selectedItem := m.rightPanelItems[m.rightPanelCursor]
	if selectedItem.Type != "port" {
		return nil
	}

	// ポート番号で検索
	for i := range m.cachedPorts {
		if m.cachedPorts[i].Port == selectedItem.Name {
			return &m.cachedPorts[i]
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
		} else if targetType == "mysql_database" {
			result = monitor.ExecuteMySQLCommand(target, action)
		} else if targetType == "redis_database" {
			result = monitor.ExecuteRedisCommand(target, action)
		} else if targetType == "process" {
			result = monitor.ExecuteNodeCommand(target, action)
		} else if targetType == "python_process" {
			result = monitor.ExecutePythonCommand(target, action)
		} else if targetType == "port" {
			result = monitor.ExecutePortCommand(target, action)
		} else if targetType == "top_process" {
			// Top 10 プロセスの操作
			if action == "kill_top_process" {
				result = monitor.ExecutePortCommand(target, "kill_port")
			} else if action == "force_kill_top_process" {
				result = monitor.ExecutePortCommand(target, "force_kill_port")
			}
		} else if targetType == "docker_system" {
			if action == "clean_dangling" {
				result = monitor.CleanDanglingImages()
			}
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

// fetchPortsDataCmd fetches port data
func (m Model) fetchPortsDataCmd() tea.Cmd {
	return func() tea.Msg {
		ports := monitor.GetListeningPorts()

		return portsDataMsg{
			Ports:     ports,
			UpdatedAt: time.Now(),
		}
	}
}

// fetchTopProcessesDataCmd fetches top processes data
func (m Model) fetchTopProcessesDataCmd() tea.Cmd {
	return func() tea.Msg {
		processes := monitor.GetTopProcesses(10)

		return topProcessesDataMsg{
			Processes: processes,
			UpdatedAt: time.Now(),
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
