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
	"os"
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
