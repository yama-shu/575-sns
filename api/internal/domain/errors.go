package domain

import (
	"errors"
	"fmt"
)

// ErrorCode は API が返すエラーコード（基本設計 05）。
//
// **利用者向けの文言ではなく、これで分岐する。** 文言は変わりうるため、
// message で分岐するとクライアントが壊れる。
type ErrorCode string

const (
	// CodeValidationFailed は入力の形式が不正（400）。
	CodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	// CodeUnauthenticated は未認証・セッション切れ（401）。
	CodeUnauthenticated ErrorCode = "UNAUTHENTICATED"
	// CodeForbidden は認証済みだが権限がない（403）。
	CodeForbidden ErrorCode = "FORBIDDEN"
	// CodeNotFound は対象が存在しない（404）。
	CodeNotFound ErrorCode = "NOT_FOUND"
	// CodeHandleTaken は識別名が使用済み（409）。
	CodeHandleTaken ErrorCode = "HANDLE_TAKEN"
	// CodeEmailTaken はメールアドレスが使用済み（409）。
	CodeEmailTaken ErrorCode = "EMAIL_TAKEN"
	// CodeInvalidCredentials は識別名かパスワードが違う（401）。
	//
	// **どちらが違うかを区別しない。** 区別すると、識別名の総当たりで
	// 登録済みの利用者を列挙できてしまう。
	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	// CodeAccountSuspended は利用停止中（403）。
	CodeAccountSuspended ErrorCode = "ACCOUNT_SUSPENDED"
	// CodeCannotFollowSelf は自分自身をフォローしようとした（422）。
	CodeCannotFollowSelf ErrorCode = "CANNOT_FOLLOW_SELF"
	// CodeBlockedUser はブロック中の相手への操作（422）。
	//
	// **自分がブロックしている場合にだけ使う。** 相手が自分をブロックしている
	// 場合にこれを返すと、ブロックされた事実が漏れる（BR-10）。
	CodeBlockedUser ErrorCode = "BLOCKED_USER"
	// CodeCannotBlockSelf は自分自身をブロックしようとした（422）。
	CodeCannotBlockSelf ErrorCode = "CANNOT_BLOCK_SELF"
	// CodeCannotReportSelf は自分の投稿を通報しようとした（422）。
	CodeCannotReportSelf ErrorCode = "CANNOT_REPORT_SELF"
	// CodeAlreadyReported は同一投稿への重複通報（409）。
	CodeAlreadyReported ErrorCode = "ALREADY_REPORTED"
	// CodeAlreadyHandled は対応済みの通報をもう一度処理しようとした（409）。
	CodeAlreadyHandled ErrorCode = "ALREADY_HANDLED"
	// CodeProsodyHacho は判定が破調（422）。
	//
	// 形式は正しく、内容が業務ルールを満たさない。400 にすると
	// クライアント実装のバグと利用者の入力の問題を区別できなくなる。
	CodeProsodyHacho ErrorCode = "PROSODY_HACHO"
	// CodeProsodyUnknownReading は読めない語がある（422）。
	//
	// **破調と区別する。** 読めなかっただけで「五七五になっていません」と
	// 伝えるのは誤りであり、利用者は直しようがない。
	CodeProsodyUnknownReading ErrorCode = "PROSODY_UNKNOWN_READING"
	// CodeProsodyUnavailable は prosody が応答しない、
	// またはサーキットブレーカーが開放中（503）。
	//
	// 詳細設計 03 の「異常なエラー」。投稿のみ停止し、閲覧は継続する。
	CodeProsodyUnavailable ErrorCode = "PROSODY_UNAVAILABLE"
	// CodeUpstreamTimeout は下流サービスのタイムアウト（504）。
	CodeUpstreamTimeout ErrorCode = "UPSTREAM_TIMEOUT"
)

// IsAbnormal は詳細設計 03 の「異常なエラー」かを返す。
//
// 利用者に起因する「正常なエラー」と分けるのは、ログレベルと通知の扱いが違うため。
// 正常なエラーで通知が飛ぶと、通知そのものが意味を失う。
func (c ErrorCode) IsAbnormal() bool {
	switch c {
	case CodeProsodyUnavailable, CodeUpstreamTimeout:
		return true
	}
	return false
}

// Error は利用者に起因するエラー（詳細設計 03 の「正常なエラー」）。
//
// プログラム上はエラーだが、システムとしては正常な状態である。
// 通知の対象にせず、直し方が分かる形で利用者に返す。
type Error struct {
	Code    ErrorCode
	Message string
	// Field は入力項目に対する指摘のとき、その項目名。
	Field string
	// Details は利用者が直すために必要な情報。
	//
	// 破調なら現在の音数、読めない語があるならその語。
	// **これが無いと利用者は直しようがない。**
	Details map[string]any
}

// WithDetails は詳細を添えた複製を返す。
//
// あらかじめ定義したエラーは共有されるため、複製して返す。
// 元の値を書き換えると、別のリクエストの詳細が混ざる。
func (e *Error) WithDetails(details map[string]any) *Error {
	return &Error{Code: e.Code, Message: e.Message, Field: e.Field, Details: details}
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Field)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewValidationError は入力項目に対する指摘をつくる。
func NewValidationError(field, message string) *Error {
	return &Error{Code: CodeValidationFailed, Message: message, Field: field}
}

// あらかじめ定義しておくエラー。errors.Is で比較する。
var (
	// ErrInvalidCredentials は識別名かパスワードが違う。
	ErrInvalidCredentials = &Error{
		Code:    CodeInvalidCredentials,
		Message: "識別名またはパスワードが違います",
	}
	// ErrUnauthenticated は未認証。
	ErrUnauthenticated = &Error{
		Code:    CodeUnauthenticated,
		Message: "ログインしてください",
	}
	// ErrHandleTaken は識別名が使用済み。
	ErrHandleTaken = &Error{
		Code:    CodeHandleTaken,
		Message: "この識別名は使われています",
	}
	// ErrEmailTaken はメールアドレスが使用済み。
	ErrEmailTaken = &Error{
		Code:    CodeEmailTaken,
		Message: "このメールアドレスは登録済みです",
	}
	// ErrAccountSuspended は利用停止中。
	ErrAccountSuspended = &Error{
		Code:    CodeAccountSuspended,
		Message: "このアカウントは利用を停止されています",
	}
	// ErrNotFound は対象が存在しない。
	ErrNotFound = &Error{
		Code:    CodeNotFound,
		Message: "見つかりませんでした",
	}
	// ErrForbidden は権限がない。
	ErrForbidden = &Error{
		Code:    CodeForbidden,
		Message: "この操作は行えません",
	}
	// ErrCannotFollowSelf は自分自身をフォローしようとした（BR-05）。
	ErrCannotFollowSelf = &Error{
		Code:    CodeCannotFollowSelf,
		Message: "自分はフォローできません",
	}
	// ErrBlockedUser はブロックしている相手への操作。
	ErrBlockedUser = &Error{
		Code:    CodeBlockedUser,
		Message: "この相手には行えません",
	}
	// ErrCannotBlockSelf は自分自身をブロックしようとした（BR-06）。
	ErrCannotBlockSelf = &Error{
		Code:    CodeCannotBlockSelf,
		Message: "自分はブロックできません",
	}
	// ErrCannotReportSelf は自分の投稿を通報しようとした（BR-07）。
	ErrCannotReportSelf = &Error{
		Code:    CodeCannotReportSelf,
		Message: "自分の投稿は通報できません",
	}
	// ErrAlreadyHandled は対応済みの通報をもう一度処理しようとした。
	//
	// **黙って成功にしない。** 別の運営が先に処理した可能性があり、
	// 気づかないまま二重に判断することになる。
	ErrAlreadyHandled = &Error{
		Code:    CodeAlreadyHandled,
		Message: "この通報はすでに処理されています",
	}
	// ErrAlreadyReported は同一投稿への重複通報。
	ErrAlreadyReported = &Error{
		Code:    CodeAlreadyReported,
		Message: "すでに通報済みです",
	}
	// ErrProsodyHacho は判定が破調で投稿できない。
	ErrProsodyHacho = &Error{
		Code:    CodeProsodyHacho,
		Message: "五七五になっていません",
	}
	// ErrProsodyUnknownReading は読めない語があり判定できない。
	ErrProsodyUnknownReading = &Error{
		Code:    CodeProsodyUnknownReading,
		Message: "読み方が分からない語が含まれています",
	}
	// ErrProsodyUnavailable は判定エンジンを利用できない。
	ErrProsodyUnavailable = &Error{
		Code:    CodeProsodyUnavailable,
		Message: "いま詠めません。しばらく経ってからお試しください",
	}
	// ErrUpstreamTimeout は判定エンジンが時間内に応答しなかった。
	ErrUpstreamTimeout = &Error{
		Code:    CodeUpstreamTimeout,
		Message: "判定に時間がかかっています。しばらく経ってからお試しください",
	}
)

// Is は errors.Is での比較に使う。コードが同じなら同じエラーとみなす。
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}
