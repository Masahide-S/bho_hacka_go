package ai

import (
	"context"

	"github.com/Masahide-S/bho_hacka_go/internal/llm"
	// monitorパッケージの直接参照は削除し、llmパッケージ経由でデータ取得します
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
// 変更点: internal/llm/data.go の CollectAllContext を使用し、JSON形式で情報を渡すように変更
func (s *Service) BuildSystemContext() (string, string) {
	// 1. システムプロンプト（デモ用・強力なコマンド誘導）
	systemPrompt := `あなたは開発環境のトラブルシューティングを行う優秀なDevOpsアシスタントです。
ユーザーから提供される「システム状況レポート(JSON形式)」を分析し、以下の手順で回答してください。

1. **現状分析**:
   - 停止しているコンテナ (Status: Exitedなど)
   - エラーが出ているデータベース
   - 異常にCPU/メモリを消費しているプロセス
   これらがないか確認してください。

2. **報告**:
   - 発見された問題点、または「システムは正常です」という結果を、**日本語で簡潔に** 述べてください。

3. **解決策の提案 (重要)**:
   - 問題を解決するために実行すべきコマンドを **1つだけ** 提案してください。
   - **コマンドは必ず <cmd> と </cmd> のタグで囲んでください。** これによりUIが自動的に実行ボタンを表示します。

---
**回答例1 (コンテナ停止時):**
PostgreSQLのコンテナが停止しています。再起動が必要です。
<cmd>docker start postgres-container</cmd>

**回答例2 (システム正常時):**
システムリソース、各サービス共に正常に稼働しています。
念のためディスク容量を確認しますか？
<cmd>df -h</cmd>
---
`

	// 2. データ収集 (RAG)
	// internal/llm/data.go のリッチな収集機能を利用
	fullCtx, err := llm.CollectAllContext()

	var userContext string
	if err != nil {
		// 万が一収集に失敗した場合のフォールバック
		userContext = "システム情報の取得に失敗しました: " + err.Error()
	} else {
		// JSON化してLLMに渡す（構造化データの方がLLMの理解度が高い）
		jsonStr, err := llm.FormatAsJSON(fullCtx)
		if err != nil {
			userContext = "コンテキストのJSON変換に失敗しました"
		} else {
			userContext = "以下は現在のシステム状況レポートです:\n" + jsonStr
		}
	}

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

// AnalyzeStream はストリーミングでOllamaに問い合わせを行います
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
