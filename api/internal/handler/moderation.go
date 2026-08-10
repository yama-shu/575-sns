package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Moderation は通報とブロックのハンドラ。
type Moderation struct {
	moderation *usecase.Moderation
}

// NewModeration をつくる。
func NewModeration(moderation *usecase.Moderation) *Moderation {
	return &Moderation{moderation: moderation}
}

type reportRequest struct {
	Reason  string `json:"reason"`
	Comment string `json:"comment"`
}

type reportResponse struct {
	ID        string              `json:"id"`
	Reason    domain.ReportReason `json:"reason"`
	Status    domain.ReportStatus `json:"status"`
	CreatedAt time.Time           `json:"created_at"`
}

type blockResponse struct {
	Blocked bool `json:"blocked"`
}

// Report は POST /api/v1/posts/:id/report。
//
// **POST を使い、重複は 409 で返す。** ブロックと違い「通報を1件作る」
// 要求であり、作られなかったことを伝える必要がある（基本設計 02 §4）。
func (h *Moderation) Report(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}
	postID, err := parsePostID(c.Param("id"))
	if err != nil {
		return Respond(c, err)
	}

	var req reportRequest
	if err := c.Bind(&req); err != nil {
		return Respond(c, domain.NewValidationError("reason", "リクエストの形式が不正です"))
	}

	report, err := h.moderation.Report(c.Request().Context(), usecase.ReportInput{
		ReporterID: user.ID,
		PostID:     postID,
		Reason:     domain.ReportReason(req.Reason),
		Comment:    req.Comment,
	})
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusCreated, reportResponse{
		ID:        formatID(report.ID),
		Reason:    report.Reason,
		Status:    report.Status,
		CreatedAt: report.CreatedAt,
	})
}

// Block は PUT /api/v1/users/:handle/block。
func (h *Moderation) Block(c echo.Context) error {
	return h.respondBlock(c, h.moderation.Block)
}

// Unblock は DELETE /api/v1/users/:handle/block。
func (h *Moderation) Unblock(c echo.Context) error {
	return h.respondBlock(c, h.moderation.Unblock)
}

func (h *Moderation) respondBlock(
	c echo.Context,
	action func(context.Context, *domain.User, string) (*domain.BlockState, error),
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
	return c.JSON(http.StatusOK, blockResponse{Blocked: state.Blocked})
}
