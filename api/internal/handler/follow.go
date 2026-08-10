package handler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Follow はフォローのハンドラ。
type Follow struct {
	follows *usecase.Follow
}

// NewFollow をつくる。
func NewFollow(follows *usecase.Follow) *Follow {
	return &Follow{follows: follows}
}

type followResponse struct {
	Following      bool `json:"following"`
	FollowersCount int  `json:"followers_count"`
}

// Follow は PUT /api/v1/users/:handle/follow。
//
// **PUT を使う。** 「フォローしている状態にする」操作であり、
// 何度実行しても結果が同じ（冪等）である（基本設計 05 §2）。
func (h *Follow) Follow(c echo.Context) error {
	return h.respond(c, h.follows.Follow)
}

// Unfollow は DELETE /api/v1/users/:handle/follow。
func (h *Follow) Unfollow(c echo.Context) error {
	return h.respond(c, h.follows.Unfollow)
}

// respond は認証と識別名の取り出しをまとめ、結果を返す。
func (h *Follow) respond(
	c echo.Context,
	action func(context.Context, *domain.User, string) (*domain.FollowState, error),
) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}
	handle := c.Param("handle")
	if handle == "" {
		return Respond(c, domain.NewValidationError("handle", "識別名を指定してください"))
	}

	state, err := action(c.Request().Context(), user, handle)
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusOK, followResponse{
		Following:      state.Following,
		FollowersCount: state.FollowersCount,
	})
}
