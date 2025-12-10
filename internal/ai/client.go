package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

const (
	// OllamaのデフォルトURL
	OllamaURL = "http://localhost:11434/api/generate"
	// 使用するモデル（事前に ollama pull llama3.2 などを実行してください）
	ModelName = "llama3.2" 
)

// OllamaRequest はAPIへのリクエスト構造体
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// OllamaResponse はAPIからのレスポンス構造体
type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Service はAI機能を提供します
type Service struct {
	client *http.Client
}

// NewService は新しいAIサービスを作成します
func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 0 * time.Second,
		},
	}
}

// BuildSystemContext は現在のシステム状態を収集してプロンプト用テキストを作成します
func (s *Service) BuildSystemContext() string {
	// 各モニターから情報を収集（RAG: Retrieval）
	sysRes := monitor.GetSystemResources()
	
	// 各チェック関数の結果を取得
	// 注意: 実際の実装ではログ取得などをここに追加するとさらに精度が上がります
	dockerStatus := monitor.CheckDocker()
	nodeStatus := monitor.CheckNodejs()
	pythonStatus := monitor.CheckPython()
	postgresStatus := monitor.CheckPostgres()
	ports := monitor.ListAllPorts()

	// プロンプトの構築
	prompt := fmt.Sprintf(`
あなたは開発環境のトラブルシューティングを行う優秀なAIアシスタントです。
以下のシステム状況を分析し、開発者が気づくべき問題点や改善案を簡潔に指摘してください。
深刻なエラーがある場合は、具体的な解決コマンドを <cmd>コマンド</cmd> の形式で提案してください。

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

回答形式:
- 短い要約
- 発見された問題点（あれば）
- 推奨されるアクション
`, 
		sysRes.CPUUsage, float64(sysRes.MemoryUsed)/1024, float64(sysRes.MemoryTotal)/1024, sysRes.MemoryPerc,
		dockerStatus, nodeStatus, pythonStatus, postgresStatus, ports,
	)

	return prompt
}

// Analyze はOllamaに問い合わせを行います
func (s *Service) Analyze(prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model:  ModelName,
		Prompt: prompt,
		Stream: false, // 簡単のためストリームなしで実装
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Post(OllamaURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("Ollama接続エラー: %v (Ollamaは起動していますか？)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("APIエラー: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", err
	}

	return ollamaResp.Response, nil
}