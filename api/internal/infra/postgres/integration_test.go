package postgres_test

// 実際の PostgreSQL に対して実行する結合テスト。
//
// 単体テスト（usecase）はリポジトリをモックするため、
// **SQL の誤りや DB 制約の挙動は検出できない**。ここで確かめる。
//
// 接続先が無い環境ではスキップする。CI では postgres のサービスコンテナを
// 起動し、マイグレーションを適用したうえで実行する。
//
//	docker compose exec api go test ./internal/infra/postgres/... -v

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/infra/postgres"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("API_DATABASE_URL")
	if dsn == "" {
		t.Skip("API_DATABASE_URL が未設定のためスキップする")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("接続できない: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// cleanup は各テストの前後でデータを消す。
//
// users を消せば sessions は外部キーの連鎖削除で消える。
// **その連鎖が効いていること自体もテスト対象**であるため、
// sessions は明示的に消さない。
//
// posts は連鎖削除されない。投稿を持つ利用者を黙って消せないようにする
// 設計であり（posts_author_id_fkey に ON DELETE CASCADE が無い）、
// 退会時の投稿の削除はアプリケーション側の責務である（FR-01-04）。
// そのため posts を先に消す。
func cleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	// reports は posts を参照し、どちらも連鎖削除されない。先に消す。
	if _, err := pool.Exec(ctx, `DELETE FROM reports`); err != nil {
		t.Fatalf("後始末に失敗した: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM posts`); err != nil {
		t.Fatalf("後始末に失敗した: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users`); err != nil {
		t.Fatalf("後始末に失敗した: %v", err)
	}
}

func newUser(handle string) *domain.User {
	return &domain.User{
		Handle:       handle,
		Email:        handle + "@example.com",
		PasswordHash: "$2a$04$abcdefghijklmnopqrstuv",
		DisplayName:  handle,
		Status:       domain.UserActive,
	}
}

func TestUserRepositoryCreateAndFind(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	if created.ID == 0 {
		t.Error("ID が採番されていない")
	}

	byHandle, err := repo.FindByHandle(ctx, "alice")
	if err != nil {
		t.Fatalf("識別名で引けない: %v", err)
	}
	byID, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ID で引けない: %v", err)
	}
	if byHandle.ID != created.ID || byID.Handle != "alice" {
		t.Error("取得した利用者が違う")
	}
}

func TestUserRepositoryReturnsNotFound(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	repo := postgres.NewUserRepository(pool)

	// pgx の型を上位へ漏らさず、domain のエラーに変換していること
	if _, err := repo.FindByHandle(context.Background(), "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ErrNotFound を期待したが %v", err)
	}
	if _, err := repo.FindByID(context.Background(), 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ErrNotFound を期待したが %v", err)
	}
}

// 事前の重複確認をすり抜けても、DB の UNIQUE 制約が最後の砦になること。
func TestUserRepositoryTranslatesUniqueViolation(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()
	if _, err := repo.Create(ctx, newUser("bob")); err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	t.Run("識別名の重複", func(t *testing.T) {
		dup := newUser("bob")
		dup.Email = "other@example.com"
		if _, err := repo.Create(ctx, dup); !errors.Is(err, domain.ErrHandleTaken) {
			t.Errorf("HANDLE_TAKEN を期待したが %v", err)
		}
	})

	t.Run("メールアドレスの重複", func(t *testing.T) {
		dup := newUser("carol")
		dup.Email = "bob@example.com"
		if _, err := repo.Create(ctx, dup); !errors.Is(err, domain.ErrEmailTaken) {
			t.Errorf("EMAIL_TAKEN を期待したが %v", err)
		}
	})
}

func TestUserRepositoryExistsByEmail(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()
	if _, err := repo.Create(ctx, newUser("dave")); err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	exists, err := repo.ExistsByEmail(ctx, "dave@example.com")
	if err != nil || !exists {
		t.Errorf("登録済みと判定されない: exists=%v err=%v", exists, err)
	}
	exists, err = repo.ExistsByEmail(ctx, "nobody@example.com")
	if err != nil || exists {
		t.Errorf("未登録と判定されない: exists=%v err=%v", exists, err)
	}
}

func TestSessionRepository(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	sessions := postgres.NewSessionRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("erin"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	id, err := domain.NewSessionID()
	if err != nil {
		t.Fatalf("セッション ID を生成できない: %v", err)
	}
	// DB の CHAR(43) と一致すること。長さが違えば INSERT で落ちる。
	if len(id) != 43 {
		t.Fatalf("セッション ID の長さが 43 でない: %d", len(id))
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	session := &domain.Session{
		ID: id, UserID: user.ID,
		ExpiresAt: now.Add(domain.SessionLifetime), CreatedAt: now, LastAccessedAt: now,
	}
	if err := sessions.Create(ctx, session); err != nil {
		t.Fatalf("セッションを保存できない: %v", err)
	}

	t.Run("セッションと持ち主を1回で取れる", func(t *testing.T) {
		got, gotUser, err := sessions.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if got.UserID != user.ID || gotUser.Handle != "erin" {
			t.Error("取得した内容が違う")
		}
	})

	t.Run("スライディング期限で延長できる", func(t *testing.T) {
		extended := now.Add(48 * time.Hour)
		if err := sessions.Touch(ctx, id, now, extended); err != nil {
			t.Fatalf("更新できない: %v", err)
		}
		got, _, err := sessions.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if !got.ExpiresAt.UTC().Equal(extended) {
			t.Errorf("有効期限が延びていない: %v", got.ExpiresAt)
		}
	})

	t.Run("存在しないセッションは NotFound", func(t *testing.T) {
		if _, _, err := sessions.FindByID(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("ErrNotFound を期待したが %v", err)
		}
	})

	t.Run("削除は冪等", func(t *testing.T) {
		if err := sessions.Delete(ctx, id); err != nil {
			t.Fatalf("削除できない: %v", err)
		}
		// 2回目も落ちない。ログアウトの二重送信で 500 を返す理由がない。
		if err := sessions.Delete(ctx, id); err != nil {
			t.Errorf("2回目の削除で失敗した: %v", err)
		}
	})
}

// 利用停止・退会でセッションが消えること。
// **DB の連鎖削除で消える**ことを確かめる（アプリ側の削除漏れがあっても残らない）。
func TestSessionsAreDeletedWhenUserIsDeleted(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	sessions := postgres.NewSessionRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("frank"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	now := time.Now()
	for range 3 {
		id, err := domain.NewSessionID()
		if err != nil {
			t.Fatalf("セッション ID を生成できない: %v", err)
		}
		if err := sessions.Create(ctx, &domain.Session{
			ID: id, UserID: user.ID,
			ExpiresAt: now.Add(domain.SessionLifetime), CreatedAt: now, LastAccessedAt: now,
		}); err != nil {
			t.Fatalf("セッションを保存できない: %v", err)
		}
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("利用者を削除できない: %v", err)
	}

	var remaining int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM sessions WHERE user_id = $1`, user.ID).Scan(&remaining); err != nil {
		t.Fatalf("件数を取得できない: %v", err)
	}
	if remaining != 0 {
		t.Errorf("セッションが %d 件残っている", remaining)
	}
}

func TestSessionRepositoryDeleteByUserIDAndExpired(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	sessions := postgres.NewSessionRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("grace"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	now := time.Now()

	live, _ := domain.NewSessionID()
	expired, _ := domain.NewSessionID()
	for id, exp := range map[string]time.Time{
		live:    now.Add(domain.SessionLifetime),
		expired: now.Add(-time.Hour),
	} {
		if err := sessions.Create(ctx, &domain.Session{
			ID: id, UserID: user.ID, ExpiresAt: exp, CreatedAt: now, LastAccessedAt: now,
		}); err != nil {
			t.Fatalf("セッションを保存できない: %v", err)
		}
	}

	t.Run("期限切れだけを消す", func(t *testing.T) {
		deleted, err := sessions.DeleteExpired(ctx, now)
		if err != nil {
			t.Fatalf("削除できない: %v", err)
		}
		if deleted != 1 {
			t.Errorf("削除件数が違う: %d", deleted)
		}
		if _, _, err := sessions.FindByID(ctx, live); err != nil {
			t.Errorf("有効なセッションまで消えている: %v", err)
		}
	})

	t.Run("利用者のセッションを一括で消す", func(t *testing.T) {
		if err := sessions.DeleteByUserID(ctx, user.ID); err != nil {
			t.Fatalf("削除できない: %v", err)
		}
		if _, _, err := sessions.FindByID(ctx, live); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("セッションが残っている: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 投稿
// ---------------------------------------------------------------------------

func newPost(authorID int64) *domain.Post {
	return &domain.Post{
		AuthorID:   authorID,
		Body:       "今日もまた会議のための会議かな",
		Reading:    "キョウモマタカイギノタメノカイギカナ",
		Verdict:    domain.VerdictTeikei,
		Break1:     5,
		Break2:     11,
		MoraKami:   5,
		MoraNaka:   7,
		MoraShimo:  5,
		Visibility: domain.VisibilityPublic,
		Status:     domain.PostPublished,
	}
}

func TestPostRepositoryCreateAndFind(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("poet"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	created, err := posts.Create(ctx, newPost(user.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}
	if created.ID == 0 {
		t.Error("ID が採番されていない")
	}
	if created.LikeCount != 0 {
		t.Errorf("いいね数の既定値が 0 でない: %d", created.LikeCount)
	}
	if created.CreatedAt.IsZero() {
		t.Error("作成日時が入っていない")
	}

	got, author, err := posts.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	if got.Body != created.Body || got.Verdict != domain.VerdictTeikei {
		t.Errorf("取得内容が違う: %+v", got)
	}
	// 投稿者を1回のクエリで取れること。
	if author.Handle != "poet" {
		t.Errorf("投稿者が違う: %+v", author)
	}
	// 区切りで本文を3句に戻せること。
	if segs := got.Segments(); segs[0] != "今日もまた" || segs[1] != "会議のための" || segs[2] != "会議かな" {
		t.Errorf("本文を3句に戻せない: %v", segs)
	}
}

// DB の CHECK 制約が、判定ロジックの誤りを最後に止めること。
// アプリケーション側の検証だけに頼ると、バグでデータが汚染される。
func TestPostRepositoryRejectsInvalidJudgement(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("guard"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	tests := map[string]func(*domain.Post){
		"上五が7モーラ":  func(p *domain.Post) { p.MoraKami = 7 },
		"中七が5モーラ":  func(p *domain.Post) { p.MoraNaka = 5 },
		"下五が9モーラ":  func(p *domain.Post) { p.MoraShimo = 9 },
		"破調":       func(p *domain.Post) { p.Verdict = domain.VerdictHacho },
		"区切りの順序が逆": func(p *domain.Post) { p.Break1, p.Break2 = 11, 5 },
		"区切りが本文の外": func(p *domain.Post) { p.Break2 = 99 },
		"公開範囲が不正":  func(p *domain.Post) { p.Visibility = "secret" },
		"状態が不正":    func(p *domain.Post) { p.Status = "draft" },
	}

	for name, corrupt := range tests {
		t.Run(name, func(t *testing.T) {
			post := newPost(user.ID)
			corrupt(post)
			if _, err := posts.Create(ctx, post); err == nil {
				t.Error("不正な投稿が保存された")
			}
		})
	}
}

func TestPostRepositoryDelete(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("deleter"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	created, err := posts.Create(ctx, newPost(user.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	deletedAt := time.Now().UTC().Truncate(time.Millisecond)
	if err := posts.Delete(ctx, created.ID, deletedAt); err != nil {
		t.Fatalf("削除できない: %v", err)
	}

	t.Run("行は残り、状態と日時が同時に変わる", func(t *testing.T) {
		got, _, err := posts.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if got.Status != domain.PostDeleted {
			t.Errorf("状態が deleted でない: %v", got.Status)
		}
		if got.DeletedAt == nil || !got.DeletedAt.UTC().Equal(deletedAt) {
			t.Errorf("削除日時が入っていない: %v", got.DeletedAt)
		}
	})

	t.Run("2回目の削除は NotFound", func(t *testing.T) {
		// 更新すると削除日時が上書きされ、いつ削除されたか分からなくなる。
		if err := posts.Delete(ctx, created.ID, time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("ErrNotFound を期待したが %v", err)
		}
	})
}

func TestPostRepositoryReturnsNotFound(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	if _, _, err := posts.FindByID(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ErrNotFound を期待したが %v", err)
	}
	if err := posts.Delete(ctx, 999999, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ErrNotFound を期待したが %v", err)
	}
}

// 公開範囲の判定に使うフォロー関係を引けること。
func TestPostRepositoryIsFollowing(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	follower, err := users.Create(ctx, newUser("follower"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	following, err := posts.IsFollowing(ctx, follower.ID, author.ID)
	if err != nil || following {
		t.Errorf("フォローしていないのに true: %v %v", following, err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`,
		follower.ID, author.ID); err != nil {
		t.Fatalf("フォローを作れない: %v", err)
	}

	following, err = posts.IsFollowing(ctx, follower.ID, author.ID)
	if err != nil || !following {
		t.Errorf("フォローしているのに false: %v %v", following, err)
	}
	// 向きが逆のフォローを拾わないこと。
	reverse, err := posts.IsFollowing(ctx, author.ID, follower.ID)
	if err != nil || reverse {
		t.Errorf("向きが逆のフォローを拾っている: %v %v", reverse, err)
	}
}

func TestPostRepositoryIsLikedBy(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("liker"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(user.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	liked, err := posts.IsLikedBy(ctx, post.ID, user.ID)
	if err != nil || liked {
		t.Errorf("いいねしていないのに true: %v %v", liked, err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO likes (user_id, post_id) VALUES ($1, $2)`, user.ID, post.ID); err != nil {
		t.Fatalf("いいねを作れない: %v", err)
	}

	liked, err = posts.IsLikedBy(ctx, post.ID, user.ID)
	if err != nil || !liked {
		t.Errorf("いいねしているのに false: %v %v", liked, err)
	}
}

// 本文の上限が DB の列定義と一致すること。
// アプリ側の上限が大きいと、検証を通った本文が DB で 500 になる。
func TestPostBodyColumnMatchesDomainLimit(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	var maxLength int
	err := pool.QueryRow(context.Background(), `
		SELECT character_maximum_length FROM information_schema.columns
		WHERE table_name = 'posts' AND column_name = 'body'`).Scan(&maxLength)
	if err != nil {
		t.Fatalf("列定義を取得できない: %v", err)
	}
	if maxLength != domain.BodyMaxLength {
		t.Errorf("posts.body は VARCHAR(%d) だが domain.BodyMaxLength は %d",
			maxLength, domain.BodyMaxLength)
	}
}

// ---------------------------------------------------------------------------
// フォロー
// ---------------------------------------------------------------------------

func TestFollowRepository(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	bob, err := users.Create(ctx, newUser("bob"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	t.Run("フォローして確認できる", func(t *testing.T) {
		if err := follows.Follow(ctx, alice.ID, bob.ID); err != nil {
			t.Fatalf("フォローできない: %v", err)
		}
		following, err := follows.IsFollowing(ctx, alice.ID, bob.ID)
		if err != nil || !following {
			t.Errorf("フォロー関係が無い: %v %v", following, err)
		}
		// 向きが逆の関係を拾わないこと。
		reverse, err := follows.IsFollowing(ctx, bob.ID, alice.ID)
		if err != nil || reverse {
			t.Errorf("向きが逆の関係を拾っている: %v %v", reverse, err)
		}
	})

	t.Run("二重フォローが主キー違反にならない", func(t *testing.T) {
		// ON CONFLICT DO NOTHING で冪等にしている。
		// 事前確認してから INSERT する実装だと、ここで 500 になる。
		for range 3 {
			if err := follows.Follow(ctx, alice.ID, bob.ID); err != nil {
				t.Fatalf("2回目以降のフォローで失敗した: %v", err)
			}
		}
		count, err := follows.CountFollowers(ctx, bob.ID)
		if err != nil || count != 1 {
			t.Errorf("フォロワー数が違う: %d %v", count, err)
		}
	})

	t.Run("解除は冪等", func(t *testing.T) {
		if err := follows.Unfollow(ctx, alice.ID, bob.ID); err != nil {
			t.Fatalf("解除できない: %v", err)
		}
		// 2回目も落ちない。リトライの二重送信で 500 を返す理由がない。
		if err := follows.Unfollow(ctx, alice.ID, bob.ID); err != nil {
			t.Errorf("2回目の解除で失敗した: %v", err)
		}
		following, err := follows.IsFollowing(ctx, alice.ID, bob.ID)
		if err != nil || following {
			t.Errorf("関係が残っている: %v %v", following, err)
		}
	})
}

// BR-05: 自分自身をフォローできない。
// アプリケーション側の検証をすり抜けても DB が最後の砦になる。
func TestFollowRepositoryRejectsSelfFollow(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("solo"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	if err := follows.Follow(ctx, user.ID, user.ID); err == nil {
		t.Error("自分自身をフォローできてしまった")
	}
}

func TestFollowRepositoryCountFollowers(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	ctx := context.Background()

	star, err := users.Create(ctx, newUser("star"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	count, err := follows.CountFollowers(ctx, star.ID)
	if err != nil || count != 0 {
		t.Errorf("初期値が 0 でない: %d %v", count, err)
	}

	for _, handle := range []string{"fan1", "fan2", "fan3"} {
		fan, err := users.Create(ctx, newUser(handle))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		if err := follows.Follow(ctx, fan.ID, star.ID); err != nil {
			t.Fatalf("フォローできない: %v", err)
		}
	}

	count, err = follows.CountFollowers(ctx, star.ID)
	if err != nil || count != 3 {
		t.Errorf("フォロワー数が違う: %d %v", count, err)
	}
	// フォロー中の数を数えていないこと（向きの取り違え）。
	fanCount, err := follows.CountFollowers(ctx, star.ID)
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	var followingCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM follows WHERE follower_id = $1`, star.ID).Scan(&followingCount); err != nil {
		t.Fatalf("件数を取得できない: %v", err)
	}
	if fanCount == followingCount && fanCount != 0 {
		t.Errorf("フォロワー数とフォロー中の数を取り違えている可能性がある: %d", fanCount)
	}
}

func TestFollowRepositoryIsBlocked(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	ctx := context.Background()

	blocker, err := users.Create(ctx, newUser("blocker"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	blocked, err := users.Create(ctx, newUser("blocked"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	if got, err := follows.IsBlocked(ctx, blocker.ID, blocked.ID); err != nil || got {
		t.Errorf("ブロックしていないのに true: %v %v", got, err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)`,
		blocker.ID, blocked.ID); err != nil {
		t.Fatalf("ブロックを作れない: %v", err)
	}

	if got, err := follows.IsBlocked(ctx, blocker.ID, blocked.ID); err != nil || !got {
		t.Errorf("ブロックしているのに false: %v %v", got, err)
	}
	// 向きが逆のブロックを拾わないこと。
	// 取り違えると、ブロックされた側の操作を誤って許してしまう。
	if got, err := follows.IsBlocked(ctx, blocked.ID, blocker.ID); err != nil || got {
		t.Errorf("向きが逆のブロックを拾っている: %v %v", got, err)
	}
}

// ---------------------------------------------------------------------------
// 通報・ブロック
// ---------------------------------------------------------------------------

// BR-08: ブロックするとフォロー関係が双方向に解除される。
//
// **1トランザクションで行われること**を確かめる。分けて実行すると
// 「ブロックはできたがフォローが残る」状態が生じ、ブロックしたのに
// 相手のタイムラインへ自分の投稿が流れ続ける。
func TestBlockRepositoryRemovesFollowsBothWays(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	bob, err := users.Create(ctx, newUser("bob"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	// 相互フォローの状態を作る。
	if err := follows.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("フォローできない: %v", err)
	}
	if err := follows.Follow(ctx, bob.ID, alice.ID); err != nil {
		t.Fatalf("フォローできない: %v", err)
	}

	if err := blocks.Block(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("ブロックできない: %v", err)
	}

	for _, c := range []struct {
		name     string
		from, to int64
	}{
		{"ブロックした側 → された側", alice.ID, bob.ID},
		{"された側 → ブロックした側", bob.ID, alice.ID},
	} {
		following, err := follows.IsFollowing(ctx, c.from, c.to)
		if err != nil {
			t.Fatalf("確認できない: %v", err)
		}
		if following {
			t.Errorf("%s のフォローが残っている", c.name)
		}
	}

	blocked, err := blocks.IsBlocked(ctx, alice.ID, bob.ID)
	if err != nil || !blocked {
		t.Errorf("ブロックが作られていない: %v %v", blocked, err)
	}
}

func TestBlockRepositoryIsIdempotent(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	bob, err := users.Create(ctx, newUser("bob"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	// ON CONFLICT DO NOTHING で冪等にしている。
	for range 3 {
		if err := blocks.Block(ctx, alice.ID, bob.ID); err != nil {
			t.Fatalf("2回目以降のブロックで失敗した: %v", err)
		}
	}
	// 解除も冪等。
	for range 2 {
		if err := blocks.Unblock(ctx, alice.ID, bob.ID); err != nil {
			t.Fatalf("2回目の解除で失敗した: %v", err)
		}
	}
	blocked, err := blocks.IsBlocked(ctx, alice.ID, bob.ID)
	if err != nil || blocked {
		t.Errorf("ブロックが残っている: %v %v", blocked, err)
	}
}

// 解除でフォロー関係は復活しない。
func TestBlockRepositoryUnblockDoesNotRestoreFollows(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	bob, err := users.Create(ctx, newUser("bob"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	if err := follows.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("フォローできない: %v", err)
	}
	if err := blocks.Block(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("ブロックできない: %v", err)
	}
	if err := blocks.Unblock(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("解除できない: %v", err)
	}

	following, err := follows.IsFollowing(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("確認できない: %v", err)
	}
	if following {
		t.Error("フォロー関係が復活している")
	}
}

// BR-06: 自分自身をブロックできない（DB の CHECK 制約）。
func TestBlockRepositoryRejectsSelfBlock(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, newUser("solo"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	if err := blocks.Block(ctx, user.ID, user.ID); err == nil {
		t.Error("自分自身をブロックできてしまった")
	}
}

// 双方向の判定が、どちらの向きのブロックも拾うこと。
func TestBlockRepositoryIsBlockedEitherWay(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	bob, err := users.Create(ctx, newUser("bob"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	if got, err := blocks.IsBlockedEitherWay(ctx, alice.ID, bob.ID); err != nil || got {
		t.Errorf("ブロックが無いのに true: %v %v", got, err)
	}

	if err := blocks.Block(ctx, bob.ID, alice.ID); err != nil {
		t.Fatalf("ブロックできない: %v", err)
	}
	// 引数の順序を入れ替えても拾うこと。
	for _, pair := range [][2]int64{{alice.ID, bob.ID}, {bob.ID, alice.ID}} {
		got, err := blocks.IsBlockedEitherWay(ctx, pair[0], pair[1])
		if err != nil || !got {
			t.Errorf("向き %v を拾えていない: %v %v", pair, got, err)
		}
	}
}

func TestReportRepository(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	reports := postgres.NewReportRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	reporter, err := users.Create(ctx, newUser("reporter"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(author.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	t.Run("通報できる", func(t *testing.T) {
		report, err := reports.Create(ctx, &domain.Report{
			ReporterID: reporter.ID, PostID: post.ID,
			Reason: domain.ReportSpam, Comment: "宣伝です", Status: domain.ReportPending,
		})
		if err != nil {
			t.Fatalf("通報できない: %v", err)
		}
		if report.ID == 0 || report.Status != domain.ReportPending {
			t.Errorf("通報の内容が違う: %+v", report)
		}
		if report.Comment != "宣伝です" {
			t.Errorf("コメントが保存されない: %q", report.Comment)
		}
	})

	t.Run("重複通報は ALREADY_REPORTED", func(t *testing.T) {
		// 事前確認ではなく UNIQUE 制約で防いでいる。
		_, err := reports.Create(ctx, &domain.Report{
			ReporterID: reporter.ID, PostID: post.ID,
			Reason: domain.ReportHarassment, Status: domain.ReportPending,
		})
		if !errors.Is(err, domain.ErrAlreadyReported) {
			t.Errorf("ALREADY_REPORTED を期待したが %v", err)
		}
	})

	t.Run("別の利用者は通報できる", func(t *testing.T) {
		other, err := users.Create(ctx, newUser("other"))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		if _, err := reports.Create(ctx, &domain.Report{
			ReporterID: other.ID, PostID: post.ID,
			Reason: domain.ReportSpam, Status: domain.ReportPending,
		}); err != nil {
			t.Errorf("別の利用者が通報できない: %v", err)
		}
	})

	t.Run("コメント無しでも通報できる", func(t *testing.T) {
		another, err := users.Create(ctx, newUser("another"))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		report, err := reports.Create(ctx, &domain.Report{
			ReporterID: another.ID, PostID: post.ID,
			Reason: domain.ReportOther, Status: domain.ReportPending,
		})
		if err != nil {
			t.Fatalf("通報できない: %v", err)
		}
		if report.Comment != "" {
			t.Errorf("コメントが空でない: %q", report.Comment)
		}
	})
}

// DB の CHECK 制約が不正な通報を拒否すること。
func TestReportRepositoryRejectsInvalidReport(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	reports := postgres.NewReportRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	reporter, err := users.Create(ctx, newUser("reporter"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(author.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	tests := map[string]*domain.Report{
		"理由が不正": {
			ReporterID: reporter.ID, PostID: post.ID,
			Reason: "whatever", Status: domain.ReportPending,
		},
		"状態が不正": {
			ReporterID: reporter.ID, PostID: post.ID,
			Reason: domain.ReportSpam, Status: "done",
		},
		"存在しない投稿": {
			ReporterID: reporter.ID, PostID: 999999,
			Reason: domain.ReportSpam, Status: domain.ReportPending,
		},
	}

	for name, report := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := reports.Create(ctx, report); err == nil {
				t.Error("不正な通報が保存された")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// タイムライン
// ---------------------------------------------------------------------------

// timelineIDs は取得結果の投稿 ID を返す。
func timelineIDs(items []domain.TimelineItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Post.ID)
	}
	return ids
}

// seedPost は指定の公開範囲で投稿を1件作る。
func seedPost(t *testing.T, posts *postgres.PostRepository, authorID int64, v domain.Visibility) int64 {
	t.Helper()
	post := newPost(authorID)
	post.Visibility = v
	created, err := posts.Create(context.Background(), post)
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}
	return created.ID
}

func TestTimelineRepositoryPublic(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	first := seedPost(t, posts, alice.ID, domain.VisibilityPublic)
	second := seedPost(t, posts, alice.ID, domain.VisibilityPublic)
	limited := seedPost(t, posts, alice.ID, domain.VisibilityFollowers)
	deleted := seedPost(t, posts, alice.ID, domain.VisibilityPublic)
	if err := posts.Delete(ctx, deleted, time.Now()); err != nil {
		t.Fatalf("削除できない: %v", err)
	}

	items, err := timelines.Public(ctx, domain.TimelineQuery{})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	got := timelineIDs(items)

	// 新しい順。followers 限定と削除済みは含まれない。
	want := []int64{second, first}
	if len(got) != len(want) {
		t.Fatalf("件数が違う: %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d 件目が違う: %d, want %d", i+1, got[i], want[i])
		}
	}
	for _, id := range got {
		if id == limited || id == deleted {
			t.Errorf("含まれてはいけない投稿がある: %d", id)
		}
	}
	// 投稿者を1回のクエリで取れていること。
	if items[0].Author.Handle != "alice" {
		t.Errorf("投稿者が違う: %+v", items[0].Author)
	}
}

// カーソルで重複も欠落もなく全件たどれること。
func TestTimelineRepositoryCursorPagination(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	created := []int64{}
	for range 5 {
		created = append(created, seedPost(t, posts, alice.ID, domain.VisibilityPublic))
	}

	limit := 2
	seen := []int64{}
	cursor := int64(0)
	for range 5 {
		items, err := timelines.Public(ctx, domain.TimelineQuery{Cursor: cursor, Limit: &limit})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if len(items) == 0 {
			break
		}
		seen = append(seen, timelineIDs(items)...)
		cursor = items[len(items)-1].Post.ID
	}

	if len(seen) != len(created) {
		t.Fatalf("件数が違う: %d, want %d", len(seen), len(created))
	}
	unique := map[int64]bool{}
	for i, id := range seen {
		if unique[id] {
			t.Errorf("重複している: %d", id)
		}
		unique[id] = true
		if i > 0 && seen[i-1] <= id {
			t.Errorf("降順になっていない: %v", seen)
		}
	}
}

// BR-09: ブロック関係にある投稿が双方向で除外されること。
func TestTimelineRepositoryExcludesBlocked(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	bob, err := users.Create(ctx, newUser("bob"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	alicePost := seedPost(t, posts, alice.ID, domain.VisibilityPublic)
	bobPost := seedPost(t, posts, bob.ID, domain.VisibilityPublic)

	if err := blocks.Block(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("ブロックできない: %v", err)
	}

	t.Run("ブロックした側から見えない", func(t *testing.T) {
		items, err := timelines.Public(ctx, domain.TimelineQuery{ViewerID: &alice.ID})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		for _, id := range timelineIDs(items) {
			if id == bobPost {
				t.Error("ブロックした相手の投稿が含まれている")
			}
		}
	})

	t.Run("ブロックされた側からも見えない", func(t *testing.T) {
		items, err := timelines.Public(ctx, domain.TimelineQuery{ViewerID: &bob.ID})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		for _, id := range timelineIDs(items) {
			if id == alicePost {
				t.Error("ブロックした側の投稿が含まれている")
			}
		}
	})

	t.Run("未ログインは影響を受けない", func(t *testing.T) {
		items, err := timelines.Public(ctx, domain.TimelineQuery{})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if len(timelineIDs(items)) != 2 {
			t.Errorf("未ログインで除外されている: %v", timelineIDs(items))
		}
	})
}

func TestTimelineRepositoryHome(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	follows := postgres.NewFollowRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)
	ctx := context.Background()

	me, err := users.Create(ctx, newUser("me"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	followee, err := users.Create(ctx, newUser("followee"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	stranger, err := users.Create(ctx, newUser("stranger"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	followeePublic := seedPost(t, posts, followee.ID, domain.VisibilityPublic)
	followeeLimited := seedPost(t, posts, followee.ID, domain.VisibilityFollowers)
	strangerPost := seedPost(t, posts, stranger.ID, domain.VisibilityPublic)
	myPost := seedPost(t, posts, me.ID, domain.VisibilityPublic)

	if err := follows.Follow(ctx, me.ID, followee.ID); err != nil {
		t.Fatalf("フォローできない: %v", err)
	}

	items, err := timelines.Home(ctx, domain.TimelineQuery{ViewerID: &me.ID})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	got := map[int64]bool{}
	for _, id := range timelineIDs(items) {
		got[id] = true
	}

	// フォロイーの投稿は followers 限定も含めて出る。
	if !got[followeePublic] || !got[followeeLimited] {
		t.Errorf("フォロイーの投稿が欠けている: %v", timelineIDs(items))
	}
	// フォローしていない相手と自分の投稿は出ない（BR-05 で自分はフォローできない）。
	if got[strangerPost] {
		t.Error("フォローしていない相手の投稿が含まれている")
	}
	if got[myPost] {
		t.Error("自分の投稿が含まれている")
	}

	t.Run("フォローを外すと消える", func(t *testing.T) {
		if err := follows.Unfollow(ctx, me.ID, followee.ID); err != nil {
			t.Fatalf("解除できない: %v", err)
		}
		items, err := timelines.Home(ctx, domain.TimelineQuery{ViewerID: &me.ID})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("解除後も投稿が残っている: %v", timelineIDs(items))
		}
	})
}

// liked_by_me が1クエリで取れていること。
func TestTimelineRepositoryLikedByMe(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, newUser("alice"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	liked := seedPost(t, posts, alice.ID, domain.VisibilityPublic)
	notLiked := seedPost(t, posts, alice.ID, domain.VisibilityPublic)

	if _, err := pool.Exec(ctx,
		`INSERT INTO likes (user_id, post_id) VALUES ($1, $2)`, alice.ID, liked); err != nil {
		t.Fatalf("いいねを作れない: %v", err)
	}

	items, err := timelines.Public(ctx, domain.TimelineQuery{ViewerID: &alice.ID})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	for _, item := range items {
		want := item.Post.ID == liked
		if item.LikedByMe != want {
			t.Errorf("投稿 %d の liked_by_me が %v", item.Post.ID, item.LikedByMe)
		}
	}
	if len(items) != 2 {
		t.Fatalf("件数が違う: %d", len(items))
	}
	_ = notLiked

	t.Run("未ログインは常に false", func(t *testing.T) {
		items, err := timelines.Public(ctx, domain.TimelineQuery{})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		for _, item := range items {
			if item.LikedByMe {
				t.Errorf("未ログインなのに liked_by_me が true: %d", item.Post.ID)
			}
		}
	})
}

// 実行計画に posts の Seq Scan が出ないこと。
//
// **行数を増やしてから確認する。** 数行しかないテーブルでは、
// PostgreSQL は正しくインデックスより Seq Scan を選ぶ。少量のまま検査すると、
// 「インデックスを使えないクエリ」と「使う必要がないほど小さいテーブル」を
// 区別できない。
//
// **実物のクエリを検査する。** テストにクエリを書き写すと、実装を変えたときに
// 古いクエリを検査し続ける（#41 で実際に起きた）。
//
// 規模を伴う測定（10万行・実測時間）は docs/perf/0002 で行う。
// ここで固定するのは**クエリの形がインデックスを使える形であること**である。
func TestTimelineQueryUsesIndex(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	ctx := context.Background()
	users := postgres.NewUserRepository(pool)

	// フォロー先を複数用意する。1人だけだと LATERAL が展開され、
	// 実運用とは違う計画になる。
	authors := make([]int64, 0, 20)
	for i := range 20 {
		u, err := users.Create(ctx, newUser(fmt.Sprintf("author%d", i)))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		authors = append(authors, u.ID)
	}
	me, err := users.Create(ctx, newUser("me"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	// プランナがインデックスを選ぶだけの行数を入れる。
	for _, id := range authors {
		if _, err := pool.Exec(ctx, `
			INSERT INTO posts (author_id, body, reading, verdict,
			                   break1, break2, mora_kami, mora_naka, mora_shimo,
			                   visibility, status)
			SELECT $1, '今日もまた会議のための会議かな', 'キョウモマタカイギノタメノカイギカナ',
			       'teikei', 5, 11, 5, 7, 5, 'public', 'published'
			FROM generate_series(1, 500)`, id); err != nil {
			t.Fatalf("投稿を投入できない: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`,
			me.ID, id); err != nil {
			t.Fatalf("フォローできない: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `VACUUM ANALYZE posts, follows, users, likes, blocks`); err != nil {
		t.Fatalf("統計を更新できない: %v", err)
	}

	limit := 20
	tests := map[string]struct {
		query     string
		wantIndex string
	}{
		// 述語が全行に当てはまる状況ではプランナが主キーの逆順スキャンを
		// 選ぶこともあるため、全体タイムラインはインデックス名を求めない。
		"全体タイムライン": {postgres.PublicTimelineQueryForTest, ""},
		// フォロー中はフォロー先ごとに辿るため、#7 が使われるはずである。
		"フォロー中タイムライン": {postgres.HomeTimelineQueryForTest, "posts_author_timeline_idx"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rows, err := pool.Query(ctx, "EXPLAIN "+tt.query, me.ID, int64(0), limit)
			if err != nil {
				t.Fatalf("実行計画を取得できない: %v", err)
			}
			defer rows.Close()

			var lines []string
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					t.Fatalf("読み取れない: %v", err)
				}
				lines = append(lines, line)
			}
			plan := strings.Join(lines, "\n")

			if strings.Contains(plan, "Seq Scan on posts") {
				t.Errorf("posts に Seq Scan が出ている:\n%s", plan)
			}
			if tt.wantIndex != "" && !strings.Contains(plan, tt.wantIndex) {
				t.Errorf("%s が使われていない:\n%s", tt.wantIndex, plan)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// いいね
// ---------------------------------------------------------------------------

// likeCountOf は posts.like_count を読む。
func likeCountOf(t *testing.T, pool *pgxpool.Pool, postID int64) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT like_count FROM posts WHERE id = $1`, postID).Scan(&count); err != nil {
		t.Fatalf("いいね数を取得できない: %v", err)
	}
	return count
}

// assertLikeCountMatches は like_count と likes の実数が一致することを確かめる。
//
// 非正規化した値がずれていないことを、テストのたびに固定する
// （基本設計 03 §4 が代償として挙げた「ずれ」の検出）。
func assertLikeCountMatches(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT p.id, p.like_count, count(l.*)
		FROM posts p LEFT JOIN likes l ON l.post_id = p.id
		GROUP BY p.id, p.like_count
		HAVING p.like_count <> count(l.*)`)
	if err != nil {
		t.Fatalf("突合できない: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var stored, actual int
		if err := rows.Scan(&id, &stored, &actual); err != nil {
			t.Fatalf("読み取れない: %v", err)
		}
		t.Errorf("投稿 %d の like_count が %d、実数が %d でずれている", id, stored, actual)
	}
}

func TestLikeRepository(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	likes := postgres.NewLikeRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	liker, err := users.Create(ctx, newUser("liker"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(author.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	t.Run("いいねで件数が1増える", func(t *testing.T) {
		count, err := likes.Like(ctx, post.ID, liker.ID)
		if err != nil {
			t.Fatalf("いいねできない: %v", err)
		}
		if count != 1 || likeCountOf(t, pool, post.ID) != 1 {
			t.Errorf("件数が違う: 戻り値=%d DB=%d", count, likeCountOf(t, pool, post.ID))
		}
	})

	t.Run("二重いいねで件数が増えない", func(t *testing.T) {
		// ON CONFLICT DO NOTHING の影響行数で分岐している。
		// 分岐しないと、連打するだけで件数が増える。
		for range 3 {
			count, err := likes.Like(ctx, post.ID, liker.ID)
			if err != nil {
				t.Fatalf("いいねできない: %v", err)
			}
			if count != 1 {
				t.Fatalf("件数が増えている: %d", count)
			}
		}
	})

	t.Run("取り消しで件数が1減る", func(t *testing.T) {
		count, err := likes.Unlike(ctx, post.ID, liker.ID)
		if err != nil {
			t.Fatalf("取り消せない: %v", err)
		}
		if count != 0 || likeCountOf(t, pool, post.ID) != 0 {
			t.Errorf("件数が違う: %d", count)
		}
	})

	t.Run("いいねしていない取り消しで件数が減らない", func(t *testing.T) {
		for range 3 {
			count, err := likes.Unlike(ctx, post.ID, liker.ID)
			if err != nil {
				t.Fatalf("取り消せない: %v", err)
			}
			if count != 0 {
				t.Fatalf("件数が負に振れた: %d", count)
			}
		}
	})

	assertLikeCountMatches(t, pool)
}

// **同時にいいねしても件数が失われないこと。**
//
// read-modify-write で実装すると、同時に2人がいいねしたときに片方が消える
// （基本設計 03 §4）。実装を読んで正しそうに見えても、
// 同時実行の失敗は目視では見つからない。
func TestLikeRepositoryIsAtomicUnderConcurrency(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	likes := postgres.NewLikeRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(author.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	const likers = 50
	ids := make([]int64, 0, likers)
	for i := range likers {
		u, err := users.Create(ctx, newUser(fmt.Sprintf("liker%d", i)))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		ids = append(ids, u.ID)
	}

	var wg sync.WaitGroup
	errs := make(chan error, likers)
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := likes.Like(ctx, post.ID, id); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("同時いいねで失敗した: %v", err)
	}

	if got := likeCountOf(t, pool, post.ID); got != likers {
		t.Errorf("いいねが失われている: like_count=%d, want %d", got, likers)
	}
	assertLikeCountMatches(t, pool)

	t.Run("同時に取り消しても失われない", func(t *testing.T) {
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = likes.Unlike(ctx, post.ID, id)
			}()
		}
		wg.Wait()

		if got := likeCountOf(t, pool, post.ID); got != 0 {
			t.Errorf("件数が 0 にならない: %d", got)
		}
		assertLikeCountMatches(t, pool)
	})
}

// 同じ利用者が同時に連打しても件数が1を超えないこと。
func TestLikeRepositoryIgnoresDuplicateUnderConcurrency(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	likes := postgres.NewLikeRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	liker, err := users.Create(ctx, newUser("liker"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(author.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = likes.Like(ctx, post.ID, liker.ID)
		}()
	}
	wg.Wait()

	if got := likeCountOf(t, pool, post.ID); got != 1 {
		t.Errorf("連打で件数が増えた: %d", got)
	}
	assertLikeCountMatches(t, pool)
}

// like_count が負にならないこと（DB の CHECK 制約）。
func TestPostsRejectNegativeLikeCount(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	ctx := context.Background()

	author, err := users.Create(ctx, newUser("author"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	post, err := posts.Create(ctx, newPost(author.ID))
	if err != nil {
		t.Fatalf("投稿できない: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE posts SET like_count = like_count - 1 WHERE id = $1`, post.ID); err == nil {
		t.Error("いいね数が負になった")
	}
}

// LATERAL 版が素朴な JOIN 版と同じ結果を返すこと（#41）。
//
// **基準となるクエリをテスト側に置く。** 実装の写しではなく、
// 「フォロー先の投稿を id 降順に並べて上から取る」という定義そのものを書く。
// 実装がどんな計画を選んでも、結果はこれと一致しなければならない。
const referenceHomeTimeline = `
	SELECT p.id
	FROM posts p
	JOIN follows f ON f.followee_id = p.author_id AND f.follower_id = $1
	WHERE p.status = 'published'
	  AND ($2::bigint = 0 OR p.id < $2)
	  AND NOT EXISTS (
	        SELECT 1 FROM blocks b
	        WHERE (b.blocker_id = $1 AND b.blocked_id = p.author_id)
	           OR (b.blocker_id = p.author_id AND b.blocked_id = $1))
	ORDER BY p.id DESC
	LIMIT $3`

func referenceIDs(t *testing.T, pool *pgxpool.Pool, viewerID, cursor int64, limit int) []int64 {
	t.Helper()
	rows, err := pool.Query(context.Background(), referenceHomeTimeline, viewerID, cursor, limit)
	if err != nil {
		t.Fatalf("基準クエリを実行できない: %v", err)
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("読み取れない: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestHomeTimelineMatchesReference(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	ctx := context.Background()
	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	blocks := postgres.NewBlockRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)

	me, err := users.Create(ctx, newUser("me"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	// 投稿数を偏らせる。
	//
	// **1人が limit を超える連続投稿を持つ状況を作る。** これが LATERAL の
	// 取りこぼしが起きうる唯一の形であり、ここが一致すれば定義どおりである。
	counts := []int{40, 3, 25, 1, 12}
	followees := make([]int64, 0, len(counts))
	for i, n := range counts {
		u, err := users.Create(ctx, newUser(fmt.Sprintf("followee%d", i)))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		followees = append(followees, u.ID)
		if _, err := pool.Exec(ctx,
			`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`, me.ID, u.ID); err != nil {
			t.Fatalf("フォローできない: %v", err)
		}
		for range n {
			seedPost(t, posts, u.ID, domain.VisibilityPublic)
		}
	}

	// フォローしていない利用者と、ブロックした相手も混ぜる。
	stranger, err := users.Create(ctx, newUser("stranger"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	for range 30 {
		seedPost(t, posts, stranger.ID, domain.VisibilityPublic)
	}
	blockedFollowee := followees[2]
	if err := blocks.Block(ctx, me.ID, blockedFollowee); err != nil {
		t.Fatalf("ブロックできない: %v", err)
	}
	// ブロックでフォローが消えるため（BR-08）、比較対象から外れることも確かめる。

	for _, limit := range []int{5, 20, 50} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {
			cursor := int64(0)
			for page := 1; page <= 5; page++ {
				want := referenceIDs(t, pool, me.ID, cursor, limit)
				items, err := timelines.Home(ctx, domain.TimelineQuery{
					ViewerID: &me.ID, Cursor: cursor, Limit: &limit,
				})
				if err != nil {
					t.Fatalf("取得できない: %v", err)
				}
				got := timelineIDs(items)

				if len(got) != len(want) {
					t.Fatalf("%dページ目の件数が違う: %d, want %d", page, len(got), len(want))
				}
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("%dページ目の %d 件目が違う: %d, want %d\n got=%v\nwant=%v",
							page, i+1, got[i], want[i], got, want)
					}
				}
				if len(got) == 0 {
					break
				}
				cursor = got[len(got)-1]
			}
		})
	}
}

// カーソルで全件を重複・欠落なくたどれること（LATERAL 版）。
func TestHomeTimelineCursorCoversAllPosts(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	ctx := context.Background()
	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)

	me, err := users.Create(ctx, newUser("me"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	created := map[int64]bool{}
	for i, n := range []int{17, 4, 23} {
		u, err := users.Create(ctx, newUser(fmt.Sprintf("f%d", i)))
		if err != nil {
			t.Fatalf("登録できない: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`, me.ID, u.ID); err != nil {
			t.Fatalf("フォローできない: %v", err)
		}
		for range n {
			created[seedPost(t, posts, u.ID, domain.VisibilityPublic)] = true
		}
	}

	limit := 7
	seen := map[int64]bool{}
	cursor := int64(0)
	var previous int64
	for range 20 {
		items, err := timelines.Home(ctx, domain.TimelineQuery{
			ViewerID: &me.ID, Cursor: cursor, Limit: &limit,
		})
		if err != nil {
			t.Fatalf("取得できない: %v", err)
		}
		if len(items) == 0 {
			break
		}
		for _, id := range timelineIDs(items) {
			if seen[id] {
				t.Errorf("重複している: %d", id)
			}
			if previous != 0 && id >= previous {
				t.Errorf("降順になっていない: %d のあとに %d", previous, id)
			}
			seen[id] = true
			previous = id
		}
		cursor = previous
	}

	if len(seen) != len(created) {
		t.Errorf("取得できた件数が違う: %d, want %d", len(seen), len(created))
	}
	for id := range created {
		if !seen[id] {
			t.Errorf("欠落している: %d", id)
		}
	}
}

// ブロックの除外を1回にまとめても、双方向に効くこと（#41）。
//
// 「自分がブロックした相手」と「自分をブロックした相手」を
// 1つの集合にまとめている。まとめ方を誤ると片方向しか効かなくなる。
func TestHomeTimelineExcludesBlockedFolloweesBothWays(t *testing.T) {
	pool := newPool(t)
	cleanup(t, pool)
	defer cleanup(t, pool)

	ctx := context.Background()
	users := postgres.NewUserRepository(pool)
	posts := postgres.NewPostRepository(pool)
	timelines := postgres.NewTimelineRepository(pool)

	me, err := users.Create(ctx, newUser("me"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	iBlock, err := users.Create(ctx, newUser("iblock"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	blocksMe, err := users.Create(ctx, newUser("blocksme"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}
	normal, err := users.Create(ctx, newUser("normal"))
	if err != nil {
		t.Fatalf("登録できない: %v", err)
	}

	hidden := []int64{}
	for _, u := range []int64{iBlock.ID, blocksMe.ID} {
		hidden = append(hidden, seedPost(t, posts, u, domain.VisibilityPublic))
	}
	visible := seedPost(t, posts, normal.ID, domain.VisibilityPublic)

	for _, u := range []int64{iBlock.ID, blocksMe.ID, normal.ID} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)`, me.ID, u); err != nil {
			t.Fatalf("フォローできない: %v", err)
		}
	}
	// ブロックは follows を消すため（BR-08）、フォローの後に直接入れる。
	if _, err := pool.Exec(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2), ($3, $1)`,
		me.ID, iBlock.ID, blocksMe.ID); err != nil {
		t.Fatalf("ブロックを作れない: %v", err)
	}

	items, err := timelines.Home(ctx, domain.TimelineQuery{ViewerID: &me.ID})
	if err != nil {
		t.Fatalf("取得できない: %v", err)
	}
	got := map[int64]bool{}
	for _, id := range timelineIDs(items) {
		got[id] = true
	}

	for _, id := range hidden {
		if got[id] {
			t.Errorf("ブロック関係の投稿が含まれている: %d", id)
		}
	}
	if !got[visible] {
		t.Errorf("ブロックしていない相手の投稿が消えている: %d", visible)
	}
}
