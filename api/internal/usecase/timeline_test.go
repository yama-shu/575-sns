package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeTimelineRepo はタイムラインの偽物。
//
// 渡された取得条件を記録し、usecase が既定値と上限を正しく解決しているかを見る。
type fakeTimelineRepo struct {
	items []domain.TimelineItem
	err   error

	publicCalls int
	homeCalls   int
	gotQuery    domain.TimelineQuery
}

func (f *fakeTimelineRepo) Public(_ context.Context, q domain.TimelineQuery) ([]domain.TimelineItem, error) {
	f.publicCalls++
	f.gotQuery = q
	return f.items, f.err
}

func (f *fakeTimelineRepo) Home(_ context.Context, q domain.TimelineQuery) ([]domain.TimelineItem, error) {
	f.homeCalls++
	f.gotQuery = q
	return f.items, f.err
}

// itemsOf は ID だけを持つ件数ぶんの項目を返す。
func itemsOf(ids ...int64) []domain.TimelineItem {
	items := make([]domain.TimelineItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, domain.TimelineItem{
			Post:   &domain.Post{ID: id},
			Author: author(),
		})
	}
	return items
}

func viewer(id int64) *int64 { return &id }

func limitOf(n int) *int { return &n }

func TestPublicTimeline(t *testing.T) {
	repo := &fakeTimelineRepo{items: itemsOf(3, 2, 1)}
	tl := usecase.NewTimeline(repo)

	got, err := tl.Public(context.Background(), domain.TimelineQuery{})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if len(got.Items) != 3 {
		t.Errorf("件数が違う: %d", len(got.Items))
	}
	// 3件しか無く limit は既定の 20。続きは無い。
	if got.NextCursor != 0 {
		t.Errorf("続きが無いのにカーソルが返る: %d", got.NextCursor)
	}
}

// limit を省略すると既定値が使われること。
func TestTimelineUsesDefaultLimit(t *testing.T) {
	repo := &fakeTimelineRepo{}
	tl := usecase.NewTimeline(repo)

	if _, err := tl.Public(context.Background(), domain.TimelineQuery{}); err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if repo.gotQuery.EffectiveLimit() != domain.DefaultTimelineLimit {
		t.Errorf("既定値が使われていない: %d", repo.gotQuery.EffectiveLimit())
	}
}

// limit ちょうど返ったときだけカーソルを返すこと。
//
// 常に返すと、クライアントが空のページを1回余計に取りに行く。
func TestTimelineNextCursor(t *testing.T) {
	tests := map[string]struct {
		items      []domain.TimelineItem
		limit      int
		wantCursor int64
	}{
		"limit ちょうど": {itemsOf(5, 4, 3), 3, 3},
		"limit 未満":   {itemsOf(5, 4), 3, 0},
		"0件":         {nil, 3, 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repo := &fakeTimelineRepo{items: tt.items}
			tl := usecase.NewTimeline(repo)

			got, err := tl.Public(context.Background(), domain.TimelineQuery{Limit: limitOf(tt.limit)})
			if err != nil {
				t.Fatalf("取得できない: %v", err)
			}
			if got.NextCursor != tt.wantCursor {
				t.Errorf("カーソルが違う: %d, want %d", got.NextCursor, tt.wantCursor)
			}
		})
	}
}

// limit の範囲を外れたら取得しない。
func TestTimelineRejectsInvalidLimit(t *testing.T) {
	for _, limit := range []int{0, -1, domain.MaxTimelineLimit + 1} {
		t.Run("", func(t *testing.T) {
			repo := &fakeTimelineRepo{}
			tl := usecase.NewTimeline(repo)

			_, err := tl.Public(context.Background(), domain.TimelineQuery{Limit: limitOf(limit)})
			var appErr *domain.Error
			if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
				t.Fatalf("limit=%d で VALIDATION_FAILED を期待したが %v", limit, err)
			}
			if repo.publicCalls != 0 {
				t.Errorf("検証に落ちたのに取得した: %d 回", repo.publicCalls)
			}
		})
	}
}

// 上限ちょうどは通ること。境界を固定する。
func TestTimelineAcceptsMaxLimit(t *testing.T) {
	repo := &fakeTimelineRepo{}
	tl := usecase.NewTimeline(repo)

	if _, err := tl.Public(context.Background(), domain.TimelineQuery{
		Limit: limitOf(domain.MaxTimelineLimit),
	}); err != nil {
		t.Errorf("上限ちょうどが弾かれた: %v", err)
	}
}

func TestTimelineRejectsNegativeCursor(t *testing.T) {
	repo := &fakeTimelineRepo{}
	tl := usecase.NewTimeline(repo)

	_, err := tl.Public(context.Background(), domain.TimelineQuery{
		Cursor: -1, Limit: limitOf(10),
	})
	var appErr *domain.Error
	if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("VALIDATION_FAILED を期待したが %v", err)
	}
	if repo.publicCalls != 0 {
		t.Errorf("検証に落ちたのに取得した: %d 回", repo.publicCalls)
	}
}

// カーソルがそのまま渡ること。
func TestTimelinePassesCursor(t *testing.T) {
	repo := &fakeTimelineRepo{}
	tl := usecase.NewTimeline(repo)

	if _, err := tl.Public(context.Background(), domain.TimelineQuery{Cursor: 42}); err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if repo.gotQuery.Cursor != 42 {
		t.Errorf("カーソルが渡っていない: %d", repo.gotQuery.Cursor)
	}
}

// フォロー中タイムラインはログインが必要。
func TestHomeTimelineRequiresLogin(t *testing.T) {
	repo := &fakeTimelineRepo{}
	tl := usecase.NewTimeline(repo)

	_, err := tl.Home(context.Background(), domain.TimelineQuery{})
	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("UNAUTHENTICATED を期待したが %v", err)
	}
	if repo.homeCalls != 0 {
		t.Errorf("未ログインなのに取得した: %d 回", repo.homeCalls)
	}
}

func TestHomeTimeline(t *testing.T) {
	repo := &fakeTimelineRepo{items: itemsOf(9, 8)}
	tl := usecase.NewTimeline(repo)

	got, err := tl.Home(context.Background(), domain.TimelineQuery{ViewerID: viewer(1)})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if len(got.Items) != 2 || repo.homeCalls != 1 {
		t.Errorf("取得内容が違う: %d 件 / %d 回", len(got.Items), repo.homeCalls)
	}
	if repo.gotQuery.ViewerID == nil || *repo.gotQuery.ViewerID != 1 {
		t.Errorf("閲覧者が渡っていない: %v", repo.gotQuery.ViewerID)
	}
}

// 全体タイムラインは未ログインでも取得できる。
func TestPublicTimelineWithoutLogin(t *testing.T) {
	repo := &fakeTimelineRepo{items: itemsOf(1)}
	tl := usecase.NewTimeline(repo)

	if _, err := tl.Public(context.Background(), domain.TimelineQuery{}); err != nil {
		t.Errorf("未ログインで取得できない: %v", err)
	}
	if repo.gotQuery.ViewerID != nil {
		t.Errorf("閲覧者が入っている: %v", repo.gotQuery.ViewerID)
	}
}

func TestTimelinePropagatesError(t *testing.T) {
	repo := &fakeTimelineRepo{err: errors.New("取得できない")}
	tl := usecase.NewTimeline(repo)

	if _, err := tl.Public(context.Background(), domain.TimelineQuery{}); err == nil {
		t.Error("エラーにならない")
	}
	if _, err := tl.Home(context.Background(), domain.TimelineQuery{ViewerID: viewer(1)}); err == nil {
		t.Error("エラーにならない")
	}
}

// ログイン済みでも limit の範囲外なら取得しない。
func TestHomeTimelineRejectsInvalidLimit(t *testing.T) {
	repo := &fakeTimelineRepo{}
	tl := usecase.NewTimeline(repo)

	_, err := tl.Home(context.Background(), domain.TimelineQuery{
		ViewerID: viewer(1), Limit: limitOf(domain.MaxTimelineLimit + 1),
	})
	var appErr *domain.Error
	if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("VALIDATION_FAILED を期待したが %v", err)
	}
	if repo.homeCalls != 0 {
		t.Errorf("検証に落ちたのに取得した: %d 回", repo.homeCalls)
	}
}
