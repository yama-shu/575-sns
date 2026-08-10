package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Like はいいねの業務ロジック。
type Like struct {
	likes      domain.LikeRepository
	visibility visibility
}

// NewLike をつくる。
func NewLike(
	likes domain.LikeRepository,
	posts domain.PostRepository,
	blocks domain.BlockRepository,
) *Like {
	return &Like{
		likes:      likes,
		visibility: visibility{posts: posts, blocks: blocks},
	}
}

// Like は投稿にいいねする。
//
// **冪等。** すでにいいね済みでも成功とし、件数は増やさない。
//
// 自分の投稿にもいいねできる。BR にも要件にも禁じるルールが無く、
// 他の利用者に害が無いため、根拠の無い制限を足さない。
func (l *Like) Like(ctx context.Context, postID, userID int64) (*domain.LikeState, error) {
	if _, _, err := l.visibility.resolve(ctx, postID, &userID); err != nil {
		return nil, err
	}
	count, err := l.likes.Like(ctx, postID, userID)
	if err != nil {
		return nil, err
	}
	return &domain.LikeState{Liked: true, LikeCount: count}, nil
}

// Unlike はいいねを取り消す。
//
// **冪等。** いいねしていなくても成功とし、件数は減らさない。
func (l *Like) Unlike(ctx context.Context, postID, userID int64) (*domain.LikeState, error) {
	if _, _, err := l.visibility.resolve(ctx, postID, &userID); err != nil {
		return nil, err
	}
	count, err := l.likes.Unlike(ctx, postID, userID)
	if err != nil {
		return nil, err
	}
	return &domain.LikeState{Liked: false, LikeCount: count}, nil
}
