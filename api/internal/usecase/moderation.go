package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Moderation は通報とブロックの業務ロジック。
type Moderation struct {
	users      domain.UserRepository
	reports    domain.ReportRepository
	blocks     domain.BlockRepository
	visibility visibility
}

// NewModeration をつくる。
func NewModeration(
	users domain.UserRepository,
	posts domain.PostRepository,
	reports domain.ReportRepository,
	blocks domain.BlockRepository,
) *Moderation {
	return &Moderation{
		users:      users,
		reports:    reports,
		blocks:     blocks,
		visibility: visibility{posts: posts, blocks: blocks},
	}
}

// ReportInput は通報の入力。
type ReportInput struct {
	ReporterID int64
	PostID     int64
	Reason     domain.ReportReason
	Comment    string
}

// Report は投稿を通報する。
//
// **冪等にしない。** 2件目を作らないだけでなく、作られなかったことを
// 伝える必要がある。黙って成功を返すと「通報が届いた」と誤解する。
func (m *Moderation) Report(ctx context.Context, in ReportInput) (*domain.Report, error) {
	// 見えない投稿は通報できない。見えないものを通報する経路が無い。
	// 削除済み・非表示・ブロックの判定は visibility に集約している。
	post, _, err := m.visibility.resolve(ctx, in.PostID, &in.ReporterID)
	if err != nil {
		return nil, err
	}
	// BR-07。DB の CHECK 制約では表現できないため、ここで担保する。
	if post.AuthorID == in.ReporterID {
		return nil, domain.ErrCannotReportSelf
	}

	report, err := domain.NewReport(in.ReporterID, in.PostID, in.Reason, in.Comment)
	if err != nil {
		return nil, err
	}
	return m.reports.Create(ctx, report)
}

// Block は handle の利用者をブロックする。
//
// **冪等。** すでにブロック済みでも成功として扱う。
// フォロー関係は双方向に解除される（BR-08）。
func (m *Moderation) Block(ctx context.Context, actor *domain.User, handle string) (*domain.BlockState, error) {
	target, err := m.resolveTarget(ctx, handle)
	if err != nil {
		return nil, err
	}
	if target.ID == actor.ID {
		return nil, domain.ErrCannotBlockSelf
	}

	if err := m.blocks.Block(ctx, actor.ID, target.ID); err != nil {
		return nil, err
	}
	return &domain.BlockState{Blocked: true}, nil
}

// Unblock はブロックを解除する。フォロー関係は復活しない。
func (m *Moderation) Unblock(ctx context.Context, actor *domain.User, handle string) (*domain.BlockState, error) {
	target, err := m.resolveTarget(ctx, handle)
	if err != nil {
		return nil, err
	}
	if target.ID == actor.ID {
		return nil, domain.ErrCannotBlockSelf
	}

	if err := m.blocks.Unblock(ctx, actor.ID, target.ID); err != nil {
		return nil, err
	}
	return &domain.BlockState{Blocked: false}, nil
}

// resolveTarget は操作の相手を引く。
//
// **相手のブロックを見ない。** ブロックされている相手をブロックし返すのは
// 妨げる理由がなく、見えなくすることが目的だからである。
// フォロー（#34）と扱いが違うのは、フォローが相手との関係を作る操作なのに対し、
// ブロックは自分の側で相手を遮断する操作であるため。
func (m *Moderation) resolveTarget(ctx context.Context, handle string) (*domain.User, error) {
	target, err := m.users.FindByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	if target.Status == domain.UserDeleted {
		return nil, domain.ErrNotFound
	}
	return target, nil
}
