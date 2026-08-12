package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/handler"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// stubRelationListRepo は一覧の偽物。
type stubRelationListRepo struct {
	items []domain.RelationListItem
	got   domain.RelationListQuery
}

func (s *stubRelationListRepo) List(
	_ context.Context, q domain.RelationListQuery,
) ([]domain.RelationListItem, error) {
	s.got = q
	return s.items, nil
}

func listedUser() domain.RelationListItem {
	return domain.RelationListItem{
		User: &domain.User{
			ID: 20, Handle: "aoi", DisplayName: "あおい",
			Bio: "五七五で暮らす", Status: domain.UserActive,
		},
		Following: true,
	}
}

// callRelationList は一覧のハンドラを1回呼ぶ。user が nil なら未ログイン。
func callRelationList(
	t *testing.T,
	path, handle string,
	repo *stubRelationListRepo,
	users *stubUserRepo,
	user *domain.User,
	invoke func(*handler.RelationList, echo.Context) error,
) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if handle != "" {
		c.SetParamNames("handle")
		c.SetParamValues(handle)
	}
	if user != nil {
		handler.SetCurrentUserForTest(c, user)
	}

	h := handler.NewRelationList(usecase.NewRelationList(users, repo, stubBlockRepo{}, stubFollowRepo{}))
	if err := invoke(h, c); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("応答を解釈できない: %v (body=%s)", err, rec.Body.String())
	}
	return rec.Code, decoded
}

func TestRelationListResponds200(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*handler.RelationList, echo.Context) error
		want   domain.RelationListKind
	}{
		{"フォロー中", (*handler.RelationList).Following, domain.RelationFollowing},
		{"フォロワー", (*handler.RelationList).Followers, domain.RelationFollowers},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubRelationListRepo{items: []domain.RelationListItem{listedUser()}}
			status, body := callRelationList(t, "/api/v1/users/yamada/following", "yamada",
				repo, usersWith(profileOwner()), nil, tt.invoke)

			if status != http.StatusOK {
				t.Fatalf("status=%d body=%v", status, body)
			}
			if repo.got.Kind != tt.want {
				t.Errorf("種類が違う: %s", repo.got.Kind)
			}
			items, ok := body["items"].([]any)
			if !ok || len(items) != 1 {
				t.Fatalf("items が違う: %v", body["items"])
			}
			first, _ := items[0].(map[string]any)
			for key, want := range map[string]any{
				"handle": "aoi", "display_name": "あおい", "bio": "五七五で暮らす", "following": true,
			} {
				if first[key] != want {
					t.Errorf("%s が違う: %v, want %v", key, first[key], want)
				}
			}
			// 続きが無ければ null。件数が limit に満たないため。
			if body["next_cursor"] != nil {
				t.Errorf("続きが無いのにカーソルが返る: %v", body["next_cursor"])
			}
		})
	}
}

func TestRelationListResponds404(t *testing.T) {
	status, body := callRelationList(t, "/api/v1/users/nobody/following", "nobody",
		&stubRelationListRepo{}, usersWith(profileOwner()), nil,
		(*handler.RelationList).Following)

	if status != http.StatusNotFound {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

func TestRelationListRequiresHandle(t *testing.T) {
	status, body := callRelationList(t, "/api/v1/users//following", "",
		&stubRelationListRepo{}, usersWith(profileOwner()), nil,
		(*handler.RelationList).Following)

	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

// **ブロック中一覧は本人だけ。** 誰をブロックしたかは他人に見せない。
func TestBlockingListRequiresLogin(t *testing.T) {
	status, body := callRelationList(t, "/api/v1/me/blocks", "",
		&stubRelationListRepo{}, usersWith(profileOwner()), nil,
		(*handler.RelationList).Blocking)

	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

func TestBlockingListResponds200(t *testing.T) {
	repo := &stubRelationListRepo{items: []domain.RelationListItem{listedUser()}}
	status, body := callRelationList(t, "/api/v1/me/blocks", "",
		repo, usersWith(profileOwner()), profileOwner(),
		(*handler.RelationList).Blocking)

	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if repo.got.Kind != domain.RelationBlocking {
		t.Errorf("種類が違う: %s", repo.got.Kind)
	}
	if repo.got.OwnerID != profileOwner().ID {
		t.Errorf("誰の一覧かが違う: %d", repo.got.OwnerID)
	}
}

func TestRelationListRejectsBadQuery(t *testing.T) {
	tests := []struct{ name, query string }{
		{"カーソルが数値でない", "?cursor=abc"},
		{"件数が数値でない", "?limit=abc"},
		{"件数が上限を超える", "?limit=51"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubRelationListRepo{}
			status, body := callRelationList(t, "/api/v1/users/yamada/following"+tt.query, "yamada",
				repo, usersWith(profileOwner()), nil, (*handler.RelationList).Following)

			if status != http.StatusBadRequest {
				t.Fatalf("status=%d body=%v", status, body)
			}
		})
	}
}
