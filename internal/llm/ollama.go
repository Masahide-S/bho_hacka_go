package llm

import (
	"bufio"
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

// APIリクエスト/レスポンス用の構造体定義
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type listModelsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// GenerateResponseStream はストリーミングレスポンス用の構造体です
// エラーが発生した場合、Errに値が入ります
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
			Timeout: 60 * time.Second, // デフォルトタイムアウト
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

// Generate はテキスト生成を行います（非ストリーミング）
func (c *OllamaClient) Generate(ctx context.Context, prompt string, model string) (string, error) {
	reqBody := generateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint+"/api/generate", bytes.NewBuffer(jsonData))
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

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}

// GenerateStream はテキスト生成をストリーミングで行います
// 改善点: 戻り値を文字列チャネルから構造体チャネルに変更し、エラー伝播可能に
// 改善点: 初期接続時にリトライロジックを適用
func (c *OllamaClient) GenerateStream(ctx context.Context, prompt string, model string) (<-chan GenerateResponseStream, error) {
	reqBody := generateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// 改善: ストリーミングでも初期接続にはリトライを使用する
	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}

	// レスポンス処理用のチャネル（構造体に変更）
	stream := make(chan GenerateResponseStream)

	go func() {
		defer close(stream)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				stream <- GenerateResponseStream{Err: ctx.Err()}
				return
			default:
				var result generateResponse
				if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
					// JSONパースエラーはスキップせず続行
					continue
				}

				// 正常なデータを送信
				stream <- GenerateResponseStream{
					Response: result.Response,
					Done:     result.Done,
				}

				if result.Done {
					return
				}
			}
		}

		// 改善: スキャンエラー（途中切断など）のチェック
		if err := scanner.Err(); err != nil {
			stream <- GenerateResponseStream{Err: fmt.Errorf("stream interrupted: %w", err)}
		}
	}()

	return stream, nil
}

// doRequestWithRetry はリトライロジック付きでリクエストを実行します
func (c *OllamaClient) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error

	for i := 0; i <= c.MaxRetries; i++ {
		// コンテキストのキャンセルチェック
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}

		resp, err := c.HTTPClient.Do(req)
		if err == nil {
			// 成功または500系以外のエラーならそのまま返す
			if resp.StatusCode < 500 {
				return resp, nil
			}
			resp.Body.Close() // サーバーエラーの場合は閉じてリトライ
			lastErr = fmt.Errorf("server error: %s", resp.Status)
		} else {
			lastErr = err
		}

		// 最後の試行でなければ待機
		if i < c.MaxRetries {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(time.Duration(i*500+500) * time.Millisecond): // エクスポネンシャルバックオフ気味に待機
				// リクエストボディを巻き戻す必要がある場合はここで対応が必要だが、
				// 今回はbytes.Bufferを使っているのでGetBodyがあれば自動で処理される
				if req.GetBody != nil {
					if body, err := req.GetBody(); err == nil {
						req.Body = body
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("リトライ回数超過: %w", lastErr)
}
