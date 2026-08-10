package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// LikeRepository はいいねの永続化。
type LikeRepository struct {
	pool *pgxpool.Pool
}

// NewLikeRepository をつくる。
func NewLikeRepository(pool *pgxpool.Pool) *LikeRepository {
	return &LikeRepository{pool: pool}
}

// Like はいいねし、posts.like_count を 1 増やす。
//
// すでにいいね済みなら件数を増やさない。`ON CONFLICT DO NOTHING` が
// 0 行を返すことで判別する。**判別しないと、同じ利用者が連打するだけで
// 件数が増える。**
func (r *LikeRepository) Like(ctx context.Context, postID, userID int64) (int, error) {
	return r.update(ctx, `
		INSERT INTO likes (user_id, post_id) VALUES ($2, $1)
		ON CONFLICT DO NOTHING`, `+ 1`, postID, userID)
}

// Unlike はいいねを取り消し、posts.like_count を 1 減らす。
//
// いいねしていなければ件数を減らさない。DELETE の影響行数で判別する。
func (r *LikeRepository) Unlike(ctx context.Context, postID, userID int64) (int, error) {
	return r.update(ctx, `
		DELETE FROM likes WHERE post_id = $1 AND user_id = $2`, `- 1`, postID, userID)
}

// update は likes の更新と like_count の更新を1トランザクションで行う。
//
// delta は `+ 1` か `- 1`。**アプリケーション側で読んで加算しない**
// （基本設計 03 §4）。read-modify-write にすると、同時に2人がいいねしたときに
// 片方の更新が失われる。
func (r *LikeRepository) update(
	ctx context.Context, mutation, delta string, postID, userID int64,
) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("トランザクションを開始できません: %w", err)
	}
	// コミット済みなら Rollback は何もしない。
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, mutation, postID, userID)
	if err != nil {
		return 0, fmt.Errorf("いいねを更新できません: %w", err)
	}

	// 状態が変わらなかった場合は件数も変えない。現在値だけを読んで返す。
	if tag.RowsAffected() == 0 {
		var count int
		if err := tx.QueryRow(ctx,
			`SELECT like_count FROM posts WHERE id = $1`, postID).Scan(&count); err != nil {
			return 0, translateLikeError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("トランザクションを確定できません: %w", err)
		}
		return count, nil
	}

	var count int
	if err := tx.QueryRow(ctx,
		`UPDATE posts SET like_count = like_count `+delta+` WHERE id = $1
		 RETURNING like_count`, postID).Scan(&count); err != nil {
		return 0, translateLikeError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("トランザクションを確定できません: %w", err)
	}
	return count, nil
}

// translateLikeError は pgx のエラーを、上位が扱えるエラーへ変える。
//
// 対象の投稿が消えていた場合に 500 ではなく 404 を返せるようにする。
// usecase が可視性を確かめたあとに削除されると起こりうる。
func translateLikeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("いいね数を更新できません: %w", err)
}

// IsLikedBy はいいね済みかを返す。
func (r *LikeRepository) IsLikedBy(ctx context.Context, postID, userID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM likes WHERE post_id = $1 AND user_id = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, postID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("いいねを確認できません: %w", err)
	}
	return exists, nil
}
