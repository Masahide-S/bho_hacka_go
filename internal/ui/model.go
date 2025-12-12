package ui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Masahide-S/bho_hacka_go/internal/ai"
	"github.com/Masahide-S/bho_hacka_go/internal/db"
	"github.com/Masahide-S/bho_hacka_go/internal/llm"
	"github.com/Masahide-S/bho_hacka_go/internal/logger"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
)

// 画面モードの定義
type viewMode int

const (
	viewMonitor viewMode = iota // 通常リスト
	viewGraphRealtime           // 直近詳細グラフ (gキー)
	viewGraphHistory            // 3日間トレンド (hキー)
)

// ▼▼▼ 完全デモモード用の定義 ▼▼▼
// デモの進行フェーズ定義
const (
	DemoPhaseNormal = 0 // 正常（初期状態）
	DemoPhaseBroken = 1 // 異常発生（PostgreSQL停止）
	DemoPhaseFixed  = 2 // 復旧完了
)

// デモ用テキストデータ（詳細ビュー用）
const (
	// PostgreSQL - 正常時
	DemoTextPostgresNormal = `✓ PostgreSQL: 実行中 [:5432]
  稼働時間: 3d 12h 45m

  データベース一覧:
  - app_main_db (125MB) | Connections: 4
  - app_test_db (45MB) | Connections: 1
  - metabase (89MB) | Connections: 2`

	// PostgreSQL - 異常時
	DemoTextPostgresBroken = `✗ PostgreSQL: 停止中
  ⚠ Connection refused on port 5432
  ⚠ Last Error: Fatal: the database system is starting up

  最終正常稼働: 5秒前`

	// Docker - 正常時
	DemoTextDockerNormal = `✓ Docker: 実行中
  コンテナ: 3個 (3 Running)

  プロジェクト: my-awesome-app
  - web-frontend [:3000] | Running | CPU: 2.1% | MEM: 128MB
  - api-server [:8080] | Running | CPU: 5.3% | MEM: 256MB
  - postgres-db [:5432] | Running | CPU: 1.2% | MEM: 512MB`

	// Docker - 異常時
	DemoTextDockerBroken = `✓ Docker: 実行中
  コンテナ: 3個 (2 Running, 1 Error)

  プロジェクト: my-awesome-app
  - web-frontend [:3000] | Running
    └─ ⚠ Error: DB Connection Timeout
  - api-server [:8080] | Running
    └─ ⚠ Warning: Retrying DB connection...
  - postgres-db [:5432] | Exited (1) 5 seconds ago
    └─ ✗ Container stopped unexpectedly`

	// Node.js - 正常時
	DemoTextNodeNormal = `✓ Node.js: 実行中

  プロセス一覧:
  - PID 12345 | [:3000] | /app/frontend
    └─ CPU: 2.1% | MEM: 150MB | Uptime: 2h 15m
  - PID 12346 | [:8080] | /app/api
    └─ CPU: 5.3% | MEM: 256MB | Uptime: 2h 15m`

	// Node.js - 異常時
	DemoTextNodeBroken = `✓ Node.js: 実行中

  プロセス一覧:
  - PID 12345 | [:3000] | /app/frontend
    └─ CPU: 2.1% | MEM: 150MB | Uptime: 2h 15m
    └─ ⚠ UnhandledPromiseRejection: DB_CONN_ERR
  - PID 12346 | [:8080] | /app/api
    └─ CPU: 45.2% | MEM: 512MB | Uptime: 2h 15m
    └─ ⚠ Error: ECONNREFUSED 127.0.0.1:5432`

	// Python - 共通
	DemoTextPython = `✓ Python: 実行中

  プロセス一覧:
  - PID 23456 | [:8000] | /app/backend (FastAPI)
    └─ CPU: 3.2% | MEM: 180MB | Uptime: 1h 30m`

	// MySQL - 共通（未稼働）
	DemoTextMySQL = `✗ MySQL: 停止中
  サービスが検出されませんでした`

	// Redis - 共通（未稼働）
	DemoTextRedis = `✗ Redis: 停止中
  サービスが検出されませんでした`

	// ポート一覧 - 正常時
	DemoTextPortsNormal = `LISTEN Ports:
  :3000  | node     | PID 12345 | /app/frontend
  :5432  | postgres | PID 34567 | PostgreSQL
  :8000  | python   | PID 23456 | FastAPI
  :8080  | node     | PID 12346 | /app/api`

	// ポート一覧 - 異常時
	DemoTextPortsBroken = `LISTEN Ports:
  :3000  | node     | PID 12345 | /app/frontend
  :8000  | python   | PID 23456 | FastAPI
  :8080  | node     | PID 12346 | /app/api

  ⚠ Port 5432 (postgres) is not responding`

	// システムリソース
	DemoTextSystemResources = `システムリソース

全体:
  CPU: 12.5%%
  メモリ: 4.2GB / 16.0GB (26%%)

TOP5 リソース使用:
  1. node (PID 12346) - CPU: 5.3%% MEM: 256MB
  2. python (PID 23456) - CPU: 3.2%% MEM: 180MB
  3. node (PID 12345) - CPU: 2.1%% MEM: 150MB
  4. docker (PID 1234) - CPU: 1.5%% MEM: 512MB
  5. postgres (PID 34567) - CPU: 1.2%% MEM: 256MB`
)

// ▲▲▲ 完全デモモード用の定義 ▲▲▲

// graphDataMsg はグラフデータ取得完了時のメッセージ
type graphDataMsg struct {
	data []float64
}

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

// postgresConnectionMsg is sent when PostgreSQL connection info is fetched
type postgresConnectionMsg monitor.PostgresConnection

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
	serviceCache             map[string]*ServiceCache
	containerStatsCache      map[string]*ContainerStatsCache // コンテナID -> 統計キャッシュ
	cachedContainers         []monitor.DockerContainer       // コンテナリストのキャッシュ
	cachedPostgresDatabases  []monitor.PostgresDatabase      // PostgreSQLデータベースのキャッシュ
	cachedMySQLDatabases     []monitor.MySQLDatabase         // MySQLデータベースのキャッシュ
	cachedRedisDatabases     []monitor.RedisDatabase         // Redisデータベースのキャッシュ
	cachedNodeProcesses      []monitor.NodeProcess           // Node.jsプロセスのキャッシュ
	cachedPythonProcesses    []monitor.PythonProcess         // Pythonプロセスのキャッシュ
	cachedPorts              []monitor.PortInfo              // ポート一覧のキャッシュ
	cachedPortsUpdatedAt     time.Time                       // ポート一覧の最終更新時刻
	cachedTopProcesses       []monitor.ProcessInfo           // Top 10プロセスのキャッシュ
	cachedPostgresConnection monitor.PostgresConnection      // PostgreSQL接続情報のキャッシュ
	tickCount                int

	// Right panel navigation
	focusedPanel     string           // "left" or "right"
	rightPanelCursor int              // 右パネルのカーソル位置
	rightPanelItems  []RightPanelItem // 右パネルの選択可能な項目
	detailScroll     int              // 詳細情報のスクロール位置

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

	// --- DB関連フィールド ---
	dbStore    *db.Store
	dbChan     chan monitor.FullSnapshot // 書き込み用キュー
	lastDBSave time.Time                 // 保存間隔制御用

	// --- Graph View State ---
	currentView viewMode
	graphData   []float64
	message     string

	// --- Proactive Demo Features ---
	hasProactiveAlertShown bool   // デモ中に一度だけ発動させるためのフラグ
	proactiveMode          bool   // 自動分析モード中かどうか
	confirmMessage         string // ダイアログに表示する動的なメッセージ

	// ▼ デモ用フィールドを追加
	demoPhase int // 現在のデモフェーズ
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

// serviceStatusResultMsg はサービス状態チェックの結果を運ぶメッセージ
type serviceStatusResultMsg struct {
	Index  int
	Status string
}

// コマンド抽出用の正規表現
var cmdRegex = regexp.MustCompile(`<cmd>(.*?)</cmd>`)

// InitialModel returns the initial model (for backward compatibility)
func InitialModel() Model {
	return InitialModelWithStore(nil)
}

// InitialModelWithStore returns the initial model with database store
// 完全デモモード: 外部データ取得を行わず、デモ用初期値を使用
func InitialModelWithStore(store *db.Store) Model {
	m := Model{
		lastUpdate:   time.Now(),
		selectedItem: 0,
		menuItems: []MenuItem{
			{Name: "AI分析", Type: "ai", Status: ""},
			{Name: "────────────", Type: "separator", Status: ""},
			{Name: "PostgreSQL", Type: "service", Status: "✓"}, // デモ: 初期は正常
			{Name: "MySQL", Type: "service", Status: "✗"},      // デモ: 未稼働
			{Name: "Redis", Type: "service", Status: "✗"},      // デモ: 未稼働
			{Name: "Docker", Type: "service", Status: "✓"},     // デモ: 稼働中
			{Name: "Node.js", Type: "service", Status: "✓"},    // デモ: 稼働中
			{Name: "Python", Type: "service", Status: "✓"},     // デモ: 稼働中
			{Name: "────────────", Type: "separator", Status: ""},
			{Name: "ポート一覧", Type: "info", Status: ""},
			{Name: "Top 10 プロセス", Type: "info", Status: ""},
			{Name: "システムリソース", Type: "info", Status: ""},
		},
		aiIssueCount: 0,
		// 完全デモモード: systemResourcesは空のまま（使用しない）
		systemResources:         monitor.SystemResources{},
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
		focusedPanel:            "left",
		rightPanelCursor:        0,
		rightPanelItems:         []RightPanelItem{},
		detailScroll:            0,
		showConfirmDialog:       false,
		confirmAction:           "",
		confirmTarget:           "",
		confirmType:             "",
		lastCommandResult:       "",
		showLogView:             false,
		logContent:              "",
		logScroll:               0,
		logTargetName:           "",
		aiService:               ai.NewService(),
		aiState:                 aiStateIdle,
		aiPendingCmd:            "",
		aiCmdResult:             "",
		ollamaAvailable:         false, // Ollamaチェックは残す（AI機能のため）
		availableModels:         []string{},
		selectedModel:           0,
		dbStore:                 store,
		dbChan:                  make(chan monitor.FullSnapshot, 50),
		currentView:             viewMonitor,
		// Proactive Demo Features
		hasProactiveAlertShown: false,
		proactiveMode:          false,
		confirmMessage:         "",
		// ▼ デモ初期化: 最初は正常(0)からスタート
		demoPhase: DemoPhaseNormal,
	}

	// 裏方（DBワーカー）を始動（完全デモモードでも残す - 使われないが害はない）
	go m.startDBWorker()

	return m
}

// startDBWorker はチャネルからデータを取り出し、UIをブロックせずにDBへ書く
func (m Model) startDBWorker() {
	if m.dbStore == nil {
		return
	}
	for snapshot := range m.dbChan {
		// Store.SaveSnapshot メソッドを呼び出す
		err := m.dbStore.SaveSnapshot(snapshot.System, snapshot.Processes)
		if err != nil {
			logger.LogIssue("DB_WRITE_ERROR", err.Error())
		}
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
		m.checkHealthCmd(),
		m.fetchModelsCmd(),
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

// updateServiceStatusCmd はサービス状態を非同期でチェックするコマンドを生成します
// 完全デモモード: 外部呼び出しを一切行わず、デモフェーズに応じた固定値を返す
func (m Model) updateServiceStatusCmd() []tea.Cmd {
	var cmds []tea.Cmd

	// デモフェーズをキャプチャ
	currentPhase := m.demoPhase

	for i, item := range m.menuItems {
		if item.Type != "service" {
			continue
		}

		index := i
		serviceName := item.Name

		cmds = append(cmds, func() tea.Msg {
			var status string

			switch serviceName {
			case "PostgreSQL":
				// PostgreSQLはデモフェーズに応じて状態が変化
				if currentPhase == DemoPhaseBroken {
					status = "✗" // 異常フェーズでは停止
				} else {
					status = "✓" // 正常/復旧フェーズでは稼働
				}

			case "Docker":
				status = "✓" // 常に稼働

			case "Node.js":
				status = "✓" // 常に稼働

			case "Python":
				status = "✓" // 常に稼働

			case "MySQL":
				status = "✗" // デモでは未稼働

			case "Redis":
				status = "✗" // デモでは未稼働

			default:
				status = "✗"
			}

			return serviceStatusResultMsg{
				Index:  index,
				Status: status,
			}
		})
	}

	return cmds
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

		// ▼▼▼ デモ用隠しトリガー (Shift + E) ▼▼▼
		// プレゼンの「ここでトラブル発生！」というセリフに合わせて押す
		case "E":
			if m.demoPhase == DemoPhaseNormal {
				m.demoPhase = DemoPhaseBroken
				m.message = "⚠️ DEMO: Injecting System Failure..."
				// 全サービスの状態を即座に更新して赤色表示にする
				return m, tea.Batch(
					m.fetchAllServicesCmd(),
					tea.Batch(m.updateServiceStatusCmd()...),
				)
			}
			return m, nil
		// ▲▲▲ デモ用隠しトリガーここまで ▲▲▲

		// ESC: グラフモードから戻る、またはダイアログを閉じる
		case "esc":
			if m.currentView != viewMonitor {
				m.currentView = viewMonitor
				m.message = ""
				return m, nil
			}
			// 通常モードでのESC処理（ダイアログなどを閉じる）
			if m.showConfirmDialog {
				m.showConfirmDialog = false
				m.confirmAction = ""
				m.confirmTarget = ""
				m.confirmMessage = "" // メッセージリセット
				m.confirmType = ""
				return m, nil
			}
			if m.showLogView {
				m.showLogView = false
				m.logContent = ""
				m.logScroll = 0
				m.logTargetName = ""
				return m, nil
			}
			return m, nil

		// g: リアルタイムグラフモードへ
		case "g":
			if !m.showConfirmDialog && !m.showLogView && m.currentView == viewMonitor {
				m.currentView = viewGraphRealtime
				m.message = "Loading Realtime Graph..."
				return m, m.fetchGraphDataCmd(viewGraphRealtime)
			}

		// h: 左パネルへ移動、または左パネル時/グラフモード時はヒストリーグラフへ
		case "h", "left":
			// 既にグラフモードならヒストリーへ切り替え
			if m.currentView != viewMonitor {
				m.currentView = viewGraphHistory
				m.message = "Loading 3-Day History..."
				return m, m.fetchGraphDataCmd(viewGraphHistory)
			}
			// // 左パネルにいる場合はヒストリーグラフへ
			// if m.focusedPanel == "left" && !m.showConfirmDialog && !m.showLogView {
			// 	m.currentView = viewGraphHistory
			// 	m.message = "Loading 3-Day History..."
			// 	return m, m.fetchGraphDataCmd(viewGraphHistory)
			// }
			// 右パネルにいる場合は左パネルへ戻る
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
				// ▼▼▼ AIプロアクティブ修復の実行 ▼▼▼
				if m.confirmType == "ai_proactive" {
					cmdStr := m.aiPendingCmd

					// ダイアログを閉じる
					m.showConfirmDialog = false
					m.confirmAction = ""
					m.confirmTarget = ""
					m.confirmMessage = "" // メッセージリセット
					m.confirmType = ""

					if cmdStr != "" {
						// コマンド実行
						m.aiCmdResult = fmt.Sprintf("🚀 AI自動修復を実行中: %s...", cmdStr)
						m.aiPendingCmd = "" // リセット
						return m, executePendingCmd(cmdStr)
					}
					return m, nil
				}
				// ▲▲▲ AIプロアクティブ修復ここまで ▲▲▲

				return m.executeCommand()
			}

		case "n", "N":
			if m.showConfirmDialog {
				m.showConfirmDialog = false
				m.confirmAction = ""
				m.confirmTarget = ""
				m.confirmMessage = "" // メッセージリセット
				m.confirmType = ""
				return m, nil
			}
			if m.showLogView {
				m.showLogView = false
				m.logContent = ""
				m.logScroll = 0
				m.logTargetName = ""
				return m, nil
			}

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
		} // switch msg.String() をここで閉じる

	case containerLogsMsg:
		// コンテナログの取得結果を処理
		if msg.err != nil {
			m.lastCommandResult = fmt.Sprintf("ログ取得失敗: %v", msg.err)
			return m, nil
		} // ← ここに if の閉じ括弧が抜けていました

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

		// ▼▼▼ デモシナリオ進行 ▼▼▼
		// 自動遷移を廃止: 手動トリガー(Shift+E)に変更
		// プレゼン中に「ここでトラブルが発生します！」と言いながら E キーを押す運用
		// ▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲

		// 毎秒: サービス起動/停止チェック（非同期コマンドに変更）
		// m.updateServiceStatusCmd() の呼び出しに変更
		cmds = append(cmds, m.updateServiceStatusCmd()...)

		// ▼▼▼ プロアクティブ監視ロジック ▼▼▼
		// 3秒に1回チェック & まだアラートを出していない & AI分析中でない場合
		// デモ中は「異常フェーズ(Broken)」のときのみ発動するように条件を追加
		if m.tickCount%3 == 0 && !m.hasProactiveAlertShown && m.aiState != aiStateLoading && m.ollamaAvailable && m.demoPhase == DemoPhaseBroken {
			// デモシナリオ: PostgreSQLが落ちていたら発動
			if m.isServiceDown("PostgreSQL") {
				m.hasProactiveAlertShown = true // フラグを立てて連打防止
				m.proactiveMode = true
				m.aiState = aiStateLoading
				m.aiResponse = ""
				m.message = "🚨 異常検知! AIによる自動解析を開始します..."

				// 自動的にAI分析を開始するコマンドを返す
				return m, tea.Batch(append(cmds, m.runProactiveAnalysisCmd("PostgreSQLデータベースのサービス停止を検知しました。"))...)
			}
		}
		// ▲▲▲ プロアクティブ監視ロジック ▲▲▲

		// 完全デモモード: システムリソース更新とDB保存は行わない

		// 選択中のサービスを優先更新
		selectedItem := m.menuItems[m.selectedItem]

		if selectedItem.Type == "service" {
			// サービス詳細: 3秒ごと（選択中）
			if m.tickCount%3 == 0 {
				cmds = append(cmds, m.fetchSelectedServiceCmd())
				// PostgreSQLが選択されている場合、接続情報も非同期で取得
				if selectedItem.Name == "PostgreSQL" {
					cmds = append(cmds, fetchPostgresConnectionCmd())
				}
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

		// 完全デモモード: ログ出力は行わない

		return m, tea.Batch(cmds...)

	case graphDataMsg:
		m.graphData = msg.data
		m.message = ""
		return m, nil

	case serviceDataMsg:
		// キャッシュ更新
		m.serviceCache[msg.ServiceName] = &ServiceCache{
			Data:      msg.Data,
			UpdatedAt: msg.UpdatedAt,
			Updating:  false,
		}
		return m, nil

	case serviceStatusResultMsg:
		// 非同期サービス状態チェックの結果を反映
		if msg.Index >= 0 && msg.Index < len(m.menuItems) {
			m.menuItems[msg.Index].Status = msg.Status
		}
		return m, nil

	case executeCommandMsg:
		// コマンド実行結果を保存
		m.lastCommandResult = msg.message

		// ▼▼▼ 復旧フェーズへの移行 ▼▼▼
		// コマンドが成功したか、または異常フェーズだった場合は「復旧フェーズ」へ
		if msg.success || m.demoPhase == DemoPhaseBroken {
			m.demoPhase = DemoPhaseFixed
		}
		// ▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲

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
			// 状態アイコンの更新もかける
			tea.Batch(m.updateServiceStatusCmd()...),
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

		return m, nil

	case postgresConnectionMsg:
		// PostgreSQL接続情報のキャッシュを更新
		m.cachedPostgresConnection = monitor.PostgresConnection(msg)
		return m, nil

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
			m.proactiveMode = false // エラー時もモード終了
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

			// ▼▼▼ プロアクティブモードなら完了後に自動でダイアログを出す ▼▼▼
			if m.proactiveMode {
				m.proactiveMode = false // モード終了
				m.showConfirmDialog = true
				m.confirmType = "ai_proactive" // 専用の確認タイプ
				m.message = ""                 // メッセージをクリア

				// ダイアログメッセージの構築
				if m.aiPendingCmd != "" {
					m.confirmMessage = fmt.Sprintf(
						"⚠️ トラブルシューティング完了\n\nAIが障害を検知し、復旧策を提案しました。\n\n提案コマンド:\n%s\n\n実行して復旧しますか？",
						m.aiPendingCmd,
					)
				} else {
					m.confirmMessage = "⚠️ トラブルシューティング完了\n\nAIが分析を完了しましたが、\n実行可能なコマンドは提案されませんでした。"
				}
				m.currentStream = nil
				return m, nil
			}
			// ▲▲▲ プロアクティブモードここまで ▲▲▲

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
		// BuildSystemContext が system と user の2つを返すようになったため対応
		sysPrompt, userContext := m.aiService.BuildSystemContext()

		// ストリーミングモードで推論実行
		stream, err := m.aiService.AnalyzeStream(context.Background(), sysPrompt, userContext)
		if err != nil {
			return aiAnalysisMsg{Err: err}
		}

		// ストリームチャネルをメッセージとして返す
		return aiStreamStartMsg(stream)
	}
}

// executePendingCmd はコマンドを実行します（デモ用にモック化）
func executePendingCmd(command string) tea.Cmd {
	return func() tea.Msg {
		// ▼▼▼ デモ用モック: 特定のコマンドを検知して「成功」を偽装 ▼▼▼
		// AIが提案するコマンド "docker start postgres-db" が含まれていれば成功扱いにする
		if strings.Contains(command, "postgres-db") || strings.Contains(command, "docker start") {
			// リアル感を出すために少し待機 (800ms)
			time.Sleep(800 * time.Millisecond)

			// 成功メッセージを返す
			successOutput := "postgres-db\nRunning...\nCheck logs for details."
			result := fmt.Sprintf("✓ 実行成功 (Demo):\n%s", successOutput)

			return cmdExecMsg{Result: result}
		}
		// ▲▲▲ デモ用モックここまで ▲▲▲

		// それ以外のコマンドは実際に実行
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
		cacheValidDuration = 3 * time.Second // サービス: 3秒
	} else if selectedItem.Type == "info" {
		cacheValidDuration = 5 * time.Second // 情報: 5秒
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

	return m.fetchServiceDataCmd(serviceName)
}

// fetchAllServicesCmd fetches all services data in background
func (m Model) fetchAllServicesCmd() tea.Cmd {
	var cmds []tea.Cmd

	for _, item := range m.menuItems {
		if item.Type == "service" || item.Type == "info" {
			cmds = append(cmds, m.fetchServiceDataCmd(item.Name))
		}
	}

	return tea.Batch(cmds...)
}

// fetchServiceDataCmd fetches service data asynchronously
// 完全デモモード: 外部データ取得を一切行わず、モックデータを返す
func (m Model) fetchServiceDataCmd(serviceName string) tea.Cmd {
	// デモフェーズをキャプチャ
	phase := m.demoPhase

	return func() tea.Msg {
		var data string

		switch serviceName {
		case "PostgreSQL":
			if phase == DemoPhaseBroken {
				data = DemoTextPostgresBroken
			} else {
				data = DemoTextPostgresNormal
			}

		case "Docker":
			if phase == DemoPhaseBroken {
				data = DemoTextDockerBroken
			} else {
				data = DemoTextDockerNormal
			}

		case "Node.js":
			if phase == DemoPhaseBroken {
				data = DemoTextNodeBroken
			} else {
				data = DemoTextNodeNormal
			}

		case "Python":
			data = DemoTextPython

		case "MySQL":
			data = DemoTextMySQL

		case "Redis":
			data = DemoTextRedis

		case "ポート一覧":
			if phase == DemoPhaseBroken {
				data = DemoTextPortsBroken
			} else {
				data = DemoTextPortsNormal
			}

		case "システムリソース":
			data = DemoTextSystemResources

		case "Top 10 プロセス":
			data = `Top 10 プロセス (CPU使用率順)

  1. node (PID 12346) - CPU: 5.3% MEM: 256MB - /app/api
  2. python (PID 23456) - CPU: 3.2% MEM: 180MB - FastAPI
  3. node (PID 12345) - CPU: 2.1% MEM: 150MB - /app/frontend
  4. docker (PID 1234) - CPU: 1.5% MEM: 512MB - daemon
  5. postgres (PID 34567) - CPU: 1.2% MEM: 256MB - PostgreSQL`

		default:
			data = serviceName + " (Demo Mode)"
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
			cmds = append(cmds, m.fetchServiceDataCmd(item.Name))
		}
	}

	return tea.Batch(cmds...)
}

// updateRightPanelItems updates the right panel items based on selected service
// 完全デモモード: 外部データ取得を一切行わず、ハードコードされたモックデータを使用
func (m Model) updateRightPanelItems() Model {
	selectedItem := m.menuItems[m.selectedItem]

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
		// デモ用モックデータ: Dockerコンテナ一覧
		isExpanded, exists := expandedState["my-awesome-app"]
		if !exists {
			isExpanded = true // デフォルトで展開
		}

		// プロジェクト
		m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
			Type:        "project",
			Name:        "my-awesome-app",
			ProjectName: "my-awesome-app",
			IsExpanded:  isExpanded,
		})

		// コンテナ一覧
		m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
			Type:        "container",
			Name:        "web-frontend",
			ProjectName: "my-awesome-app",
			ContainerID: "mock_web_frontend",
		})
		m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
			Type:        "container",
			Name:        "api-server",
			ProjectName: "my-awesome-app",
			ContainerID: "mock_api_server",
		})
		m.rightPanelItems = append(m.rightPanelItems, RightPanelItem{
			Type:        "container",
			Name:        "postgres-db",
			ProjectName: "my-awesome-app",
			ContainerID: "mock_postgres_db",
		})

	case "PostgreSQL":
		// デモ用モックデータ: PostgreSQLデータベース一覧
		m.rightPanelItems = append(m.rightPanelItems,
			RightPanelItem{Type: "database", Name: "app_main_db"},
			RightPanelItem{Type: "database", Name: "app_test_db"},
			RightPanelItem{Type: "database", Name: "metabase"},
		)

	case "Node.js":
		// デモ用モックデータ: Node.jsプロセス一覧
		m.rightPanelItems = append(m.rightPanelItems,
			RightPanelItem{Type: "process", Name: "12345"},
			RightPanelItem{Type: "process", Name: "12346"},
		)

	case "MySQL":
		// デモ: MySQL未稼働
		m.rightPanelItems = []RightPanelItem{}

	case "Redis":
		// デモ: Redis未稼働
		m.rightPanelItems = []RightPanelItem{}

	case "Python":
		// デモ用モックデータ: Pythonプロセス一覧
		m.rightPanelItems = append(m.rightPanelItems,
			RightPanelItem{Type: "process", Name: "23456"},
		)

	case "ポート一覧":
		// デモ用モックデータ: ポート一覧
		m.rightPanelItems = append(m.rightPanelItems,
			RightPanelItem{Type: "port", Name: "3000"},
			RightPanelItem{Type: "port", Name: "5432"},
			RightPanelItem{Type: "port", Name: "8000"},
			RightPanelItem{Type: "port", Name: "8080"},
		)

	case "Top 10 プロセス":
		// デモ用モックデータ: Top 10プロセス
		m.rightPanelItems = append(m.rightPanelItems,
			RightPanelItem{Type: "process_item", Name: "node", ProcessPID: "12346"},
			RightPanelItem{Type: "process_item", Name: "python", ProcessPID: "23456"},
			RightPanelItem{Type: "process_item", Name: "node", ProcessPID: "12345"},
			RightPanelItem{Type: "process_item", Name: "docker", ProcessPID: "1234"},
			RightPanelItem{Type: "process_item", Name: "postgres", ProcessPID: "34567"},
		)

	default:
		m.rightPanelItems = []RightPanelItem{}
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
// 完全デモモード: 何も取得しない（右パネルのモックデータで十分）
func (m Model) fetchContainerStatsCmd() tea.Cmd {
	return nil
}

// fetchPortsDataCmd fetches port data
// 完全デモモード: 何も取得しない
func (m Model) fetchPortsDataCmd() tea.Cmd {
	return nil
}

// fetchTopProcessesDataCmd fetches top processes data
// 完全デモモード: 何も取得しない
func (m Model) fetchTopProcessesDataCmd() tea.Cmd {
	return nil
}

// fetchPostgresConnectionCmd fetches PostgreSQL connection info asynchronously
// 完全デモモード: 何も取得しない
func fetchPostgresConnectionCmd() tea.Cmd {
	return nil
}

// fetchGraphDataCmd はグラフデータを非同期で取得
func (m Model) fetchGraphDataCmd(mode viewMode) tea.Cmd {
	return func() tea.Msg {
		if m.dbStore == nil {
			return graphDataMsg{data: []float64{}}
		}

		var data []float64
		var err error

		if mode == viewGraphRealtime {
			data, err = m.dbStore.GetRecentMetrics(100)
		} else if mode == viewGraphHistory {
			data, err = m.dbStore.GetLongTermMetrics(3)
		}

		if err != nil {
			return graphDataMsg{data: []float64{}}
		}
		return graphDataMsg{data: data}
	}
}

// Run starts the TUI (for backward compatibility)
func Run() error {
	return RunWithStore(nil)
}

// RunWithStore starts the TUI with database store
func RunWithStore(store *db.Store) error {
	p := tea.NewProgram(
		InitialModelWithStore(store),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}

// ▼▼▼ プロアクティブ監視用ヘルパーメソッド ▼▼▼

// isServiceDown は指定したサービスのステータスが異常かチェックします
func (m Model) isServiceDown(serviceName string) bool {
	for _, item := range m.menuItems {
		if item.Name == serviceName && item.Status == "✗" {
			return true
		}
	}
	return false
}

// runProactiveAnalysisCmd はデモ用に特化したAI分析を実行します
// 完全デモモード: 固定のコンテキストをAIに渡し、確実に正しい復旧コマンドを提案させる
func (m Model) runProactiveAnalysisCmd(issue string) tea.Cmd {
	return func() tea.Msg {
		// デモ用の強力なシステムプロンプト（強化版）
		sysPrompt := `あなたは優秀なSRE(Site Reliability Engineer)です。
システムに発生した障害を検知しました。
即座に状況を分析し、復旧のためのDockerコマンドを提示してください。

重要ルール:
1. 解説は極めて短くすること（1行程度）。
2. 必ず実行コマンドを <cmd> と </cmd> のタグで囲んで出力すること。
3. 余計なマークダウン装飾はしないこと。

回答例:
PostgreSQLコンテナが停止しています。再起動します。
<cmd>docker start postgres-db</cmd>`

		// デモ用固定コンテキスト: 実際のシステム情報を参照せず、台本通りのデータを渡す
		// 実際のデータ収集結果（docker inspect による OOM KILLED 検出）を反映した体裁
		userContext := `緊急アラート: PostgreSQLデータベースサービスの停止を検知しました。

【検知されたエラー】
- PostgreSQL: Connection refused on port 5432
- Dockerコンテナ 'postgres-db' が停止 (Exited with code 137)
  └─ Info: ⚠️ **OOM KILLED** (メモリ不足によるプロセス強制終了)

【影響を受けているサービス】
- web-frontend: DB Connection Timeout エラー
- api-server: ECONNREFUSED 127.0.0.1:5432 エラー

【現在のコンテナ状態】
| ID | Image | Status | Ports | CPU | Mem | Info |
|---|---|---|---|---|---|---|
| a1b2 | node:18 | Up 2h | :3000 | 2.1% | 128MB | |
| c3d4 | node:18 | Up 2h | :8080 | 5.3% | 256MB | |
| e5f6 | postgres:15 | Exited (137) 5s | :5432 | - | - | ⚠️ **OOM KILLED** |

状況を分析し、docker start コマンドでpostgres-dbコンテナを再起動するコマンドを提案してください。`

		// ストリーミング分析開始
		stream, err := m.aiService.AnalyzeStream(context.Background(), sysPrompt, userContext)
		if err != nil {
			return aiAnalysisMsg{Err: err}
		}

		return aiStreamStartMsg(stream)
	}
}

// ▲▲▲ プロアクティブ監視用ヘルパーメソッド ▲▲▲
