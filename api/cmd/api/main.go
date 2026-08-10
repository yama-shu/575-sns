// api は 575 の業務ロジックとデータの永続化を担うサービス。
//
// 本ファイルは開発環境構築（#2）時点の骨組みである。
// 認証・投稿・タイムライン等のエンドポイントは M2 以降で実装する。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/yama-shu/575-sns/api/internal/config"
	"github.com/yama-shu/575-sns/api/internal/handler"
	"github.com/yama-shu/575-sns/api/internal/infra/postgres"
	"github.com/yama-shu/575-sns/api/internal/password"
	"github.com/yama-shu/575-sns/api/internal/prosody"
	"github.com/yama-shu/575-sns/api/internal/requestid"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api を起動できませんでした", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 詳細設計 03 §3 の必須フィールド。3サービスのログが混ざるため service は必ず付ける。
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}),
	).With("service", "api")
	slog.SetDefault(logger)

	// 接続プールの生成は接続の確立を待たない。DB が未起動でも api は起動し、
	// /readyz が false を返す。閲覧系まで巻き添えにしないための設計（NFR-02-03）。
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("データベース接続プールを作成できません: %w", err)
	}
	defer pool.Close()

	// 判定エンジンへのクライアントは1つだけ作る。
	// **サーキットブレーカーの状態を共有するため。** リクエストごとに作ると、
	// 失敗の観測が毎回リセットされて遮断器が機能しない。
	prosodyClient := prosody.New(cfg.ProsodyURL, prosody.Options{Timeout: cfg.ProsodyTimeout})

	health := &handler.Health{
		DB:             pool,
		DBTimeout:      cfg.DatabaseTimeout,
		Prosody:        prosodyClient,
		ProsodyTimeout: cfg.ProsodyTimeout,
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID()) // 基本設計 01 §7: リクエスト ID をサービス間で引き回す
	e.Use(propagateRequestID())
	e.Use(requestLogger())

	// 依存を組み立てるのはここだけ。usecase は PostgreSQL を知らず、
	// domain が定義したインターフェースに対して操作する（詳細設計 02 §2）。
	authUsecase := usecase.NewAuth(
		postgres.NewUserRepository(pool),
		postgres.NewSessionRepository(pool),
		password.NewHasher(cfg.BcryptCost),
		time.Now,
	)
	authHandler := handler.NewAuth(authUsecase, cfg.SecureCookie)
	prosodyHandler := handler.NewProsody(usecase.NewProsody(prosodyClient))
	blockRepo := postgres.NewBlockRepository(pool)
	postHandler := handler.NewPost(usecase.NewPost(
		postgres.NewPostRepository(pool), prosodyClient, blockRepo, time.Now,
	))
	likeHandler := handler.NewLike(usecase.NewLike(
		postgres.NewLikeRepository(pool), postgres.NewPostRepository(pool), blockRepo,
	))
	timelineHandler := handler.NewTimeline(usecase.NewTimeline(
		postgres.NewTimelineRepository(pool),
	))
	moderationHandler := handler.NewModeration(usecase.NewModeration(
		postgres.NewUserRepository(pool),
		postgres.NewPostRepository(pool),
		postgres.NewReportRepository(pool),
		blockRepo,
	))
	followHandler := handler.NewFollow(usecase.NewFollow(
		postgres.NewUserRepository(pool), postgres.NewFollowRepository(pool),
	))

	e.GET("/healthz", health.Healthz)
	e.GET("/readyz", health.Readyz)

	v1 := e.Group("/api/v1")
	v1.POST("/auth/signup", authHandler.SignUp)
	v1.POST("/auth/login", authHandler.LogIn)
	v1.POST("/auth/logout", authHandler.LogOut)
	v1.GET("/me", authHandler.Me, handler.RequireAuth(authUsecase))
	v1.POST("/prosody/check", prosodyHandler.Check, handler.RequireAuth(authUsecase))
	v1.POST("/posts", postHandler.Create, handler.RequireAuth(authUsecase))
	// 未ログインでも取得できる。ログイン済みなら liked_by_me を返せるよう
	// 利用者を載せる（OptionalAuth）。
	v1.GET("/posts/:id", postHandler.Get, handler.OptionalAuth(authUsecase))
	v1.DELETE("/posts/:id", postHandler.Delete, handler.RequireAuth(authUsecase))
	v1.PUT("/users/:handle/follow", followHandler.Follow, handler.RequireAuth(authUsecase))
	v1.DELETE("/users/:handle/follow", followHandler.Unfollow, handler.RequireAuth(authUsecase))
	v1.POST("/posts/:id/report", moderationHandler.Report, handler.RequireAuth(authUsecase))
	v1.PUT("/users/:handle/block", moderationHandler.Block, handler.RequireAuth(authUsecase))
	v1.DELETE("/users/:handle/block", moderationHandler.Unblock, handler.RequireAuth(authUsecase))
	// 全体タイムラインは未ログインでも見られる。ログイン済みならブロックの除外と
	// liked_by_me が効くよう、利用者を載せる（OptionalAuth）。
	v1.GET("/timelines/public", timelineHandler.Public, handler.OptionalAuth(authUsecase))
	v1.GET("/timelines/home", timelineHandler.Home, handler.RequireAuth(authUsecase))
	v1.PUT("/posts/:id/like", likeHandler.Like, handler.RequireAuth(authUsecase))
	v1.DELETE("/posts/:id/like", likeHandler.Unlike, handler.RequireAuth(authUsecase))

	address := fmt.Sprintf(":%d", cfg.Port)
	go func() {
		slog.Info("api を起動しました", "address", address, "prosody_url", cfg.ProsodyURL)
		if err := e.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP サーバが異常終了しました", "error", err)
			os.Exit(1)
		}
	}()

	// 処理中のリクエストを打ち切らずに終了する（ローリングアップデート時に 5xx を出さないため）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("終了シグナルを受け取りました。処理中のリクエストの完了を待ちます。")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.Shutdown(ctx)
}

// propagateRequestID は echo が採番したリクエスト ID を context に載せる。
//
// 下流（prosody）を呼ぶ層は echo.Context を持たない。
// context に載せておかないと、api で切れて 3サービスのログが繋がらない。
func propagateRequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Response().Header().Get(echo.HeaderXRequestID)
			req := c.Request()
			c.SetRequest(req.WithContext(requestid.With(req.Context(), id)))
			return next(c)
		}
	}
}

// requestLogger はアクセスログを構造化データとして出力するミドルウェアを返す。
//
// 詳細設計 03 §3 のとおり、`message` ではなく `event` で集計できるようにする。
// 文言を変えてもログの集計が壊れないようにするためである。
// リクエストボディは記録しない（投稿本文が含まれるため）。
func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogLatency:   true,
		LogRequestID: true,
		LogError:     true,
		HandleError:  true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []any{
				"event", "http_request",
				"request_id", v.RequestID,
				"method", v.Method,
				"path", v.URI,
				"status", v.Status,
				"duration_ms", v.Latency.Milliseconds(),
			}
			if v.Error != nil {
				slog.Error("リクエストの処理に失敗しました",
					append(attrs, "error_detail", v.Error.Error())...)
				return nil
			}
			slog.Info("リクエストを処理しました", attrs...)
			return nil
		},
	})
}

func parseLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
