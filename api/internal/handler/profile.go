package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Profile はユーザーページのハンドラ（S-04）。
type Profile struct {
	profiles *usecase.Profile
}

// NewProfile をつくる。
func NewProfile(profiles *usecase.Profile) *Profile {
	return &Profile{profiles: profiles}
}

// profileResponse はプロフィール。
//
// **メールアドレスを含めない。** 他人が見られる情報であり、
// 本人以外に渡す理由がない。
type profileResponse struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	// PostCount は閲覧者から見える投稿の数。
	PostCount      int `json:"post_count"`
	FollowingCount int `json:"following_count"`
	FollowerCount  int `json:"follower_count"`
	// Following は閲覧者がこの利用者をフォローしているか。未ログインなら false。
	Following bool `json:"following"`
	// Blocking は閲覧者がこの利用者をブロックしているか。未ログインなら false。
	Blocking bool `json:"blocking"`
}

// Get は GET /api/v1/users/:handle。ログインは不要。
//
// 見えない相手は 404 とする。理由を区別しない（BR-10）。
func (h *Profile) Get(c echo.Context) error {
	handle := c.Param("handle")
	if handle == "" {
		return Respond(c, domain.NewValidationError("handle", "識別名を指定してください"))
	}

	profile, err := h.profiles.Get(c.Request().Context(), handle, viewerID(c))
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusOK, profileResponse{
		Handle:         profile.User.Handle,
		DisplayName:    profile.User.DisplayName,
		Bio:            profile.User.Bio,
		AvatarURL:      profile.User.AvatarURL,
		CreatedAt:      profile.User.CreatedAt.UTC().Format(time.RFC3339Nano),
		PostCount:      profile.Counts.Posts,
		FollowingCount: profile.Counts.Following,
		FollowerCount:  profile.Counts.Followers,
		Following:      profile.Following,
		Blocking:       profile.Blocking,
	})
}

// Posts は GET /api/v1/users/:handle/posts。ログインは不要。
func (h *Profile) Posts(c echo.Context) error {
	handle := c.Param("handle")
	if handle == "" {
		return Respond(c, domain.NewValidationError("handle", "識別名を指定してください"))
	}

	base, err := parseTimelineQuery(c)
	if err != nil {
		return Respond(c, err)
	}

	timeline, err := h.profiles.Posts(c.Request().Context(), handle,
		domain.UserPostQuery{TimelineQuery: base})
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

// viewerID は閲覧者の ID を返す。未ログインなら nil。
func viewerID(c echo.Context) *int64 {
	user, ok := CurrentUser(c)
	if !ok {
		return nil
	}
	return &user.ID
}
