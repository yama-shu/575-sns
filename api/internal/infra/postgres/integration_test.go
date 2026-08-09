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
// ここでは users だけを消す。
func cleanup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM users`); err != nil {
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
