package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// ProfileRepository はプロフィールの数え上げ。
type ProfileRepository struct {
	pool *pgxpool.Pool
}

// NewProfileRepository をつくる。
func NewProfileRepository(pool *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{pool: pool}
}

// profileCountsQuery は3つの数を1回で取る。
//
// **別々に問い合わせない。** プロフィール1枚で4回の往復になる。
//
// それぞれ既存のインデックスで数えられる。
//
//	投稿数     : posts_author_timeline_idx (author_id, id DESC) WHERE status='published'
//	フォロー数 : follows_pkey (follower_id, followee_id)
//	フォロワー数: follows_followee_id_idx (followee_id)
//
// **実行計画のテストはこの定数を使う。** テスト側に書き写すと、
// 実装を変えたときにテストが古いクエリを検査し続ける（#41 で判明）。
const profileCountsQuery = `
	SELECT
		(SELECT count(*) FROM posts p
		  WHERE p.author_id = $1
		    AND p.status = 'published'
		    AND ($2 OR p.visibility = 'public')),
		(SELECT count(*) FROM follows f WHERE f.follower_id = $1),
		(SELECT count(*) FROM follows f WHERE f.followee_id = $1)`

// Counts は投稿数・フォロー数・フォロワー数を返す。
func (r *ProfileRepository) Counts(
	ctx context.Context, userID int64, includeFollowersOnly bool,
) (domain.ProfileCounts, error) {
	var counts domain.ProfileCounts
	err := r.pool.QueryRow(ctx, profileCountsQuery, userID, includeFollowersOnly).
		Scan(&counts.Posts, &counts.Following, &counts.Followers)
	if err != nil {
		return domain.ProfileCounts{}, fmt.Errorf("プロフィールの件数を取得できません: %w", err)
	}
	return counts, nil
}
