package domain

import "context"

// Profile はユーザーページ（S-04）に出す情報。
//
// 閲覧者から見た関係（フォロー中・ブロック中）を含む。
// 同じ利用者でも閲覧者が違えば内容が変わる。
type Profile struct {
	User   *User
	Counts ProfileCounts
	// Following は閲覧者がこの利用者をフォローしているか。未ログインなら false。
	Following bool
	// Blocking は閲覧者がこの利用者をブロックしているか。未ログインなら false。
	//
	// **ブロック「されている」かは返さない。** 返すと BR-10 に反する。
	// そもそもブロックされていれば、この構造体を組み立てる前に 404 になる。
	Blocking bool
}

// ProfileCounts はプロフィールに出す数。
type ProfileCounts struct {
	// Posts は投稿数。
	//
	// **閲覧者から見える数である。** フォロワー限定を含めた総数を返すと、
	// 一覧に出ている件数と合わない。「10句」と出ているのに3件しか
	// 見えない状態は、数え方の説明がつかない。
	Posts     int
	Following int
	Followers int
}

// ProfileRepository はプロフィールの数え上げ。
type ProfileRepository interface {
	// Counts は投稿数・フォロー数・フォロワー数を返す。
	//
	// **1回のクエリで返す。** 別々に問い合わせると、プロフィール1枚で
	// 4回の往復になる。
	//
	// includeFollowersOnly はフォロワー限定の投稿を数に含めるか。
	// 本人とフォロワーだけが true になる。
	Counts(ctx context.Context, userID int64, includeFollowersOnly bool) (ProfileCounts, error)
}
