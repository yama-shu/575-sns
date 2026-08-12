package usecase

import (
	"context"
	"time"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Admin は運営の業務ロジック（S-13）。
//
// **認可はここで見ない。** 運営かどうかの判定は handler の RequireAdmin が行い、
// 通らなければハンドラに到達しない。二重に書くと、片方だけ直したときに食い違う。
type Admin struct {
	admins domain.AdminRepository
	now    Clock
}

// NewAdmin をつくる。
func NewAdmin(admins domain.AdminRepository, now Clock) *Admin {
	if now == nil {
		now = time.Now
	}
	return &Admin{admins: admins, now: now}
}

// PendingReports は未対応の通報を古い順に返す。
func (a *Admin) PendingReports(
	ctx context.Context, q domain.PendingReportQuery,
) (*domain.PendingReports, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	items, err := a.admins.PendingReports(ctx, q)
	if err != nil {
		return nil, err
	}

	list := &domain.PendingReports{Items: items}
	// limit に満たなければ続きが無い。常に返すと空のページを1回余計に取りに行く。
	if len(items) == q.EffectiveLimit() {
		list.NextCursor = items[len(items)-1].Report.ID
	}
	return list, nil
}

// Resolve は通報に対応する。投稿を非表示にする。
func (a *Admin) Resolve(ctx context.Context, admin *domain.User, reportID int64) error {
	return a.admins.Resolve(ctx, reportID, admin.ID, a.now())
}

// Reject は通報を却下する。投稿は変わらない。
func (a *Admin) Reject(ctx context.Context, admin *domain.User, reportID int64) error {
	return a.admins.Reject(ctx, reportID, admin.ID, a.now())
}
