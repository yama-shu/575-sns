package prosody_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yama-shu/575-sns/api/internal/circuitbreaker"
	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/prosody"
	"github.com/yama-shu/575-sns/api/internal/requestid"
)

// 定型の応答。prosody/openapi.json の AnalyzeResponse と同じ形。
const teikeiJSON = `{
  "verdict": "teikei",
  "normalized_text": "今日もまた会議のための会議かな",
  "reading": "キョウモマタカイギノタメノカイギカナ",
  "total_mora": 17,
  "segments": [
    {"text": "今日もまた", "reading": "キョウモマタ", "mora": 5, "expected": 5, "diff": 0},
    {"text": "会議のための", "reading": "カイギノタメノ", "mora": 7, "expected": 7, "diff": 0},
    {"text": "会議かな", "reading": "カイギカナ", "mora": 5, "expected": 5, "diff": 0}
  ],
  "reason": null,
  "unreadable": null
}`

// テスト用の設定。実時間を待たないよう、待ち時間を最小にする。
func fastOptions() prosody.Options {
	return prosody.Options{
		Timeout:    500 * time.Millisecond,
		RetryDelay: time.Millisecond,
	}
}

// newServer は prosody を模したサーバを立て、呼び出し回数を返す。
func newServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

// waitForCalls は呼び出し回数が want になるまで待つ。
//
// **数えるのはサーバのハンドラ、待つのはクライアントである。**
// リクエストが送られていても、ハンドラが動いて加算するより先に
// クライアントがタイムアウトで諦めると、その時点ではまだ増えていない。
// 返った直後に読むと不定期に落ちる（#69）。
func waitForCalls(t *testing.T, calls *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("呼び出し回数が %d にならない: calls=%d", want, calls.Load())
}

// slowHandler は d だけ待ってから応答する。
//
// **r.Context().Done() で打ち切らない。** 打ち切りを待つと、
// クライアントの切断をサーバが検知するまで httptest.Server.Close() が戻らず、
// テストが停止する。待ち時間を有限にして、確実に終わるようにする。
func slowHandler(d time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(d)
		w.WriteHeader(http.StatusOK)
	}
}

func respondJSON(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestAnalyzeReturnsTeikei(t *testing.T) {
	server, calls := newServer(t, respondJSON(teikeiJSON))
	client := prosody.New(server.URL, fastOptions())

	got, err := client.Analyze(context.Background(), "今日もまた会議のための会議かな")
	if err != nil {
		t.Fatalf("判定できない: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("成功時に %d 回呼ばれた（1回であるべき）", calls.Load())
	}
	if got.Verdict != domain.VerdictTeikei || got.TotalMora != 17 {
		t.Errorf("判定結果が違う: %+v", got)
	}
	if len(got.Segments) != 3 || got.Segments[1].Mora != 7 {
		t.Errorf("句の内訳が違う: %+v", got.Segments)
	}
	if got.NormalizedText != "今日もまた会議のための会議かな" {
		t.Errorf("正規化後の本文が違う: %q", got.NormalizedText)
	}
	if !got.Verdict.Postable() {
		t.Error("定型が投稿不可になっている")
	}
}

// 破調・unknown はエラーではない。判定を求められて判定を返している。
func TestAnalyzeReturnsHachoAndUnknownAsResult(t *testing.T) {
	tests := map[string]struct {
		body        string
		wantVerdict domain.Verdict
		wantReason  domain.Reason
		wantPost    bool
	}{
		"破調": {
			body: `{"verdict":"hacho","normalized_text":"今日は疲れた","reading":"キョウハツカレタ",
			        "total_mora":8,"segments":null,"reason":"TOO_FEW_MORA","unreadable":null}`,
			wantVerdict: domain.VerdictHacho,
			wantReason:  domain.ReasonTooFewMora,
		},
		"読めない": {
			body: `{"verdict":"unknown","normalized_text":"甃","reading":null,
			        "total_mora":null,"segments":null,
			        "reason":"READING_UNAVAILABLE","unreadable":["甃"]}`,
			wantVerdict: domain.VerdictUnknown,
			wantReason:  domain.ReasonReadingUnavailable,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server, _ := newServer(t, respondJSON(tt.body))
			client := prosody.New(server.URL, fastOptions())

			got, err := client.Analyze(context.Background(), "本文")
			if err != nil {
				t.Fatalf("エラーとして返された: %v", err)
			}
			if got.Verdict != tt.wantVerdict || got.Reason != tt.wantReason {
				t.Errorf("判定結果が違う: verdict=%v reason=%v", got.Verdict, got.Reason)
			}
			if got.Verdict.Postable() != tt.wantPost {
				t.Errorf("投稿可否が違う: %v", got.Verdict.Postable())
			}
		})
	}
}

// unknown のとき、読めなかった語が返ること。これが無いと利用者は直せない。
func TestAnalyzeReturnsUnreadableWords(t *testing.T) {
	server, _ := newServer(t, respondJSON(
		`{"verdict":"unknown","normalized_text":"甃","reading":null,"total_mora":null,
		  "segments":null,"reason":"READING_UNAVAILABLE","unreadable":["甃","閖"]}`))
	client := prosody.New(server.URL, fastOptions())

	got, err := client.Analyze(context.Background(), "甃閖")
	if err != nil {
		t.Fatalf("判定できない: %v", err)
	}
	if len(got.Unreadable) != 2 || got.Unreadable[0] != "甃" {
		t.Errorf("読めなかった語が返らない: %v", got.Unreadable)
	}
}

// 知らない値を黙って通さないこと。
// 通すと、投稿可否の判断を誤ったまま保存する。
func TestAnalyzeRejectsUnknownEnumValues(t *testing.T) {
	tests := map[string]string{
		"未知の判定": `{"verdict":"chouka","normalized_text":"x","reading":"エックス",
		               "total_mora":3,"segments":null,"reason":null,"unreadable":null}`,
		"未知の理由": `{"verdict":"hacho","normalized_text":"x","reading":"エックス",
		               "total_mora":3,"segments":null,"reason":"SOMETHING_NEW","unreadable":null}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			server, _ := newServer(t, respondJSON(body))
			client := prosody.New(server.URL, fastOptions())

			if _, err := client.Analyze(context.Background(), "本文"); err == nil {
				t.Error("知らない値が通ってしまった")
			}
		})
	}
}

// 失敗したら1回だけリトライすること。**回数を増やすと回復を妨げる。**
func TestAnalyzeRetriesOnce(t *testing.T) {
	server, calls := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := prosody.New(server.URL, fastOptions())

	if _, err := client.Analyze(context.Background(), "本文"); err == nil {
		t.Fatal("エラーにならない")
	}
	if calls.Load() != 2 {
		t.Errorf("試行回数が %d（初回 + リトライ1回であるべき）", calls.Load())
	}
}

// 1回目が失敗しても2回目が成功すれば結果を返すこと。
func TestAnalyzeSucceedsOnRetry(t *testing.T) {
	var attempt atomic.Int32
	server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if attempt.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		respondJSON(teikeiJSON)(w, nil)
	})
	client := prosody.New(server.URL, fastOptions())

	got, err := client.Analyze(context.Background(), "本文")
	if err != nil {
		t.Fatalf("リトライで回復しない: %v", err)
	}
	if got.Verdict != domain.VerdictTeikei {
		t.Errorf("判定結果が違う: %v", got.Verdict)
	}
}

// タイムアウトは UPSTREAM_TIMEOUT になること。
func TestAnalyzeTimeoutReturnsUpstreamTimeout(t *testing.T) {
	server, _ := newServer(t, slowHandler(300*time.Millisecond))
	client := prosody.New(server.URL, prosody.Options{
		Timeout:    30 * time.Millisecond,
		RetryDelay: time.Millisecond,
	})

	_, err := client.Analyze(context.Background(), "本文")
	if !errors.Is(err, domain.ErrUpstreamTimeout) {
		t.Errorf("UPSTREAM_TIMEOUT を期待したが %v", err)
	}
}

// タイムアウトは1回の試行に掛かること。
// リトライを含めた合計に掛けると、1秒に決めた意味が無くなる。
func TestAnalyzeTimeoutAppliesPerAttempt(t *testing.T) {
	server, calls := newServer(t, slowHandler(300*time.Millisecond))
	client := prosody.New(server.URL, prosody.Options{
		Timeout:    30 * time.Millisecond,
		RetryDelay: time.Millisecond,
	})

	_, _ = client.Analyze(context.Background(), "本文")

	// タイムアウトで諦めた側から見ると、2回目の到達はまだ記録されていないことがある。
	waitForCalls(t, calls, 2)
}

// 到達できない場合は PROSODY_UNAVAILABLE になること。
func TestAnalyzeUnreachableReturnsProsodyUnavailable(t *testing.T) {
	// 立てたサーバを閉じ、確実に到達できない URL を得る。
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	client := prosody.New(url, fastOptions())
	_, err := client.Analyze(context.Background(), "本文")
	if !errors.Is(err, domain.ErrProsodyUnavailable) {
		t.Errorf("PROSODY_UNAVAILABLE を期待したが %v", err)
	}
}

// 解釈できない応答も PROSODY_UNAVAILABLE になること。
func TestAnalyzeBrokenJSONReturnsProsodyUnavailable(t *testing.T) {
	server, _ := newServer(t, respondJSON(`{"verdict":`))
	client := prosody.New(server.URL, fastOptions())

	_, err := client.Analyze(context.Background(), "本文")
	if !errors.Is(err, domain.ErrProsodyUnavailable) {
		t.Errorf("PROSODY_UNAVAILABLE を期待したが %v", err)
	}
}

// 失敗が続くと遮断器が開き、**prosody を呼ばなくなる**こと。
// これが遮断器の目的そのものである。
func TestAnalyzeStopsCallingWhenBreakerOpens(t *testing.T) {
	server, calls := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := prosody.New(server.URL, prosody.Options{
		Timeout:    500 * time.Millisecond,
		RetryDelay: time.Millisecond,
		Breaker: circuitbreaker.Settings{
			WindowSize:       4,
			FailureThreshold: 2,
			OpenDuration:     time.Hour,
		},
	})

	// 2回の Analyze で遮断器から見た失敗は2回。閾値に達する。
	for range 2 {
		_, _ = client.Analyze(context.Background(), "本文")
	}
	if client.BreakerState() != circuitbreaker.StateOpen {
		t.Fatalf("遮断器が開かない: %v", client.BreakerState())
	}

	before := calls.Load()
	for range 5 {
		_, _ = client.Analyze(context.Background(), "本文")
	}
	if got := calls.Load() - before; got != 0 {
		t.Errorf("開放中に %d 回 prosody を呼んだ", got)
	}
}

// 開放中は PROSODY_UNAVAILABLE を返すこと（タイムアウトではない）。
func TestAnalyzeOpenBreakerReturnsProsodyUnavailable(t *testing.T) {
	server, _ := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := prosody.New(server.URL, prosody.Options{
		Timeout:    500 * time.Millisecond,
		RetryDelay: time.Millisecond,
		Breaker: circuitbreaker.Settings{
			WindowSize: 4, FailureThreshold: 2, OpenDuration: time.Hour,
		},
	})
	for range 2 {
		_, _ = client.Analyze(context.Background(), "本文")
	}

	_, err := client.Analyze(context.Background(), "本文")
	if !errors.Is(err, domain.ErrProsodyUnavailable) {
		t.Errorf("PROSODY_UNAVAILABLE を期待したが %v", err)
	}
}

// リクエスト ID を下流へ渡すこと。渡さないと 3サービスのログが繋がらない。
func TestAnalyzePropagatesRequestID(t *testing.T) {
	var got string
	server, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(requestid.Header)
		respondJSON(teikeiJSON)(w, r)
	})
	client := prosody.New(server.URL, fastOptions())

	ctx := requestid.With(context.Background(), "test-request-id")
	if _, err := client.Analyze(ctx, "本文"); err != nil {
		t.Fatalf("判定できない: %v", err)
	}
	if got != "test-request-id" {
		t.Errorf("リクエスト ID が渡っていない: %q", got)
	}
}

// 呼び出し元が諦めたらリトライしないこと。捨てられる結果のために下流を叩かない。
func TestAnalyzeDoesNotRetryAfterCancel(t *testing.T) {
	server, calls := newServer(t, slowHandler(300*time.Millisecond))
	client := prosody.New(server.URL, fastOptions())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, _ = client.Analyze(ctx, "本文")

	// まず1回目が届いたことを確かめる。中断した側から見ると、
	// 記録されるのが Analyze の戻りより後になることがある。
	waitForCalls(t, calls, 1)
	// そのうえで増えないことを見る。リトライが起きるならこの間に届く
	// （RetryDelay は 1ms + 揺らぎ）。
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 1 {
		t.Errorf("中断後にリトライした: calls=%d", calls.Load())
	}
}

// Ping は遮断器を通さないこと。
// 遮断中でも「いま到達できるか」を知る必要がある。
func TestPingIgnoresBreaker(t *testing.T) {
	var ready atomic.Bool
	server, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			if !ready.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := prosody.New(server.URL, prosody.Options{
		Timeout:    500 * time.Millisecond,
		RetryDelay: time.Millisecond,
		Breaker: circuitbreaker.Settings{
			WindowSize: 4, FailureThreshold: 2, OpenDuration: time.Hour,
		},
	})

	for range 2 {
		_, _ = client.Analyze(context.Background(), "本文")
	}
	if client.BreakerState() != circuitbreaker.StateOpen {
		t.Fatalf("遮断器が開かない: %v", client.BreakerState())
	}

	if err := client.Ping(context.Background()); err == nil {
		t.Error("準備できていないのに Ping が成功した")
	}
	ready.Store(true)
	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("開放中に Ping が遮断された: %v", err)
	}
}

// 設定の未指定項目が既定値で埋まること。
func TestZeroOptionsFallBackToDefaults(t *testing.T) {
	server, _ := newServer(t, respondJSON(teikeiJSON))
	client := prosody.New(server.URL, prosody.Options{})

	if _, err := client.Analyze(context.Background(), "本文"); err != nil {
		t.Errorf("既定値で動かない: %v", err)
	}
	if client.BreakerState() != circuitbreaker.StateClosed {
		t.Errorf("初期状態が closed でない: %v", client.BreakerState())
	}
}

// 待ち時間に揺らぎが加わっても動くこと。
//
// 揺らぎの値そのものは乱数のため固定できない。
// ここでは「揺らぎを挟んでもリトライが成立する」ことを固定する。
func TestAnalyzeWithJitter(t *testing.T) {
	var attempt atomic.Int32
	server, calls := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if attempt.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		respondJSON(teikeiJSON)(w, nil)
	})
	client := prosody.New(server.URL, prosody.Options{
		Timeout:    500 * time.Millisecond,
		RetryDelay: 10 * time.Millisecond,
	})

	if _, err := client.Analyze(context.Background(), "本文"); err != nil {
		t.Fatalf("揺らぎ付きで失敗した: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("試行回数が %d", calls.Load())
	}
}

// prosody へ到達できない場合、Ping が失敗すること。
// /readyz が false を返す根拠であり、Kubernetes の readiness probe が依存する。
func TestPingFailsWhenUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	client := prosody.New(url, fastOptions())
	if err := client.Ping(context.Background()); err == nil {
		t.Error("到達できないのに Ping が成功した")
	}
}
