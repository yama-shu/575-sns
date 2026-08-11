package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/password"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// ---------------------------------------------------------------------------
// テスト用のリポジトリ。
//
// **DB を使わない。** domain が定義したインターフェースを満たすだけの
// 偽物を差し込む（詳細設計 02 §2）。呼び出し回数も記録し、
// 「エラーを返しつつ保存してしまう」実装を検出できるようにする。
// ---------------------------------------------------------------------------

type fakeUserRepo struct {
	byHandle    map[string]*domain.User
	emails      map[string]bool
	createCalls int
	nextID      int64
	createErr   error
	findErr     error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byHandle: map[string]*domain.User{},
		emails:   map[string]bool{},
		nextID:   1,
	}
}

func (r *fakeUserRepo) Create(_ context.Context, user *domain.User) (*domain.User, error) {
	r.createCalls++
	if r.createErr != nil {
		return nil, r.createErr
	}
	created := *user
	created.ID = r.nextID
	r.nextID++
	r.byHandle[created.Handle] = &created
	r.emails[created.Email] = true
	return &created, nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	for _, u := range r.byHandle {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeUserRepo) FindByHandle(_ context.Context, handle string) (*domain.User, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if u, ok := r.byHandle[handle]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakeUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	return r.emails[email], nil
}

func (r *fakeUserRepo) UpdateProfile(
	context.Context, int64, string, string,
) (*domain.User, error) {
	return nil, errors.New("使わない")
}

type fakeSessionRepo struct {
	sessions    map[string]*domain.Session
	users       map[int64]*domain.User
	createCalls int
	touchCalls  int
	deleteCalls int
	createErr   error
	touchErr    error
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{
		sessions: map[string]*domain.Session{},
		users:    map[int64]*domain.User{},
	}
}

func (r *fakeSessionRepo) Create(_ context.Context, s *domain.Session) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	copied := *s
	r.sessions[s.ID] = &copied
	return nil
}

func (r *fakeSessionRepo) FindByID(_ context.Context, id string) (*domain.Session, *domain.User, error) {
	s, ok := r.sessions[id]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	u, ok := r.users[s.UserID]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	return s, u, nil
}

func (r *fakeSessionRepo) Touch(_ context.Context, id string, now, expiresAt time.Time) error {
	r.touchCalls++
	if r.touchErr != nil {
		return r.touchErr
	}
	if s, ok := r.sessions[id]; ok {
		s.LastAccessedAt = now
		s.ExpiresAt = expiresAt
	}
	return nil
}

func (r *fakeSessionRepo) Delete(_ context.Context, id string) error {
	r.deleteCalls++
	delete(r.sessions, id)
	return nil
}

func (r *fakeSessionRepo) DeleteByUserID(_ context.Context, userID int64) error {
	for id, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, id)
		}
	}
	return nil
}

func (r *fakeSessionRepo) DeleteExpired(_ context.Context, now time.Time) (int64, error) {
	var n int64
	for id, s := range r.sessions {
		if s.IsExpired(now) {
			delete(r.sessions, id)
			n++
		}
	}
	return n, nil
}

// ---------------------------------------------------------------------------

var fixedNow = time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

// テストのハッシュ化コストは最小にする。既定の 12 は1回あたり数百ミリ秒かかり、
// テスト全体が現実的な時間で終わらなくなる。
const testBcryptCost = 4

func newAuth(t *testing.T) (*usecase.Auth, *fakeUserRepo, *fakeSessionRepo) {
	t.Helper()
	users := newFakeUserRepo()
	sessions := newFakeSessionRepo()
	auth := usecase.NewAuth(users, sessions, password.NewHasher(testBcryptCost),
		func() time.Time { return fixedNow })
	return auth, users, sessions
}

func validSignUp() usecase.SignUpInput {
	return usecase.SignUpInput{
		Handle:      "yamada",
		Email:       "yamada@example.com",
		Password:    "correct-horse-battery",
		DisplayName: "やまだ",
	}
}

// ---------------------------------------------------------------------------
// 登録
// ---------------------------------------------------------------------------

func TestSignUp(t *testing.T) {
	auth, users, sessions := newAuth(t)

	user, session, err := auth.SignUp(context.Background(), validSignUp())
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}

	if user.Handle != "yamada" {
		t.Errorf("識別名が違う: %q", user.Handle)
	}
	if user.Status != domain.UserActive {
		t.Errorf("状態が active でない: %q", user.Status)
	}
	// パスワードが平文で保存されていないこと（NFR-04-01）
	if user.PasswordHash == validSignUp().Password {
		t.Error("パスワードが平文で保存されている")
	}
	if user.PasswordHash == "" {
		t.Error("パスワードハッシュが空")
	}
	// セッション ID は 43 文字（32バイトの base64url）
	if len(session.ID) != 43 {
		t.Errorf("セッション ID の長さが 43 でない: %d (%q)", len(session.ID), session.ID)
	}
	if session.ExpiresAt != fixedNow.Add(domain.SessionLifetime) {
		t.Errorf("有効期限が 30 日後でない: %v", session.ExpiresAt)
	}
	if users.createCalls != 1 || sessions.createCalls != 1 {
		t.Errorf("保存の回数が想定と違う: user=%d session=%d", users.createCalls, sessions.createCalls)
	}
}

func TestSignUpRejectsDuplicateHandle(t *testing.T) {
	auth, users, _ := newAuth(t)
	if _, _, err := auth.SignUp(context.Background(), validSignUp()); err != nil {
		t.Fatalf("1件目の登録に失敗した: %v", err)
	}
	before := users.createCalls

	in := validSignUp()
	in.Email = "other@example.com"
	_, _, err := auth.SignUp(context.Background(), in)

	if !errors.Is(err, domain.ErrHandleTaken) {
		t.Fatalf("HANDLE_TAKEN を期待したが %v", err)
	}
	// **保存されていないこと**を確認する。エラーを返しつつ保存する実装を検出する。
	if users.createCalls != before {
		t.Error("重複なのに保存されている")
	}
}

func TestSignUpRejectsDuplicateEmail(t *testing.T) {
	auth, _, _ := newAuth(t)
	if _, _, err := auth.SignUp(context.Background(), validSignUp()); err != nil {
		t.Fatalf("1件目の登録に失敗した: %v", err)
	}

	in := validSignUp()
	in.Handle = "other"
	_, _, err := auth.SignUp(context.Background(), in)

	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("EMAIL_TAKEN を期待したが %v", err)
	}
}

func TestSignUpValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*usecase.SignUpInput)
		field  string
	}{
		{"識別名が空", func(in *usecase.SignUpInput) { in.Handle = "" }, "handle"},
		{"識別名に記号", func(in *usecase.SignUpInput) { in.Handle = "yama-da" }, "handle"},
		{"識別名が長すぎる", func(in *usecase.SignUpInput) { in.Handle = "abcdefghijklmnopqrstu" }, "handle"},
		{"メールアドレスが空", func(in *usecase.SignUpInput) { in.Email = "" }, "email"},
		{"メールアドレスの形式", func(in *usecase.SignUpInput) { in.Email = "not-an-email" }, "email"},
		{"パスワードが短い", func(in *usecase.SignUpInput) { in.Password = "short" }, "password"},
		{"パスワードが長すぎる", func(in *usecase.SignUpInput) {
			in.Password = string(make([]byte, 73))
		}, "password"},
		{"表示名が空", func(in *usecase.SignUpInput) { in.DisplayName = "" }, "display_name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth, users, _ := newAuth(t)
			in := validSignUp()
			tc.mutate(&in)

			_, _, err := auth.SignUp(context.Background(), in)

			var appErr *domain.Error
			if !errors.As(err, &appErr) {
				t.Fatalf("domain.Error を期待したが %v", err)
			}
			if appErr.Code != domain.CodeValidationFailed {
				t.Errorf("VALIDATION_FAILED を期待したが %q", appErr.Code)
			}
			if appErr.Field != tc.field {
				t.Errorf("項目が違う: expected=%q actual=%q", tc.field, appErr.Field)
			}
			if users.createCalls != 0 {
				t.Error("検証に失敗したのに保存されている")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ログイン
// ---------------------------------------------------------------------------

func TestLogIn(t *testing.T) {
	auth, _, sessions := newAuth(t)
	in := validSignUp()
	if _, _, err := auth.SignUp(context.Background(), in); err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	before := sessions.createCalls

	user, session, err := auth.LogIn(context.Background(), in.Handle, in.Password)
	if err != nil {
		t.Fatalf("ログインに失敗した: %v", err)
	}
	if user.Handle != in.Handle {
		t.Errorf("利用者が違う: %q", user.Handle)
	}
	if session == nil || len(session.ID) != 43 {
		t.Error("セッションが発行されていない")
	}
	if sessions.createCalls != before+1 {
		t.Error("セッションが保存されていない")
	}
}

// 存在しない識別名とパスワード誤りで、返るエラーが同一であること。
// 区別できると、識別名の総当たりで登録済みの利用者を列挙できる。
func TestLogInDoesNotRevealWhetherUserExists(t *testing.T) {
	auth, _, _ := newAuth(t)
	in := validSignUp()
	if _, _, err := auth.SignUp(context.Background(), in); err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}

	_, _, errUnknownUser := auth.LogIn(context.Background(), "nobody", in.Password)
	_, _, errWrongPassword := auth.LogIn(context.Background(), in.Handle, "wrong-password")

	if !errors.Is(errUnknownUser, domain.ErrInvalidCredentials) {
		t.Errorf("存在しない識別名: INVALID_CREDENTIALS を期待したが %v", errUnknownUser)
	}
	if !errors.Is(errWrongPassword, domain.ErrInvalidCredentials) {
		t.Errorf("パスワード誤り: INVALID_CREDENTIALS を期待したが %v", errWrongPassword)
	}
	if errUnknownUser.Error() != errWrongPassword.Error() {
		t.Errorf("エラーが区別できてしまう: %q / %q", errUnknownUser, errWrongPassword)
	}
}

func TestLogInRejectsSuspendedUser(t *testing.T) {
	auth, users, _ := newAuth(t)
	in := validSignUp()
	if _, _, err := auth.SignUp(context.Background(), in); err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	users.byHandle[in.Handle].Status = domain.UserSuspended

	_, _, err := auth.LogIn(context.Background(), in.Handle, in.Password)

	if !errors.Is(err, domain.ErrAccountSuspended) {
		t.Fatalf("ACCOUNT_SUSPENDED を期待したが %v", err)
	}
}

func TestLogInTreatsDeletedUserAsUnknown(t *testing.T) {
	// 退会の事実を漏らさない。存在しない識別名と同じ応答にする。
	auth, users, _ := newAuth(t)
	in := validSignUp()
	if _, _, err := auth.SignUp(context.Background(), in); err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	users.byHandle[in.Handle].Status = domain.UserDeleted

	_, _, err := auth.LogIn(context.Background(), in.Handle, in.Password)

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("INVALID_CREDENTIALS を期待したが %v", err)
	}
}

// ---------------------------------------------------------------------------
// セッションの検証
// ---------------------------------------------------------------------------

func TestAuthenticate(t *testing.T) {
	auth, users, sessions := newAuth(t)
	in := validSignUp()
	user, session, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sessions.users[user.ID] = users.byHandle[in.Handle]

	got, err := auth.Authenticate(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("認証に失敗した: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("利用者が違う: %d", got.ID)
	}
	// スライディング期限が延びていること
	if sessions.touchCalls != 1 {
		t.Errorf("有効期限が延長されていない: touch=%d", sessions.touchCalls)
	}
}

func TestAuthenticateRejectsEmptyAndUnknownSession(t *testing.T) {
	auth, _, _ := newAuth(t)

	for _, id := range []string{"", "does-not-exist"} {
		if _, err := auth.Authenticate(context.Background(), id); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Errorf("%q: UNAUTHENTICATED を期待したが %v", id, err)
		}
	}
}

func TestAuthenticateRejectsExpiredSessionAndDeletesIt(t *testing.T) {
	auth, users, sessions := newAuth(t)
	in := validSignUp()
	user, session, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sessions.users[user.ID] = users.byHandle[in.Handle]
	// 期限を過去にする
	sessions.sessions[session.ID].ExpiresAt = fixedNow.Add(-time.Second)

	_, err = auth.Authenticate(context.Background(), session.ID)

	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("UNAUTHENTICATED を期待したが %v", err)
	}
	// 期限切れの行は読み出した時点で消す。定期ジョブを待たない。
	if _, ok := sessions.sessions[session.ID]; ok {
		t.Error("期限切れのセッションが残っている")
	}
}

func TestAuthenticateRejectsSuspendedUser(t *testing.T) {
	// 停止時にセッションを消す運用だが、消し漏れがあっても止まること。
	auth, users, sessions := newAuth(t)
	in := validSignUp()
	user, session, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	stored := users.byHandle[in.Handle]
	stored.Status = domain.UserSuspended
	sessions.users[user.ID] = stored

	if _, err := auth.Authenticate(context.Background(), session.ID); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("UNAUTHENTICATED を期待したが %v", err)
	}
}

// ---------------------------------------------------------------------------
// ログアウトと一括破棄
// ---------------------------------------------------------------------------

func TestLogOutRemovesSessionImmediately(t *testing.T) {
	auth, users, sessions := newAuth(t)
	in := validSignUp()
	user, session, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sessions.users[user.ID] = users.byHandle[in.Handle]

	if err := auth.LogOut(context.Background(), session.ID); err != nil {
		t.Fatalf("ログアウトに失敗した: %v", err)
	}

	// **次のリクエストから即座に効くこと。** これがサーバー側セッションを
	// 選んだ理由そのものである（ADR-0006 が JWT を却下した点）。
	if _, err := auth.Authenticate(context.Background(), session.ID); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("ログアウト後も認証が通る: %v", err)
	}
}

func TestRevokeAllSessions(t *testing.T) {
	auth, users, sessions := newAuth(t)
	in := validSignUp()
	user, first, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sessions.users[user.ID] = users.byHandle[in.Handle]
	_, second, err := auth.LogIn(context.Background(), in.Handle, in.Password)
	if err != nil {
		t.Fatalf("ログインに失敗した: %v", err)
	}

	if err := auth.RevokeAllSessions(context.Background(), user.ID); err != nil {
		t.Fatalf("一括破棄に失敗した: %v", err)
	}

	for _, id := range []string{first.ID, second.ID} {
		if _, err := auth.Authenticate(context.Background(), id); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Errorf("破棄後も認証が通る: %s", id)
		}
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	auth, _, sessions := newAuth(t)
	in := validSignUp()
	_, session, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sessions.sessions[session.ID].ExpiresAt = fixedNow.Add(-time.Hour)

	deleted, err := auth.DeleteExpiredSessions(context.Background())
	if err != nil {
		t.Fatalf("削除に失敗した: %v", err)
	}
	if deleted != 1 {
		t.Errorf("削除件数が違う: %d", deleted)
	}
}

// ---------------------------------------------------------------------------
// リポジトリのエラーを握りつぶさないこと
// ---------------------------------------------------------------------------

func TestSignUpPropagatesRepositoryError(t *testing.T) {
	auth, users, _ := newAuth(t)
	sentinel := errors.New("データベースに接続できません")
	users.createErr = sentinel

	_, _, err := auth.SignUp(context.Background(), validSignUp())

	if !errors.Is(err, sentinel) {
		t.Fatalf("リポジトリのエラーが伝わっていない: %v", err)
	}
}

func TestLogInPropagatesRepositoryError(t *testing.T) {
	auth, users, _ := newAuth(t)
	sentinel := errors.New("データベースに接続できません")
	users.findErr = sentinel

	_, _, err := auth.LogIn(context.Background(), "yamada", "password123")

	if !errors.Is(err, sentinel) {
		t.Fatalf("リポジトリのエラーが伝わっていない: %v", err)
	}
}

func TestNewAuthDefaultsClockToNow(t *testing.T) {
	// now に nil を渡すと time.Now が使われること。
	auth := usecase.NewAuth(newFakeUserRepo(), newFakeSessionRepo(),
		password.NewHasher(testBcryptCost), nil)

	_, session, err := auth.SignUp(context.Background(), validSignUp())
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	if time.Until(session.ExpiresAt) < domain.SessionLifetime-time.Minute {
		t.Errorf("有効期限が現在時刻を起点にしていない: %v", session.ExpiresAt)
	}
}

func TestSignUpPropagatesSessionRepositoryError(t *testing.T) {
	auth, _, sessions := newAuth(t)
	sentinel := errors.New("セッションを保存できません")
	sessions.createErr = sentinel

	_, _, err := auth.SignUp(context.Background(), validSignUp())

	if !errors.Is(err, sentinel) {
		t.Fatalf("セッション保存のエラーが伝わっていない: %v", err)
	}
}

func TestLogInPropagatesSessionRepositoryError(t *testing.T) {
	auth, _, sessions := newAuth(t)
	in := validSignUp()
	if _, _, err := auth.SignUp(context.Background(), in); err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sentinel := errors.New("セッションを保存できません")
	sessions.createErr = sentinel

	_, _, err := auth.LogIn(context.Background(), in.Handle, in.Password)

	if !errors.Is(err, sentinel) {
		t.Fatalf("セッション保存のエラーが伝わっていない: %v", err)
	}
}

func TestSignUpPropagatesFindByHandleError(t *testing.T) {
	// 重複確認そのものが失敗した場合。ErrNotFound 以外は握りつぶさない。
	auth, users, _ := newAuth(t)
	sentinel := errors.New("データベースに接続できません")
	users.findErr = sentinel

	_, _, err := auth.SignUp(context.Background(), validSignUp())

	if !errors.Is(err, sentinel) {
		t.Fatalf("重複確認のエラーが伝わっていない: %v", err)
	}
}

func TestAuthenticatePropagatesRepositoryError(t *testing.T) {
	// FindByID が ErrNotFound 以外を返した場合は 401 にせず伝える。
	// 認証基盤の障害を「未認証」と扱うと、原因が見えなくなる。
	auth, users, sessions := newAuth(t)
	in := validSignUp()
	user, session, err := auth.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("登録に失敗した: %v", err)
	}
	sessions.users[user.ID] = users.byHandle[in.Handle]
	sessions.touchErr = errors.New("セッションを更新できません")

	if _, err := auth.Authenticate(context.Background(), session.ID); !errors.Is(err, sessions.touchErr) {
		t.Fatalf("Touch のエラーが伝わっていない: %v", err)
	}
}
