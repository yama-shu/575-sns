package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// updateProfileRequest はプロフィールの更新内容。
//
// **ポインタで受ける。** 省略と空文字を区別するためである。
// `omitempty` を使うと空文字が送られなくなり、自己紹介を消せなくなる。
type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
}

// meResponse は本人の情報。Me と同じ形にする。
type profileUpdateResponse struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// UpdateProfile は PATCH /api/v1/me/profile。
func (h *Profile) UpdateProfile(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}

	var req updateProfileRequest
	if err := c.Bind(&req); err != nil {
		return Respond(c, domain.NewValidationError("body", "リクエストの形式が不正です"))
	}

	updated, err := h.profiles.UpdateProfile(c.Request().Context(), user, usecase.UpdateProfileInput{
		DisplayName: req.DisplayName,
		Bio:         req.Bio,
	})
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusOK, profileUpdateResponse{
		Handle:      updated.Handle,
		DisplayName: updated.DisplayName,
		Bio:         updated.Bio,
		AvatarURL:   updated.AvatarURL,
	})
}
