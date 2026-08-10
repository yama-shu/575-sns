package domain

import "context"

// FollowState はフォロー操作の結果。
//
// 「フォローしているか」と「相手のフォロワー数」を返す
// （基本設計 05 の `{"following": true, "followers_count": 42}`）。
type FollowState struct {
	Following      bool
	FollowersCount int
}

// FollowRepository はフォロー関係の永続化。
type FollowRepository interface {
	// Follow はフォロー関係をつくる。すでにあれば何もしない。
	//
	// **冪等にする。** 事前に存在確認してから INSERT すると、
	// 確認と INSERT のあいだに同じ操作が挟まったときに主キー違反になる。
	Follow(ctx context.Context, followerID, followeeID int64) error
	// Unfollow はフォロー関係を消す。無ければ何もしない。
	Unfollow(ctx context.Context, followerID, followeeID int64) error
	// IsFollowing はフォローしているか。
	IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error)
	// CountFollowers はフォロワー数を返す。
	CountFollowers(ctx context.Context, userID int64) (int, error)
	// IsBlocked は blockerID が blockedID をブロックしているか。
	//
	// フォローの可否に使う。ブロックを作る手段は別 Issue で実装する。
	IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error)
}
