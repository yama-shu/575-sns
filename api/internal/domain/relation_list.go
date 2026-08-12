package domain

import "context"

// RelationListKind はどの関係の一覧か。
type RelationListKind string

const (
	// RelationFollowing は対象がフォローしている相手の一覧（S-05）。
	RelationFollowing RelationListKind = "following"
	// RelationFollowers は対象をフォローしている相手の一覧（S-06）。
	RelationFollowers RelationListKind = "followers"
	// RelationBlocking は対象がブロックしている相手の一覧（S-11）。
	RelationBlocking RelationListKind = "blocking"
)

// RelationListQuery は一覧の取得条件。
type RelationListQuery struct {
	Kind RelationListKind
	// OwnerID は誰の一覧か。
	OwnerID int64
	// ViewerID は閲覧者。nil なら未ログイン。
	//
	// 一覧から誰を外すか（閲覧者をブロックしている相手）と、
	// Following の判定に使う。
	ViewerID *int64
	// Cursor はこの利用者 ID より前を取得する。0 なら先頭から。
	//
	// **相手の利用者 ID を基準にする。** follows と blocks に id 列は無く、
	// created_at は同時刻の並びが定まらない（基本設計 03 §5）。
	Cursor int64
	Limit  *int
}

// Validate は取得条件を検証し、未指定の項目に既定値を入れる。
func (q *RelationListQuery) Validate() error {
	if q.Limit == nil {
		limit := DefaultTimelineLimit
		q.Limit = &limit
		return nil
	}
	if *q.Limit < 1 || *q.Limit > MaxTimelineLimit {
		return NewValidationError("limit", "取得件数の指定が不正です")
	}
	if q.Cursor < 0 {
		return NewValidationError("cursor", "カーソルの指定が不正です")
	}
	return nil
}

// EffectiveLimit は検証後の取得件数を返す。Validate の後に呼ぶ。
func (q RelationListQuery) EffectiveLimit() int {
	if q.Limit == nil {
		return DefaultTimelineLimit
	}
	return *q.Limit
}

// RelationListItem は一覧の1件。
//
// **プロフィールをそのまま持たない。** 投稿数などの数え上げが人数ぶん走る。
// 一覧に要るのは誰であるかと、閲覧者から見た関係だけである。
type RelationListItem struct {
	User *User
	// Following は閲覧者がこの相手をフォローしているか。未ログインなら false。
	Following bool
}

// RelationList は1ページぶんの一覧。
type RelationList struct {
	Items []RelationListItem
	// NextCursor は次のページの起点。続きが無ければ 0。
	NextCursor int64
}

// RelationListRepository は関係の一覧の取得。
type RelationListRepository interface {
	// List は一覧を返す。
	//
	// フォロー中・フォロワーからは、閲覧者をブロックしている相手と
	// 利用停止・退会した相手を除く。一覧に出しても開けば 404 になるためである
	// （#58 の扱いに揃える）。
	//
	// **ブロック中一覧では除かない。** 自分が行った操作の記録であり、
	// 相手の状態で消えると解除できなくなる（#71）。
	List(ctx context.Context, q RelationListQuery) ([]RelationListItem, error)
}
