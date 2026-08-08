// Package prosody は判定エンジンへの HTTP クライアントを提供する。
//
// 本パッケージは開発環境構築（#2）時点では疎通確認のみを担う。
// 判定呼び出し（Analyze）とタイムアウト / リトライ / サーキットブレーカーは M2 で実装する。
package prosody

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Client は prosody への HTTP クライアント。
type Client struct {
	baseURL string
	http    *http.Client
}

// New は prosody クライアントを生成する。
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

// Ping は prosody が判定を受け付けられる状態かを確認する。
//
// /healthz ではなく /readyz を見る。prosody はプロセスが起動していても
// 辞書のロードが終わるまで判定できないため、liveness では不十分である。
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return fmt.Errorf("prosody へのリクエストを作成できません: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("prosody へ到達できません: %w", err)
	}
	// Close の失敗は呼び出し側にできることがないため無視する。
	// ただし無視していることを明示する（暗黙に捨てない）。
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("prosody が準備できていません: status=%d", res.StatusCode)
	}
	return nil
}
