// Package job は定期的に実行する処理を持つ。
//
// **実行の間隔を持たない。** いつ走らせるかは実行基盤（Kubernetes の CronJob）が決める。
// アプリケーション側にも間隔を持たせると、次の実行時刻を決める場所が2つになり、
// どちらの設定が効いているのか分からなくなる。
package job

import (
	"context"
	"log/slog"
	"time"
)

// ExpiredSessionDeleter は期限切れセッションを削除する。
//
// usecase.Auth が満たす。ここで狭いインターフェースにしているのは、
// ジョブが認証の他の機能（登録・ログイン）に触れられないようにするためである。
type ExpiredSessionDeleter interface {
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

// SessionCleanup は期限切れセッションを削除するジョブ。
//
// ADR-0006 が、サーバー側セッションを選んだ代償として定期的な削除を
// 要求している。放置すると、ログインのたびに増える行が一度も減らない。
type SessionCleanup struct {
	sessions ExpiredSessionDeleter
}

// NewSessionCleanup をつくる。
func NewSessionCleanup(sessions ExpiredSessionDeleter) *SessionCleanup {
	return &SessionCleanup{sessions: sessions}
}

// Run は削除を1回実行し、削除件数を返す。
//
// 0件でもエラーにしない。消すものが無いのは正常な状態である。
func (j *SessionCleanup) Run(ctx context.Context) (int64, error) {
	started := time.Now()

	deleted, err := j.sessions.DeleteExpiredSessions(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "期限切れセッションの削除に失敗しました",
			"event", "session_cleanup_failed",
			"error_detail", err.Error(),
		)
		return 0, err
	}

	// **セッション ID と利用者 ID を出さない。** セッション ID は
	// ログイン状態そのものであり、ログを読める者が他人になりすませる
	// （詳細設計 03「ログに出してはいけないもの」）。
	slog.InfoContext(ctx, "期限切れセッションを削除しました",
		"event", "session_cleanup_completed",
		"deleted", deleted,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return deleted, nil
}
