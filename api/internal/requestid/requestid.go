// Package requestid はリクエスト ID を context で持ち回す。
//
// 基本設計 01 §7 のとおり、1つのリクエストが web → api → prosody と
// 流れるあいだ同じ ID を引き回す。ID が無いと、3サービスのログを
// 1本の線として追えない。
//
// handler が echo の値を context に載せ、下流を呼ぶ層が取り出す。
// echo.Context を直接渡すと、業務ロジックが HTTP を知ることになる。
package requestid

import "context"

// Header はサービス間で受け渡すヘッダ名。3サービスで揃える。
const Header = "X-Request-ID"

// contextKey は context のキー。
//
// 文字列を直接使うと、他のパッケージが同じ文字列を使ったときに衝突する。
type contextKey struct{}

// With はリクエスト ID を載せた context を返す。
func With(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// From はリクエスト ID を取り出す。無ければ空文字列を返す。
func From(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
