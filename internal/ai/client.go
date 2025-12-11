package ai

import (
	"context"
	"fmt"

	"github.com/Masahide-S/bho_hacka_go/internal/llm"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

const (
	// OllamaのデフォルトURL
	OllamaEndpoint = "http://localhost:11434"
	// デフォルトモデル名
	DefaultModelName = "llama3.2"
)

// Service はAI機能を提供します
type Service struct {
	client *llm.OllamaClient
	Model  string // 現在選択中のモデル
}

// NewService は新しいAIサービスを作成します
func NewService() *Service {
	return &Service{
		client: llm.NewOllamaClient(OllamaEndpoint),
		Model:  DefaultModelName,
	}
}

// SetModel は使用するモデルを変更します
func (s *Service) SetModel(model string) {
	s.Model = model
}

// GetModel は現在使用中のモデル名を取得します
func (s *Service) GetModel() string {
	return s.Model
}

// BuildSystemContext は現在のシステム状態を収集し、システムプロンプトとユーザーコンテキストを返します
// 戻り値: (systemPrompt, userContext)
func (s *Service) BuildSystemContext() (string, string) {
	// AIへの役割指示（System Prompt）
	systemPrompt := `あなたは開発環境のトラブルシューティングを行う優秀なAIアシスタントです。
提供されるシステム状況を分析し、開発者が気づくべき問題点や改善案を簡潔に指摘してください。
深刻なエラーがある場合は、具体的な解決コマンドを <cmd>コマンド</cmd> の形式で提案してください。
回答形式:
- 短い要約
- 発見された問題点（あれば）
- 推奨されるアクション`

	// 各モニターから情報を収集（RAG: Retrieval）
	sysRes := monitor.GetSystemResources()

	dockerStatus := monitor.CheckDocker()
	nodeStatus := monitor.CheckNodejs()
	pythonStatus := monitor.CheckPython()
	postgresStatus := monitor.CheckPostgres()
	ports := monitor.ListAllPorts()

	// 分析対象のデータ（User Context）
	userContext := fmt.Sprintf(`
[システムリソース]
CPU: %.1f%%
Memory: %.1fGB / %.1fGB (%.0f%%)

[Docker状態]
%s

[Node.js状態]
%s

[Python状態]
%s

[データベース状態]
%s

[ポート使用状況]
%s
`,
		sysRes.CPUUsage, float64(sysRes.MemoryUsed)/1024, float64(sysRes.MemoryTotal)/1024, sysRes.MemoryPerc,
		dockerStatus, nodeStatus, pythonStatus, postgresStatus, ports,
	)

	return systemPrompt, userContext
}

// buildMessages はシステムプロンプトとユーザー入力をチャットメッセージ形式に変換します
func (s *Service) buildMessages(systemPrompt, userPrompt string) []llm.Message {
	return []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
}

// Analyze はOllamaに問い合わせを行います（非ストリーミング）
func (s *Service) Analyze(prompt string) (string, error) {
	ctx := context.Background()
	msgs := []llm.Message{
		{Role: "user", Content: prompt},
	}
	return s.client.Generate(ctx, msgs, s.Model)
}

// AnalyzeWithContext はコンテキスト付きでOllamaに問い合わせを行います
func (s *Service) AnalyzeWithContext(ctx context.Context, prompt string) (string, error) {
	msgs := []llm.Message{
		{Role: "user", Content: prompt},
	}
	return s.client.Generate(ctx, msgs, s.Model)
}

// AnalyzeStream はストリーミングでOllamaに問い合わせを行います（システムプロンプトとユーザープロンプトを分離）
func (s *Service) AnalyzeStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan llm.GenerateResponseStream, error) {
	msgs := s.buildMessages(systemPrompt, userPrompt)
	return s.client.GenerateStream(ctx, msgs, s.Model)
}

// CheckHealth はOllamaサーバーへの接続を確認します
func (s *Service) CheckHealth(ctx context.Context) error {
	return s.client.CheckHealth(ctx)
}

// ListModels は利用可能なモデル一覧を取得します
func (s *Service) ListModels(ctx context.Context) ([]string, error) {
	return s.client.ListModels(ctx)
}
