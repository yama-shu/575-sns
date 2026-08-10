package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// TimelineRepository はタイムラインの取得。
type TimelineRepository struct {
	pool *pgxpool.Pool
}

// NewTimelineRepository をつくる。
func NewTimelineRepository(pool *pgxpool.Pool) *TimelineRepository {
	return &TimelineRepository{pool: pool}
}

// selectColumns は投稿・投稿者・閲覧者から見た状態をまとめて取る。
//
// **liked_by_me を EXISTS で取る。** 投稿1件ごとに問い合わせると、
// 20件のタイムラインで 21 回のクエリになる（N+1）。
// likes の主キーは (user_id, post_id) であり、この検索に効く。
const selectColumns = `
	p.id, p.author_id, p.body, p.reading, p.verdict, p.break1, p.break2,
	p.mora_kami, p.mora_naka, p.mora_shimo, p.visibility, p.status,
	p.like_count, p.created_at, p.deleted_at,
	u.id, u.handle, u.display_name, COALESCE(u.avatar_url, ''), u.status,
	CASE WHEN $1::bigint IS NULL THEN false ELSE EXISTS (
		SELECT 1 FROM likes l WHERE l.post_id = p.id AND l.user_id = $1
	) END AS liked_by_me`

// notBlocked はブロック関係にある投稿を除外する条件。
//
// **双方向で見る**（BR-09）。片方向だと、ブロックされた側のタイムラインに
// 相手の投稿が流れ続ける。閲覧者が未ログイン（$1 が NULL）なら常に真になる。
const notBlocked = `
	($1::bigint IS NULL OR NOT EXISTS (
		SELECT 1 FROM blocks b
		WHERE (b.blocker_id = $1 AND b.blocked_id = p.author_id)
		   OR (b.blocker_id = p.author_id AND b.blocked_id = $1)
	))`

// Public は全体タイムラインを返す。
//
// インデックス #6（`(id DESC) WHERE status='published' AND visibility='public'`）を
// そのまま辿れるよう、条件を部分インデックスの述語と一致させる。
func (r *TimelineRepository) Public(
	ctx context.Context, q domain.TimelineQuery,
) ([]domain.TimelineItem, error) {
	query := `
		SELECT ` + selectColumns + `
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.status = 'published'
		  AND p.visibility = 'public'
		  AND ($2::bigint = 0 OR p.id < $2)
		  AND ` + notBlocked + `
		ORDER BY p.id DESC
		LIMIT $3`

	rows, err := r.pool.Query(ctx, query, q.ViewerID, q.Cursor, q.EffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("全体タイムラインを取得できません: %w", err)
	}
	return scanTimeline(rows)
}

// Home はフォロー中タイムラインを返す。
//
// **visibility で絞らない。** フォローしている相手の followers 限定の投稿は
// 見えるべきであり、インデックス #7 が visibility を条件に含めていないのも
// このためである。
func (r *TimelineRepository) Home(
	ctx context.Context, q domain.TimelineQuery,
) ([]domain.TimelineItem, error) {
	query := `
		SELECT ` + selectColumns + `
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN follows f ON f.followee_id = p.author_id AND f.follower_id = $1
		WHERE p.status = 'published'
		  AND ($2::bigint = 0 OR p.id < $2)
		  AND ` + notBlocked + `
		ORDER BY p.id DESC
		LIMIT $3`

	rows, err := r.pool.Query(ctx, query, q.ViewerID, q.Cursor, q.EffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("フォロー中タイムラインを取得できません: %w", err)
	}
	return scanTimeline(rows)
}

// scanTimeline は行を読み取る。
func scanTimeline(rows pgx.Rows) ([]domain.TimelineItem, error) {
	defer rows.Close()

	items := []domain.TimelineItem{}
	for rows.Next() {
		var post domain.Post
		var author domain.User
		var likedByMe bool
		if err := rows.Scan(
			&post.ID, &post.AuthorID, &post.Body, &post.Reading, &post.Verdict,
			&post.Break1, &post.Break2,
			&post.MoraKami, &post.MoraNaka, &post.MoraShimo,
			&post.Visibility, &post.Status,
			&post.LikeCount, &post.CreatedAt, &post.DeletedAt,
			&author.ID, &author.Handle, &author.DisplayName, &author.AvatarURL, &author.Status,
			&likedByMe,
		); err != nil {
			return nil, fmt.Errorf("タイムラインを読み取れません: %w", err)
		}
		items = append(items, domain.TimelineItem{
			Post: &post, Author: &author, LikedByMe: likedByMe,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("タイムラインを読み取れません: %w", err)
	}
	return items, nil
}
