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
	rows, err := r.pool.Query(ctx, publicTimelineQuery, q.ViewerID, q.Cursor, q.EffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("全体タイムラインを取得できません: %w", err)
	}
	return scanTimeline(rows)
}

// notBlockedFollowee はブロック関係にあるフォロー先を除外する条件。
//
// **投稿ではなくフォロー先の単位で除外する。** ブロックは利用者どうしの関係であり、
// ブロックした相手の投稿は1件残らず見えない。フォロー先の段階で落とせば、
// LATERAL が「フォロー数 × limit 件」を読む前提が崩れない。
//
// **相手の一覧を1回で集めてから除外する。** フォロー先ごとに NOT EXISTS を
// 評価すると、500 フォローで 49.2 ms かかった。1回にまとめて 18.3 ms になる
// （docs/perf/0002 §10）。
const notBlockedFollowee = `
	f.followee_id <> ALL (
		SELECT CASE WHEN b.blocker_id = $1 THEN b.blocked_id ELSE b.blocker_id END
		FROM blocks b
		WHERE b.blocker_id = $1 OR b.blocked_id = $1
	)`

// Home はフォロー中タイムラインを返す。
//
// **visibility で絞らない。** フォローしている相手の followers 限定の投稿は
// 見えるべきであり、インデックス #7 が visibility を条件に含めていないのも
// このためである。
//
// # LATERAL を使う理由
//
// 素直に JOIN で書くと、プランナは posts を id 降順に辿りながら1行ずつ
// follows を引いて絞る計画を選ぶ。読む行数は
// 「limit ÷ フォロー先の投稿が全体に占める割合」に比例し、
// **フォロー数が少ない利用者ほど大量に読む**（docs/perf/0002 §6-B）。
//
// フォロー先ごとに上位 limit 件を取ってから全体で上位 limit 件を選べば、
// 読む行数は「フォロー数 × limit」で頭打ちになり、投稿総数に依存しない。
// インデックス #7（author_id, id DESC）をフォロー先ごとに辿るため、
// 設計が想定した使われ方にもなる。
//
// 取りこぼしは起きない。ある投稿が全体の上位 limit 件に入るなら、
// その投稿者の中でも上位 limit 件に入るためである。
func (r *TimelineRepository) Home(
	ctx context.Context, q domain.TimelineQuery,
) ([]domain.TimelineItem, error) {
	rows, err := r.pool.Query(ctx, homeTimelineQuery, q.ViewerID, q.Cursor, q.EffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("フォロー中タイムラインを取得できません: %w", err)
	}
	return scanTimeline(rows)
}

// publicTimelineQuery は全体タイムラインのクエリ。
//
// インデックス #6（`(id DESC) WHERE status='published' AND visibility='public'`）を
// そのまま辿れるよう、条件を部分インデックスの述語と一致させる。
//
// **実行計画のテストはこの定数を使う。** テスト側にクエリを書き写すと、
// 実装を変えたときにテストが古いクエリを検査し続ける（#41 で判明）。
const publicTimelineQuery = `
	SELECT ` + selectColumns + `
	FROM posts p
	JOIN users u ON u.id = p.author_id
	WHERE p.status = 'published'
	  AND p.visibility = 'public'
	  AND ($2::bigint = 0 OR p.id < $2)
	  AND ` + notBlocked + `
	ORDER BY p.id DESC
	LIMIT $3`

// homeTimelineQuery はフォロー中タイムラインのクエリ。
const homeTimelineQuery = `
	SELECT ` + selectColumns + `
	FROM (
		SELECT p.*
		FROM follows f
		JOIN LATERAL (
			SELECT *
			FROM posts p
			WHERE p.author_id = f.followee_id
			  AND p.status = 'published'
			  AND ($2::bigint = 0 OR p.id < $2)
			ORDER BY p.id DESC
			LIMIT $3
		) p ON true
		WHERE f.follower_id = $1
		  AND ` + notBlockedFollowee + `
	) p
	JOIN users u ON u.id = p.author_id
	ORDER BY p.id DESC
	LIMIT $3`

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
