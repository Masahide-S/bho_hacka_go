package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient はOllama APIとの通信を担当するクライアントです
type OllamaClient struct {
	Endpoint   string
	HTTPClient *http.Client
	MaxRetries int
}

// Message はチャットメッセージを表します
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Options はモデルパラメータを表します
type Options struct {
	Temperature float32 `json:"temperature,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

// ChatRequest はChat APIへのリクエスト構造体です
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  *Options  `json:"options,omitempty"`
}

// chatResponse はChat APIからのレスポンス構造体です
type chatResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
	Error   string  `json:"error,omitempty"` // APIが返すエラーメッセージ
}

// GenerateResponseStream はストリーミングレスポンス用の構造体です
type GenerateResponseStream struct {
	Response string
	Done     bool
	Err      error
}

// NewOllamaClient は新しいクライアントを初期化します
func NewOllamaClient(endpoint string) *OllamaClient {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	return &OllamaClient{
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			// 修正: タイムアウトを無効化（LLMの生成は長時間かかる場合があるためContextで制御推奨）
			// 必要であれば呼び出し側でタイムアウト付きContextを渡す
			Timeout: 0,
		},
		MaxRetries: 3,
	}
}

// CheckHealth はOllamaサーバーへの接続を確認します
func (c *OllamaClient) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return fmt.Errorf("Ollama接続エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollamaステータス異常: %s", resp.Status)
	}

	return nil
}

// ListModels は利用可能なモデル一覧を取得します
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Endpoint+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// レスポンス構造体定義（内部利用）
	type listModelsResponse struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	var result listModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	return models, nil
}

// Generate はテキスト生成を行います（非ストリーミング・Chat API使用）
func (c *OllamaClient) Generate(ctx context.Context, messages []Message, model string) (string, error) {
	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options: &Options{
			NumCtx: 4096, // デフォルトより少し大きめに確保
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("APIエラー (%s): %s", resp.Status, string(body))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("Ollama API error: %s", result.Error)
	}

	return result.Message.Content, nil
}

// GenerateStream はテキスト生成をストリーミングで行います（Chat API使用）
func (c *OllamaClient) GenerateStream(ctx context.Context, messages []Message, model string) (<-chan GenerateResponseStream, error) {
	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		Options: &Options{
			NumCtx: 8192, // ログ分析用に大きめのコンテキストを確保
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}

	// 修正: ステータスコードチェックを追加
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("APIエラー (%s): %s", resp.Status, string(body))
	}

	stream := make(chan GenerateResponseStream)

	go func() {
		defer close(stream)
		defer resp.Body.Close()

		// 修正: bufio.Scanner ではなく json.Decoder を使用して安全にパース
		decoder := json.NewDecoder(resp.Body)

		for {
			// コンテキストのキャンセルチェック
			select {
			case <-ctx.Done():
				stream <- GenerateResponseStream{Err: ctx.Err()}
				return
			default:
				// 続行
			}

			var result chatResponse
			if err := decoder.Decode(&result); err != nil {
				if err == io.EOF {
					break
				}
				stream <- GenerateResponseStream{Err: fmt.Errorf("stream decode error: %w", err)}
				return
			}

			// APIエラーチェック
			if result.Error != "" {
				stream <- GenerateResponseStream{Err: fmt.Errorf("Ollama API error: %s", result.Error)}
				return
			}

			stream <- GenerateResponseStream{
				Response: result.Message.Content,
				Done:     result.Done,
			}

			if result.Done {
				return
			}
		}
	}()

	return stream, nil
}

// doRequestWithRetry はリトライロジック付きでリクエストを実行します
func (c *OllamaClient) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error

	for i := 0; i <= c.MaxRetries; i++ {
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}

		// リトライ時にBodyを巻き戻す必要がある
		if i > 0 && req.GetBody != nil {
			if body, err := req.GetBody(); err == nil {
				req.Body = body
			}
		}

		resp, err := c.HTTPClient.Do(req)
		if err == nil {
			// 500系エラーのみリトライ対象とする
			if resp.StatusCode < 500 {
				return resp, nil
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %s", resp.Status)
		} else {
			lastErr = err
		}

		if i < c.MaxRetries {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(time.Duration(i*500+500) * time.Millisecond):
				continue
			}
		}
	}

	return nil, fmt.Errorf("リトライ回数超過: %w", lastErr)
}
