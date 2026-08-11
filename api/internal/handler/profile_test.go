package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/handler"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// stubUserRepo は識別名で引くことだけに絞った偽物。
type stubUserRepo struct {
	users map[string]*domain.User
	err   error
}

func (s *stubUserRepo) Create(context.Context, *domain.User) (*domain.User, error) {
	return nil, errors.New("使わない")
}
func (s *stubUserRepo) FindByID(context.Context, int64) (*domain.User, error) {
	return nil, errors.New("使わない")
}
func (s *stubUserRepo) FindByHandle(_ context.Context, handle string) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	user, ok := s.users[handle]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}
func (s *stubUserRepo) ExistsByEmail(context.Context, string) (bool, error) {
	return false, errors.New("使わない")
}
func (s *stubUserRepo) UpdateProfile(
	_ context.Context, _ int64, displayName, bio string,
) (*domain.User, error) {
	updated := *profileOwner()
	updated.DisplayName = displayName
	updated.Bio = bio
	return &updated, nil
}

// stubProfileRepo は数え上げの偽物。
type stubProfileRepo struct {
	counts domain.ProfileCounts
}

func (s stubProfileRepo) Counts(
	context.Context, int64, bool,
) (domain.ProfileCounts, error) {
	return s.counts, nil
}

// stubUserTimelineRepo はユーザーの投稿一覧の偽物。
type stubUserTimelineRepo struct {
	items []domain.TimelineItem
	got   domain.UserPostQuery
}

func (s *stubUserTimelineRepo) Public(
	context.Context, domain.TimelineQuery,
) ([]domain.TimelineItem, error) {
	return nil, errors.New("使わない")
}
func (s *stubUserTimelineRepo) Home(
	context.Context, domain.TimelineQuery,
) ([]domain.TimelineItem, error) {
	return nil, errors.New("使わない")
}
func (s *stubUserTimelineRepo) UserPosts(
	_ context.Context, q domain.UserPostQuery,
) ([]domain.TimelineItem, error) {
	s.got = q
	return s.items, nil
}

// stubFollowRepo はフォローの偽物。常に「フォローしていない」。
type stubFollowRepo struct{}

func (stubFollowRepo) Follow(context.Context, int64, int64) error   { return nil }
func (stubFollowRepo) Unfollow(context.Context, int64, int64) error { return nil }
func (stubFollowRepo) IsFollowing(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (stubFollowRepo) CountFollowers(context.Context, int64) (int, error) { return 0, nil }
func (stubFollowRepo) IsBlocked(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func profileOwner() *domain.User {
	return &domain.User{
		ID: 10, Handle: "yamada", DisplayName: "やまだ",
		Bio:       "五七五で暮らす",
		Status:    domain.UserActive,
		CreatedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
	}
}

// callProfile はプロフィール系のハンドラを1回呼ぶ。user が nil なら未ログイン。
func callProfile(
	t *testing.T,
	path, handle string,
	users *stubUserRepo,
	timelines *stubUserTimelineRepo,
	user *domain.User,
	invoke func(*handler.Profile, echo.Context) error,
) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("handle")
	c.SetParamValues(handle)
	if user != nil {
		handler.SetCurrentUserForTest(c, user)
	}

	h := handler.NewProfile(usecase.NewProfile(
		users,
		stubProfileRepo{counts: domain.ProfileCounts{Posts: 7, Following: 5, Followers: 3}},
		timelines,
		stubFollowRepo{},
		stubBlockRepo{},
	))
	if err := invoke(h, c); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("応答を解釈できない: %v (body=%s)", err, rec.Body.String())
	}
	return rec.Code, decoded
}

func usersWith(list ...*domain.User) *stubUserRepo {
	byHandle := map[string]*domain.User{}
	for _, u := range list {
		byHandle[u.Handle] = u
	}
	return &stubUserRepo{users: byHandle}
}

func TestGetProfileResponds200(t *testing.T) {
	status, body := callProfile(t, "/api/v1/users/yamada", "yamada",
		usersWith(profileOwner()), &stubUserTimelineRepo{}, nil,
		(*handler.Profile).Get)

	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	for key, want := range map[string]any{
		"handle":          "yamada",
		"display_name":    "やまだ",
		"bio":             "五七五で暮らす",
		"post_count":      float64(7),
		"following_count": float64(5),
		"follower_count":  float64(3),
		"following":       false,
		"blocking":        false,
	} {
		if body[key] != want {
			t.Errorf("%s が違う: %v, want %v", key, body[key], want)
		}
	}
	if body["created_at"] != "2026-08-01T09:00:00Z" {
		t.Errorf("登録日が違う: %v", body["created_at"])
	}
}

// **メールアドレスを他人に見せない。** 他人が見られる情報であり、渡す理由がない。
func TestGetProfileHidesEmail(t *testing.T) {
	owner := profileOwner()
	owner.Email = "yamada@example.com"

	_, body := callProfile(t, "/api/v1/users/yamada", "yamada",
		usersWith(owner), &stubUserTimelineRepo{}, nil,
		(*handler.Profile).Get)

	for key, value := range body {
		if str, ok := value.(string); ok && str == "yamada@example.com" {
			t.Fatalf("メールアドレスが %s に含まれている", key)
		}
	}
}

func TestGetProfileResponds404(t *testing.T) {
	status, body := callProfile(t, "/api/v1/users/nobody", "nobody",
		usersWith(profileOwner()), &stubUserTimelineRepo{}, nil,
		(*handler.Profile).Get)

	if status != http.StatusNotFound {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

func TestGetUserPostsResponds200(t *testing.T) {
	timelines := &stubUserTimelineRepo{items: []domain.TimelineItem{
		{Post: publishedPost(), Author: profileOwner()},
	}}

	status, body := callProfile(t, "/api/v1/users/yamada/posts", "yamada",
		usersWith(profileOwner()), timelines, nil,
		(*handler.Profile).Posts)

	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items が違う: %v", body["items"])
	}
	// 続きが無ければ null。件数が limit に満たないため。
	if body["next_cursor"] != nil {
		t.Errorf("続きが無いのにカーソルが返る: %v", body["next_cursor"])
	}
	if timelines.got.AuthorID != 10 {
		t.Errorf("投稿者が違う: %d", timelines.got.AuthorID)
	}
}

func TestGetUserPostsRejectsBadCursor(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/yamada/posts?cursor=abc", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("handle")
	c.SetParamValues("yamada")

	h := handler.NewProfile(usecase.NewProfile(
		usersWith(profileOwner()), stubProfileRepo{}, &stubUserTimelineRepo{},
		stubFollowRepo{}, stubBlockRepo{},
	))
	if err := h.Posts(c); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProfileRequiresHandle(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*handler.Profile, echo.Context) error
	}{
		{"プロフィール", (*handler.Profile).Get},
		{"投稿一覧", (*handler.Profile).Posts},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := callProfile(t, "/api/v1/users/", "",
				usersWith(profileOwner()), &stubUserTimelineRepo{}, nil, tt.invoke)

			if status != http.StatusBadRequest {
				t.Fatalf("status=%d body=%v", status, body)
			}
		})
	}
}

// ----------------------------------------------------------------------
// プロフィールの更新（FR-01-03）
// ----------------------------------------------------------------------

// callUpdateProfile は PATCH /api/v1/me/profile を1回呼ぶ。
func callUpdateProfile(
	t *testing.T, body string, user *domain.User, users *stubUserRepo,
) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/profile", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		handler.SetCurrentUserForTest(c, user)
	}

	h := handler.NewProfile(usecase.NewProfile(
		users, stubProfileRepo{}, &stubUserTimelineRepo{}, stubFollowRepo{}, stubBlockRepo{},
	))
	if err := h.UpdateProfile(c); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}
	var decoded map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("応答を解釈できない: %v (body=%s)", err, rec.Body.String())
		}
	}
	return rec.Code, decoded
}

func TestUpdateProfileResponds200(t *testing.T) {
	owner := profileOwner()
	status, body := callUpdateProfile(t,
		`{"display_name":"やまだ改","bio":"五七五で暮らす"}`, owner, usersWith(owner))

	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if body["display_name"] != "やまだ改" || body["bio"] != "五七五で暮らす" {
		t.Errorf("更新後の値が返らない: %v", body)
	}
}

// **省略と空文字を区別する。** 区別できないと自己紹介を消せない。
func TestUpdateProfileDistinguishesOmittedFromEmpty(t *testing.T) {
	owner := profileOwner()

	_, cleared := callUpdateProfile(t, `{"bio":""}`, owner, usersWith(owner))
	if cleared["bio"] != "" {
		t.Errorf("空文字で消えない: %v", cleared["bio"])
	}
	if cleared["display_name"] != owner.DisplayName {
		t.Errorf("触れていない表示名が変わった: %v", cleared["display_name"])
	}

	_, untouched := callUpdateProfile(t, `{"display_name":"やまだ改"}`, owner, usersWith(owner))
	if untouched["bio"] != owner.Bio {
		t.Errorf("触れていない自己紹介が変わった: %v", untouched["bio"])
	}
}

func TestUpdateProfileRejects(t *testing.T) {
	owner := profileOwner()

	tests := []struct {
		name string
		body string
		user *domain.User
		want int
	}{
		{"表示名を空にする", `{"display_name":""}`, owner, http.StatusBadRequest},
		{"形式が不正", `{`, owner, http.StatusBadRequest},
		{"未ログイン", `{"bio":"x"}`, nil, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := callUpdateProfile(t, tt.body, tt.user, usersWith(owner))
			if status != tt.want {
				t.Fatalf("status=%d, want %d body=%v", status, tt.want, body)
			}
		})
	}
}
