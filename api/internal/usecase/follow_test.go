package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeFollowRepo はフォロー関係の偽物。
//
// 関係を集合として持ち、実際の追加・削除を再現する。
// 「エラーを返しつつ関係を作ってしまう」実装を検出するため、
// 呼び出し回数も記録する。
type fakeFollowRepo struct {
	following map[[2]int64]bool
	blocks    map[[2]int64]bool

	followCalls   int
	unfollowCalls int

	followErr,
	unfollowErr,
	isFollowingErr,
	countErr,
	isBlockedErr error
	// isBlockedErrFor は特定の向きの確認だけを失敗させる。
	// 向きごとに別の呼び出しであることを検証するために使う。
	isBlockedErrFor *[2]int64
}

func newFollowRepo() *fakeFollowRepo {
	return &fakeFollowRepo{
		following: map[[2]int64]bool{},
		blocks:    map[[2]int64]bool{},
	}
}

func (f *fakeFollowRepo) Follow(_ context.Context, followerID, followeeID int64) error {
	f.followCalls++
	if f.followErr != nil {
		return f.followErr
	}
	f.following[[2]int64{followerID, followeeID}] = true
	return nil
}

func (f *fakeFollowRepo) Unfollow(_ context.Context, followerID, followeeID int64) error {
	f.unfollowCalls++
	if f.unfollowErr != nil {
		return f.unfollowErr
	}
	delete(f.following, [2]int64{followerID, followeeID})
	return nil
}

func (f *fakeFollowRepo) IsFollowing(_ context.Context, followerID, followeeID int64) (bool, error) {
	return f.following[[2]int64{followerID, followeeID}], f.isFollowingErr
}

func (f *fakeFollowRepo) CountFollowers(_ context.Context, userID int64) (int, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	count := 0
	for key := range f.following {
		if key[1] == userID {
			count++
		}
	}
	return count, nil
}

func (f *fakeFollowRepo) IsBlocked(_ context.Context, blockerID, blockedID int64) (bool, error) {
	key := [2]int64{blockerID, blockedID}
	if f.isBlockedErrFor != nil && *f.isBlockedErrFor == key {
		return false, errors.New("ブロックを確認できない")
	}
	return f.blocks[key], f.isBlockedErr
}

// fakeUserRepo は auth_test.go の fakeUserRepo を使わず、
// 識別名で引くことだけに絞った偽物を用意する。
type followUserRepo struct {
	byHandle map[string]*domain.User
	err      error
	updated  bool
}

func (r *followUserRepo) Create(context.Context, *domain.User) (*domain.User, error) {
	return nil, errors.New("使わない")
}
func (r *followUserRepo) FindByID(context.Context, int64) (*domain.User, error) {
	return nil, errors.New("使わない")
}
func (r *followUserRepo) FindByHandle(_ context.Context, handle string) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	user, ok := r.byHandle[handle]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}
func (r *followUserRepo) ExistsByEmail(context.Context, string) (bool, error) {
	return false, errors.New("使わない")
}

// UpdateProfile は更新後の利用者を返す。**保存したかどうかも記録する。**
// 検証に失敗したのに保存する実装を検出するために使う。
func (r *followUserRepo) UpdateProfile(
	_ context.Context, _ int64, displayName, bio string,
) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.updated = true
	return &domain.User{ID: 1, Handle: "alice", DisplayName: displayName, Bio: bio}, nil
}

func actor() *domain.User {
	return &domain.User{ID: 1, Handle: "alice", DisplayName: "アリス", Status: domain.UserActive}
}

func target() *domain.User {
	return &domain.User{ID: 2, Handle: "bob", DisplayName: "ボブ", Status: domain.UserActive}
}

func newFollowUsecase(users *followUserRepo, follows *fakeFollowRepo) *usecase.Follow {
	return usecase.NewFollow(users, follows)
}

func usersOf(list ...*domain.User) *followUserRepo {
	byHandle := map[string]*domain.User{}
	for _, u := range list {
		byHandle[u.Handle] = u
	}
	return &followUserRepo{byHandle: byHandle}
}

func TestFollow(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	state, err := f.Follow(context.Background(), actor(), "bob")
	if err != nil {
		t.Fatalf("フォローできない: %v", err)
	}
	if !state.Following || state.FollowersCount != 1 {
		t.Errorf("状態が違う: %+v", state)
	}
}

// すでにフォロー済みでも成功する（冪等）。409 にしない。
func TestFollowIsIdempotent(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	for i := range 3 {
		state, err := f.Follow(context.Background(), actor(), "bob")
		if err != nil {
			t.Fatalf("%d 回目で失敗した: %v", i+1, err)
		}
		if !state.Following || state.FollowersCount != 1 {
			t.Errorf("%d 回目の状態が違う: %+v", i+1, state)
		}
	}
}

func TestUnfollow(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	if _, err := f.Follow(context.Background(), actor(), "bob"); err != nil {
		t.Fatalf("フォローできない: %v", err)
	}
	state, err := f.Unfollow(context.Background(), actor(), "bob")
	if err != nil {
		t.Fatalf("解除できない: %v", err)
	}
	if state.Following || state.FollowersCount != 0 {
		t.Errorf("状態が違う: %+v", state)
	}
}

// フォローしていない相手の解除も成功する（冪等）。
func TestUnfollowIsIdempotent(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	state, err := f.Unfollow(context.Background(), actor(), "bob")
	if err != nil {
		t.Fatalf("解除できない: %v", err)
	}
	if state.Following || state.FollowersCount != 0 {
		t.Errorf("状態が違う: %+v", state)
	}
}

// BR-05: 自分自身をフォローできない。
func TestFollowSelfIsRejected(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor()), follows)

	_, err := f.Follow(context.Background(), actor(), "alice")
	if !errors.Is(err, domain.ErrCannotFollowSelf) {
		t.Fatalf("CANNOT_FOLLOW_SELF を期待したが %v", err)
	}
	if follows.followCalls != 0 {
		t.Errorf("関係が作られた: %d 回", follows.followCalls)
	}
}

func TestUnfollowSelfIsRejected(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor()), follows)

	if _, err := f.Unfollow(context.Background(), actor(), "alice"); !errors.Is(err, domain.ErrCannotFollowSelf) {
		t.Errorf("CANNOT_FOLLOW_SELF を期待したが %v", err)
	}
}

// 自分がブロックしている相手はフォローできない。
// 自分のブロックは自分が知っているため、理由を返してよい。
func TestFollowBlockedUserIsRejected(t *testing.T) {
	follows := newFollowRepo()
	follows.blocks[[2]int64{1, 2}] = true
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	_, err := f.Follow(context.Background(), actor(), "bob")
	if !errors.Is(err, domain.ErrBlockedUser) {
		t.Fatalf("BLOCKED_USER を期待したが %v", err)
	}
	if follows.followCalls != 0 {
		t.Errorf("関係が作られた: %d 回", follows.followCalls)
	}
}

// **相手が自分をブロックしている場合は 404。**
// BLOCKED_USER を返すとブロックされた事実が漏れる（BR-10）。
func TestFollowWhenBlockedByTargetLooksLikeNotFound(t *testing.T) {
	follows := newFollowRepo()
	follows.blocks[[2]int64{2, 1}] = true
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	_, err := f.Follow(context.Background(), actor(), "bob")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("NOT_FOUND を期待したが %v", err)
	}
	// 応答が存在しない識別名と同じであること。
	_, missingErr := f.Follow(context.Background(), actor(), "nobody")
	var blockedApp, missingApp *domain.Error
	if !errors.As(err, &blockedApp) || !errors.As(missingErr, &missingApp) {
		t.Fatalf("domain.Error を期待した: %v / %v", err, missingErr)
	}
	if blockedApp.Code != missingApp.Code || blockedApp.Message != missingApp.Message {
		t.Errorf("ブロックの事実が漏れている: %+v vs %+v", blockedApp, missingApp)
	}
	if follows.followCalls != 0 {
		t.Errorf("関係が作られた: %d 回", follows.followCalls)
	}
}

// ブロックされていても解除はできる。
// 「フォローしていない状態にする」要求であり、拒む理由がない…
// ただし相手が見えないため 404 になる。
func TestUnfollowWhenBlockedByTargetIsNotFound(t *testing.T) {
	follows := newFollowRepo()
	follows.blocks[[2]int64{2, 1}] = true
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	if _, err := f.Unfollow(context.Background(), actor(), "bob"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

// 自分がブロックしている相手の解除は通る。
func TestUnfollowBlockedUserIsAllowed(t *testing.T) {
	follows := newFollowRepo()
	follows.following[[2]int64{1, 2}] = true
	follows.blocks[[2]int64{1, 2}] = true
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	state, err := f.Unfollow(context.Background(), actor(), "bob")
	if err != nil {
		t.Fatalf("解除できない: %v", err)
	}
	if state.Following {
		t.Error("解除されていない")
	}
}

func TestFollowMissingUser(t *testing.T) {
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor()), follows)

	if _, err := f.Follow(context.Background(), actor(), "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

// 退会済みはフォローできない。外部キーは行の存在しか見ないため、
// アプリケーション側で確かめないと退会した利用者をフォローできてしまう。
func TestFollowDeletedUser(t *testing.T) {
	deleted := target()
	deleted.Status = domain.UserDeleted
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor(), deleted), follows)

	_, err := f.Follow(context.Background(), actor(), "bob")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("NOT_FOUND を期待したが %v", err)
	}
	if follows.followCalls != 0 {
		t.Errorf("関係が作られた: %d 回", follows.followCalls)
	}
}

// 利用停止は 404 にしない。一時的な状態であり、
// 解除後にフォロー関係が残っているのが自然である。
func TestFollowSuspendedUserIsAllowed(t *testing.T) {
	suspended := target()
	suspended.Status = domain.UserSuspended
	follows := newFollowRepo()
	f := newFollowUsecase(usersOf(actor(), suspended), follows)

	if _, err := f.Follow(context.Background(), actor(), "bob"); err != nil {
		t.Errorf("利用停止中の利用者をフォローできない: %v", err)
	}
}

// フォロワー数が実際の関係の数と一致すること。
func TestFollowersCount(t *testing.T) {
	follows := newFollowRepo()
	carol := &domain.User{ID: 3, Handle: "carol", Status: domain.UserActive}
	f := newFollowUsecase(usersOf(actor(), target(), carol), follows)

	if _, err := f.Follow(context.Background(), actor(), "bob"); err != nil {
		t.Fatalf("フォローできない: %v", err)
	}
	state, err := f.Follow(context.Background(), carol, "bob")
	if err != nil {
		t.Fatalf("フォローできない: %v", err)
	}
	if state.FollowersCount != 2 {
		t.Errorf("フォロワー数が違う: %d", state.FollowersCount)
	}
}

// リポジトリの失敗を伝えること。
func TestFollowPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("失敗")

	tests := map[string]func(*fakeFollowRepo){
		"利用者を引けない":      func(*fakeFollowRepo) {},
		"ブロックを確認できない":   func(r *fakeFollowRepo) { r.isBlockedErr = boom },
		"関係を作れない":       func(r *fakeFollowRepo) { r.followErr = boom },
		"関係を確認できない":     func(r *fakeFollowRepo) { r.isFollowingErr = boom },
		"フォロワー数を数えられない": func(r *fakeFollowRepo) { r.countErr = boom },
	}

	for name, corrupt := range tests {
		t.Run(name, func(t *testing.T) {
			follows := newFollowRepo()
			corrupt(follows)
			users := usersOf(actor(), target())
			if name == "利用者を引けない" {
				users.err = boom
			}
			f := newFollowUsecase(users, follows)

			if _, err := f.Follow(context.Background(), actor(), "bob"); err == nil {
				t.Error("エラーにならない")
			}
		})
	}
}

func TestUnfollowPropagatesRepositoryErrors(t *testing.T) {
	follows := newFollowRepo()
	follows.unfollowErr = errors.New("失敗")
	f := newFollowUsecase(usersOf(actor(), target()), follows)

	if _, err := f.Unfollow(context.Background(), actor(), "bob"); err == nil {
		t.Error("エラーにならない")
	}
}

// ブロックの確認は向きごとに行われること。
//
// 相手→自分（BR-10 の判定）と自分→相手（BLOCKED_USER の判定）は
// 別の問い合わせであり、片方の失敗がもう片方に紛れてはならない。
func TestFollowChecksBothBlockDirections(t *testing.T) {
	t.Run("自分から相手への確認が失敗する", func(t *testing.T) {
		follows := newFollowRepo()
		follows.isBlockedErrFor = &[2]int64{1, 2}
		f := newFollowUsecase(usersOf(actor(), target()), follows)

		if _, err := f.Follow(context.Background(), actor(), "bob"); err == nil {
			t.Error("エラーにならない")
		}
		if follows.followCalls != 0 {
			t.Errorf("関係が作られた: %d 回", follows.followCalls)
		}
	})

	t.Run("相手から自分への確認が失敗する", func(t *testing.T) {
		follows := newFollowRepo()
		follows.isBlockedErrFor = &[2]int64{2, 1}
		f := newFollowUsecase(usersOf(actor(), target()), follows)

		if _, err := f.Follow(context.Background(), actor(), "bob"); err == nil {
			t.Error("エラーにならない")
		}
	})
}
