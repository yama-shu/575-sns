// Package usecase は業務ロジックを持つ。
//
// **PostgreSQL に保存することを知らない。** domain が定義した
// リポジトリのインターフェースに対して操作するだけである（詳細設計 02 §2）。
// これにより単体テストで DB が不要になる。
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/password"
)

// Clock は現在時刻を返す。テストで時刻を固定するために差し替える。
type Clock func() time.Time

// Auth は認証の業務ロジック。
type Auth struct {
	users    domain.UserRepository
	sessions domain.SessionRepository
	hasher   *password.Hasher
	now      Clock
}

// NewAuth は認証のユースケースをつくる。
func NewAuth(
	users domain.UserRepository,
	sessions domain.SessionRepository,
	hasher *password.Hasher,
	now Clock,
) *Auth {
	if now == nil {
		now = time.Now
	}
	return &Auth{users: users, sessions: sessions, hasher: hasher, now: now}
}

// SignUpInput は登録の入力。
type SignUpInput struct {
	Handle      string
	Email       string
	Password    string
	DisplayName string
}

// SignUp は利用者を登録し、ログイン済みのセッションを返す。
//
// 登録後にログインさせるのは、直後にログイン画面へ戻す体験を避けるためである。
func (a *Auth) SignUp(ctx context.Context, in SignUpInput) (*domain.User, *domain.Session, error) {
	if err := validateSignUp(in); err != nil {
		return nil, nil, err
	}

	// 先に重複を確認して、利用者に具体的な指摘を返せるようにする。
	// **ただしこれだけに頼らない。** 確認と登録のあいだに他の登録が挟まりうるため、
	// DB の UNIQUE 制約が最後の砦になる（infra 層が制約違反を domain.Error に変換する）。
	if existing, err := a.users.FindByHandle(ctx, in.Handle); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, nil, err
		}
	} else if existing != nil {
		return nil, nil, domain.ErrHandleTaken
	}

	taken, err := a.users.ExistsByEmail(ctx, in.Email)
	if err != nil {
		return nil, nil, err
	}
	if taken {
		return nil, nil, domain.ErrEmailTaken
	}

	hash, err := a.hasher.Hash(in.Password)
	if err != nil {
		return nil, nil, err
	}

	created, err := a.users.Create(ctx, &domain.User{
		Handle:       in.Handle,
		Email:        in.Email,
		PasswordHash: hash,
		DisplayName:  in.DisplayName,
		Status:       domain.UserActive,
	})
	if err != nil {
		return nil, nil, err
	}

	session, err := a.issueSession(ctx, created.ID)
	if err != nil {
		return nil, nil, err
	}
	return created, session, nil
}

// LogIn は識別名とパスワードで認証し、セッションを発行する。
func (a *Auth) LogIn(ctx context.Context, handle, plain string) (*domain.User, *domain.Session, error) {
	user, err := a.users.FindByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// **見つからない場合もハッシュ検証を行う。**
			// 即座に返すと応答時間の差で識別名の存在が分かってしまう。
			a.hasher.VerifyDummy(plain)
			return nil, nil, domain.ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if err := a.hasher.Verify(user.PasswordHash, plain); err != nil {
		if errors.Is(err, password.ErrMismatch) {
			// 識別名が違うのかパスワードが違うのかを区別しない
			return nil, nil, domain.ErrInvalidCredentials
		}
		return nil, nil, err
	}

	// 状態の確認はパスワード検証の**後**に行う。
	// 先に行うと、パスワードを知らなくても「停止中の識別名かどうか」が分かる。
	if !user.Status.CanLogIn() {
		if user.Status == domain.UserSuspended {
			return nil, nil, domain.ErrAccountSuspended
		}
		// 退会済みは「存在しない」と同じ扱いにする。退会の事実を漏らさない。
		return nil, nil, domain.ErrInvalidCredentials
	}

	session, err := a.issueSession(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

// LogOut はセッションを破棄する。
//
// **サーバー側の行を消す。** Cookie を消すだけでは、復元されると
// ログインしたままになる（ADR-0006 が JWT を却下した理由と同じ）。
func (a *Auth) LogOut(ctx context.Context, sessionID string) error {
	return a.sessions.Delete(ctx, sessionID)
}

// Authenticate はセッション ID から利用者を特定する。
//
// 認証のたびに DB を引く。ADR-0006 が受け入れた代償である。
// 有効期限を延長するのもここで行う（スライディング期限）。
func (a *Auth) Authenticate(ctx context.Context, sessionID string) (*domain.User, error) {
	if sessionID == "" {
		return nil, domain.ErrUnauthenticated
	}

	session, user, err := a.sessions.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthenticated
		}
		return nil, err
	}

	now := a.now()
	if session.IsExpired(now) {
		// 期限切れの行は定期ジョブが消すが、消えるまで使わせない。
		// ここで消しておけば、同じセッションで再び来ても DB を引かずに済む。
		if err := a.sessions.Delete(ctx, sessionID); err != nil {
			return nil, err
		}
		return nil, domain.ErrUnauthenticated
	}

	// 利用停止中はセッションが残っていても通さない。
	// 停止時にセッションを消す運用だが、消し漏れがあっても止まるようにする。
	if !user.Status.CanLogIn() {
		return nil, domain.ErrUnauthenticated
	}

	if err := a.sessions.Touch(ctx, sessionID, now, now.Add(domain.SessionLifetime)); err != nil {
		return nil, err
	}
	return user, nil
}

// RevokeAllSessions は利用者のセッションをすべて破棄する。
//
// 利用停止（FR-05-04）で使う。次のリクエストから即座に効く。
func (a *Auth) RevokeAllSessions(ctx context.Context, userID int64) error {
	return a.sessions.DeleteByUserID(ctx, userID)
}

// DeleteExpiredSessions は期限切れのセッションを削除する。
//
// ADR-0006 が「セッションテーブルが際限なく増える」代償として
// 定期ジョブを要求している。その実体。
func (a *Auth) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	return a.sessions.DeleteExpired(ctx, a.now())
}

func (a *Auth) issueSession(ctx context.Context, userID int64) (*domain.Session, error) {
	id, err := domain.NewSessionID()
	if err != nil {
		return nil, err
	}
	now := a.now()
	session := &domain.Session{
		ID:             id,
		UserID:         userID,
		ExpiresAt:      now.Add(domain.SessionLifetime),
		CreatedAt:      now,
		LastAccessedAt: now,
	}
	if err := a.sessions.Create(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func validateSignUp(in SignUpInput) error {
	if err := domain.ValidateHandle(in.Handle); err != nil {
		return err
	}
	if err := domain.ValidateEmail(in.Email); err != nil {
		return err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return err
	}
	return domain.ValidateDisplayName(in.DisplayName)
}
