package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FollowRepository はフォロー関係の永続化。
type FollowRepository struct {
	pool *pgxpool.Pool
}

// NewFollowRepository をつくる。
func NewFollowRepository(pool *pgxpool.Pool) *FollowRepository {
	return &FollowRepository{pool: pool}
}

// Follow はフォロー関係をつくる。すでにあれば何もしない。
//
// **`ON CONFLICT DO NOTHING` で冪等にする。** 事前に存在確認してから
// INSERT すると、確認と INSERT のあいだに同じ操作が挟まったときに
// 主キー違反になる。リトライの二重送信で 500 を返す理由がない。
func (r *FollowRepository) Follow(ctx context.Context, followerID, followeeID int64) error {
	const query = `
		INSERT INTO follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`

	if _, err := r.pool.Exec(ctx, query, followerID, followeeID); err != nil {
		return fmt.Errorf("フォローできません: %w", err)
	}
	return nil
}

// Unfollow はフォロー関係を消す。無ければ何もしない。
func (r *FollowRepository) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	const query = `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`

	if _, err := r.pool.Exec(ctx, query, followerID, followeeID); err != nil {
		return fmt.Errorf("フォローを解除できません: %w", err)
	}
	return nil
}

// IsFollowing はフォローしているか。
func (r *FollowRepository) IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, followerID, followeeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("フォロー関係を確認できません: %w", err)
	}
	return exists, nil
}

// CountFollowers はフォロワー数を返す。
//
// 非正規化していない。follows_followers_list_idx があるため数えても速く、
// 列を持つと更新漏れで実数とずれたときに気づけない。
func (r *FollowRepository) CountFollowers(ctx context.Context, userID int64) (int, error) {
	const query = `SELECT count(*) FROM follows WHERE followee_id = $1`

	var count int
	if err := r.pool.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("フォロワー数を取得できません: %w", err)
	}
	return count, nil
}

// IsBlocked は blockerID が blockedID をブロックしているか。
func (r *FollowRepository) IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, blockerID, blockedID).Scan(&exists); err != nil {
		return false, fmt.Errorf("ブロック関係を確認できません: %w", err)
	}
	return exists, nil
}
