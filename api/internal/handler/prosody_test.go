package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/handler"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

type stubAnalyzer struct {
	result *domain.Analysis
	err    error
}

func (s *stubAnalyzer) Analyze(context.Context, string) (*domain.Analysis, error) {
	return s.result, s.err
}

// call は判定エンドポイントを1回呼び、ステータスと本文を返す。
func call(t *testing.T, analyzer domain.Analyzer, body string) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prosody/check", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handler.NewProsody(usecase.NewProsody(analyzer))
	if err := h.Check(c); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("応答を解釈できない: %v (body=%s)", err, rec.Body.String())
	}
	return rec.Code, decoded
}

func TestCheckRespondsWithSegments(t *testing.T) {
	analyzer := &stubAnalyzer{result: &domain.Analysis{
		Verdict:   domain.VerdictTeikei,
		Reading:   "キョウモマタカイギノタメノカイギカナ",
		TotalMora: 17,
		Segments: []domain.Segment{
			{Text: "今日もまた", Reading: "キョウモマタ", Mora: 5, Expected: 5, Diff: 0},
			{Text: "会議のための", Reading: "カイギノタメノ", Mora: 7, Expected: 7, Diff: 0},
			{Text: "会議かな", Reading: "カイギカナ", Mora: 5, Expected: 5, Diff: 0},
		},
	}}

	status, body := call(t, analyzer, `{"body":"今日もまた会議のための会議かな"}`)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	if body["verdict"] != "teikei" || body["total_mora"] != float64(17) {
		t.Errorf("判定結果が違う: %v", body)
	}
	segments, ok := body["segments"].([]any)
	if !ok || len(segments) != 3 {
		t.Fatalf("句の内訳が違う: %v", body["segments"])
	}
	if segments[1].(map[string]any)["mora"] != float64(7) {
		t.Errorf("中七のモーラ数が違う: %v", segments[1])
	}
}

// **破調でも 200 を返す。** 判定を求められて判定を返しているためエラーではない。
func TestCheckRespondsWith200ForHacho(t *testing.T) {
	analyzer := &stubAnalyzer{result: &domain.Analysis{
		Verdict:   domain.VerdictHacho,
		Reading:   "キョウハツカレタ",
		TotalMora: 8,
		Reason:    domain.ReasonTooFewMora,
	}}

	status, body := call(t, analyzer, `{"body":"今日は疲れた"}`)
	if status != http.StatusOK {
		t.Fatalf("破調が %d で返された", status)
	}
	if body["reason"] != "TOO_FEW_MORA" {
		t.Errorf("理由が返らない: %v", body)
	}
	// 五七五に区切れないため区切りが定義できない。
	if body["segments"] != nil {
		t.Errorf("破調なのに区切りが返っている: %v", body["segments"])
	}
}

// unknown は total_mora を null にする。
// 0 を返すと「0 モーラだった」と読めてしまう。
func TestCheckRespondsWithNullMoraForUnknown(t *testing.T) {
	analyzer := &stubAnalyzer{result: &domain.Analysis{
		Verdict:    domain.VerdictUnknown,
		Reason:     domain.ReasonReadingUnavailable,
		Unreadable: []string{"甃"},
	}}

	status, body := call(t, analyzer, `{"body":"甃"}`)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	if _, present := body["total_mora"]; !present || body["total_mora"] != nil {
		t.Errorf("total_mora が null でない: %v", body["total_mora"])
	}
	unreadable, ok := body["unreadable"].([]any)
	if !ok || len(unreadable) != 1 || unreadable[0] != "甃" {
		t.Errorf("読めなかった語が返らない: %v", body["unreadable"])
	}
}

// 判定エンジンを利用できない場合のステータス（詳細設計 03 §2）。
func TestCheckMapsUpstreamErrorsToStatus(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		"到達できない": {domain.ErrProsodyUnavailable, http.StatusServiceUnavailable, "PROSODY_UNAVAILABLE"},
		"タイムアウト": {domain.ErrUpstreamTimeout, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			status, body := call(t, &stubAnalyzer{err: tt.err}, `{"body":"今日もまた会議のための会議かな"}`)
			if status != tt.wantStatus {
				t.Errorf("status=%d, want %d", status, tt.wantStatus)
			}
			detail, ok := body["error"].(map[string]any)
			if !ok || detail["code"] != tt.wantCode {
				t.Errorf("エラーコードが違う: %v", body)
			}
		})
	}
}

func TestCheckRejectsEmptyBody(t *testing.T) {
	status, body := call(t, &stubAnalyzer{}, `{"body":""}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d", status)
	}
	detail, ok := body["error"].(map[string]any)
	if !ok || detail["code"] != "VALIDATION_FAILED" {
		t.Errorf("エラーコードが違う: %v", body)
	}
}

func TestCheckRejectsMalformedJSON(t *testing.T) {
	status, body := call(t, &stubAnalyzer{}, `{"body":`)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d", status)
	}
	detail, ok := body["error"].(map[string]any)
	if !ok || detail["code"] != "VALIDATION_FAILED" {
		t.Errorf("エラーコードが違う: %v", body)
	}
}
