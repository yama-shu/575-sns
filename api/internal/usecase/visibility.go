package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// visibility は投稿が閲覧者に見えるかを判定する。
//
// **判定を1箇所に集める。** 投稿の取得・通報・いいねはいずれも
// 「見える投稿だけを対象にする」必要があり、別々に書くと
// 片方だけ直したときに食い違う。見えない投稿を操作できると、
// 応答の違いから存在と活動が推測できてしまう。
type visibility struct {
	posts  domain.PostRepository
	blocks domain.BlockRepository
}

// resolve は投稿と投稿者を返す。見えない場合は ErrNotFound を返す。
//
// viewerID が nil なら未ログイン。
//
// **見えない理由を区別しない。** 削除済み・非表示・ブロック・
// フォロワー限定のいずれも 404 とする。区別すると、
// 「その ID の投稿は存在するが見せない」ことを教えてしまう。
func (v visibility) resolve(
	ctx context.Context, postID int64, viewerID *int64,
) (*domain.Post, *domain.User, error) {
	post, author, err := v.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, nil, err
	}
	if post.Status != domain.PostPublished {
		return nil, nil, domain.ErrNotFound
	}

	// BR-09 は双方向に効く。未ログインは確認しない
	// （誰でもない相手をブロックすることはできない）。
	if viewerID != nil {
		blocked, err := v.blocks.IsBlockedEitherWay(ctx, *viewerID, post.AuthorID)
		if err != nil {
			return nil, nil, err
		}
		if blocked {
			return nil, nil, domain.ErrNotFound
		}
	}

	isFollower := false
	if viewerID != nil && post.Visibility == domain.VisibilityFollowers {
		isFollower, err = v.posts.IsFollowing(ctx, *viewerID, post.AuthorID)
		if err != nil {
			return nil, nil, err
		}
	}
	if !post.IsVisibleTo(viewerID, isFollower) {
		return nil, nil, domain.ErrNotFound
	}
	return post, author, nil
}
