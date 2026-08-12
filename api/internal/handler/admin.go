package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Admin は運営のハンドラ（S-13）。
type Admin struct {
	admins *usecase.Admin
}

// NewAdmin をつくる。
func NewAdmin(admins *usecase.Admin) *Admin {
	return &Admin{admins: admins}
}

// pendingReportResponse は運営が判断するための1件。
//
// **投稿の本文を含める。** 見なければ判断できない。
type pendingReportResponse struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	Comment   string `json:"comment,omitempty"`
	CreatedAt string `json:"created_at"`
	Post      struct {
		ID        string   `json:"id"`
		Body      string   `json:"body"`
		Segments  []string `json:"segments"`
		Verdict   string   `json:"verdict"`
		CreatedAt string   `json:"created_at"`
		Author    struct {
			Handle      string `json:"handle"`
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"post"`
	Reporter struct {
		Handle      string `json:"handle"`
		DisplayName string `json:"display_name"`
	} `json:"reporter"`
}

type pendingReportsResponse struct {
	Items []pendingReportResponse `json:"items"`
	// NextCursor は続きが無ければ null。
	NextCursor *string `json:"next_cursor"`
}

// Reports は GET /api/v1/admin/reports。
func (h *Admin) Reports(c echo.Context) error {
	query := domain.PendingReportQuery{}
	if raw := c.QueryParam("cursor"); raw != "" {
		cursor, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || cursor <= 0 {
			return Respond(c, domain.NewValidationError("cursor", "カーソルの指定が不正です"))
		}
		query.Cursor = cursor
	}
	if raw := c.QueryParam("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return Respond(c, domain.NewValidationError("limit", "取得件数の指定が不正です"))
		}
		query.Limit = &limit
	}

	list, err := h.admins.PendingReports(c.Request().Context(), query)
	if err != nil {
		return Respond(c, err)
	}

	items := make([]pendingReportResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, toPendingReportResponse(item))
	}
	res := pendingReportsResponse{Items: items}
	if list.NextCursor != 0 {
		cursor := formatID(list.NextCursor)
		res.NextCursor = &cursor
	}
	return c.JSON(http.StatusOK, res)
}

// Resolve は POST /api/v1/admin/reports/:id/resolve。
func (h *Admin) Resolve(c echo.Context) error {
	return h.handle(c, h.admins.Resolve)
}

// Reject は POST /api/v1/admin/reports/:id/reject。
func (h *Admin) Reject(c echo.Context) error {
	return h.handle(c, h.admins.Reject)
}

func (h *Admin) handle(
	c echo.Context,
	action func(context.Context, *domain.User, int64) error,
) error {
	admin, ok := CurrentUser(c)
	if !ok {
		// RequireAdmin を通っていれば起こらない。念のため 404 に揃える。
		return Respond(c, domain.ErrNotFound)
	}
	id, err := parseReportID(c.Param("id"))
	if err != nil {
		return Respond(c, err)
	}
	if err := action(c.Request().Context(), admin, id); err != nil {
		return Respond(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func parseReportID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewValidationError("id", "通報の指定が不正です")
	}
	return id, nil
}

func toPendingReportResponse(item domain.PendingReport) pendingReportResponse {
	res := pendingReportResponse{
		ID:        formatID(item.Report.ID),
		Reason:    string(item.Report.Reason),
		Comment:   item.Report.Comment,
		CreatedAt: item.Report.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	res.Post.ID = formatID(item.Post.ID)
	res.Post.Body = item.Post.Body
	res.Post.Verdict = string(item.Post.Verdict)
	res.Post.CreatedAt = item.Post.CreatedAt.UTC().Format(time.RFC3339Nano)
	segments := item.Post.Segments()
	res.Post.Segments = segments[:]
	res.Post.Author.Handle = item.Author.Handle
	res.Post.Author.DisplayName = item.Author.DisplayName
	res.Reporter.Handle = item.Reporter.Handle
	res.Reporter.DisplayName = item.Reporter.DisplayName
	return res
}
