package domain

import "context"

// ページネーションの既定値と上限（基本設計 05 §1）。
const (
	// DefaultTimelineLimit は limit を省略したときの件数。
	DefaultTimelineLimit = 20
	// MaxTimelineLimit は1回で取得できる上限。
	//
	// 上限を設けるのは、1回のリクエストで大量の行を読ませないためである。
	MaxTimelineLimit = 50
)

// TimelineQuery はタイムラインの取得条件。
type TimelineQuery struct {
	// ViewerID は閲覧者。nil なら未ログイン。
	//
	// ブロックの除外と liked_by_me の判定に使う。
	// 未ログインではどちらも成立しないため、条件から外す。
	ViewerID *int64
	// Cursor はこの ID より前を取得する。0 なら最新から。
	Cursor int64
	// Limit は取得件数。
	//
	// **nil は「指定なし」で、0 とは違う。** int の 0 を未指定の印にすると、
	// `?limit=0` という明らかに不正な指定を既定値に読み替えてしまう。
	Limit *int
}

// Validate は取得条件を検証し、未指定の項目に既定値を入れる。
func (q *TimelineQuery) Validate() error {
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
func (q TimelineQuery) EffectiveLimit() int {
	if q.Limit == nil {
		return DefaultTimelineLimit
	}
	return *q.Limit
}

// UserPostQuery はユーザーページの投稿一覧の取得条件。
//
// タイムラインと同じ形で読むため、TimelineQuery を含む。
type UserPostQuery struct {
	TimelineQuery
	// AuthorID は誰の投稿を取るか。
	AuthorID int64
	// IncludeFollowersOnly はフォロワー限定の投稿を含めるか。
	//
	// 本人とフォロワーだけが true になる（FR-02-08）。
	// **この判断を SQL でやらない。** 「見えるか」の規則は usecase が持つ
	// （Post.IsVisibleTo と同じ規則を2箇所に書かない）。
	IncludeFollowersOnly bool
}

// TimelineItem はタイムラインの1件。
//
// 投稿・投稿者・閲覧者から見た状態を1回のクエリでまとめて取る。
// 分けて取ると、20件のタイムラインで 21 回のクエリになる。
type TimelineItem struct {
	Post      *Post
	Author    *User
	LikedByMe bool
}

// Timeline は1ページぶんのタイムライン。
type Timeline struct {
	Items []TimelineItem
	// NextCursor は次のページの起点。続きが無ければ 0。
	NextCursor int64
}

// TimelineRepository はタイムラインの取得。
type TimelineRepository interface {
	// Public は全体タイムラインを返す。公開投稿だけが対象。
	Public(ctx context.Context, q TimelineQuery) ([]TimelineItem, error)
	// Home はフォロー中タイムラインを返す。
	//
	// フォローしている相手の followers 限定の投稿も含む。
	// ViewerID は必須である。
	Home(ctx context.Context, q TimelineQuery) ([]TimelineItem, error)
	// UserPosts はある利用者の投稿一覧を返す（S-04）。
	//
	// ブロックの除外を条件に入れない。**プロフィールを引く時点で 404 になる**ため、
	// ここに到達する時点でブロック関係は無い。二重に書くと、
	// 片方だけ直したときに食い違う。
	UserPosts(ctx context.Context, q UserPostQuery) ([]TimelineItem, error)
}
