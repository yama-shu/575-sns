package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeAdminRepo は運営の操作の偽物。
type fakeAdminRepo struct {
	items []domain.PendingReport
	err   error

	got          domain.PendingReportQuery
	resolvedID   int64
	rejectedID   int64
	adminID      int64
	handledAt    time.Time
	resolveCalls int
	rejectCalls  int
}

func (f *fakeAdminRepo) PendingReports(
	_ context.Context, q domain.PendingReportQuery,
) ([]domain.PendingReport, error) {
	f.got = q
	return f.items, f.err
}

func (f *fakeAdminRepo) Resolve(_ context.Context, reportID, adminID int64, now time.Time) error {
	f.resolveCalls++
	f.resolvedID, f.adminID, f.handledAt = reportID, adminID, now
	return f.err
}

func (f *fakeAdminRepo) Reject(_ context.Context, reportID, adminID int64, now time.Time) error {
	f.rejectCalls++
	f.rejectedID, f.adminID, f.handledAt = reportID, adminID, now
	return f.err
}

func pendingItems(ids ...int64) []domain.PendingReport {
	items := make([]domain.PendingReport, 0, len(ids))
	for _, id := range ids {
		items = append(items, domain.PendingReport{
			Report: &domain.Report{ID: id, Status: domain.ReportPending},
			Post:   &domain.Post{ID: id},
			Author: actor(), Reporter: target(),
		})
	}
	return items
}

func adminUser() *domain.User {
	return &domain.User{ID: 99, Handle: "admin", Status: domain.UserActive, IsAdmin: true}
}

func TestAdminPendingReports(t *testing.T) {
	repo := &fakeAdminRepo{items: pendingItems(10, 20, 30)}
	fixed := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	admin := usecase.NewAdmin(repo, func() time.Time { return fixed })

	got, err := admin.PendingReports(context.Background(), domain.PendingReportQuery{
		Limit: limitOf(3),
	})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	// **カーソルはタイムラインと向きが逆。** 古い順に処理するため、最後の ID が起点になる。
	if got.NextCursor != 30 {
		t.Errorf("カーソルが違う: %d", got.NextCursor)
	}
	if len(got.Items) != 3 {
		t.Errorf("件数が違う: %d", len(got.Items))
	}
}

func TestAdminPendingReportsNoCursorWhenShort(t *testing.T) {
	repo := &fakeAdminRepo{items: pendingItems(10)}
	admin := usecase.NewAdmin(repo, nil)

	got, err := admin.PendingReports(context.Background(), domain.PendingReportQuery{
		Limit: limitOf(3),
	})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if got.NextCursor != 0 {
		t.Errorf("続きが無いのにカーソルが返る: %d", got.NextCursor)
	}
}

func TestAdminRejectsBadLimit(t *testing.T) {
	for _, limit := range []int{0, 51} {
		repo := &fakeAdminRepo{}
		admin := usecase.NewAdmin(repo, nil)

		_, err := admin.PendingReports(context.Background(), domain.PendingReportQuery{
			Limit: limitOf(limit),
		})

		var validation *domain.Error
		if !errors.As(err, &validation) || validation.Code != domain.CodeValidationFailed {
			t.Fatalf("limit=%d が検証エラーにならない: %v", limit, err)
		}
	}
}

func TestAdminResolveAndReject(t *testing.T) {
	fixed := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)

	t.Run("対応する", func(t *testing.T) {
		repo := &fakeAdminRepo{}
		admin := usecase.NewAdmin(repo, func() time.Time { return fixed })

		if err := admin.Resolve(context.Background(), adminUser(), 42); err != nil {
			t.Fatalf("対応できない: %v", err)
		}
		if repo.resolvedID != 42 || repo.adminID != 99 {
			t.Errorf("渡した値が違う: report=%d admin=%d", repo.resolvedID, repo.adminID)
		}
		// **対応した時刻を記録する。** 誰がいつ判断したかが残らないと後から追えない。
		if !repo.handledAt.Equal(fixed) {
			t.Errorf("時刻が違う: %v", repo.handledAt)
		}
		if repo.rejectCalls != 0 {
			t.Error("却下も呼んでいる")
		}
	})

	t.Run("却下する", func(t *testing.T) {
		repo := &fakeAdminRepo{}
		admin := usecase.NewAdmin(repo, func() time.Time { return fixed })

		if err := admin.Reject(context.Background(), adminUser(), 7); err != nil {
			t.Fatalf("却下できない: %v", err)
		}
		if repo.rejectedID != 7 {
			t.Errorf("渡した値が違う: %d", repo.rejectedID)
		}
		if repo.resolveCalls != 0 {
			t.Error("対応も呼んでいる")
		}
	})
}

func TestAdminErrorPropagates(t *testing.T) {
	repo := &fakeAdminRepo{err: domain.ErrAlreadyHandled}
	admin := usecase.NewAdmin(repo, nil)

	if err := admin.Resolve(context.Background(), adminUser(), 1); !errors.Is(err, domain.ErrAlreadyHandled) {
		t.Fatalf("エラーが伝わっていない: %v", err)
	}
}
