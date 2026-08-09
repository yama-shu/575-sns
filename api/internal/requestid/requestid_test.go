package requestid_test

import (
	"context"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/requestid"
)

func TestRoundTrip(t *testing.T) {
	ctx := requestid.With(context.Background(), "abc123")
	if got := requestid.From(ctx); got != "abc123" {
		t.Errorf("取り出せない: %q", got)
	}
}

// 空の ID を載せないこと。
// 載せると、下流へ空のヘッダを送って ID を上書きしてしまう。
func TestEmptyIDIsNotStored(t *testing.T) {
	ctx := requestid.With(context.Background(), "")
	if got := requestid.From(ctx); got != "" {
		t.Errorf("空の ID が載っている: %q", got)
	}
}

func TestMissingIDReturnsEmpty(t *testing.T) {
	if got := requestid.From(context.Background()); got != "" {
		t.Errorf("空文字列を期待したが %q", got)
	}
}

// 他のパッケージが同じ文字列キーを使っても衝突しないこと。
func TestKeyDoesNotCollideWithStringKey(t *testing.T) {
	//nolint:staticcheck // 衝突しないことの確認のため、あえて文字列キーを使う
	ctx := context.WithValue(context.Background(), "request_id", "他人の値")
	if got := requestid.From(ctx); got != "" {
		t.Errorf("文字列キーの値を拾っている: %q", got)
	}
}
