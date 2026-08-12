package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeRelationListRepo は一覧の偽物。
//
// **渡された条件を記録する。** 誰の一覧を、誰の目線で、どこから引くかを
// 取り違えると、他人のブロック中一覧が見えるような誤りになる。
type fakeRelationListRepo struct {
	items []domain.RelationListItem
	err   error

	calls int
	got   domain.RelationListQuery
}

func (f *fakeRelationListRepo) List(
	_ context.Context, q domain.RelationListQuery,
) ([]domain.RelationListItem, error) {
	f.calls++
	f.got = q
	return f.items, f.err
}

func listItems(ids ...int64) []domain.RelationListItem {
	items := make([]domain.RelationListItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, domain.RelationListItem{
			User: &domain.User{ID: id, Handle: "u", Status: domain.UserActive},
		})
	}
	return items
}

type listFixture struct {
	usecase *usecase.RelationList
	users   *followUserRepo
	lists   *fakeRelationListRepo
	blocks  *fakeBlockRepo
	follows *fakeFollowRepo
}

func newListFixture(users ...*domain.User) *listFixture {
	f := &listFixture{
		users:   usersOf(users...),
		lists:   &fakeRelationListRepo{},
		blocks:  newBlockRepo(),
		follows: newFollowRepo(),
	}
	f.usecase = usecase.NewRelationList(f.users, f.lists, f.blocks, f.follows)
	return f
}

func TestRelationListOfUser(t *testing.T) {
	tests := []struct {
		name string
		kind domain.RelationListKind
	}{
		{"フォロー中", domain.RelationFollowing},
		{"フォロワー", domain.RelationFollowers},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newListFixture(actor(), target())
			f.lists.items = listItems(30, 20, 10)

			got, err := f.usecase.OfUser(context.Background(), "bob", domain.RelationListQuery{
				Kind: tt.kind, ViewerID: viewer(1), Limit: limitOf(3),
			})
			if err != nil {
				t.Fatalf("取得できない: %v", err)
			}

			if f.lists.got.OwnerID != 2 {
				t.Errorf("誰の一覧かが違う: %d", f.lists.got.OwnerID)
			}
			if f.lists.got.Kind != tt.kind {
				t.Errorf("種類が違う: %s", f.lists.got.Kind)
			}
			// 続きがあるときだけカーソルを返す。
			if got.NextCursor != 10 {
				t.Errorf("カーソルが違う: %d", got.NextCursor)
			}
		})
	}
}

func TestRelationListNoNextCursorWhenShort(t *testing.T) {
	f := newListFixture(actor(), target())
	f.lists.items = listItems(30, 20)

	got, err := f.usecase.OfUser(context.Background(), "bob", domain.RelationListQuery{
		Kind: domain.RelationFollowing, Limit: limitOf(3),
	})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if got.NextCursor != 0 {
		t.Errorf("続きが無いのにカーソルが返る: %d", got.NextCursor)
	}
}

func TestRelationListOfUserHidden(t *testing.T) {
	suspended := &domain.User{ID: 3, Handle: "carol", Status: domain.UserSuspended}

	tests := []struct {
		name   string
		handle string
		setup  func(*listFixture)
	}{
		{"存在しない識別名", "nobody", func(*listFixture) {}},
		{"利用停止", "carol", func(*listFixture) {}},
		{"相手が閲覧者をブロック", "bob", func(f *listFixture) {
			f.blocks.blocks[[2]int64{2, 1}] = true
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newListFixture(actor(), target(), suspended)
			tt.setup(f)

			_, err := f.usecase.OfUser(context.Background(), tt.handle, domain.RelationListQuery{
				Kind: domain.RelationFollowing, ViewerID: viewer(1),
			})

			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("404 にならない: %v", err)
			}
			if f.lists.calls != 0 {
				t.Error("見えない相手の一覧を引いている")
			}
		})
	}
}

// **本人だけが見られる。** 誰をブロックしたかは他人に見せない。
func TestRelationListBlocking(t *testing.T) {
	f := newListFixture(actor())
	f.lists.items = listItems(5)

	got, err := f.usecase.Blocking(context.Background(), actor(), domain.RelationListQuery{})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if f.lists.got.Kind != domain.RelationBlocking {
		t.Errorf("種類が違う: %s", f.lists.got.Kind)
	}
	if f.lists.got.OwnerID != 1 {
		t.Errorf("誰の一覧かが違う: %d", f.lists.got.OwnerID)
	}
	// **閲覧者を本人に固定する。** 他人を指定できると別人の一覧が見える。
	if f.lists.got.ViewerID == nil || *f.lists.got.ViewerID != 1 {
		t.Errorf("閲覧者が本人でない: %v", f.lists.got.ViewerID)
	}
	if len(got.Items) != 1 {
		t.Errorf("件数が違う: %d", len(got.Items))
	}
}

func TestRelationListBlockingRequiresLogin(t *testing.T) {
	f := newListFixture(actor())

	_, err := f.usecase.Blocking(context.Background(), nil, domain.RelationListQuery{})

	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("401 にならない: %v", err)
	}
	if f.lists.calls != 0 {
		t.Error("未ログインで一覧を引いている")
	}
}

// **他人が指定した OwnerID を信じない。** 信じると別人のブロック中一覧が見える。
func TestRelationListBlockingIgnoresGivenOwner(t *testing.T) {
	f := newListFixture(actor())

	if _, err := f.usecase.Blocking(context.Background(), actor(), domain.RelationListQuery{
		OwnerID: 999, ViewerID: viewer(999),
	}); err != nil {
		t.Fatalf("取得できない: %v", err)
	}

	if f.lists.got.OwnerID != 1 {
		t.Errorf("指定された OwnerID が通っている: %d", f.lists.got.OwnerID)
	}
}

func TestRelationListRejectsBadLimit(t *testing.T) {
	for _, limit := range []int{0, 51} {
		f := newListFixture(actor(), target())

		_, err := f.usecase.OfUser(context.Background(), "bob", domain.RelationListQuery{
			Kind: domain.RelationFollowing, Limit: limitOf(limit),
		})

		var validation *domain.Error
		if !errors.As(err, &validation) || validation.Code != domain.CodeValidationFailed {
			t.Fatalf("limit=%d が検証エラーにならない: %v", limit, err)
		}
		if f.lists.calls != 0 {
			t.Error("不正な条件で引いている")
		}
	}
}

func TestRelationListErrorPropagates(t *testing.T) {
	f := newListFixture(actor(), target())
	f.lists.err = errors.New("読めない")

	if _, err := f.usecase.OfUser(context.Background(), "bob", domain.RelationListQuery{
		Kind: domain.RelationFollowing,
	}); err == nil {
		t.Fatal("エラーが伝わっていない")
	}
}
