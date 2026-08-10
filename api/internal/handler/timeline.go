package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Timeline はタイムラインのハンドラ。
type Timeline struct {
	timelines *usecase.Timeline
}

// NewTimeline をつくる。
func NewTimeline(timelines *usecase.Timeline) *Timeline {
	return &Timeline{timelines: timelines}
}

type timelineResponse struct {
	Items []postResponse `json:"items"`
	// NextCursor は続きが無ければ null。
	NextCursor *string `json:"next_cursor"`
}

// Public は GET /api/v1/timelines/public。未ログインでも取得できる。
func (h *Timeline) Public(c echo.Context) error {
	return h.respond(c, h.timelines.Public)
}

// Home は GET /api/v1/timelines/home。ログインが必要。
func (h *Timeline) Home(c echo.Context) error {
	return h.respond(c, h.timelines.Home)
}

func (h *Timeline) respond(
	c echo.Context,
	fetch func(context.Context, domain.TimelineQuery) (*domain.Timeline, error),
) error {
	query, err := parseTimelineQuery(c)
	if err != nil {
		return Respond(c, err)
	}

	timeline, err := fetch(c.Request().Context(), query)
	if err != nil {
		return Respond(c, err)
	}

	items := make([]postResponse, 0, len(timeline.Items))
	for _, item := range timeline.Items {
		items = append(items, toPostResponse(&usecase.PostView{
			Post: item.Post, Author: item.Author, LikedByMe: item.LikedByMe,
		}))
	}

	res := timelineResponse{Items: items}
	if timeline.NextCursor != 0 {
		cursor := formatID(timeline.NextCursor)
		res.NextCursor = &cursor
	}
	return c.JSON(http.StatusOK, res)
}

// parseTimelineQuery はクエリパラメータを取り出す。
func parseTimelineQuery(c echo.Context) (domain.TimelineQuery, error) {
	query := domain.TimelineQuery{}
	if user, ok := CurrentUser(c); ok {
		query.ViewerID = &user.ID
	}

	if raw := c.QueryParam("cursor"); raw != "" {
		cursor, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || cursor <= 0 {
			return query, domain.NewValidationError("cursor", "カーソルの指定が不正です")
		}
		query.Cursor = cursor
	}
	if raw := c.QueryParam("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return query, domain.NewValidationError("limit", "取得件数の指定が不正です")
		}
		query.Limit = &limit
	}
	return query, nil
}
