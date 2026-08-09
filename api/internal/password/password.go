// Package password はパスワードのハッシュ化と検証を行う。
//
// NFR-04-01 は「ソルト付きハッシュ。平文・可逆暗号での保存を行わない」と定める。
// 方式の比較と bcrypt を選んだ理由は #25 に記録した。要点は次のとおり。
//
//   - パラメータがコスト1つで、誤設定の余地が小さい
//   - 72 バイトの切り捨ては、入力の上限を 72 バイトにすれば影響しない
//   - argon2id の GPU 耐性は、575 の脅威モデルでは決定的でない
//
// bcrypt のハッシュ文字列は方式とコストを内包する（`$2a$12$...`）。
// そのため**保存済みのハッシュを読みながら別方式へ移行できる**。
package password

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// DefaultCost はハッシュ化のコスト。
//
// bcrypt の既定（10）より1段上げる。コストは2の冪で効くため、
// 12 は 10 の約4倍の計算量になる。ログインの応答時間と
// 総当たりへの耐性の釣り合いで決めた値であり、
// 実測して体感が悪ければ下げる。
const DefaultCost = 12

// ErrMismatch はパスワードが一致しない。
var ErrMismatch = errors.New("パスワードが一致しません")

// Hasher はパスワードのハッシュ化と検証。
type Hasher struct {
	cost int
}

// NewHasher はハッシュ化器をつくる。cost が 0 なら DefaultCost を使う。
func NewHasher(cost int) *Hasher {
	if cost <= 0 {
		cost = DefaultCost
	}
	return &Hasher{cost: cost}
}

// Hash はパスワードをハッシュ化する。ソルトは bcrypt が内部で生成する。
func (h *Hasher) Hash(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// Verify はパスワードがハッシュと一致するか検証する。
//
// 一致しない場合は ErrMismatch を返す。bcrypt の比較は
// タイミング攻撃に耐える実装になっている。
func (h *Hasher) Verify(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrMismatch
		}
		return err
	}
	return nil
}

// dummyHash は「利用者が見つからなかった」場合に検証させるためのハッシュ。
//
// 見つからないときに即座に返すと、**応答時間の差で識別名の存在が分かる**。
// 存在する場合はハッシュ検証（コスト 12 で数百ミリ秒）が走り、
// 存在しない場合は走らないためである。
// 見つからない場合もこのハッシュに対して検証を行い、時間を揃える。
var dummyHash = []byte("$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewKyDGONYCFbBHPa")

// VerifyDummy は利用者が見つからなかった場合に呼ぶ。
//
// 結果は使わない。応答時間を揃えることだけが目的である。
func (h *Hasher) VerifyDummy(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(plain))
}
