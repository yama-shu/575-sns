// Package handler は HTTP の入出力を担う。
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Pinger は疎通確認だけを要求する最小のインターフェース。
// api は「PostgreSQL である」ことを知る必要がないため、pgxpool に直接依存しない。
type Pinger interface {
	Ping(ctx context.Context) error
}

// Health は liveness / readiness を返すハンドラ。
type Health struct {
	DB             Pinger
	DBTimeout      time.Duration
	Prosody        Pinger
	ProsodyTimeout time.Duration
}

// Healthz は liveness probe。プロセスが生きているかだけを返す。
//
// ここで依存先を確認してはならない。DB が落ちているだけでプロセスが
// 再起動され続ける（再起動しても直らないのに）ことになるためである。
func (h *Health) Healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz は readiness probe。依存先へ到達できるかを返す。
//
// 到達できない依存があれば 503 を返し、Service がこの Pod へ
// トラフィックを流さないようにする。
func (h *Health) Readyz(c echo.Context) error {
	ctx := c.Request().Context()

	dependencies := map[string]bool{
		"database": h.check(ctx, h.DB, h.DBTimeout),
		"prosody":  h.check(ctx, h.Prosody, h.ProsodyTimeout),
	}

	ready := true
	for _, ok := range dependencies {
		if !ok {
			ready = false
		}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	return c.JSON(status, map[string]any{
		"ready":        ready,
		"dependencies": dependencies,
	})
}

func (h *Health) check(ctx context.Context, p Pinger, timeout time.Duration) bool {
	if p == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.Ping(ctx) == nil
}
