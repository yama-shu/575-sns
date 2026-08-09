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

// SessionRepository はセッションの永続化。
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository をつくる。
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

// Create はセッションを保存する。
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	const query = `
		INSERT INTO sessions (id, user_id, expires_at, created_at, last_accessed_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query,
		session.ID, session.UserID, session.ExpiresAt, session.CreatedAt, session.LastAccessedAt)
	if err != nil {
		return fmt.Errorf("セッションを保存できません: %w", err)
	}
	return nil
}

// FindByID はセッションと持ち主を1回のクエリで返す。
//
// 認証のたびに呼ばれるため、往復を2回にしない。
// どちらも主キー / 外部キーの検索であり、実行計画は Index Scan になる。
func (r *SessionRepository) FindByID(
	ctx context.Context, id string,
) (*domain.Session, *domain.User, error) {
	const query = `
		SELECT s.id, s.user_id, s.expires_at, s.created_at, s.last_accessed_at,
		       u.id, u.handle, u.email, u.password_hash, u.display_name,
		       COALESCE(u.bio, ''), COALESCE(u.avatar_url, ''), u.status, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1`

	var session domain.Session
	var user domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt, &session.LastAccessedAt,
		&user.ID, &user.Handle, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.Bio, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, fmt.Errorf("セッションを取得できません: %w", err)
	}
	return &session, &user, nil
}

// Touch は最終アクセス日時と有効期限を更新する（スライディング期限）。
func (r *SessionRepository) Touch(
	ctx context.Context, id string, now time.Time, expiresAt time.Time,
) error {
	const query = `UPDATE sessions SET last_accessed_at = $2, expires_at = $3 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id, now, expiresAt); err != nil {
		return fmt.Errorf("セッションを更新できません: %w", err)
	}
	return nil
}

// Delete はセッションを消す。
//
// 存在しない ID を渡してもエラーにしない。ログアウトは何度実行しても
// 同じ結果になるべきで、二重送信で 500 を返す理由がない。
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("セッションを削除できません: %w", err)
	}
	return nil
}

// DeleteByUserID はその利用者のセッションをすべて消す（利用停止・退会）。
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("セッションを一括削除できません: %w", err)
	}
	return nil
}

// DeleteExpired は期限切れのセッションを消し、削除件数を返す。
func (r *SessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("期限切れセッションを削除できません: %w", err)
	}
	return tag.RowsAffected(), nil
}
