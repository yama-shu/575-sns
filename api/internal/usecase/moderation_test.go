package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeReportRepo は通報リポジトリの偽物。
//
// UNIQUE 制約の代わりに (reporter, post) の集合で重複を判定する。
type fakeReportRepo struct {
	seen  map[[2]int64]bool
	err   error
	calls int
	nextI int64
}

func newReportRepo() *fakeReportRepo {
	return &fakeReportRepo{seen: map[[2]int64]bool{}}
}

func (f *fakeReportRepo) Create(_ context.Context, report *domain.Report) (*domain.Report, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	key := [2]int64{report.ReporterID, report.PostID}
	if f.seen[key] {
		return nil, domain.ErrAlreadyReported
	}
	f.seen[key] = true
	f.nextI++
	stored := *report
	stored.ID = f.nextI
	return &stored, nil
}

func reporter() *domain.User {
	return &domain.User{ID: 1, Handle: "alice", Status: domain.UserActive}
}

func newModeration(
	users *followUserRepo, posts *fakePostRepo,
	reports *fakeReportRepo, blocks *fakeBlockRepo,
) *usecase.Moderation {
	return usecase.NewModeration(users, posts, reports, blocks)
}

// postBy は author が書いた公開投稿を返す。
func postBy(authorID int64) *domain.Post {
	p := storedPost()
	p.AuthorID = authorID
	return p
}

func TestReport(t *testing.T) {
	reports := newReportRepo()
	posts := &fakePostRepo{post: postBy(2), author: target()}
	m := newModeration(usersOf(reporter(), target()), posts, reports, newBlockRepo())

	report, err := m.Report(context.Background(), usecase.ReportInput{
		ReporterID: 1, PostID: 1, Reason: domain.ReportSpam, Comment: "宣伝です",
	})
	if err != nil {
		t.Fatalf("通報できない: %v", err)
	}
	if report.ID == 0 || report.Status != domain.ReportPending {
		t.Errorf("通報の内容が違う: %+v", report)
	}
}

// 重複通報は 409。**冪等にしない。**
// 黙って成功を返すと「通報が届いた」と誤解する。
func TestReportRejectsDuplicate(t *testing.T) {
	reports := newReportRepo()
	posts := &fakePostRepo{post: postBy(2), author: target()}
	m := newModeration(usersOf(reporter(), target()), posts, reports, newBlockRepo())

	in := usecase.ReportInput{ReporterID: 1, PostID: 1, Reason: domain.ReportSpam}
	if _, err := m.Report(context.Background(), in); err != nil {
		t.Fatalf("通報できない: %v", err)
	}
	_, err := m.Report(context.Background(), in)
	if !errors.Is(err, domain.ErrAlreadyReported) {
		t.Errorf("ALREADY_REPORTED を期待したが %v", err)
	}
}

// 別の利用者の通報は独立して作られる。
func TestReportByAnotherUserIsIndependent(t *testing.T) {
	reports := newReportRepo()
	posts := &fakePostRepo{post: postBy(3), author: target()}
	carol := &domain.User{ID: 2, Handle: "carol", Status: domain.UserActive}
	m := newModeration(usersOf(reporter(), carol), posts, reports, newBlockRepo())

	for _, id := range []int64{1, 2} {
		if _, err := m.Report(context.Background(), usecase.ReportInput{
			ReporterID: id, PostID: 1, Reason: domain.ReportSpam,
		}); err != nil {
			t.Fatalf("利用者 %d が通報できない: %v", id, err)
		}
	}
	if len(reports.seen) != 2 {
		t.Errorf("通報が独立していない: %d 件", len(reports.seen))
	}
}

// BR-07: 自分の投稿は通報できない。
// DB の CHECK 制約では表現できないため、ここで担保する。
func TestReportOwnPostIsRejected(t *testing.T) {
	reports := newReportRepo()
	posts := &fakePostRepo{post: postBy(1), author: reporter()}
	m := newModeration(usersOf(reporter()), posts, reports, newBlockRepo())

	_, err := m.Report(context.Background(), usecase.ReportInput{
		ReporterID: 1, PostID: 1, Reason: domain.ReportSpam,
	})
	if !errors.Is(err, domain.ErrCannotReportSelf) {
		t.Fatalf("CANNOT_REPORT_SELF を期待したが %v", err)
	}
	if reports.calls != 0 {
		t.Errorf("通報が作られた: %d 回", reports.calls)
	}
}

func TestReportValidatesInput(t *testing.T) {
	tests := map[string]usecase.ReportInput{
		"理由が未知":     {ReporterID: 1, PostID: 1, Reason: "whatever"},
		"理由が空":      {ReporterID: 1, PostID: 1, Reason: ""},
		"コメントが長すぎる": {ReporterID: 1, PostID: 1, Reason: domain.ReportSpam, Comment: strings.Repeat("あ", domain.ReportCommentMaxLength+1)},
	}

	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			reports := newReportRepo()
			posts := &fakePostRepo{post: postBy(2), author: target()}
			m := newModeration(usersOf(reporter(), target()), posts, reports, newBlockRepo())

			_, err := m.Report(context.Background(), in)
			var appErr *domain.Error
			if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
				t.Fatalf("VALIDATION_FAILED を期待したが %v", err)
			}
			if reports.calls != 0 {
				t.Errorf("通報が作られた: %d 回", reports.calls)
			}
		})
	}
}

// 削除済み・非表示の投稿は通報できない。運営が対応する対象がすでに無い。
func TestReportUnavailablePost(t *testing.T) {
	for name, status := range map[string]domain.PostStatus{
		"削除済み": domain.PostDeleted,
		"非表示":  domain.PostHidden,
	} {
		t.Run(name, func(t *testing.T) {
			post := postBy(2)
			post.Status = status
			reports := newReportRepo()
			m := newModeration(usersOf(reporter(), target()),
				&fakePostRepo{post: post, author: target()}, reports, newBlockRepo())

			_, err := m.Report(context.Background(), usecase.ReportInput{
				ReporterID: 1, PostID: 1, Reason: domain.ReportSpam,
			})
			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("NOT_FOUND を期待したが %v", err)
			}
			if reports.calls != 0 {
				t.Errorf("通報が作られた: %d 回", reports.calls)
			}
		})
	}
}

// 見えない投稿は通報できない。見えないものを通報する経路が無い。
func TestReportBlockedPost(t *testing.T) {
	for name, block := range map[string][2]int64{
		"自分が相手をブロック": {1, 2},
		"相手が自分をブロック": {2, 1},
	} {
		t.Run(name, func(t *testing.T) {
			blocks := newBlockRepo()
			blocks.blocks[block] = true
			reports := newReportRepo()
			m := newModeration(usersOf(reporter(), target()),
				&fakePostRepo{post: postBy(2), author: target()}, reports, blocks)

			_, err := m.Report(context.Background(), usecase.ReportInput{
				ReporterID: 1, PostID: 1, Reason: domain.ReportSpam,
			})
			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("NOT_FOUND を期待したが %v", err)
			}
			if reports.calls != 0 {
				t.Errorf("通報が作られた: %d 回", reports.calls)
			}
		})
	}
}

func TestReportMissingPost(t *testing.T) {
	reports := newReportRepo()
	m := newModeration(usersOf(reporter()),
		&fakePostRepo{findErr: domain.ErrNotFound}, reports, newBlockRepo())

	_, err := m.Report(context.Background(), usecase.ReportInput{
		ReporterID: 1, PostID: 999, Reason: domain.ReportSpam,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

func TestReportPropagatesErrors(t *testing.T) {
	t.Run("ブロックを確認できない", func(t *testing.T) {
		blocks := newBlockRepo()
		blocks.err = errors.New("確認できない")
		m := newModeration(usersOf(reporter(), target()),
			&fakePostRepo{post: postBy(2), author: target()}, newReportRepo(), blocks)

		if _, err := m.Report(context.Background(), usecase.ReportInput{
			ReporterID: 1, PostID: 1, Reason: domain.ReportSpam,
		}); err == nil {
			t.Error("エラーにならない")
		}
	})

	t.Run("保存できない", func(t *testing.T) {
		reports := newReportRepo()
		reports.err = errors.New("保存できない")
		m := newModeration(usersOf(reporter(), target()),
			&fakePostRepo{post: postBy(2), author: target()}, reports, newBlockRepo())

		if _, err := m.Report(context.Background(), usecase.ReportInput{
			ReporterID: 1, PostID: 1, Reason: domain.ReportSpam,
		}); err == nil {
			t.Error("エラーにならない")
		}
	})
}

// ---------------------------------------------------------------------------
// ブロック
// ---------------------------------------------------------------------------

func TestBlock(t *testing.T) {
	blocks := newBlockRepo()
	m := newModeration(usersOf(reporter(), target()), &fakePostRepo{}, newReportRepo(), blocks)

	state, err := m.Block(context.Background(), reporter(), "bob")
	if err != nil {
		t.Fatalf("ブロックできない: %v", err)
	}
	if !state.Blocked {
		t.Error("ブロックされていない")
	}
	if !blocks.blocks[[2]int64{1, 2}] {
		t.Error("関係が作られていない")
	}
}

// すでにブロック済みでも成功する（冪等）。
func TestBlockIsIdempotent(t *testing.T) {
	blocks := newBlockRepo()
	m := newModeration(usersOf(reporter(), target()), &fakePostRepo{}, newReportRepo(), blocks)

	for i := range 3 {
		if _, err := m.Block(context.Background(), reporter(), "bob"); err != nil {
			t.Fatalf("%d 回目で失敗した: %v", i+1, err)
		}
	}
}

func TestUnblock(t *testing.T) {
	blocks := newBlockRepo()
	blocks.blocks[[2]int64{1, 2}] = true
	m := newModeration(usersOf(reporter(), target()), &fakePostRepo{}, newReportRepo(), blocks)

	state, err := m.Unblock(context.Background(), reporter(), "bob")
	if err != nil {
		t.Fatalf("解除できない: %v", err)
	}
	if state.Blocked {
		t.Error("解除されていない")
	}
	if blocks.blocks[[2]int64{1, 2}] {
		t.Error("関係が残っている")
	}
}

// ブロックしていない相手の解除も成功する（冪等）。
func TestUnblockIsIdempotent(t *testing.T) {
	blocks := newBlockRepo()
	m := newModeration(usersOf(reporter(), target()), &fakePostRepo{}, newReportRepo(), blocks)

	if _, err := m.Unblock(context.Background(), reporter(), "bob"); err != nil {
		t.Errorf("解除できない: %v", err)
	}
}

// BR-06: 自分自身をブロックできない。
func TestBlockSelfIsRejected(t *testing.T) {
	blocks := newBlockRepo()
	m := newModeration(usersOf(reporter()), &fakePostRepo{}, newReportRepo(), blocks)

	_, err := m.Block(context.Background(), reporter(), "alice")
	if !errors.Is(err, domain.ErrCannotBlockSelf) {
		t.Fatalf("CANNOT_BLOCK_SELF を期待したが %v", err)
	}
	if blocks.blockCalls != 0 {
		t.Errorf("関係が作られた: %d 回", blocks.blockCalls)
	}
}

func TestUnblockSelfIsRejected(t *testing.T) {
	m := newModeration(usersOf(reporter()), &fakePostRepo{}, newReportRepo(), newBlockRepo())

	if _, err := m.Unblock(context.Background(), reporter(), "alice"); !errors.Is(err, domain.ErrCannotBlockSelf) {
		t.Errorf("CANNOT_BLOCK_SELF を期待したが %v", err)
	}
}

// **ブロックされている相手をブロックし返せること。**
// フォロー（#34）と違い、相手のブロックを理由に拒まない。
// ブロックは自分の側で相手を遮断する操作であり、妨げる理由がない。
func TestBlockWhenBlockedByTargetIsAllowed(t *testing.T) {
	blocks := newBlockRepo()
	blocks.blocks[[2]int64{2, 1}] = true
	m := newModeration(usersOf(reporter(), target()), &fakePostRepo{}, newReportRepo(), blocks)

	if _, err := m.Block(context.Background(), reporter(), "bob"); err != nil {
		t.Fatalf("ブロックし返せない: %v", err)
	}
	if !blocks.blocks[[2]int64{1, 2}] {
		t.Error("関係が作られていない")
	}
}

func TestBlockMissingUser(t *testing.T) {
	m := newModeration(usersOf(reporter()), &fakePostRepo{}, newReportRepo(), newBlockRepo())

	if _, err := m.Block(context.Background(), reporter(), "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

func TestBlockDeletedUser(t *testing.T) {
	deleted := target()
	deleted.Status = domain.UserDeleted
	blocks := newBlockRepo()
	m := newModeration(usersOf(reporter(), deleted), &fakePostRepo{}, newReportRepo(), blocks)

	if _, err := m.Block(context.Background(), reporter(), "bob"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
	if blocks.blockCalls != 0 {
		t.Errorf("関係が作られた: %d 回", blocks.blockCalls)
	}
}

func TestBlockPropagatesErrors(t *testing.T) {
	blocks := newBlockRepo()
	blocks.err = errors.New("失敗")
	m := newModeration(usersOf(reporter(), target()), &fakePostRepo{}, newReportRepo(), blocks)

	if _, err := m.Block(context.Background(), reporter(), "bob"); err == nil {
		t.Error("エラーにならない")
	}
	if _, err := m.Unblock(context.Background(), reporter(), "bob"); err == nil {
		t.Error("エラーにならない")
	}
}

func TestUnblockMissingUser(t *testing.T) {
	blocks := newBlockRepo()
	m := newModeration(usersOf(reporter()), &fakePostRepo{}, newReportRepo(), blocks)

	if _, err := m.Unblock(context.Background(), reporter(), "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
	if blocks.unblockCalls != 0 {
		t.Errorf("解除が呼ばれた: %d 回", blocks.unblockCalls)
	}
}
