package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Profile はユーザーページの業務ロジック（S-04）。
//
// # ブロックの向きで扱いが変わる
//
// [BR-09] は「ブロックした相手の**投稿**は表示されない」であり、
// プロフィールそのものには触れていない。[BR-10] は「ブロックされた側は
// その事実を知らされない」である。両方を満たす扱いは向きで分かれる。
//
//	相手が閲覧者をブロック : 404。存在しない識別名と区別がつかないようにする
//	閲覧者が相手をブロック : プロフィールは見える。投稿は0件になる
//
// **閲覧者が相手をブロックした場合まで 404 にしない。** 404 にすると、
// 自分がブロックした相手のページから解除できなくなる。
// 解除の導線がブロック中一覧（S-11）にしか無いのは不便であり、
// BR-09 も BR-10 もそこまでは要求していない。
type Profile struct {
	users     domain.UserRepository
	profiles  domain.ProfileRepository
	timelines domain.TimelineRepository
	follows   domain.FollowRepository
	blocks    domain.BlockRepository
}

// NewProfile をつくる。
func NewProfile(
	users domain.UserRepository,
	profiles domain.ProfileRepository,
	timelines domain.TimelineRepository,
	follows domain.FollowRepository,
	blocks domain.BlockRepository,
) *Profile {
	return &Profile{
		users:     users,
		profiles:  profiles,
		timelines: timelines,
		follows:   follows,
		blocks:    blocks,
	}
}

// Get は handle のプロフィールを返す。viewerID が nil なら未ログイン。
func (p *Profile) Get(
	ctx context.Context, handle string, viewerID *int64,
) (*domain.Profile, error) {
	target, rel, err := p.resolve(ctx, handle, viewerID)
	if err != nil {
		return nil, err
	}

	counts, err := p.profiles.Counts(ctx, target.ID, rel.canSeeFollowersOnly)
	if err != nil {
		return nil, err
	}
	// ブロックした相手の投稿は見えない（BR-09）。数も0にする。
	// 見えない投稿を数に含めると、一覧と食い違う。
	if rel.blocking {
		counts.Posts = 0
	}

	return &domain.Profile{
		User:      target,
		Counts:    counts,
		Following: rel.following,
		Blocking:  rel.blocking,
	}, nil
}

// Posts は handle の投稿一覧を返す。
func (p *Profile) Posts(
	ctx context.Context, handle string, q domain.UserPostQuery,
) (*domain.Timeline, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	target, rel, err := p.resolve(ctx, handle, q.ViewerID)
	if err != nil {
		return nil, err
	}
	// ブロックした相手の投稿は1件も見えない（BR-09）。問い合わせるまでもない。
	if rel.blocking {
		return &domain.Timeline{Items: []domain.TimelineItem{}}, nil
	}

	q.AuthorID = target.ID
	q.IncludeFollowersOnly = rel.canSeeFollowersOnly

	items, err := p.timelines.UserPosts(ctx, q)
	if err != nil {
		return nil, err
	}
	return page(items, q.EffectiveLimit()), nil
}

// relation は閲覧者から見たこの利用者との関係。
type relation struct {
	following bool
	// blocking は閲覧者が相手をブロックしているか。
	blocking bool
	// canSeeFollowersOnly はフォロワー限定の投稿が見えるか。
	// 本人か、フォロワーであるときに真になる（FR-02-08）。
	canSeeFollowersOnly bool
}

// resolve は対象の利用者と、閲覧者から見た関係を返す。
//
// 見えない相手は ErrNotFound を返し、**理由を区別しない**。
// ブロックされていることを 403 や 422 で伝えると、その事実が分かってしまう（BR-10）。
//
// 利用停止も 404 にする。基本設計 02 §3 が「投稿は非表示になる」と
// しており、プロフィールだけ見えて投稿が空になるより一貫している。
func (p *Profile) resolve(
	ctx context.Context, handle string, viewerID *int64,
) (*domain.User, relation, error) {
	target, err := p.users.FindByHandle(ctx, handle)
	if err != nil {
		return nil, relation{}, err
	}
	if target.Status != domain.UserActive {
		return nil, relation{}, domain.ErrNotFound
	}

	if viewerID == nil {
		return target, relation{}, nil
	}
	if *viewerID == target.ID {
		// 本人。自分をフォローもブロックもできないため、確認する意味がない。
		return target, relation{canSeeFollowersOnly: true}, nil
	}

	// 相手が自分をブロックしている場合だけ 404 にする（BR-10）。
	blockedByTarget, err := p.blocks.IsBlocked(ctx, target.ID, *viewerID)
	if err != nil {
		return nil, relation{}, err
	}
	if blockedByTarget {
		return nil, relation{}, domain.ErrNotFound
	}

	blocking, err := p.blocks.IsBlocked(ctx, *viewerID, target.ID)
	if err != nil {
		return nil, relation{}, err
	}
	following, err := p.follows.IsFollowing(ctx, *viewerID, target.ID)
	if err != nil {
		return nil, relation{}, err
	}
	return target, relation{
		following:           following,
		blocking:            blocking,
		canSeeFollowersOnly: following,
	}, nil
}
