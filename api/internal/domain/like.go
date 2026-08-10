package domain

import "context"

// LikeState はいいね操作の結果。
type LikeState struct {
	Liked bool
	// LikeCount は操作後のいいね数。
	LikeCount int
}

// LikeRepository はいいねの永続化。
type LikeRepository interface {
	// Like はいいねし、posts.like_count を 1 増やす。
	//
	// **2つの更新を同一トランザクションで行う。** 片方だけ成功すると、
	// 基本設計 03 §4 が非正規化の代償として挙げた「ずれ」がそのまま起きる。
	//
	// すでにいいね済みなら件数を増やさない（冪等）。
	// 操作後のいいね数を返す。
	Like(ctx context.Context, postID, userID int64) (int, error)
	// Unlike はいいねを取り消し、posts.like_count を 1 減らす。
	//
	// いいねしていなければ件数を減らさない（冪等）。
	Unlike(ctx context.Context, postID, userID int64) (int, error)
	// IsLikedBy はいいね済みかを返す。
	IsLikedBy(ctx context.Context, postID, userID int64) (bool, error)
}
