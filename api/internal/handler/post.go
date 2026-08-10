package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Post は投稿のハンドラ。
type Post struct {
	posts *usecase.Post
}

// NewPost をつくる。
func NewPost(posts *usecase.Post) *Post {
	return &Post{posts: posts}
}

// createPostRequest は投稿の入力。
//
// **verdict / segments を受け取らない。** クライアントの判定結果を
// 保存に使うと、「判定OK」という嘘を添えるだけで破調が保存できる
// （基本設計 01 §4）。送られてきても読まない。
type createPostRequest struct {
	Body       string `json:"body"`
	Visibility string `json:"visibility"`
}

type postResponse struct {
	// ID は文字列。BIGINT を JSON の数値で返すと、
	// JavaScript の Number（53bit）で精度が落ちる。
	ID         string                `json:"id"`
	Body       string                `json:"body"`
	Verdict    domain.Verdict        `json:"verdict"`
	Segments   []postSegmentResponse `json:"segments"`
	Visibility domain.Visibility     `json:"visibility"`
	LikeCount  int                   `json:"like_count"`
	LikedByMe  bool                  `json:"liked_by_me"`
	Author     postAuthorResponse    `json:"author"`
	CreatedAt  time.Time             `json:"created_at"`
}

type postSegmentResponse struct {
	Text string `json:"text"`
	Mora int    `json:"mora"`
}

type postAuthorResponse struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// Create は POST /api/v1/posts。
func (h *Post) Create(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}

	var req createPostRequest
	if err := c.Bind(&req); err != nil {
		return Respond(c, domain.NewValidationError("body", "リクエストの形式が不正です"))
	}

	view, err := h.posts.Create(c.Request().Context(), usecase.CreateInput{
		Author:     user,
		Body:       req.Body,
		Visibility: domain.Visibility(req.Visibility),
	})
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusCreated, toPostResponse(view))
}

// Get は GET /api/v1/posts/:id。未ログインでも取得できる。
func (h *Post) Get(c echo.Context) error {
	id, err := parsePostID(c.Param("id"))
	if err != nil {
		return Respond(c, err)
	}

	var viewerID *int64
	if user, ok := CurrentUser(c); ok {
		viewerID = &user.ID
	}

	view, err := h.posts.Get(c.Request().Context(), id, viewerID)
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusOK, toPostResponse(view))
}

// Delete は DELETE /api/v1/posts/:id。
func (h *Post) Delete(c echo.Context) error {
	user, ok := CurrentUser(c)
	if !ok {
		return Respond(c, domain.ErrUnauthenticated)
	}
	id, err := parsePostID(c.Param("id"))
	if err != nil {
		return Respond(c, err)
	}

	if err := h.posts.Delete(c.Request().Context(), id, user.ID); err != nil {
		return Respond(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// parsePostID は経路の ID を数値に変える。
//
// 数値でない ID は 400 とする。404 にすると、
// クライアントの組み立てミスと「存在しない投稿」を区別できない。
func parsePostID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewValidationError("id", "投稿 ID の形式が不正です")
	}
	return id, nil
}

func toPostResponse(v *usecase.PostView) postResponse {
	texts := v.Post.Segments()
	morae := [3]int{v.Post.MoraKami, v.Post.MoraNaka, v.Post.MoraShimo}

	segments := make([]postSegmentResponse, 0, 3)
	for i := range texts {
		segments = append(segments, postSegmentResponse{Text: texts[i], Mora: morae[i]})
	}

	return postResponse{
		ID:         strconv.FormatInt(v.Post.ID, 10),
		Body:       v.Post.Body,
		Verdict:    v.Post.Verdict,
		Segments:   segments,
		Visibility: v.Post.Visibility,
		LikeCount:  v.Post.LikeCount,
		LikedByMe:  v.LikedByMe,
		Author: postAuthorResponse{
			Handle:      v.Author.Handle,
			DisplayName: v.Author.DisplayName,
			AvatarURL:   v.Author.AvatarURL,
		},
		CreatedAt: v.Post.CreatedAt,
	}
}
