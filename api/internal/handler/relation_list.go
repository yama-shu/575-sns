package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// RelationList は関係の一覧のハンドラ（S-05 / S-06 / S-11）。
type RelationList struct {
	lists *usecase.RelationList
}

// NewRelationList をつくる。
func NewRelationList(lists *usecase.RelationList) *RelationList {
	return &RelationList{lists: lists}
}

// relationUserResponse は一覧に出す1人。
//
// **プロフィールと同じ形にしない。** 一覧では投稿数などを数えない。
type relationUserResponse struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	// Following は閲覧者がこの相手をフォローしているか。未ログインなら false。
	Following bool `json:"following"`
}

type relationListResponse struct {
	Items []relationUserResponse `json:"items"`
	// NextCursor は続きが無ければ null。
	NextCursor *string `json:"next_cursor"`
}

// Following は GET /api/v1/users/:handle/following。ログインは不要。
func (h *RelationList) Following(c echo.Context) error {
	return h.ofUser(c, domain.RelationFollowing)
}

// Followers は GET /api/v1/users/:handle/followers。ログインは不要。
func (h *RelationList) Followers(c echo.Context) error {
	return h.ofUser(c, domain.RelationFollowers)
}

// Blocking は GET /api/v1/me/blocks。本人だけが見られる。
func (h *RelationList) Blocking(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}
	query, err := parseRelationListQuery(c)
	if err != nil {
		return Respond(c, err)
	}

	list, err := h.lists.Blocking(c.Request().Context(), user, query)
	if err != nil {
		return Respond(c, err)
	}
	return respondRelationList(c, list)
}

func (h *RelationList) ofUser(c echo.Context, kind domain.RelationListKind) error {
	handle := c.Param("handle")
	if handle == "" {
		return Respond(c, domain.NewValidationError("handle", "識別名を指定してください"))
	}
	query, err := parseRelationListQuery(c)
	if err != nil {
		return Respond(c, err)
	}
	query.Kind = kind

	list, err := h.lists.OfUser(c.Request().Context(), handle, query)
	if err != nil {
		return Respond(c, err)
	}
	return respondRelationList(c, list)
}

func respondRelationList(c echo.Context, list *domain.RelationList) error {
	items := make([]relationUserResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, relationUserResponse{
			Handle:      item.User.Handle,
			DisplayName: item.User.DisplayName,
			Bio:         item.User.Bio,
			AvatarURL:   item.User.AvatarURL,
			Following:   item.Following,
		})
	}
	res := relationListResponse{Items: items}
	if list.NextCursor != 0 {
		cursor := formatID(list.NextCursor)
		res.NextCursor = &cursor
	}
	return c.JSON(http.StatusOK, res)
}

// parseRelationListQuery はクエリパラメータを取り出す。
//
// タイムラインと同じ形にする（基本設計 05 §1）。
func parseRelationListQuery(c echo.Context) (domain.RelationListQuery, error) {
	query := domain.RelationListQuery{}
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
