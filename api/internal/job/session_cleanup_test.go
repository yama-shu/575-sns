package job_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/job"
)

type fakeDeleter struct {
	deleted int64
	err     error
	calls   int
}

func (f *fakeDeleter) DeleteExpiredSessions(context.Context) (int64, error) {
	f.calls++
	return f.deleted, f.err
}

// captureLogs は実行中のログを1件ずつ JSON として取り出す。
//
// **文言ではなく event と項目を検証する。** 文言は変わりうるため、
// message で検証するとログの文言を直すたびにテストが壊れる（詳細設計 03 §3）。
func captureLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()

	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(original)

	fn()

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("ログを解釈できない: %v (line=%s)", err, line)
		}
		records = append(records, record)
	}
	return records
}

func TestRunDeletesAndReports(t *testing.T) {
	deleter := &fakeDeleter{deleted: 42}
	var deleted int64
	var err error

	records := captureLogs(t, func() {
		deleted, err = job.NewSessionCleanup(deleter).Run(context.Background())
	})

	if err != nil {
		t.Fatalf("削除できない: %v", err)
	}
	if deleted != 42 || deleter.calls != 1 {
		t.Errorf("削除件数=%d 呼び出し回数=%d", deleted, deleter.calls)
	}
	if len(records) != 1 {
		t.Fatalf("ログが1件でない: %v", records)
	}
	record := records[0]
	if record["event"] != "session_cleanup_completed" {
		t.Errorf("event が違う: %v", record["event"])
	}
	if record["deleted"] != float64(42) {
		t.Errorf("削除件数がログに出ない: %v", record["deleted"])
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Error("処理時間がログに出ない")
	}
}

// 0件はエラーではない。消すものが無いのは正常な状態である。
func TestRunSucceedsWithNothingToDelete(t *testing.T) {
	deleter := &fakeDeleter{deleted: 0}

	records := captureLogs(t, func() {
		if _, err := job.NewSessionCleanup(deleter).Run(context.Background()); err != nil {
			t.Errorf("0件でエラーになった: %v", err)
		}
	})

	if len(records) != 1 || records[0]["level"] != "INFO" {
		t.Errorf("0件が異常として記録されている: %v", records)
	}
}

// 失敗は呼び出し元へ伝える。握りつぶすと CronJob が成功したことになる。
func TestRunPropagatesError(t *testing.T) {
	wantErr := errors.New("接続できない")
	deleter := &fakeDeleter{err: wantErr}

	var err error
	records := captureLogs(t, func() {
		_, err = job.NewSessionCleanup(deleter).Run(context.Background())
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("エラーが伝わらない: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("ログが1件でない: %v", records)
	}
	if records[0]["event"] != "session_cleanup_failed" || records[0]["level"] != "ERROR" {
		t.Errorf("失敗として記録されていない: %v", records[0])
	}
}

// ログに機微情報を出さないこと。
//
// セッション ID はログイン状態そのものであり、
// ログを読める者が他人になりすませる。
func TestRunDoesNotLogSensitiveValues(t *testing.T) {
	deleter := &fakeDeleter{deleted: 3}

	records := captureLogs(t, func() {
		_, _ = job.NewSessionCleanup(deleter).Run(context.Background())
	})

	raw, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("ログを直列化できない: %v", err)
	}
	for _, forbidden := range []string{"session_id", "user_id", "password", "email"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("ログに %s が含まれている: %s", forbidden, raw)
		}
	}
}
