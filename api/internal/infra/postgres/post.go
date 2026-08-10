package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// PostRepository は投稿の永続化。
type PostRepository struct {
	pool *pgxpool.Pool
}

// NewPostRepository をつくる。
func NewPostRepository(pool *pgxpool.Pool) *PostRepository {
	return &PostRepository{pool: pool}
}

// Create は投稿を保存する。
//
// DB の CHECK 制約が判定結果の妥当性を最後に確かめる。
// アプリケーション側の誤りでデータが汚染されるのを防ぐ（基本設計 03 §2）。
func (r *PostRepository) Create(ctx context.Context, post *domain.Post) (*domain.Post, error) {
	const query = `
		INSERT INTO posts (author_id, body, reading, verdict,
		                   break1, break2, mora_kami, mora_naka, mora_shimo,
		                   visibility, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, author_id, body, reading, verdict, break1, break2,
		          mora_kami, mora_naka, mora_shimo, visibility, status,
		          like_count, created_at, deleted_at`

	var created domain.Post
	err := r.pool.QueryRow(ctx, query,
		post.AuthorID, post.Body, post.Reading, string(post.Verdict),
		post.Break1, post.Break2, post.MoraKami, post.MoraNaka, post.MoraShimo,
		string(post.Visibility), string(post.Status),
	).Scan(
		&created.ID, &created.AuthorID, &created.Body, &created.Reading, &created.Verdict,
		&created.Break1, &created.Break2,
		&created.MoraKami, &created.MoraNaka, &created.MoraShimo,
		&created.Visibility, &created.Status,
		&created.LikeCount, &created.CreatedAt, &created.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("投稿を保存できません: %w", err)
	}
	return &created, nil
}

// FindByID は投稿と投稿者を1回のクエリで返す。
//
// 削除済み・非表示も返す。表示できるかの判断は usecase の責務であり、
// ここで絞ると「削除済みだから 404」と「そもそも無いから 404」を区別できない。
func (r *PostRepository) FindByID(ctx context.Context, id int64) (*domain.Post, *domain.User, error) {
	const query = `
		SELECT p.id, p.author_id, p.body, p.reading, p.verdict, p.break1, p.break2,
		       p.mora_kami, p.mora_naka, p.mora_shimo, p.visibility, p.status,
		       p.like_count, p.created_at, p.deleted_at,
		       u.id, u.handle, u.display_name, COALESCE(u.avatar_url, ''), u.status
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.id = $1`

	var post domain.Post
	var author domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&post.ID, &post.AuthorID, &post.Body, &post.Reading, &post.Verdict,
		&post.Break1, &post.Break2,
		&post.MoraKami, &post.MoraNaka, &post.MoraShimo,
		&post.Visibility, &post.Status,
		&post.LikeCount, &post.CreatedAt, &post.DeletedAt,
		&author.ID, &author.Handle, &author.DisplayName, &author.AvatarURL, &author.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("投稿を取得できません: %w", err)
	}
	return &post, &author, nil
}

// Delete は論理削除する。
//
// status と deleted_at を同時に更新する。片方だけだと
// posts_deleted_at_consistency_check に弾かれる。
//
// **すでに削除済みの行は更新しない。** 更新すると削除日時が上書きされ、
// いつ削除されたかが分からなくなる。
func (r *PostRepository) Delete(ctx context.Context, id int64, now time.Time) error {
	const query = `
		UPDATE posts
		SET status = 'deleted', deleted_at = $2
		WHERE id = $1 AND status <> 'deleted'`

	tag, err := r.pool.Exec(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("投稿を削除できません: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// IsFollowing はフォロー関係の有無を返す。
//
// フォロー機能は M3 だが、follows テーブルは作成済みである。
// 公開範囲の判定にいま必要なため、参照だけ先に実装する。
func (r *PostRepository) IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, followerID, followeeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("フォロー関係を確認できません: %w", err)
	}
	return exists, nil
}

// IsLikedBy はいいね済みかを返す。
//
// いいねは M3 だが、応答に liked_by_me が含まれる（基本設計 05）。
// 常に false を返す実装にすると、M3 で直し忘れたときに気づけない。
func (r *PostRepository) IsLikedBy(ctx context.Context, postID, userID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM likes WHERE post_id = $1 AND user_id = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, postID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("いいねを確認できません: %w", err)
	}
	return exists, nil
}
