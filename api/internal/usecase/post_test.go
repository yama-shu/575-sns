package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakePostRepo は投稿リポジトリの偽物。
//
// **DB を使わない。** 呼び出し回数も記録し、
// 「エラーを返しつつ保存してしまう」実装を検出できるようにする。
type fakePostRepo struct {
	post   *domain.Post
	author *domain.User
	findErr,
	createErr,
	deleteErr error

	following    bool
	liked        bool
	followingErr error
	likedErr     error

	createCalls int
	deleteCalls int
	created     *domain.Post
	deletedAt   time.Time
}

func (f *fakePostRepo) Create(_ context.Context, post *domain.Post) (*domain.Post, error) {
	f.createCalls++
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = post
	stored := *post
	stored.ID = 1
	stored.CreatedAt = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	return &stored, nil
}

func (f *fakePostRepo) FindByID(context.Context, int64) (*domain.Post, *domain.User, error) {
	if f.findErr != nil {
		return nil, nil, f.findErr
	}
	return f.post, f.author, nil
}

func (f *fakePostRepo) Delete(_ context.Context, _ int64, now time.Time) error {
	f.deleteCalls++
	f.deletedAt = now
	return f.deleteErr
}

func (f *fakePostRepo) IsFollowing(context.Context, int64, int64) (bool, error) {
	return f.following, f.followingErr
}

func (f *fakePostRepo) IsLikedBy(context.Context, int64, int64) (bool, error) {
	return f.liked, f.likedErr
}

// fakeBlockRepo はブロックの偽物。可視性の判定にだけ使う。
type fakeBlockRepo struct {
	blocks map[[2]int64]bool
	err    error

	blockCalls   int
	unblockCalls int
}

func newBlockRepo() *fakeBlockRepo {
	return &fakeBlockRepo{blocks: map[[2]int64]bool{}}
}

func (f *fakeBlockRepo) Block(_ context.Context, blockerID, blockedID int64) error {
	f.blockCalls++
	if f.err != nil {
		return f.err
	}
	f.blocks[[2]int64{blockerID, blockedID}] = true
	return nil
}

func (f *fakeBlockRepo) Unblock(_ context.Context, blockerID, blockedID int64) error {
	f.unblockCalls++
	if f.err != nil {
		return f.err
	}
	delete(f.blocks, [2]int64{blockerID, blockedID})
	return nil
}

func (f *fakeBlockRepo) IsBlocked(_ context.Context, blockerID, blockedID int64) (bool, error) {
	return f.blocks[[2]int64{blockerID, blockedID}], f.err
}

func (f *fakeBlockRepo) IsBlockedEitherWay(_ context.Context, a, b int64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.blocks[[2]int64{a, b}] || f.blocks[[2]int64{b, a}], nil
}

func author() *domain.User {
	return &domain.User{ID: 10, Handle: "yamada", DisplayName: "やまだ"}
}

func fixedClock() usecase.Clock {
	return func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
}

func storedPost() *domain.Post {
	return &domain.Post{
		ID: 1, AuthorID: 10,
		Body: "今日もまた会議のための会議かな", Reading: "キョウモマタカイギノタメノカイギカナ",
		Verdict: domain.VerdictTeikei, Break1: 5, Break2: 11,
		MoraKami: 5, MoraNaka: 7, MoraShimo: 5,
		Visibility: domain.VisibilityPublic, Status: domain.PostPublished,
	}
}

func TestCreatePostStoresJudgement(t *testing.T) {
	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: teikei()}, newBlockRepo(), fixedClock())

	view, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた会議のための会議かな",
	})
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}
	if view.Post.ID == 0 {
		t.Error("ID が採番されていない")
	}
	if view.Author.Handle != "yamada" {
		t.Errorf("投稿者が違う: %v", view.Author)
	}

	got := repo.created
	if got.Verdict != domain.VerdictTeikei {
		t.Errorf("判定結果が保存されない: %v", got.Verdict)
	}
	if got.MoraKami != 5 || got.MoraNaka != 7 || got.MoraShimo != 5 {
		t.Errorf("モーラ数が違う: %d/%d/%d", got.MoraKami, got.MoraNaka, got.MoraShimo)
	}
	if got.Break1 != 5 || got.Break2 != 11 {
		t.Errorf("区切り位置が違う: %d, %d", got.Break1, got.Break2)
	}
	if got.Visibility != domain.VisibilityPublic {
		t.Errorf("公開範囲の既定値が public でない: %v", got.Visibility)
	}
}

// 保存するのは正規化後の本文。
// 元の入力を保存すると、区切り位置が本文の位置とずれる。
func TestCreatePostStoresNormalizedText(t *testing.T) {
	analysis := teikei()
	analysis.NormalizedText = "今日もまた会議のための会議かな"

	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: analysis}, newBlockRepo(), fixedClock())

	// 全角空白を含む入力。正規化で圧縮される。
	if _, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた　　会議のための会議かな",
	}); err != nil {
		t.Fatalf("投稿できない: %v", err)
	}
	if repo.created.Body != "今日もまた会議のための会議かな" {
		t.Errorf("正規化前の本文が保存されている: %q", repo.created.Body)
	}
	// 区切りで本文を3句に戻せること。
	if segs := repo.created.Segments(); segs[0] != "今日もまた" || segs[2] != "会議かな" {
		t.Errorf("本文を3句に戻せない: %v", segs)
	}
}

// 破調は保存しない。
func TestCreatePostRejectsHachoWithoutSaving(t *testing.T) {
	repo := &fakePostRepo{}
	analysis := &domain.Analysis{
		Verdict: domain.VerdictHacho, Reason: domain.ReasonTooFewMora, TotalMora: 8,
	}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: analysis}, newBlockRepo(), fixedClock())

	_, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日は疲れた",
	})
	var appErr *domain.Error
	if !errors.As(err, &appErr) || appErr.Code != domain.CodeProsodyHacho {
		t.Fatalf("PROSODY_HACHO を期待したが %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("破調なのに %d 回保存された", repo.createCalls)
	}
	// 利用者が直すための情報が付くこと。
	if appErr.Details["total_mora"] != 8 || appErr.Details["reason"] != "TOO_FEW_MORA" {
		t.Errorf("詳細が足りない: %v", appErr.Details)
	}
}

// 読めない語は破調と区別する。読めなかっただけの本文に
// 「五七五になっていません」と返すと、利用者は直しようがない。
func TestCreatePostRejectsUnknownReadingSeparately(t *testing.T) {
	repo := &fakePostRepo{}
	analysis := &domain.Analysis{
		Verdict: domain.VerdictUnknown, Reason: domain.ReasonReadingUnavailable,
		Unreadable: []string{"甃"},
	}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: analysis}, newBlockRepo(), fixedClock())

	_, err := p.Create(context.Background(), usecase.CreateInput{Author: author(), Body: "甃"})
	var appErr *domain.Error
	if !errors.As(err, &appErr) || appErr.Code != domain.CodeProsodyUnknownReading {
		t.Fatalf("PROSODY_UNKNOWN_READING を期待したが %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("保存された: %d 回", repo.createCalls)
	}
	unreadable, ok := appErr.Details["unreadable"].([]string)
	if !ok || len(unreadable) != 1 || unreadable[0] != "甃" {
		t.Errorf("読めなかった語が返らない: %v", appErr.Details)
	}
}

// 判定エンジンが使えないときは保存しない。
func TestCreatePostPropagatesAnalyzerError(t *testing.T) {
	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{err: domain.ErrProsodyUnavailable}, newBlockRepo(), fixedClock())

	_, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた会議のための会議かな",
	})
	if !errors.Is(err, domain.ErrProsodyUnavailable) {
		t.Fatalf("PROSODY_UNAVAILABLE を期待したが %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("判定できていないのに保存された: %d 回", repo.createCalls)
	}
}

func TestCreatePostValidatesInput(t *testing.T) {
	tests := map[string]usecase.CreateInput{
		"空の本文":   {Author: author(), Body: ""},
		"空白のみ":   {Author: author(), Body: "  "},
		"長すぎる本文": {Author: author(), Body: strings.Repeat("あ", domain.BodyMaxLength+1)},
		"不正な公開範囲": {
			Author: author(), Body: "今日もまた会議のための会議かな", Visibility: "secret",
		},
	}

	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			repo := &fakePostRepo{}
			analyzer := &fakeAnalyzer{result: teikei()}
			p := usecase.NewPost(repo, analyzer, newBlockRepo(), fixedClock())

			_, err := p.Create(context.Background(), in)
			var appErr *domain.Error
			if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
				t.Fatalf("VALIDATION_FAILED を期待したが %v", err)
			}
			if repo.createCalls != 0 {
				t.Errorf("保存された: %d 回", repo.createCalls)
			}
		})
	}
}

// 本文の上限が DB の VARCHAR(100) と一致すること。
// ずれていると、API を通った本文が DB で 500 になる。
func TestPostBodyLimitMatchesSchema(t *testing.T) {
	if domain.BodyMaxLength != 100 {
		t.Errorf("本文の上限が posts.body VARCHAR(100) と違う: %d", domain.BodyMaxLength)
	}

	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: teikei()}, newBlockRepo(), fixedClock())
	if _, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: strings.Repeat("あ", domain.BodyMaxLength),
	}); err != nil {
		t.Errorf("上限ちょうどが弾かれた: %v", err)
	}
}

// 公開範囲を指定できること。
func TestCreatePostAcceptsFollowersVisibility(t *testing.T) {
	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: teikei()}, newBlockRepo(), fixedClock())

	if _, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた会議のための会議かな",
		Visibility: domain.VisibilityFollowers,
	}); err != nil {
		t.Fatalf("投稿できない: %v", err)
	}
	if repo.created.Visibility != domain.VisibilityFollowers {
		t.Errorf("公開範囲が保存されない: %v", repo.created.Visibility)
	}
}

func TestGetPost(t *testing.T) {
	repo := &fakePostRepo{post: storedPost(), author: author(), liked: true}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	viewer := int64(99)
	view, err := p.Get(context.Background(), 1, &viewer)
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if view.Post.Body != "今日もまた会議のための会議かな" || !view.LikedByMe {
		t.Errorf("取得内容が違う: %+v", view)
	}
}

// 未ログインでも公開投稿は見られる。いいねの状態は問い合わせない。
func TestGetPostWithoutLogin(t *testing.T) {
	repo := &fakePostRepo{post: storedPost(), author: author(), liked: true}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	view, err := p.Get(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if view.LikedByMe {
		t.Error("未ログインなのに liked_by_me が true")
	}
}

// 削除済み・非表示は 404。存在自体を返さない。
func TestGetPostHidesDeletedAndHidden(t *testing.T) {
	for name, status := range map[string]domain.PostStatus{
		"削除済み": domain.PostDeleted,
		"非表示":  domain.PostHidden,
	} {
		t.Run(name, func(t *testing.T) {
			post := storedPost()
			post.Status = status
			repo := &fakePostRepo{post: post, author: author()}
			p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

			if _, err := p.Get(context.Background(), 1, nil); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("NOT_FOUND を期待したが %v", err)
			}
		})
	}
}

// フォロワー限定の投稿の可視性。
func TestGetFollowersOnlyPost(t *testing.T) {
	stranger, follower, self := int64(99), int64(88), int64(10)

	tests := map[string]struct {
		viewer    *int64
		following bool
		visible   bool
	}{
		"未ログイン":     {nil, false, false},
		"フォローしていない": {&stranger, false, false},
		"フォローしている":  {&follower, true, true},
		"投稿者本人":     {&self, false, true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			post := storedPost()
			post.Visibility = domain.VisibilityFollowers
			repo := &fakePostRepo{post: post, author: author(), following: tt.following}
			p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

			_, err := p.Get(context.Background(), 1, tt.viewer)
			if tt.visible && err != nil {
				t.Errorf("見えるはずが %v", err)
			}
			if !tt.visible && !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("NOT_FOUND を期待したが %v", err)
			}
		})
	}
}

func TestDeletePostByAuthor(t *testing.T) {
	repo := &fakePostRepo{post: storedPost(), author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	if err := p.Delete(context.Background(), 1, 10); err != nil {
		t.Fatalf("削除できない: %v", err)
	}
	if repo.deleteCalls != 1 {
		t.Errorf("削除が呼ばれた回数が違う: %d", repo.deleteCalls)
	}
	if !repo.deletedAt.Equal(fixedClock()()) {
		t.Errorf("削除日時が違う: %v", repo.deletedAt)
	}
}

// 他人の投稿は削除できない（BR-03）。
func TestDeletePostByOtherUserIsForbidden(t *testing.T) {
	repo := &fakePostRepo{post: storedPost(), author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	err := p.Delete(context.Background(), 1, 99)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("FORBIDDEN を期待したが %v", err)
	}
	if repo.deleteCalls != 0 {
		t.Errorf("他人の投稿が削除された: %d 回", repo.deleteCalls)
	}
}

// 削除済みの再削除は 404。持ち主の判定より先に行う。
func TestDeleteAlreadyDeletedPost(t *testing.T) {
	post := storedPost()
	post.Status = domain.PostDeleted
	repo := &fakePostRepo{post: post, author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	if err := p.Delete(context.Background(), 1, 10); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
	if repo.deleteCalls != 0 {
		t.Errorf("削除済みなのに削除された: %d 回", repo.deleteCalls)
	}
}

// 削除済み投稿を他人が消そうとしたとき、403 ではなく 404 になること。
// 403 を返すと「その ID の投稿は存在した」と分かってしまう。
func TestDeleteAlreadyDeletedPostByOtherUser(t *testing.T) {
	post := storedPost()
	post.Status = domain.PostDeleted
	repo := &fakePostRepo{post: post, author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	if err := p.Delete(context.Background(), 1, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

func TestDeleteMissingPost(t *testing.T) {
	repo := &fakePostRepo{findErr: domain.ErrNotFound}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	if err := p.Delete(context.Background(), 999, 10); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

func TestGetMissingPost(t *testing.T) {
	repo := &fakePostRepo{findErr: domain.ErrNotFound}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

	if _, err := p.Get(context.Background(), 999, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("NOT_FOUND を期待したが %v", err)
	}
}

// 保存に失敗したらエラーを返す。
func TestCreatePostPropagatesRepositoryError(t *testing.T) {
	repo := &fakePostRepo{createErr: errors.New("保存できない")}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: teikei()}, newBlockRepo(), fixedClock())

	if _, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた会議のための会議かな",
	}); err == nil {
		t.Error("エラーにならない")
	}
}

// 時計を渡さなければ実時間を使うこと。
func TestNewPostDefaultsClock(t *testing.T) {
	repo := &fakePostRepo{post: storedPost(), author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), nil)

	before := time.Now()
	if err := p.Delete(context.Background(), 1, 10); err != nil {
		t.Fatalf("削除できない: %v", err)
	}
	if repo.deletedAt.Before(before) {
		t.Errorf("削除日時が実時間でない: %v", repo.deletedAt)
	}
}

// 付随情報の取得に失敗したら、投稿を返さずエラーにする。
// 部分的に欠けた応答を返すと、クライアントは真偽を判断できない。
func TestGetPostPropagatesLookupErrors(t *testing.T) {
	viewer := int64(99)

	t.Run("フォロー関係を確認できない", func(t *testing.T) {
		post := storedPost()
		post.Visibility = domain.VisibilityFollowers
		repo := &fakePostRepo{
			post: post, author: author(), followingErr: errors.New("確認できない"),
		}
		p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

		if _, err := p.Get(context.Background(), 1, &viewer); err == nil {
			t.Error("エラーにならない")
		}
	})

	t.Run("いいねを確認できない", func(t *testing.T) {
		repo := &fakePostRepo{
			post: storedPost(), author: author(), likedErr: errors.New("確認できない"),
		}
		p := usecase.NewPost(repo, &fakeAnalyzer{}, newBlockRepo(), fixedClock())

		if _, err := p.Get(context.Background(), 1, &viewer); err == nil {
			t.Error("エラーにならない")
		}
	})
}

// 正規化後の本文が上限を超える場合も、DB の制約に当てる前に弾くこと。
// 制約違反は 500 になり、利用者が直せるエラーとして返せない。
func TestCreatePostRejectsOverlongNormalizedText(t *testing.T) {
	long := strings.Repeat("あ", domain.BodyMaxLength+1)
	analysis := teikei()
	analysis.NormalizedText = long

	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: analysis}, newBlockRepo(), fixedClock())

	_, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた会議のための会議かな",
	})
	var appErr *domain.Error
	if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("VALIDATION_FAILED を期待したが %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("保存された: %d 回", repo.createCalls)
	}
}

// 句が3つ揃っていない判定結果は保存しない。
func TestCreatePostRejectsMalformedAnalysis(t *testing.T) {
	analysis := teikei()
	analysis.Segments = analysis.Segments[:2]

	repo := &fakePostRepo{}
	p := usecase.NewPost(repo, &fakeAnalyzer{result: analysis}, newBlockRepo(), fixedClock())

	if _, err := p.Create(context.Background(), usecase.CreateInput{
		Author: author(), Body: "今日もまた会議のための会議かな",
	}); err == nil {
		t.Error("エラーにならない")
	}
	if repo.createCalls != 0 {
		t.Errorf("保存された: %d 回", repo.createCalls)
	}
}

// BR-09: ブロックした相手の投稿は表示されない。
//
// **双方向で確認する。** 片方向だと、ブロックされた側は投稿を読み続けられ、
// 「見られたくない」という意図が満たされない。
func TestGetPostHiddenByBlock(t *testing.T) {
	viewer := int64(99)

	tests := map[string][2]int64{
		"閲覧者が投稿者をブロックしている": {99, 10},
		"投稿者が閲覧者をブロックしている": {10, 99},
	}

	for name, block := range tests {
		t.Run(name, func(t *testing.T) {
			blocks := newBlockRepo()
			blocks.blocks[block] = true
			repo := &fakePostRepo{post: storedPost(), author: author()}
			p := usecase.NewPost(repo, &fakeAnalyzer{}, blocks, fixedClock())

			if _, err := p.Get(context.Background(), 1, &viewer); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("NOT_FOUND を期待したが %v", err)
			}
		})
	}
}

// 未ログインはブロックの影響を受けない。
// 誰でもない相手をブロックすることはできない。
func TestGetPostWithoutLoginIgnoresBlocks(t *testing.T) {
	blocks := newBlockRepo()
	blocks.blocks[[2]int64{10, 99}] = true
	repo := &fakePostRepo{post: storedPost(), author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, blocks, fixedClock())

	if _, err := p.Get(context.Background(), 1, nil); err != nil {
		t.Errorf("未ログインで取得できない: %v", err)
	}
}

// ブロックを解除すると再び見える。
func TestGetPostVisibleAfterUnblock(t *testing.T) {
	viewer := int64(99)
	blocks := newBlockRepo()
	blocks.blocks[[2]int64{99, 10}] = true
	repo := &fakePostRepo{post: storedPost(), author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, blocks, fixedClock())

	if _, err := p.Get(context.Background(), 1, &viewer); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ブロック中に見えている: %v", err)
	}
	delete(blocks.blocks, [2]int64{99, 10})
	if _, err := p.Get(context.Background(), 1, &viewer); err != nil {
		t.Errorf("解除後に見えない: %v", err)
	}
}

// ブロックの確認に失敗したら投稿を返さない。
func TestGetPostPropagatesBlockLookupError(t *testing.T) {
	viewer := int64(99)
	blocks := newBlockRepo()
	blocks.err = errors.New("確認できない")
	repo := &fakePostRepo{post: storedPost(), author: author()}
	p := usecase.NewPost(repo, &fakeAnalyzer{}, blocks, fixedClock())

	if _, err := p.Get(context.Background(), 1, &viewer); err == nil {
		t.Error("エラーにならない")
	}
}
