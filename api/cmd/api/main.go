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
	"github.com/yama-shu/575-sns/api/internal/prosody"
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

	health := &handler.Health{
		DB:             pool,
		DBTimeout:      cfg.DatabaseTimeout,
		Prosody:        prosody.New(cfg.ProsodyURL, cfg.ProsodyTimeout),
		ProsodyTimeout: cfg.ProsodyTimeout,
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID()) // 基本設計 01 §7: リクエスト ID をサービス間で引き回す
	e.Use(requestLogger())

	e.GET("/healthz", health.Healthz)
	e.GET("/readyz", health.Readyz)

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
