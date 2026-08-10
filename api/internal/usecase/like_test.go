package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeLikeRepo はいいねの偽物。
//
// 冪等性を再現する。すでにいいね済みなら件数を増やさず、
// いいねしていなければ減らさない。
type fakeLikeRepo struct {
	liked map[[2]int64]bool
	count int
	err   error

	likeCalls   int
	unlikeCalls int
}

func newLikeRepo() *fakeLikeRepo {
	return &fakeLikeRepo{liked: map[[2]int64]bool{}}
}

func (f *fakeLikeRepo) Like(_ context.Context, postID, userID int64) (int, error) {
	f.likeCalls++
	if f.err != nil {
		return 0, f.err
	}
	key := [2]int64{postID, userID}
	if !f.liked[key] {
		f.liked[key] = true
		f.count++
	}
	return f.count, nil
}

func (f *fakeLikeRepo) Unlike(_ context.Context, postID, userID int64) (int, error) {
	f.unlikeCalls++
	if f.err != nil {
		return 0, f.err
	}
	key := [2]int64{postID, userID}
	if f.liked[key] {
		delete(f.liked, key)
		f.count--
	}
	return f.count, nil
}

func (f *fakeLikeRepo) IsLikedBy(_ context.Context, postID, userID int64) (bool, error) {
	return f.liked[[2]int64{postID, userID}], f.err
}

func newLikeUsecase(likes *fakeLikeRepo, posts *fakePostRepo, blocks *fakeBlockRepo) *usecase.Like {
	return usecase.NewLike(likes, posts, blocks)
}

func TestLike(t *testing.T) {
	likes := newLikeRepo()
	l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, newBlockRepo())

	state, err := l.Like(context.Background(), 1, 99)
	if err != nil {
		t.Fatalf("いいねできない: %v", err)
	}
	if !state.Liked || state.LikeCount != 1 {
		t.Errorf("状態が違う: %+v", state)
	}
}

// すでにいいね済みでも成功し、件数は増えない（冪等）。
func TestLikeIsIdempotent(t *testing.T) {
	likes := newLikeRepo()
	l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, newBlockRepo())

	for i := range 3 {
		state, err := l.Like(context.Background(), 1, 99)
		if err != nil {
			t.Fatalf("%d 回目で失敗した: %v", i+1, err)
		}
		if state.LikeCount != 1 {
			t.Errorf("%d 回目で件数が %d", i+1, state.LikeCount)
		}
	}
}

func TestUnlike(t *testing.T) {
	likes := newLikeRepo()
	l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, newBlockRepo())

	if _, err := l.Like(context.Background(), 1, 99); err != nil {
		t.Fatalf("いいねできない: %v", err)
	}
	state, err := l.Unlike(context.Background(), 1, 99)
	if err != nil {
		t.Fatalf("取り消せない: %v", err)
	}
	if state.Liked || state.LikeCount != 0 {
		t.Errorf("状態が違う: %+v", state)
	}
}

// いいねしていない投稿の取り消しも成功し、件数は減らない（冪等）。
func TestUnlikeIsIdempotent(t *testing.T) {
	likes := newLikeRepo()
	l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, newBlockRepo())

	for i := range 3 {
		state, err := l.Unlike(context.Background(), 1, 99)
		if err != nil {
			t.Fatalf("%d 回目で失敗した: %v", i+1, err)
		}
		if state.LikeCount != 0 {
			t.Errorf("%d 回目で件数が %d", i+1, state.LikeCount)
		}
	}
}

// 自分の投稿にもいいねできる。禁じる根拠が BR にも要件にも無い。
func TestLikeOwnPostIsAllowed(t *testing.T) {
	likes := newLikeRepo()
	// storedPost の AuthorID は 10。
	l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, newBlockRepo())

	if _, err := l.Like(context.Background(), 1, 10); err != nil {
		t.Errorf("自分の投稿にいいねできない: %v", err)
	}
}

// 見えない投稿にはいいねできない。
//
// できると、like_count の増加から存在と活動が推測できてしまう。
func TestLikeInvisiblePost(t *testing.T) {
	viewer := int64(99)

	t.Run("削除済み", func(t *testing.T) {
		post := storedPost()
		post.Status = domain.PostDeleted
		likes := newLikeRepo()
		l := newLikeUsecase(likes, &fakePostRepo{post: post, author: author()}, newBlockRepo())

		if _, err := l.Like(context.Background(), 1, viewer); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("NOT_FOUND を期待したが %v", err)
		}
		if likes.likeCalls != 0 {
			t.Errorf("いいねされた: %d 回", likes.likeCalls)
		}
	})

	t.Run("非表示", func(t *testing.T) {
		post := storedPost()
		post.Status = domain.PostHidden
		likes := newLikeRepo()
		l := newLikeUsecase(likes, &fakePostRepo{post: post, author: author()}, newBlockRepo())

		if _, err := l.Like(context.Background(), 1, viewer); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("NOT_FOUND を期待したが %v", err)
		}
	})

	for name, block := range map[string][2]int64{
		"閲覧者が投稿者をブロック": {99, 10},
		"投稿者が閲覧者をブロック": {10, 99},
	} {
		t.Run(name, func(t *testing.T) {
			blocks := newBlockRepo()
			blocks.blocks[block] = true
			likes := newLikeRepo()
			l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, blocks)

			if _, err := l.Like(context.Background(), 1, viewer); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("NOT_FOUND を期待したが %v", err)
			}
			if likes.likeCalls != 0 {
				t.Errorf("いいねされた: %d 回", likes.likeCalls)
			}
		})
	}

	t.Run("フォロワー限定でフォローしていない", func(t *testing.T) {
		post := storedPost()
		post.Visibility = domain.VisibilityFollowers
		likes := newLikeRepo()
		repo := &fakePostRepo{post: post, author: author(), following: false}
		l := newLikeUsecase(likes, repo, newBlockRepo())

		if _, err := l.Like(context.Background(), 1, viewer); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("NOT_FOUND を期待したが %v", err)
		}
		if likes.likeCalls != 0 {
			t.Errorf("いいねされた: %d 回", likes.likeCalls)
		}
	})

	t.Run("フォロワー限定でフォローしている", func(t *testing.T) {
		post := storedPost()
		post.Visibility = domain.VisibilityFollowers
		repo := &fakePostRepo{post: post, author: author(), following: true}
		l := newLikeUsecase(newLikeRepo(), repo, newBlockRepo())

		if _, err := l.Like(context.Background(), 1, viewer); err != nil {
			t.Errorf("フォロワーがいいねできない: %v", err)
		}
	})
}

// 取り消しも見えない投稿では行わない。
func TestUnlikeInvisiblePost(t *testing.T) {
	post := storedPost()
	post.Status = domain.PostDeleted
	likes := newLikeRepo()
	l := newLikeUsecase(likes, &fakePostRepo{post: post, author: author()}, newBlockRepo())

	if _, err := l.Unlike(context.Background(), 1, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("NOT_FOUND を期待したが %v", err)
	}
	if likes.unlikeCalls != 0 {
		t.Errorf("取り消された: %d 回", likes.unlikeCalls)
	}
}

func TestLikeMissingPost(t *testing.T) {
	likes := newLikeRepo()
	l := newLikeUsecase(likes, &fakePostRepo{findErr: domain.ErrNotFound}, newBlockRepo())

	if _, err := l.Like(context.Background(), 999, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

func TestLikePropagatesErrors(t *testing.T) {
	t.Run("いいねの更新に失敗", func(t *testing.T) {
		likes := newLikeRepo()
		likes.err = errors.New("更新できない")
		l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, newBlockRepo())

		if _, err := l.Like(context.Background(), 1, 99); err == nil {
			t.Error("エラーにならない")
		}
		if _, err := l.Unlike(context.Background(), 1, 99); err == nil {
			t.Error("エラーにならない")
		}
	})

	t.Run("ブロックを確認できない", func(t *testing.T) {
		blocks := newBlockRepo()
		blocks.err = errors.New("確認できない")
		likes := newLikeRepo()
		l := newLikeUsecase(likes, &fakePostRepo{post: storedPost(), author: author()}, blocks)

		if _, err := l.Like(context.Background(), 1, 99); err == nil {
			t.Error("エラーにならない")
		}
		if likes.likeCalls != 0 {
			t.Errorf("いいねされた: %d 回", likes.likeCalls)
		}
	})

	t.Run("フォロー関係を確認できない", func(t *testing.T) {
		post := storedPost()
		post.Visibility = domain.VisibilityFollowers
		repo := &fakePostRepo{post: post, author: author(), followingErr: errors.New("確認できない")}
		likes := newLikeRepo()
		l := newLikeUsecase(likes, repo, newBlockRepo())

		if _, err := l.Like(context.Background(), 1, 99); err == nil {
			t.Error("エラーにならない")
		}
	})
}
