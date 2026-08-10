package usecase

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Post は投稿の業務ロジック。
type Post struct {
	posts      domain.PostRepository
	analyzer   domain.Analyzer
	visibility visibility
	now        Clock
}

// NewPost は投稿のユースケースをつくる。
func NewPost(
	posts domain.PostRepository,
	analyzer domain.Analyzer,
	blocks domain.BlockRepository,
	now Clock,
) *Post {
	if now == nil {
		now = time.Now
	}
	return &Post{
		posts:      posts,
		analyzer:   analyzer,
		visibility: visibility{posts: posts, blocks: blocks},
		now:        now,
	}
}

// CreateInput は投稿の入力。
//
// **判定結果を含まない。** クライアントが送ってきた判定は受け取らない
// （基本設計 01 §4）。受け取ると、「判定OK」という嘘を添えるだけで
// 破調が保存できてしまう。
type CreateInput struct {
	Author     *domain.User
	Body       string
	Visibility domain.Visibility
}

// PostView は投稿と、閲覧者から見た付随情報。
type PostView struct {
	Post   *domain.Post
	Author *domain.User
	// LikedByMe は閲覧者がいいねしているか。未ログインなら常に false。
	LikedByMe bool
}

// Create は本文を判定し、投稿できるものだけを保存する。
func (p *Post) Create(ctx context.Context, in CreateInput) (*PostView, error) {
	if err := validatePostBody(in.Body); err != nil {
		return nil, err
	}
	if in.Visibility == "" {
		in.Visibility = domain.VisibilityPublic
	}
	if !in.Visibility.Valid() {
		return nil, domain.NewValidationError("visibility", "公開範囲の指定が不正です")
	}

	// **ここが唯一の正。** 入力中の判定は体験のためのもので、信頼しない。
	analysis, err := p.analyzer.Analyze(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	if err := rejectUnpostable(analysis); err != nil {
		return nil, err
	}

	post, err := domain.NewPost(in.Author.ID, analysis, in.Visibility)
	if err != nil {
		return nil, err
	}
	// 正規化で 100 文字を超えることはないが、DB の制約に当てる前に確かめる。
	// 制約違反は 500 になり、利用者が直せるエラーとして返せない。
	if err := validatePostBody(post.Body); err != nil {
		return nil, err
	}

	created, err := p.posts.Create(ctx, post)
	if err != nil {
		return nil, err
	}
	return &PostView{Post: created, Author: in.Author}, nil
}

// Get は投稿を取得する。viewerID が nil なら未ログイン。
//
// 見られない投稿は 404 とする。403 にすると
// 「その ID の投稿は存在するがフォロワー限定である」ことを教えてしまう。
func (p *Post) Get(ctx context.Context, id int64, viewerID *int64) (*PostView, error) {
	post, author, err := p.visibility.resolve(ctx, id, viewerID)
	if err != nil {
		return nil, err
	}

	view := &PostView{Post: post, Author: author}
	if viewerID != nil {
		view.LikedByMe, err = p.posts.IsLikedBy(ctx, post.ID, *viewerID)
		if err != nil {
			return nil, err
		}
	}
	return view, nil
}

// Delete は投稿を論理削除する。削除できるのは投稿者本人だけ（BR-03）。
func (p *Post) Delete(ctx context.Context, id int64, userID int64) error {
	post, _, err := p.posts.FindByID(ctx, id)
	if err != nil {
		return err
	}
	// 削除済みは「無い」と同じ扱いにする。存在の判定より先に行うと、
	// 削除済み投稿の持ち主を他人が探れる。
	if post.Status == domain.PostDeleted {
		return domain.ErrNotFound
	}
	if post.AuthorID != userID {
		return domain.ErrForbidden
	}
	return p.posts.Delete(ctx, id, p.now())
}

// rejectUnpostable は投稿できない判定を、利用者が直せるエラーに変える。
//
// **破調と unknown を区別する。** 読めなかっただけの本文に
// 「五七五になっていません」と返すのは誤りで、利用者は直しようがない。
func rejectUnpostable(a *domain.Analysis) error {
	if a.Verdict.Postable() {
		return nil
	}
	if a.Verdict == domain.VerdictUnknown {
		return domain.ErrProsodyUnknownReading.WithDetails(map[string]any{
			"verdict":    string(a.Verdict),
			"unreadable": a.Unreadable,
		})
	}
	return domain.ErrProsodyHacho.WithDetails(map[string]any{
		"verdict":    string(a.Verdict),
		"total_mora": a.TotalMora,
		"reason":     string(a.Reason),
	})
}

// validatePostBody は本文の形を確かめる。判定と投稿で同じ規則を使う。
//
// 文字数はルーン単位で数える。バイト数で数えると日本語が上限の 1/3 で弾かれる。
// 上限は domain.BodyMaxLength（= DB の VARCHAR(100)）と一致させる。
func validatePostBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return domain.NewValidationError("body", "本文を入力してください")
	}
	if utf8.RuneCountInString(body) > domain.BodyMaxLength {
		return domain.NewValidationError("body", "本文が長すぎます")
	}
	return nil
}
