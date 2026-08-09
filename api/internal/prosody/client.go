// Package prosody は判定エンジンへの HTTP クライアントを提供する。
//
// prosody が遅くなったり落ちたりしても api を巻き込まないよう、
// タイムアウト・リトライ・サーキットブレーカーで防御する（詳細設計 03 §4）。
//
//	prosody の応答が 30 秒かかるようになる
//	  → api のリクエスト処理が 30 秒間ブロックされる
//	  → api の接続プールが埋まる
//	  → タイムライン取得のリクエストも処理できなくなる
//	  → 判定だけの障害が、サービス全体の障害になる
//
// これを防ぐのが NFR-02-03 の縮退運転である。
// **prosody の障害を prosody の中に閉じ込める。**
package prosody

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/yama-shu/575-sns/api/internal/circuitbreaker"
	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/requestid"
)

const (
	// maxAttempts は1リクエストあたりの試行回数（初回 + リトライ1回）。
	//
	// **増やさない。** 過負荷で遅くなっている相手に対して、リトライは追加の負荷である。
	// 回数を増やすほど回復を妨げる。判定に副作用がないため1回は安全に試せる。
	maxAttempts = 2
	// defaultTimeout は1回の試行の上限（基本設計 05 §5）。
	//
	// NFR-01-01 の 150ms に対して十分な余裕がある。1秒かかるのは異常である。
	defaultTimeout = 1 * time.Second
	// defaultRetryDelay はリトライまでの待ち時間。
	defaultRetryDelay = 100 * time.Millisecond
)

// Options はクライアントの調整項目。ゼロ値は既定値になる。
type Options struct {
	// Timeout は1回の試行に掛かる上限。リトライを含めた合計ではない。
	Timeout time.Duration
	// RetryDelay はリトライまでの待ち時間。
	//
	// 実際の待ち時間は RetryDelay + 0〜RetryDelay/2 の揺らぎになる。
	// 揺らぎを外す手段は用意しない。全クライアントが同時にリトライすると
	// 波ができ、回復しかけた prosody を再び潰すためである。
	RetryDelay time.Duration
	// Breaker は遮断器の設定。
	Breaker circuitbreaker.Settings
}

// Client は prosody への HTTP クライアント。
type Client struct {
	baseURL    string
	http       *http.Client
	breaker    breaker
	retryDelay time.Duration
}

// breaker は遮断器の最小限の口。
type breaker interface {
	Do(fn func() error) error
	State() circuitbreaker.State
}

// New は prosody クライアントを生成する。
//
// **アプリケーションで1つだけ作る。** リクエストごとに作ると
// 遮断器の観測が毎回リセットされ、開放条件に到達しない。
func New(baseURL string, opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = defaultRetryDelay
	}
	if opts.Breaker.OnStateChange == nil {
		opts.Breaker.OnStateChange = logStateChange
	}
	return &Client{
		baseURL:    baseURL,
		http:       &http.Client{Timeout: opts.Timeout},
		breaker:    circuitbreaker.New(opts.Breaker),
		retryDelay: opts.RetryDelay,
	}
}

// BreakerState は遮断器の現在の状態を返す。運用時の確認とテストに使う。
func (c *Client) BreakerState() circuitbreaker.State { return c.breaker.State() }

// Ping は prosody が判定を受け付けられる状態かを確認する。
//
// /healthz ではなく /readyz を見る。prosody はプロセスが起動していても
// 辞書のロードが終わるまで判定できないため、liveness では不十分である。
//
// **遮断器を通さない。** ヘルスチェックは「いま到達できるか」を知るためのもので、
// 遮断中でも実際の状態を返す必要がある。
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

// Analyze は本文を判定する。domain.Analyzer を満たす。
//
// 破調・unknown はエラーではなく *domain.Analysis として返る。
// error になるのは prosody へ到達できないか、応答を解釈できない場合だけである。
func (c *Client) Analyze(ctx context.Context, text string) (*domain.Analysis, error) {
	var result *domain.Analysis

	err := c.breaker.Do(func() error {
		var err error
		result, err = c.analyzeWithRetry(ctx, text)
		return err
	})
	if err != nil {
		return nil, c.translate(ctx, err)
	}
	return result, nil
}

// analyzeWithRetry は試行とリトライを行う。
func (c *Client) analyzeWithRetry(ctx context.Context, text string) (*domain.Analysis, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := c.analyzeOnce(ctx, text)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// 呼び出し元が諦めている場合は、リトライしても捨てられるだけである。
		if ctx.Err() != nil {
			break
		}
		if attempt < maxAttempts {
			sleepContext(ctx, c.retryDelay+jitter(c.retryDelay/2))
		}
	}
	return nil, lastErr
}

// analyzeOnce は1回だけ呼ぶ。
func (c *Client) analyzeOnce(ctx context.Context, text string) (*domain.Analysis, error) {
	payload, err := json.Marshal(analyzeRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("リクエストを組み立てられません: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/v1/analyze", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("prosody へのリクエストを作成できません: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 3サービスのログを1本の線として追えるようにする（基本設計 01 §7）。
	if id := requestid.From(ctx); id != "" {
		req.Header.Set(requestid.Header, id)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prosody へ到達できません: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prosody が異常を返しました: status=%d", res.StatusCode)
	}

	var body analyzeResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("prosody の応答を解釈できません: %w", err)
	}
	return body.toDomain()
}

// translate は内部のエラーを利用者に返すエラーへ変える。
//
// **ここでログを出す。** 呼び出し側（usecase）は HTTP を知らないため、
// 何が起きたかを最もよく知っているのはこの層である。
func (c *Client) translate(ctx context.Context, err error) error {
	event := "prosody_call_failed"
	message := "prosody への呼び出しが失敗しました"
	appErr := domain.ErrProsodyUnavailable

	switch {
	case errors.Is(err, circuitbreaker.ErrOpen):
		event = "prosody_circuit_open"
		message = "サーキットブレーカーが開放中のため prosody を呼びませんでした"
	case isTimeout(err):
		message = "prosody への呼び出しがタイムアウトしました"
		appErr = domain.ErrUpstreamTimeout
	}

	// **本文を記録しない。** 投稿本文が含まれる。
	slog.ErrorContext(ctx, message,
		"event", event,
		"error_code", string(appErr.Code),
		"breaker_state", c.breaker.State().String(),
		"error_detail", err.Error(),
	)
	return appErr
}

// logStateChange は遮断器の遷移を記録する。
//
// 遮断中は prosody へのアクセスログが一切出ないため、
// 記録が無いと「静かに壊れている」のか「遮断している」のか区別できない。
func logStateChange(from, to circuitbreaker.State) {
	slog.Warn("prosody のサーキットブレーカーの状態が変わりました",
		"event", "prosody_circuit_state_changed",
		"from", from.String(),
		"to", to.String(),
	)
}

// isTimeout はタイムアウト由来のエラーかを返す。
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeouter interface{ Timeout() bool }
	return errors.As(err, &timeouter) && timeouter.Timeout()
}

// jitter は 0 以上 limit 未満の待ち時間を返す。
func jitter(limit time.Duration) time.Duration {
	if limit <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(limit)))
	if err != nil {
		// 乱数を引けないことは判定の失敗理由にならない。揺らぎ無しで続ける。
		return 0
	}
	return time.Duration(n.Int64())
}

// sleepContext は待つ。ただし呼び出し元が諦めたら即座に戻る。
func sleepContext(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
