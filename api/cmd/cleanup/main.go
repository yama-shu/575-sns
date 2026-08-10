// cleanup は期限切れのセッションを削除するコマンド。
//
// ADR-0006 が、サーバー側セッションを選んだ代償として定期的な削除を
// 要求している。放置すると、ログインのたびに増える行が一度も減らない。
//
// **api の中でタイマーを回さない。** 本番では api が複数 Pod にスケールするため、
// Pod の数だけ同じ削除が走る。削除自体は冪等だが、同じ行を狙う DELETE が
// 同時に走ってロックを取り合い、「1つのジョブが定期的に走る」という意図が
// Pod 数に依存して崩れる。
//
// migrate と同じく独立したバイナリとし、api と同じイメージに入れる。
// 本番では Kubernetes の CronJob として実行する（M5）。
//
//	cleanup
//
// 接続先は api 本体と同じ環境変数 API_DATABASE_URL から読む。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/infra/postgres"
	"github.com/yama-shu/575-sns/api/internal/job"
	"github.com/yama-shu/575-sns/api/internal/password"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// timeout は1回の実行に許す時間。
//
// **上限を設ける。** CronJob は前の実行が終わらないうちに次を起動しうる。
// 止まったまま残り続けると、実行が積み上がってコネクションを食い潰す。
const timeout = 5 * time.Minute

func main() {
	// api 本体と同じ構造化ログにする（詳細設計 03 §3）。
	// service を付けないと、3サービスのログが混ざったときに出所が分からなくなる。
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	).With("service", "cleanup"))

	if err := run(); err != nil {
		// 終了コードで失敗を伝える。Kubernetes の CronJob はこれを見て
		// 失敗を検知し、再実行やアラートの判断をする。
		slog.Error("期限切れセッションの削除を実行できませんでした",
			"event", "session_cleanup_failed",
			"error_detail", err.Error(),
		)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("API_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("環境変数 API_DATABASE_URL が設定されていません")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("データベース接続プールを作成できません: %w", err)
	}
	defer pool.Close()

	// pgxpool.New は接続の確立を待たない。ここで到達性を確かめないと、
	// DB が落ちていても「0件削除して成功」に見えてしまう。
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("データベースへ接続できません: %w", err)
	}

	// 認証と同じユースケースを使う。削除の条件を2箇所に書くと、
	// 片方だけ直したときに食い違う。
	//
	// パスワードのハッシュ化はこのコマンドでは使わないが、
	// ユースケースの組み立てに必要なため既定値で渡す。
	auth := usecase.NewAuth(
		postgres.NewUserRepository(pool),
		postgres.NewSessionRepository(pool),
		password.NewHasher(0),
		time.Now,
	)

	_, err = job.NewSessionCleanup(auth).Run(ctx)
	return err
}
