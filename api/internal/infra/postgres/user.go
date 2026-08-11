// Package postgres は domain が定義したリポジトリを PostgreSQL で実装する。
//
// **依存の向きはここから domain へ向かう。** usecase はこのパッケージを知らない。
// 起動時に main が注入する（詳細設計 02 §2）。
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// UserRepository は利用者の永続化。
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository をつくる。
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// 一意制約の名前。DB 側の制約違反を、利用者に返せるエラーへ変換するために使う。
const (
	constraintUsersHandle = "users_handle_key"
	constraintUsersEmail  = "users_email_key"
)

// Create は利用者を登録する。
//
// 事前の重複確認をすり抜けた場合でも、**DB の UNIQUE 制約が最後の砦**になる。
// 確認と登録のあいだに他の登録が挟まると、確認だけでは防げない。
func (r *UserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	const query = `
		INSERT INTO users (handle, email, password_hash, display_name, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, handle, email, password_hash, display_name,
		          COALESCE(bio, ''), COALESCE(avatar_url, ''), status, created_at, updated_at`

	var created domain.User
	err := r.pool.QueryRow(ctx, query,
		user.Handle, user.Email, user.PasswordHash, user.DisplayName, string(user.Status),
	).Scan(
		&created.ID, &created.Handle, &created.Email, &created.PasswordHash,
		&created.DisplayName, &created.Bio, &created.AvatarURL,
		&created.Status, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, translateUniqueViolation(err)
	}
	return &created, nil
}

// UpdateProfile は表示名と自己紹介を更新する。
//
// **更新後の行を返す。** 送った値をそのまま画面に残すと、
// サーバーが正規化した結果と食い違う。
//
// 自己紹介は空文字を許す。列は NULL 可だが、空文字と NULL を区別しない
// （読み出しは COALESCE で空文字に寄せている）。
func (r *UserRepository) UpdateProfile(
	ctx context.Context, userID int64, displayName, bio string,
) (*domain.User, error) {
	const query = `
		UPDATE users
		SET display_name = $2, bio = $3, updated_at = now()
		WHERE id = $1 AND status = 'active'
		RETURNING id, handle, email, password_hash, display_name,
		          COALESCE(bio, ''), COALESCE(avatar_url, ''), status, created_at, updated_at`

	var updated domain.User
	err := r.pool.QueryRow(ctx, query, userID, displayName, bio).Scan(
		&updated.ID, &updated.Handle, &updated.Email, &updated.PasswordHash,
		&updated.DisplayName, &updated.Bio, &updated.AvatarURL,
		&updated.Status, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// 利用停止・退会済みは更新させない。存在しない ID と区別しない。
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("プロフィールを更新できません: %w", err)
	}
	return &updated, nil
}

// FindByID は ID で利用者を引く。
func (r *UserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	const query = `
		SELECT id, handle, email, password_hash, display_name,
		       COALESCE(bio, ''), COALESCE(avatar_url, ''), status, created_at, updated_at
		FROM users WHERE id = $1`
	return r.queryOne(ctx, query, id)
}

// FindByHandle は識別名で利用者を引く。
func (r *UserRepository) FindByHandle(ctx context.Context, handle string) (*domain.User, error) {
	const query = `
		SELECT id, handle, email, password_hash, display_name,
		       COALESCE(bio, ''), COALESCE(avatar_url, ''), status, created_at, updated_at
		FROM users WHERE handle = $1`
	return r.queryOne(ctx, query, handle)
}

// ExistsByEmail はメールアドレスが登録済みか。
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, email).Scan(&exists); err != nil {
		return false, fmt.Errorf("メールアドレスの重複を確認できません: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) queryOne(ctx context.Context, query string, arg any) (*domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&user.ID, &user.Handle, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.Bio, &user.AvatarURL,
		&user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// pgx の型を上位へ漏らさない。usecase は domain のエラーだけを見る。
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("利用者を取得できません: %w", err)
	}
	return &user, nil
}

// translateUniqueViolation は一意制約違反を、利用者に返せるエラーへ変換する。
func translateUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != codeUniqueViolation {
		return fmt.Errorf("利用者を登録できません: %w", err)
	}
	switch pgErr.ConstraintName {
	case constraintUsersHandle:
		return domain.ErrHandleTaken
	case constraintUsersEmail:
		return domain.ErrEmailTaken
	default:
		return fmt.Errorf("利用者を登録できません: %w", err)
	}
}
