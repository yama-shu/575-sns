package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/handler"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// stubAdminRepo は運営の操作の偽物。
type stubAdminRepo struct {
	items      []domain.PendingReport
	err        error
	resolvedID int64
	rejectedID int64
}

func (s *stubAdminRepo) PendingReports(
	context.Context, domain.PendingReportQuery,
) ([]domain.PendingReport, error) {
	return s.items, s.err
}

func (s *stubAdminRepo) Resolve(_ context.Context, reportID, _ int64, _ time.Time) error {
	s.resolvedID = reportID
	return s.err
}

func (s *stubAdminRepo) Reject(_ context.Context, reportID, _ int64, _ time.Time) error {
	s.rejectedID = reportID
	return s.err
}

func reportedItem() domain.PendingReport {
	return domain.PendingReport{
		Report: &domain.Report{
			ID: 5, ReporterID: 20, PostID: 1234,
			Reason: domain.ReportSpam, Comment: "宣伝です",
			Status: domain.ReportPending, CreatedAt: time.Date(2026, 8, 12, 1, 0, 0, 0, time.UTC),
		},
		Post:     publishedPost(),
		Author:   poster(),
		Reporter: &domain.User{ID: 20, Handle: "aoi", DisplayName: "あおい"},
	}
}

func adminOwner() *domain.User {
	return &domain.User{ID: 99, Handle: "admin", DisplayName: "運営", IsAdmin: true}
}

// callAdmin は運営のハンドラを1回呼ぶ。
func callAdmin(
	t *testing.T,
	method, path, id string,
	repo *stubAdminRepo,
	user *domain.User,
	invoke func(*handler.Admin, echo.Context) error,
) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if id != "" {
		c.SetParamNames("id")
		c.SetParamValues(id)
	}
	if user != nil {
		handler.SetCurrentUserForTest(c, user)
	}

	h := handler.NewAdmin(usecase.NewAdmin(repo, func() time.Time {
		return time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	}))
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

func TestAdminReportsResponds200(t *testing.T) {
	repo := &stubAdminRepo{items: []domain.PendingReport{reportedItem()}}
	status, body := callAdmin(t, http.MethodGet, "/api/v1/admin/reports", "",
		repo, adminOwner(), (*handler.Admin).Reports)

	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items が違う: %v", body["items"])
	}
	first, _ := items[0].(map[string]any)
	if first["reason"] != "spam" || first["comment"] != "宣伝です" {
		t.Errorf("通報の内容が違う: %v", first)
	}

	// **本文が入っていること。** 運営は投稿を見なければ判断できない。
	post, _ := first["post"].(map[string]any)
	if post["body"] == "" || post["body"] == nil {
		t.Errorf("本文が欠けている: %v", post)
	}
	segments, _ := post["segments"].([]any)
	if len(segments) != 3 {
		t.Errorf("句が3つでない: %v", post["segments"])
	}
	author, _ := post["author"].(map[string]any)
	if author["handle"] != "yamada" {
		t.Errorf("投稿者が違う: %v", author)
	}
	reporter, _ := first["reporter"].(map[string]any)
	if reporter["handle"] != "aoi" {
		t.Errorf("通報者が違う: %v", reporter)
	}
}

func TestAdminResolveAndRejectRespond204(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*handler.Admin, echo.Context) error
		check  func(*stubAdminRepo) int64
	}{
		{"対応する", (*handler.Admin).Resolve, func(s *stubAdminRepo) int64 { return s.resolvedID }},
		{"却下する", (*handler.Admin).Reject, func(s *stubAdminRepo) int64 { return s.rejectedID }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubAdminRepo{}
			status, _ := callAdmin(t, http.MethodPost, "/api/v1/admin/reports/5/resolve", "5",
				repo, adminOwner(), tt.invoke)

			if status != http.StatusNoContent {
				t.Fatalf("status=%d", status)
			}
			if tt.check(repo) != 5 {
				t.Errorf("通報の ID が渡っていない: %d", tt.check(repo))
			}
		})
	}
}

func TestAdminRejectsBadReportID(t *testing.T) {
	for _, id := range []string{"abc", "0", "-1"} {
		status, _ := callAdmin(t, http.MethodPost, "/api/v1/admin/reports/x/resolve", id,
			&stubAdminRepo{}, adminOwner(), (*handler.Admin).Resolve)

		if status != http.StatusBadRequest {
			t.Errorf("id=%s が 400 にならない: %d", id, status)
		}
	}
}

// 処理済みは 409 になること。**黙って成功にしない。**
func TestAdminAlreadyHandledResponds409(t *testing.T) {
	repo := &stubAdminRepo{err: domain.ErrAlreadyHandled}
	status, body := callAdmin(t, http.MethodPost, "/api/v1/admin/reports/5/resolve", "5",
		repo, adminOwner(), (*handler.Admin).Resolve)

	if status != http.StatusConflict {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if errObj, _ := body["error"].(map[string]any); errObj["code"] != "ALREADY_HANDLED" {
		t.Errorf("code が違う: %v", body["error"])
	}
}
