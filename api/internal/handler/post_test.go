package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/handler"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// stubPostRepo は投稿リポジトリの偽物。handler の応答の形だけを見る。
type stubPostRepo struct {
	post   *domain.Post
	author *domain.User
	err    error
	// createdBody は Create に渡された本文。
	createdBody string
}

func (s *stubPostRepo) Create(_ context.Context, post *domain.Post) (*domain.Post, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.createdBody = post.Body
	stored := *post
	stored.ID = 1234
	stored.CreatedAt = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	return &stored, nil
}

func (s *stubPostRepo) FindByID(context.Context, int64) (*domain.Post, *domain.User, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.post, s.author, nil
}

func (s *stubPostRepo) Delete(context.Context, int64, time.Time) error { return s.err }

// stubBlockRepo はブロックの偽物。handler のテストでは常に「ブロック無し」。
type stubBlockRepo struct{}

func (stubBlockRepo) Block(context.Context, int64, int64) error   { return nil }
func (stubBlockRepo) Unblock(context.Context, int64, int64) error { return nil }
func (stubBlockRepo) IsBlocked(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (stubBlockRepo) IsBlockedEitherWay(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (s *stubPostRepo) IsFollowing(context.Context, int64, int64) (bool, error) { return false, nil }
func (s *stubPostRepo) IsLikedBy(context.Context, int64, int64) (bool, error)   { return false, nil }

func poster() *domain.User {
	return &domain.User{ID: 10, Handle: "yamada", DisplayName: "やまだ"}
}

// publishedPost は公開済みの投稿。
func publishedPost() *domain.Post {
	return &domain.Post{
		ID: 1234, AuthorID: 10,
		Body:    "今日もまた会議のための会議かな",
		Verdict: domain.VerdictTeikei, Break1: 5, Break2: 11,
		MoraKami: 5, MoraNaka: 7, MoraShimo: 5,
		Visibility: domain.VisibilityPublic, Status: domain.PostPublished,
		CreatedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}
}

func hachoAnalysis() *domain.Analysis {
	return &domain.Analysis{
		Verdict: domain.VerdictHacho, Reason: domain.ReasonTooFewMora,
		Reading: "キョウハツカレタ", TotalMora: 8,
	}
}

func unknownAnalysis() *domain.Analysis {
	return &domain.Analysis{
		Verdict: domain.VerdictUnknown, Reason: domain.ReasonReadingUnavailable,
		Unreadable: []string{"甃"},
	}
}

func teikeiAnalysis() *domain.Analysis {
	return &domain.Analysis{
		Verdict:        domain.VerdictTeikei,
		NormalizedText: "今日もまた会議のための会議かな",
		Reading:        "キョウモマタカイギノタメノカイギカナ",
		TotalMora:      17,
		Segments: []domain.Segment{
			{Text: "今日もまた", Mora: 5, Expected: 5},
			{Text: "会議のための", Mora: 7, Expected: 7},
			{Text: "会議かな", Mora: 5, Expected: 5},
		},
	}
}

// callPost は投稿系のハンドラを1回呼ぶ。user が nil なら未ログイン。
func callPost(
	t *testing.T,
	method, path, body string,
	repo domain.PostRepository, analyzer domain.Analyzer,
	user *domain.User,
	invoke func(*handler.Post, echo.Context) error,
) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(strings.TrimPrefix(path, "/api/v1/posts/"))
	if user != nil {
		handler.SetCurrentUserForTest(c, user)
	}

	h := handler.NewPost(usecase.NewPost(repo, analyzer, stubBlockRepo{}, nil))
	if err := invoke(h, c); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}
	if rec.Body.Len() == 0 {
		return rec.Code, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("応答を解釈できない: %v (body=%s)", err, rec.Body.String())
	}
	return rec.Code, decoded
}

func TestCreatePostResponds201(t *testing.T) {
	repo := &stubPostRepo{}
	status, body := callPost(t, http.MethodPost, "/api/v1/posts",
		`{"body":"今日もまた会議のための会議かな","visibility":"public"}`,
		repo, &stubAnalyzer{result: teikeiAnalysis()}, poster(),
		(*handler.Post).Create)

	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%v", status, body)
	}
	// ID は文字列。数値で返すと JavaScript の Number で精度が落ちる。
	if body["id"] != "1234" {
		t.Errorf("id が文字列でない: %v", body["id"])
	}
	if body["verdict"] != "teikei" || body["like_count"] != float64(0) || body["liked_by_me"] != false {
		t.Errorf("応答の内容が違う: %v", body)
	}
	segments, ok := body["segments"].([]any)
	if !ok || len(segments) != 3 {
		t.Fatalf("句が3つでない: %v", body["segments"])
	}
	if segments[0].(map[string]any)["text"] != "今日もまた" {
		t.Errorf("上五が違う: %v", segments[0])
	}
	author, ok := body["author"].(map[string]any)
	if !ok || author["handle"] != "yamada" {
		t.Errorf("投稿者が違う: %v", body["author"])
	}
}

// クライアントが判定結果を添えても無視すること。
// 読むと「判定OK」という嘘を添えるだけで破調が保存できる。
func TestCreatePostIgnoresClientSuppliedJudgement(t *testing.T) {
	repo := &stubPostRepo{}
	// 破調の本文に「定型である」という嘘を添える。
	analyzer := &stubAnalyzer{result: &domain.Analysis{
		Verdict: domain.VerdictHacho, Reason: domain.ReasonTooFewMora, TotalMora: 8,
	}}

	status, body := callPost(t, http.MethodPost, "/api/v1/posts",
		`{"body":"今日は疲れた","verdict":"teikei","total_mora":17,
		  "segments":[{"text":"うそ","mora":5}]}`,
		repo, analyzer, poster(), (*handler.Post).Create)

	if status != http.StatusUnprocessableEntity {
		t.Fatalf("嘘の判定が通った: status=%d", status)
	}
	detail, ok := body["error"].(map[string]any)
	if !ok || detail["code"] != "PROSODY_HACHO" {
		t.Errorf("エラーコードが違う: %v", body)
	}
	if repo.createdBody != "" {
		t.Errorf("保存された: %q", repo.createdBody)
	}
}

func TestCreatePostRejectsHachoAndUnknown(t *testing.T) {
	tests := map[string]struct {
		analysis *domain.Analysis
		wantCode string
		wantKey  string
	}{
		"破調": {
			&domain.Analysis{
				Verdict: domain.VerdictHacho, Reason: domain.ReasonTooFewMora, TotalMora: 8,
			},
			"PROSODY_HACHO", "total_mora",
		},
		"読めない": {
			&domain.Analysis{
				Verdict: domain.VerdictUnknown, Reason: domain.ReasonReadingUnavailable,
				Unreadable: []string{"甃"},
			},
			"PROSODY_UNKNOWN_READING", "unreadable",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			status, body := callPost(t, http.MethodPost, "/api/v1/posts", `{"body":"本文"}`,
				&stubPostRepo{}, &stubAnalyzer{result: tt.analysis}, poster(),
				(*handler.Post).Create)

			if status != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d", status)
			}
			detail, ok := body["error"].(map[string]any)
			if !ok || detail["code"] != tt.wantCode {
				t.Fatalf("エラーコードが違う: %v", body)
			}
			details, ok := detail["details"].(map[string]any)
			if !ok || details[tt.wantKey] == nil {
				t.Errorf("利用者が直すための情報が無い: %v", detail)
			}
		})
	}
}

func TestCreatePostRequiresLogin(t *testing.T) {
	status, body := callPost(t, http.MethodPost, "/api/v1/posts", `{"body":"本文"}`,
		&stubPostRepo{}, &stubAnalyzer{result: teikeiAnalysis()}, nil,
		(*handler.Post).Create)

	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

func TestCreatePostRejectsMalformedJSON(t *testing.T) {
	status, _ := callPost(t, http.MethodPost, "/api/v1/posts", `{"body":`,
		&stubPostRepo{}, &stubAnalyzer{result: teikeiAnalysis()}, poster(),
		(*handler.Post).Create)

	if status != http.StatusBadRequest {
		t.Errorf("status=%d", status)
	}
}

func TestGetPostRespondsWithSegments(t *testing.T) {
	repo := &stubPostRepo{
		post: &domain.Post{
			ID: 1234, AuthorID: 10,
			Body:    "今日もまた会議のための会議かな",
			Verdict: domain.VerdictTeikei, Break1: 5, Break2: 11,
			MoraKami: 5, MoraNaka: 7, MoraShimo: 5,
			Visibility: domain.VisibilityPublic, Status: domain.PostPublished,
		},
		author: poster(),
	}

	status, body := callPost(t, http.MethodGet, "/api/v1/posts/1234", "",
		repo, &stubAnalyzer{}, nil, (*handler.Post).Get)

	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	segments := body["segments"].([]any)
	texts := []string{}
	for _, s := range segments {
		texts = append(texts, s.(map[string]any)["text"].(string))
	}
	want := []string{"今日もまた", "会議のための", "会議かな"}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("%d 句目が違う: %q, want %q", i+1, texts[i], want[i])
		}
	}
}

// 経路の ID が数値でないときは 400。
// 404 にすると、クライアントの組み立てミスと「存在しない投稿」を区別できない。
func TestGetPostRejectsMalformedID(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-1", ""} {
		t.Run(raw, func(t *testing.T) {
			status, body := callPost(t, http.MethodGet, "/api/v1/posts/"+raw, "",
				&stubPostRepo{}, &stubAnalyzer{}, nil, (*handler.Post).Get)

			if status != http.StatusBadRequest {
				t.Fatalf("status=%d body=%v", status, body)
			}
		})
	}
}

func TestDeletePostResponds204(t *testing.T) {
	repo := &stubPostRepo{
		post: &domain.Post{
			ID: 1234, AuthorID: 10, Body: "本文",
			Status: domain.PostPublished, Visibility: domain.VisibilityPublic,
		},
		author: poster(),
	}

	status, _ := callPost(t, http.MethodDelete, "/api/v1/posts/1234", "",
		repo, &stubAnalyzer{}, poster(), (*handler.Post).Delete)

	if status != http.StatusNoContent {
		t.Errorf("status=%d", status)
	}
}

func TestDeletePostByOtherUserResponds403(t *testing.T) {
	repo := &stubPostRepo{
		post: &domain.Post{
			ID: 1234, AuthorID: 10, Body: "本文",
			Status: domain.PostPublished, Visibility: domain.VisibilityPublic,
		},
		author: poster(),
	}
	other := &domain.User{ID: 99, Handle: "other", DisplayName: "他人"}

	status, body := callPost(t, http.MethodDelete, "/api/v1/posts/1234", "",
		repo, &stubAnalyzer{}, other, (*handler.Post).Delete)

	if status != http.StatusForbidden {
		t.Fatalf("status=%d", status)
	}
	detail, ok := body["error"].(map[string]any)
	if !ok || detail["code"] != "FORBIDDEN" {
		t.Errorf("エラーコードが違う: %v", body)
	}
}

func TestDeletePostRequiresLogin(t *testing.T) {
	status, _ := callPost(t, http.MethodDelete, "/api/v1/posts/1234", "",
		&stubPostRepo{}, &stubAnalyzer{}, nil, (*handler.Post).Delete)

	if status != http.StatusUnauthorized {
		t.Errorf("status=%d", status)
	}
}

func TestDeletePostRejectsMalformedID(t *testing.T) {
	status, _ := callPost(t, http.MethodDelete, "/api/v1/posts/abc", "",
		&stubPostRepo{}, &stubAnalyzer{}, poster(), (*handler.Post).Delete)

	if status != http.StatusBadRequest {
		t.Errorf("status=%d", status)
	}
}

// prosody が使えないときは 503。閲覧系は別途 200 を返し続ける（縮退運転）。
func TestCreatePostResponds503WhenProsodyIsDown(t *testing.T) {
	status, body := callPost(t, http.MethodPost, "/api/v1/posts",
		`{"body":"今日もまた会議のための会議かな"}`,
		&stubPostRepo{}, &stubAnalyzer{err: domain.ErrProsodyUnavailable}, poster(),
		(*handler.Post).Create)

	if status != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", status)
	}
	detail, ok := body["error"].(map[string]any)
	if !ok || detail["code"] != "PROSODY_UNAVAILABLE" {
		t.Errorf("エラーコードが違う: %v", body)
	}
}
