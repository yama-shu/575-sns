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

// AdminRepository は運営の操作。
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository をつくる。
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// pendingReportsQuery は未対応の通報を古い順に返す。
//
// **古い順である。** タイムラインは新しい順だが、通報は待たせている順に処理する。
// インデックス #14（`(created_at) WHERE status='pending'`）がこの並びのためにある。
//
// 投稿と投稿者と通報者を JOIN で一度に取る。1件ずつ引くと N+1 になり、
// 運営が判断するのに必要な本文が人数ぶんの往復になる。
//
// **実行計画のテストはこの定数を使う。** テスト側に書き写すと、
// 実装を変えたときにテストが古いクエリを検査し続ける（#41 で判明）。
const pendingReportsQuery = `
	SELECT r.id, r.reporter_id, r.post_id, r.reason, COALESCE(r.comment, ''), r.status, r.created_at,
	       p.id, p.author_id, p.body, p.reading, p.verdict, p.break1, p.break2,
	       p.mora_kami, p.mora_naka, p.mora_shimo, p.visibility, p.status,
	       p.like_count, p.created_at, p.deleted_at,
	       a.id, a.handle, a.display_name, a.status,
	       u.id, u.handle, u.display_name, u.status
	FROM reports r
	JOIN posts p ON p.id = r.post_id
	JOIN users a ON a.id = p.author_id
	JOIN users u ON u.id = r.reporter_id
	WHERE r.status = 'pending'
	  AND ($1::bigint = 0 OR r.id > $1)
	ORDER BY r.created_at, r.id
	LIMIT $2`

// PendingReports は未対応の通報を古い順に返す。
func (r *AdminRepository) PendingReports(
	ctx context.Context, q domain.PendingReportQuery,
) ([]domain.PendingReport, error) {
	rows, err := r.pool.Query(ctx, pendingReportsQuery, q.Cursor, q.EffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("通報一覧を取得できません: %w", err)
	}
	defer rows.Close()

	items := []domain.PendingReport{}
	for rows.Next() {
		var report domain.Report
		var post domain.Post
		var author, reporter domain.User
		if err := rows.Scan(
			&report.ID, &report.ReporterID, &report.PostID, &report.Reason,
			&report.Comment, &report.Status, &report.CreatedAt,
			&post.ID, &post.AuthorID, &post.Body, &post.Reading, &post.Verdict,
			&post.Break1, &post.Break2,
			&post.MoraKami, &post.MoraNaka, &post.MoraShimo,
			&post.Visibility, &post.Status,
			&post.LikeCount, &post.CreatedAt, &post.DeletedAt,
			&author.ID, &author.Handle, &author.DisplayName, &author.Status,
			&reporter.ID, &reporter.Handle, &reporter.DisplayName, &reporter.Status,
		); err != nil {
			return nil, fmt.Errorf("通報一覧を読み取れません: %w", err)
		}
		items = append(items, domain.PendingReport{
			Report: &report, Post: &post, Author: &author, Reporter: &reporter,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("通報一覧を読み取れません: %w", err)
	}
	return items, nil
}

// Resolve は投稿を非表示にし、その投稿への未対応の通報をすべて対応済みにする。
func (r *AdminRepository) Resolve(
	ctx context.Context, reportID, adminID int64, now time.Time,
) error {
	return r.handle(ctx, reportID, adminID, now, domain.ReportResolved, true)
}

// Reject は通報を却下する。投稿は変えない。
func (r *AdminRepository) Reject(
	ctx context.Context, reportID, adminID int64, now time.Time,
) error {
	return r.handle(ctx, reportID, adminID, now, domain.ReportRejected, false)
}

// handle は通報を処理する。
//
// **1トランザクションで行う。** 分けると「投稿は消えたが通報が未対応のまま」
// 「通報は閉じたが投稿が見えたまま」が生じる（#36 のブロックと同じ理由）。
func (r *AdminRepository) handle(
	ctx context.Context,
	reportID, adminID int64,
	now time.Time,
	status domain.ReportStatus,
	hidePost bool,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("通報を処理できません: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// **対象の投稿を、未対応であることと併せて取る。**
	// 先に投稿だけを引くと、その間に別の運営が処理する余地が残る。
	var postID int64
	err = tx.QueryRow(ctx,
		`SELECT post_id FROM reports WHERE id = $1 AND status = 'pending' FOR UPDATE`,
		reportID).Scan(&postID)
	if errors.Is(err, pgx.ErrNoRows) {
		// 存在しないか、すでに処理済み。**どちらも同じ扱いにしない。**
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM reports WHERE id = $1)`, reportID).Scan(&exists); err != nil {
			return fmt.Errorf("通報を確認できません: %w", err)
		}
		if exists {
			return domain.ErrAlreadyHandled
		}
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("通報を取得できません: %w", err)
	}

	if hidePost {
		// 削除済みの投稿は触らない。削除日時が上書きされ、いつ消えたか分からなくなる。
		if _, err := tx.Exec(ctx,
			`UPDATE posts SET status = 'hidden' WHERE id = $1 AND status = 'published'`,
			postID); err != nil {
			return fmt.Errorf("投稿を非表示にできません: %w", err)
		}
	}

	// 同じ投稿への未対応の通報をまとめて閉じる（基本設計 02 §4）。
	if _, err := tx.Exec(ctx, `
		UPDATE reports
		SET status = $2, resolved_at = $3, resolved_by = $4
		WHERE post_id = $1 AND status = 'pending'`,
		postID, string(status), now, adminID); err != nil {
		return fmt.Errorf("通報を更新できません: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("通報の処理を確定できません: %w", err)
	}
	return nil
}
