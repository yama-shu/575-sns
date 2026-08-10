package handler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Like はいいねのハンドラ。
type Like struct {
	likes *usecase.Like
}

// NewLike をつくる。
func NewLike(likes *usecase.Like) *Like {
	return &Like{likes: likes}
}

type likeResponse struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

// Like は PUT /api/v1/posts/:id/like。
//
// **PUT を使う。** 「いいねしている状態にする」操作であり、
// 何度実行しても結果が同じ（基本設計 05 §2）。
func (h *Like) Like(c echo.Context) error {
	return h.respond(c, h.likes.Like)
}

// Unlike は DELETE /api/v1/posts/:id/like。
func (h *Like) Unlike(c echo.Context) error {
	return h.respond(c, h.likes.Unlike)
}

func (h *Like) respond(
	c echo.Context,
	action func(context.Context, int64, int64) (*domain.LikeState, error),
) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}
	postID, err := parsePostID(c.Param("id"))
	if err != nil {
		return Respond(c, err)
	}

	state, err := action(c.Request().Context(), postID, user.ID)
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusOK, likeResponse{
		Liked: state.Liked, LikeCount: state.LikeCount,
	})
}
