package domain

import (
	"context"
	"fmt"
	"time"
)

// Visibility は投稿の公開範囲（基本設計 03 §2）。
type Visibility string

const (
	// VisibilityPublic は誰でも見られる。
	VisibilityPublic Visibility = "public"
	// VisibilityFollowers はフォロワーだけが見られる。
	VisibilityFollowers Visibility = "followers"
)

// Valid は受け付ける値かを返す。
func (v Visibility) Valid() bool {
	return v == VisibilityPublic || v == VisibilityFollowers
}

// PostStatus は投稿の状態（基本設計 03 §2）。
type PostStatus string

const (
	// PostPublished は通常の状態。
	PostPublished PostStatus = "published"
	// PostHidden は運営が非表示にした状態。
	PostHidden PostStatus = "hidden"
	// PostDeleted は投稿者が削除した状態。行は消さない。
	PostDeleted PostStatus = "deleted"
)

// BodyMaxLength は本文の最大文字数。
//
// DB の `posts.body VARCHAR(100)` と一致させる
// （基本設計 03 §2「body の上限を100文字とする根拠」）。
// 投稿は最大 20 モーラであり、すべて拗音でも 40 文字。
// 記号・空白の余地を含めて 100 文字あれば足りる。
const BodyMaxLength = 100

// Post は詠まれた五七五。
//
// **判定結果を含む。** 投稿時に確定し、以後変更されない（BR-04）。
// 表示のたびに再判定しないのは、辞書の更新で区切りが変わりうるためと、
// prosody が落ちていても閲覧を続けられるようにするためである（基本設計 01 §4）。
type Post struct {
	ID       int64
	AuthorID int64
	// Body は正規化後の本文。
	//
	// **利用者が送った文字列そのものではない。** Break1 / Break2 は
	// この文字列上の位置であり、元の入力を保存すると位置がずれる。
	Body    string
	Reading string
	Verdict Verdict
	// Break1 は上五と中七の境界。Body の文字位置（バイト位置ではない）。
	Break1 int
	// Break2 は中七と下五の境界。
	Break2     int
	MoraKami   int
	MoraNaka   int
	MoraShimo  int
	Visibility Visibility
	Status     PostStatus
	LikeCount  int
	CreatedAt  time.Time
	DeletedAt  *time.Time
}

// Segments は本文を上五・中七・下五に分けて返す。
//
// 3句を別々に保存せず、境界の位置だけを持つ。
// 分けて保存すると、連結して元の本文に戻らない状態を作れてしまう。
func (p *Post) Segments() [3]string {
	runes := []rune(p.Body)
	return [3]string{
		string(runes[:p.Break1]),
		string(runes[p.Break1:p.Break2]),
		string(runes[p.Break2:]),
	}
}

// IsVisibleTo は viewer がこの投稿を見られるかを返す。
//
// viewer が nil なら未ログイン。isFollower は viewer が投稿者を
// フォローしているか（フォロー機能は M3 だが、条件は先に入れておく。
// 後から絞る実装は忘れられるため）。
func (p *Post) IsVisibleTo(viewerID *int64, isFollower bool) bool {
	if p.Status != PostPublished {
		return false
	}
	if p.Visibility == VisibilityPublic {
		return true
	}
	if viewerID == nil {
		return false
	}
	// 投稿者本人は公開範囲にかかわらず見られる。
	return *viewerID == p.AuthorID || isFollower
}

// NewPost は判定結果から保存する投稿を組み立てる。
//
// **判定結果をクライアントから受け取らない。** サーバー側で再判定した
// *Analysis だけを材料にする（基本設計 01 §4）。
func NewPost(authorID int64, analysis *Analysis, visibility Visibility) (*Post, error) {
	if !analysis.Verdict.Postable() {
		return nil, fmt.Errorf("投稿できない判定です: verdict=%s", analysis.Verdict)
	}
	if len(analysis.Segments) != 3 {
		return nil, fmt.Errorf("句が3つではありません: %d", len(analysis.Segments))
	}

	// 文字数で数える。バイト数で数えると、本文を3句に復元できない。
	break1 := len([]rune(analysis.Segments[0].Text))
	break2 := break1 + len([]rune(analysis.Segments[1].Text))

	return &Post{
		AuthorID:   authorID,
		Body:       analysis.NormalizedText,
		Reading:    analysis.Reading,
		Verdict:    analysis.Verdict,
		Break1:     break1,
		Break2:     break2,
		MoraKami:   analysis.Segments[0].Mora,
		MoraNaka:   analysis.Segments[1].Mora,
		MoraShimo:  analysis.Segments[2].Mora,
		Visibility: visibility,
		Status:     PostPublished,
	}, nil
}

// PostRepository は投稿の永続化。
type PostRepository interface {
	Create(ctx context.Context, post *Post) (*Post, error)
	// FindByID は投稿と、その投稿者を返す。
	//
	// 応答に投稿者の識別名と表示名が要るため、1回のクエリで取る。
	// 削除済み・非表示の投稿も返す。表示できるかの判断は usecase が行う。
	FindByID(ctx context.Context, id int64) (*Post, *User, error)
	// Delete は論理削除する。status と deleted_at を同時に更新する。
	//
	// 片方だけ更新すると DB の整合性制約に弾かれる。
	Delete(ctx context.Context, id int64, now time.Time) error
	// IsFollowing は followerID が followeeID をフォローしているか。
	IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error)
	// IsLikedBy は userID がこの投稿にいいねしているか。
	IsLikedBy(ctx context.Context, postID, userID int64) (bool, error)
}
