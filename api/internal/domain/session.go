package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"
)

// SessionLifetime はセッションの有効期間（ADR-0006）。
//
// アクセスのたびに延長する（スライディング期限）。
const SessionLifetime = 30 * 24 * time.Hour

// SessionIDBytes はセッション ID の乱数の長さ。
//
// 32 バイトを base64url すると 43 文字になる。DB の CHAR(43) と一致する。
const SessionIDBytes = 32

// Session はログインセッション。
type Session struct {
	ID             string
	UserID         int64
	ExpiresAt      time.Time
	CreatedAt      time.Time
	LastAccessedAt time.Time
}

// IsExpired は期限切れか。
//
// 期限切れの行は定期ジョブが消すが、消えるまでの間も使わせてはならない。
// **削除を待たず、読み出した時点で判定する。**
func (s *Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

// NewSessionID は暗号論的に安全な乱数からセッション ID をつくる（ADR-0006）。
//
// **連番や推測可能な値を使わない。** 推測できると、他人のセッションを
// 乗っ取られる。base64url を使うのは Cookie にそのまま載せられるためである。
func NewSessionID() (string, error) {
	buf := make([]byte, SessionIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// UserRepository は利用者の永続化。**実装は infra 層に置く。**
type UserRepository interface {
	Create(ctx context.Context, user *User) (*User, error)
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByHandle(ctx context.Context, handle string) (*User, error)
	// ExistsByEmail はメールアドレスが登録済みか。
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// UpdateProfile は表示名と自己紹介を更新し、更新後の利用者を返す。
	//
	// **更新後の値を返す。** 送った値をそのまま画面に残すと、
	// サーバーが正規化した結果と食い違う。
	UpdateProfile(ctx context.Context, userID int64, displayName, bio string) (*User, error)
}

// SessionRepository はセッションの永続化。
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	// FindByID はセッションと、その持ち主を返す。
	//
	// 認証のたびに呼ばれる。セッションと利用者を1回のクエリで取るのは、
	// 往復を2回にしないためである（ADR-0006 が受け入れた代償を最小にする）。
	FindByID(ctx context.Context, id string) (*Session, *User, error)
	// Touch は最終アクセス日時と有効期限を更新する（スライディング期限）。
	Touch(ctx context.Context, id string, now time.Time, expiresAt time.Time) error
	Delete(ctx context.Context, id string) error
	// DeleteByUserID はその利用者のセッションをすべて消す。
	//
	// 利用停止（FR-05-04）で使う。**次のリクエストから即座に効く**ことが
	// サーバー側セッションを選んだ理由そのものである（ADR-0006）。
	DeleteByUserID(ctx context.Context, userID int64) error
	// DeleteExpired は期限切れを消す。定期ジョブから呼ぶ。
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
