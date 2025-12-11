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
			{Name: "システムリソース", Type: "info", Status: ""},
		},
		aiIssueCount:    0,
		systemResources: monitor.GetSystemResources(),
		serviceCache:    make(map[string]*ServiceCache),
		tickCount:       0,

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

		case "up", "k":
			m.selectedItem--
			if m.selectedItem >= 0 && m.menuItems[m.selectedItem].Type == "separator" {
				m.selectedItem--
			}
			if m.selectedItem < 0 {
				m.selectedItem = len(m.menuItems) - 1
			}
			// 選択変更時、キャッシュが古ければ再取得
			return m, m.fetchSelectedServiceCmd()

		case "down", "j":
			m.selectedItem++
			if m.selectedItem < len(m.menuItems) && m.menuItems[m.selectedItem].Type == "separator" {
				m.selectedItem++
			}
			if m.selectedItem >= len(m.menuItems) {
				m.selectedItem = 0
			}
			// 選択変更時、キャッシュが古ければ再取得
			return m, m.fetchSelectedServiceCmd()

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

	// 更新中フラグを立てる
	if _, exists := m.serviceCache[serviceName]; exists {
		m.serviceCache[serviceName].Updating = true
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

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(
		InitialModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
