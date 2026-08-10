package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Timeline はタイムラインの業務ロジック。
type Timeline struct {
	timelines domain.TimelineRepository
}

// NewTimeline をつくる。
func NewTimeline(timelines domain.TimelineRepository) *Timeline {
	return &Timeline{timelines: timelines}
}

// Public は全体タイムラインを返す。未ログインでも取得できる。
func (t *Timeline) Public(ctx context.Context, q domain.TimelineQuery) (*domain.Timeline, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	items, err := t.timelines.Public(ctx, q)
	if err != nil {
		return nil, err
	}
	return page(items, q.EffectiveLimit()), nil
}

// Home はフォロー中タイムラインを返す。ログインが必要。
func (t *Timeline) Home(ctx context.Context, q domain.TimelineQuery) (*domain.Timeline, error) {
	if q.ViewerID == nil {
		return nil, domain.ErrUnauthenticated
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	items, err := t.timelines.Home(ctx, q)
	if err != nil {
		return nil, err
	}
	return page(items, q.EffectiveLimit()), nil
}

// page は次のカーソルを決める。
//
// **limit に満たなければ続きが無い。** 返した件数が limit ちょうどのときだけ
// カーソルを返す。常に返すと、クライアントが空のページを1回余計に取りに行く。
func page(items []domain.TimelineItem, limit int) *domain.Timeline {
	timeline := &domain.Timeline{Items: items}
	if len(items) == limit {
		timeline.NextCursor = items[len(items)-1].Post.ID
	}
	return timeline
}
