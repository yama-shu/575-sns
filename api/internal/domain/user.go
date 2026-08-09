// Package domain は 575 の型とビジネスルールを持つ。
//
// **この層は何にも依存しない。** データベースにも HTTP にも依存せず、
// 外部との接続は infra 層が domain の定義したインターフェースを実装する形で行う
// （詳細設計 02 §2）。これにより usecase の単体テストで DB が不要になる。
package domain

import (
	"regexp"
	"time"
	"unicode/utf8"
)

// UserStatus は利用者の状態（基本設計 02 §3 の状態遷移）。
type UserStatus string

const (
	// UserActive は通常の状態。
	UserActive UserStatus = "active"
	// UserSuspended は運営が停止した状態。ログインできず、既存セッションは破棄される。
	UserSuspended UserStatus = "suspended"
	// UserDeleted は本人が退会した状態。識別名は再利用させない。
	UserDeleted UserStatus = "deleted"
)

// CanLogIn はこの状態でログインできるか。
func (s UserStatus) CanLogIn() bool {
	return s == UserActive
}

// User は利用者。
type User struct {
	ID           int64
	Handle       string
	Email        string
	PasswordHash string
	DisplayName  string
	Bio          string
	AvatarURL    string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// 入力の上限。DB の制約（基本設計 03 §2）と一致させる。
const (
	HandleMaxLength      = 20
	EmailMaxLength       = 255
	DisplayNameMaxLength = 50
	BioMaxLength         = 200

	// PasswordMinLength は下限。短すぎるパスワードは総当たりで破られる。
	PasswordMinLength = 8
	// PasswordMaxLength はバイト数の上限。
	//
	// bcrypt は入力を 72 バイトで切り捨てるため、それを超える長さを許すと
	// 「73 バイト目以降が無視される」ことに利用者も実装者も気づけない。
	// 上限を設けて、切り捨てが起こらないようにする。
	PasswordMaxLength = 72
)

// 識別名は半角英数字とアンダースコアのみ（基本設計 03 §2）。
// DB にも同じ制約があるが、利用者に返すエラーを具体的にするためここでも検査する。
var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// メールアドレスの検査は「@ の前後に文字があり、空白を含まない」までとする。
//
// RFC に厳密な正規表現は現実の実装と食い違い、正当なアドレスを弾く。
// 到達性は結局送ってみないと分からないため、ここでは明らかな誤りだけを弾く。
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// ValidateHandle は識別名を検査する。
func ValidateHandle(handle string) error {
	switch {
	case handle == "":
		return NewValidationError("handle", "識別名を入力してください")
	case utf8.RuneCountInString(handle) > HandleMaxLength:
		return NewValidationError("handle", "識別名は20文字以内で入力してください")
	case !handlePattern.MatchString(handle):
		return NewValidationError("handle", "識別名は半角英数字とアンダースコアのみ使えます")
	}
	return nil
}

// ValidateEmail はメールアドレスを検査する。
func ValidateEmail(email string) error {
	switch {
	case email == "":
		return NewValidationError("email", "メールアドレスを入力してください")
	case len(email) > EmailMaxLength:
		return NewValidationError("email", "メールアドレスが長すぎます")
	case !emailPattern.MatchString(email):
		return NewValidationError("email", "メールアドレスの形式が正しくありません")
	}
	return nil
}

// ValidatePassword はパスワードを検査する。
func ValidatePassword(password string) error {
	switch {
	case utf8.RuneCountInString(password) < PasswordMinLength:
		return NewValidationError("password", "パスワードは8文字以上で入力してください")
	case len(password) > PasswordMaxLength:
		// 文字数ではなくバイト数で判定する。bcrypt が切り捨てるのはバイト単位のため。
		return NewValidationError("password", "パスワードが長すぎます")
	}
	return nil
}

// ValidateDisplayName は表示名を検査する。
func ValidateDisplayName(displayName string) error {
	switch {
	case displayName == "":
		return NewValidationError("display_name", "表示名を入力してください")
	case utf8.RuneCountInString(displayName) > DisplayNameMaxLength:
		return NewValidationError("display_name", "表示名は50文字以内で入力してください")
	}
	return nil
}
