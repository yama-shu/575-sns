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
