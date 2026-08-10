package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

const (
	// constraintReportsReporterPost は「1人1投稿につき1件」の一意制約。
	constraintReportsReporterPost = "reports_reporter_id_post_id_key"
	// codeUniqueViolation は PostgreSQL の一意制約違反（23505）。
	codeUniqueViolation = "23505"
)

// ReportRepository は通報の永続化。
type ReportRepository struct {
	pool *pgxpool.Pool
}

// NewReportRepository をつくる。
func NewReportRepository(pool *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{pool: pool}
}

// Create は通報を1件作る。
//
// 重複は DB の UNIQUE 制約で防ぐ。事前に存在確認してから INSERT すると、
// 確認と INSERT のあいだに同じ操作が挟まったときに 500 になる。
func (r *ReportRepository) Create(ctx context.Context, report *domain.Report) (*domain.Report, error) {
	const query = `
		INSERT INTO reports (reporter_id, post_id, reason, comment, status)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		RETURNING id, reporter_id, post_id, reason, COALESCE(comment, ''), status, created_at`

	var created domain.Report
	err := r.pool.QueryRow(ctx, query,
		report.ReporterID, report.PostID, string(report.Reason),
		report.Comment, string(report.Status),
	).Scan(
		&created.ID, &created.ReporterID, &created.PostID,
		&created.Reason, &created.Comment, &created.Status, &created.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == codeUniqueViolation &&
			pgErr.ConstraintName == constraintReportsReporterPost {
			return nil, domain.ErrAlreadyReported
		}
		return nil, fmt.Errorf("通報を保存できません: %w", err)
	}
	return &created, nil
}

// BlockRepository はブロックの永続化。
type BlockRepository struct {
	pool *pgxpool.Pool
}

// NewBlockRepository をつくる。
func NewBlockRepository(pool *pgxpool.Pool) *BlockRepository {
	return &BlockRepository{pool: pool}
}

// Block はブロックし、フォロー関係を双方向に解除する（BR-08）。
//
// **1トランザクションで行う。** 分けて実行すると「ブロックはできたが
// フォローが残る」状態が生じ、ブロックしたのに相手のタイムラインへ
// 自分の投稿が流れ続ける。BR-08 が防ごうとしたものがそのまま起きる。
func (r *BlockRepository) Block(ctx context.Context, blockerID, blockedID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("トランザクションを開始できません: %w", err)
	}
	// コミット済みなら Rollback は何もしない。失敗経路で明示的に呼ぶ必要をなくす。
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		blockerID, blockedID); err != nil {
		return fmt.Errorf("ブロックできません: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM follows
		WHERE (follower_id = $1 AND followee_id = $2)
		   OR (follower_id = $2 AND followee_id = $1)`,
		blockerID, blockedID); err != nil {
		return fmt.Errorf("フォロー関係を解除できません: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("トランザクションを確定できません: %w", err)
	}
	return nil
}

// Unblock はブロックを解除する。
//
// **フォロー関係は復活させない。** ブロックの前にフォローしていたかを
// 記録していないうえ、解除が意図せぬ再フォローを起こすべきではない。
func (r *BlockRepository) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	const query = `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`

	if _, err := r.pool.Exec(ctx, query, blockerID, blockedID); err != nil {
		return fmt.Errorf("ブロックを解除できません: %w", err)
	}
	return nil
}

// IsBlocked は blockerID が blockedID をブロックしているか。
func (r *BlockRepository) IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = $2)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, blockerID, blockedID).Scan(&exists); err != nil {
		return false, fmt.Errorf("ブロック関係を確認できません: %w", err)
	}
	return exists, nil
}

// IsBlockedEitherWay は2者のあいだにどちらの向きでもブロックがあるか。
//
// 可視性の判定に使う（BR-09）。1回のクエリで両方向を見る。
// 2回に分けると往復が増え、読み取り経路すべてに効く。
func (r *BlockRepository) IsBlockedEitherWay(ctx context.Context, userA, userB int64) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM blocks
		WHERE (blocker_id = $1 AND blocked_id = $2)
		   OR (blocker_id = $2 AND blocked_id = $1))`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, userA, userB).Scan(&exists); err != nil {
		return false, fmt.Errorf("ブロック関係を確認できません: %w", err)
	}
	return exists, nil
}
