package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// SessionCookieName はセッション Cookie の名前。
//
// 接頭辞 `__Host-` は付けない。付けると Secure 必須かつ Domain 指定不可になり、
// HTTP で動かすローカル開発が成立しなくなる。
const SessionCookieName = "session"

// Auth は認証のハンドラ。
type Auth struct {
	auth *usecase.Auth
	// secureCookie は Cookie に Secure を付けるか。
	// 本番では必ず true。ローカル開発（HTTP）では false にしないと
	// ブラウザが Cookie を保存しない。
	secureCookie bool
}

// NewAuth をつくる。
func NewAuth(auth *usecase.Auth, secureCookie bool) *Auth {
	return &Auth{auth: auth, secureCookie: secureCookie}
}

type signUpRequest struct {
	Handle      string `json:"handle"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type logInRequest struct {
	Handle   string `json:"handle"`
	Password string `json:"password"`
}

// userResponse は利用者の公開情報。
//
// **メールアドレスとパスワードハッシュを含めない。** 本人向けの応答でも、
// 返す必要がないものは返さない。
type userResponse struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	// IsAdmin は運営かどうか。**本人向けの応答にしか使わない**ため、
	// 他人にこの値が漏れることはない（#76）。
	//
	// 画面がナビゲーションの出し分けに使う。出し分けずに全員へ出すと、
	// 運営向けの経路があることを教えることになる。
	IsAdmin bool `json:"is_admin"`
}

func toUserResponse(user *domain.User) userResponse {
	return userResponse{
		Handle:      user.Handle,
		DisplayName: user.DisplayName,
		Bio:         user.Bio,
		AvatarURL:   user.AvatarURL,
		IsAdmin:     user.IsAdmin,
	}
}

// SignUp は POST /api/v1/auth/signup。
func (h *Auth) SignUp(c echo.Context) error {
	var req signUpRequest
	if err := c.Bind(&req); err != nil {
		// 形式が壊れている（400）。業務ルール上の問題（422）とは区別する。
		return Respond(c, domain.NewValidationError("body", "リクエストの形式が正しくありません"))
	}

	user, session, err := h.auth.SignUp(c.Request().Context(), usecase.SignUpInput{
		Handle:      req.Handle,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		return Respond(c, err)
	}

	h.setSessionCookie(c, session)
	return c.JSON(http.StatusCreated, toUserResponse(user))
}

// LogIn は POST /api/v1/auth/login。
func (h *Auth) LogIn(c echo.Context) error {
	var req logInRequest
	if err := c.Bind(&req); err != nil {
		return Respond(c, domain.NewValidationError("body", "リクエストの形式が正しくありません"))
	}

	user, session, err := h.auth.LogIn(c.Request().Context(), req.Handle, req.Password)
	if err != nil {
		return Respond(c, err)
	}

	h.setSessionCookie(c, session)
	return c.JSON(http.StatusOK, toUserResponse(user))
}

// LogOut は POST /api/v1/auth/logout。
//
// **サーバー側のセッション行を消してから** Cookie を失効させる。
// Cookie だけ消しても、復元されればログインしたままになる。
func (h *Auth) LogOut(c echo.Context) error {
	cookie, err := c.Cookie(SessionCookieName)
	if err == nil && cookie.Value != "" {
		if err := h.auth.LogOut(c.Request().Context(), cookie.Value); err != nil {
			return Respond(c, err)
		}
	}
	h.clearSessionCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// Me は GET /api/v1/me。認証済みであることが前提。
func (h *Auth) Me(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}
	return c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Auth) setSessionCookie(c echo.Context, session *domain.Session) {
	c.SetCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
		Path:  "/",
		// JavaScript から読めなくする。XSS でトークンを盗まれても持ち出せない。
		HttpOnly: true,
		// HTTPS 上でのみ送信する。平文通信での漏洩を防ぐ。
		Secure: h.secureCookie,
		// クロスサイトからのリクエストで送らない。CSRF の主要な経路を塞ぐ。
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
	})
}

func (h *Auth) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
