package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// currentUserKey は echo.Context に利用者を載せる際のキー。
//
// 文字列リテラルを直接使わないのは、他のミドルウェアと衝突させないためである。
const currentUserKey = "current_user"

// RequireAuth はセッション Cookie を検証し、未認証を 401 で弾む。
//
// **認証のたびに DB を引く。** ADR-0006 がサーバー側セッションを選んだ
// 代償であり、その代わりログアウトと利用停止が次のリクエストから即座に効く。
func RequireAuth(auth *usecase.Auth) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := authenticate(c, auth)
			if err != nil {
				return Respond(c, err)
			}
			c.Set(currentUserKey, user)
			return next(c)
		}
	}
}

// OptionalAuth はログインしていれば利用者を載せ、していなくても通す。
//
// 未ログインでも閲覧できる画面（全体タイムライン・投稿詳細・ユーザーページ）で使う。
// ログイン済みなら「いいね済みか」などを返せるようにするためである。
func OptionalAuth(auth *usecase.Auth) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if user, err := authenticate(c, auth); err == nil {
				c.Set(currentUserKey, user)
			}
			// 認証に失敗しても通す。エラーは握りつぶさず、
			// 「未ログインとして扱う」という明示的な判断である。
			return next(c)
		}
	}
}

func authenticate(c echo.Context, auth *usecase.Auth) (*domain.User, error) {
	cookie, err := c.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, domain.ErrUnauthenticated
	}
	return auth.Authenticate(c.Request().Context(), cookie.Value)
}

// CurrentUser は認証済みの利用者を取り出す。
func CurrentUser(c echo.Context) (*domain.User, bool) {
	user, ok := c.Get(currentUserKey).(*domain.User)
	return user, ok
}
