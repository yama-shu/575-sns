package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeProfileRepo は数え上げの偽物。
//
// **渡された includeFollowersOnly を記録する。** フォロワー限定の投稿を
// 数に含めるかは閲覧者との関係で決まり、そこを取り違えると
// 「10句」と出ているのに3件しか見えない状態になる。
type fakeProfileRepo struct {
	counts domain.ProfileCounts
	err    error

	calls                   int
	gotIncludeFollowersOnly bool
}

func (f *fakeProfileRepo) Counts(
	_ context.Context, _ int64, includeFollowersOnly bool,
) (domain.ProfileCounts, error) {
	f.calls++
	f.gotIncludeFollowersOnly = includeFollowersOnly
	return f.counts, f.err
}

type profileFixture struct {
	usecase   *usecase.Profile
	users     *followUserRepo
	profiles  *fakeProfileRepo
	timelines *fakeTimelineRepo
	follows   *fakeFollowRepo
	blocks    *fakeBlockRepo
}

func newProfileFixture(users ...*domain.User) *profileFixture {
	f := &profileFixture{
		users:     usersOf(users...),
		profiles:  &fakeProfileRepo{counts: domain.ProfileCounts{Posts: 3, Following: 2, Followers: 1}},
		timelines: &fakeTimelineRepo{},
		follows:   newFollowRepo(),
		blocks:    newBlockRepo(),
	}
	f.usecase = usecase.NewProfile(f.users, f.profiles, f.timelines, f.follows, f.blocks)
	return f
}

// ----------------------------------------------------------------------
// プロフィールの取得
// ----------------------------------------------------------------------

func TestProfileGetUnauthenticated(t *testing.T) {
	f := newProfileFixture(actor(), target())

	got, err := f.usecase.Get(context.Background(), "bob", nil)
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if got.User.Handle != "bob" {
		t.Errorf("識別名が違う: %s", got.User.Handle)
	}
	if got.Following || got.Blocking {
		t.Error("未ログインなのに関係が真になっている")
	}
	if f.profiles.gotIncludeFollowersOnly {
		t.Error("未ログインにフォロワー限定の投稿を数えている")
	}
	if got.Counts.Posts != 3 || got.Counts.Following != 2 || got.Counts.Followers != 1 {
		t.Errorf("数が違う: %+v", got.Counts)
	}
}

func TestProfileGetSelfSeesFollowersOnly(t *testing.T) {
	f := newProfileFixture(actor())

	got, err := f.usecase.Get(context.Background(), "alice", viewer(1))
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if !f.profiles.gotIncludeFollowersOnly {
		t.Error("本人にフォロワー限定の投稿を数えていない")
	}
	if got.Following {
		t.Error("自分をフォローしていることになっている")
	}
}

func TestProfileGetFollowerSeesFollowersOnly(t *testing.T) {
	f := newProfileFixture(actor(), target())
	f.follows.following[[2]int64{1, 2}] = true

	got, err := f.usecase.Get(context.Background(), "bob", viewer(1))
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if !got.Following {
		t.Error("フォロー中が偽になっている")
	}
	if !f.profiles.gotIncludeFollowersOnly {
		t.Error("フォロワーにフォロワー限定の投稿を数えていない")
	}
}

func TestProfileGetStrangerDoesNotSeeFollowersOnly(t *testing.T) {
	f := newProfileFixture(actor(), target())

	if _, err := f.usecase.Get(context.Background(), "bob", viewer(1)); err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if f.profiles.gotIncludeFollowersOnly {
		t.Error("フォローしていないのにフォロワー限定の投稿を数えている")
	}
}

// ----------------------------------------------------------------------
// ブロックの向き（BR-09 / BR-10）
// ----------------------------------------------------------------------

func TestProfileGetBlockedByTargetIsNotFound(t *testing.T) {
	f := newProfileFixture(actor(), target())
	// bob が alice をブロックしている。
	f.blocks.blocks[[2]int64{2, 1}] = true

	_, err := f.usecase.Get(context.Background(), "bob", viewer(1))

	// **BLOCKED_USER を返さない。** 返すとブロックされた事実が分かる（BR-10）。
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("404 にならない: %v", err)
	}
}

func TestProfileGetBlockingTargetIsVisible(t *testing.T) {
	f := newProfileFixture(actor(), target())
	// alice が bob をブロックしている。
	f.blocks.blocks[[2]int64{1, 2}] = true

	got, err := f.usecase.Get(context.Background(), "bob", viewer(1))
	if err != nil {
		t.Fatalf("自分がブロックした相手が見えない: %v", err)
	}

	if !got.Blocking {
		t.Error("ブロック中が偽になっている")
	}
	// ブロックした相手の投稿は見えない（BR-09）。数も 0 にする。
	if got.Counts.Posts != 0 {
		t.Errorf("ブロックした相手の投稿数が 0 でない: %d", got.Counts.Posts)
	}
	// フォロワー数・フォロー数は投稿ではないため残る。
	if got.Counts.Followers != 1 {
		t.Errorf("フォロワー数まで消えている: %d", got.Counts.Followers)
	}
}

func TestProfileGetHiddenUsers(t *testing.T) {
	suspended := &domain.User{ID: 3, Handle: "carol", Status: domain.UserSuspended}
	deleted := &domain.User{ID: 4, Handle: "dave", Status: domain.UserDeleted}

	tests := []struct {
		name   string
		handle string
	}{
		{"利用停止", "carol"},
		{"退会済み", "dave"},
		{"存在しない識別名", "nobody"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newProfileFixture(actor(), suspended, deleted)

			_, err := f.usecase.Get(context.Background(), tt.handle, viewer(1))

			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("404 にならない: %v", err)
			}
		})
	}
}

// ----------------------------------------------------------------------
// 投稿一覧
// ----------------------------------------------------------------------

func TestProfilePostsPassesAuthorAndVisibility(t *testing.T) {
	f := newProfileFixture(actor(), target())
	f.follows.following[[2]int64{1, 2}] = true
	f.timelines.items = itemsOf(30, 20, 10)

	got, err := f.usecase.Posts(context.Background(), "bob", domain.UserPostQuery{
		TimelineQuery: domain.TimelineQuery{ViewerID: viewer(1), Limit: limitOf(3)},
	})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if f.timelines.gotUserQuery.AuthorID != 2 {
		t.Errorf("投稿者が違う: %d", f.timelines.gotUserQuery.AuthorID)
	}
	if !f.timelines.gotUserQuery.IncludeFollowersOnly {
		t.Error("フォロワーにフォロワー限定の投稿を渡していない")
	}
	if got.NextCursor != 10 {
		t.Errorf("カーソルが違う: %d", got.NextCursor)
	}
}

func TestProfilePostsBlockingReturnsEmpty(t *testing.T) {
	f := newProfileFixture(actor(), target())
	f.blocks.blocks[[2]int64{1, 2}] = true
	f.timelines.items = itemsOf(30, 20, 10)

	got, err := f.usecase.Posts(context.Background(), "bob", domain.UserPostQuery{
		TimelineQuery: domain.TimelineQuery{ViewerID: viewer(1)},
	})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if len(got.Items) != 0 {
		t.Errorf("ブロックした相手の投稿が返っている: %d件", len(got.Items))
	}
	// **問い合わせるまでもない。** 呼んだうえで捨てると、無駄な負荷がかかる。
	if f.timelines.userPostsCalls != 0 {
		t.Errorf("ブロックしているのに問い合わせている: %d回", f.timelines.userPostsCalls)
	}
	if got.NextCursor != 0 {
		t.Errorf("続きがあることになっている: %d", got.NextCursor)
	}
}

func TestProfilePostsBlockedByTargetIsNotFound(t *testing.T) {
	f := newProfileFixture(actor(), target())
	f.blocks.blocks[[2]int64{2, 1}] = true

	_, err := f.usecase.Posts(context.Background(), "bob", domain.UserPostQuery{
		TimelineQuery: domain.TimelineQuery{ViewerID: viewer(1)},
	})

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("404 にならない: %v", err)
	}
}

func TestProfilePostsRejectsBadLimit(t *testing.T) {
	f := newProfileFixture(actor(), target())

	_, err := f.usecase.Posts(context.Background(), "bob", domain.UserPostQuery{
		TimelineQuery: domain.TimelineQuery{Limit: limitOf(0)},
	})

	var validation *domain.Error
	if !errors.As(err, &validation) || validation.Code != domain.CodeValidationFailed {
		t.Fatalf("検証エラーにならない: %v", err)
	}
	if f.timelines.userPostsCalls != 0 {
		t.Error("不正な条件で問い合わせている")
	}
}

func TestProfilePostsUnauthenticatedSeesPublicOnly(t *testing.T) {
	f := newProfileFixture(target())

	if _, err := f.usecase.Posts(context.Background(), "bob", domain.UserPostQuery{}); err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if f.timelines.gotUserQuery.IncludeFollowersOnly {
		t.Error("未ログインにフォロワー限定の投稿を渡している")
	}
}

// ----------------------------------------------------------------------
// エラーの伝播
// ----------------------------------------------------------------------

func TestProfileErrorsPropagate(t *testing.T) {
	broken := errors.New("壊れている")

	tests := []struct {
		name  string
		setup func(*profileFixture)
	}{
		{"利用者を引けない", func(f *profileFixture) { f.users.err = broken }},
		{"ブロックを確認できない", func(f *profileFixture) { f.blocks.err = broken }},
		{"フォローを確認できない", func(f *profileFixture) { f.follows.isFollowingErr = broken }},
		{"数えられない", func(f *profileFixture) { f.profiles.err = broken }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newProfileFixture(actor(), target())
			tt.setup(f)

			if _, err := f.usecase.Get(context.Background(), "bob", viewer(1)); err == nil {
				t.Fatal("エラーが伝わっていない")
			}
		})
	}
}

func TestProfilePostsErrorPropagates(t *testing.T) {
	f := newProfileFixture(actor(), target())
	f.timelines.err = errors.New("読めない")

	if _, err := f.usecase.Posts(context.Background(), "bob", domain.UserPostQuery{
		TimelineQuery: domain.TimelineQuery{ViewerID: viewer(1)},
	}); err == nil {
		t.Fatal("エラーが伝わっていない")
	}
}
