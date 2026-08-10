package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// Follow はフォローの業務ロジック。
type Follow struct {
	users   domain.UserRepository
	follows domain.FollowRepository
}

// NewFollow をつくる。
func NewFollow(users domain.UserRepository, follows domain.FollowRepository) *Follow {
	return &Follow{users: users, follows: follows}
}

// Follow は handle の利用者をフォローする。
//
// **冪等。** すでにフォロー済みでも成功として扱う。
// 「フォローされている状態にする」という要求は満たされているためである。
func (f *Follow) Follow(ctx context.Context, actor *domain.User, handle string) (*domain.FollowState, error) {
	target, err := f.resolveTarget(ctx, actor, handle)
	if err != nil {
		return nil, err
	}
	if target.ID == actor.ID {
		return nil, domain.ErrCannotFollowSelf
	}

	// 自分がブロックしている相手はフォローできない。
	// 逆向き（相手が自分をブロックしている）は resolveTarget が 404 にしている。
	blocked, err := f.follows.IsBlocked(ctx, actor.ID, target.ID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, domain.ErrBlockedUser
	}

	if err := f.follows.Follow(ctx, actor.ID, target.ID); err != nil {
		return nil, err
	}
	return f.state(ctx, actor.ID, target.ID)
}

// Unfollow は handle の利用者へのフォローを解除する。
//
// **冪等。** フォローしていなくても成功として扱う。
//
// ブロックの有無を確認しない。ブロックしている相手への解除は
// 「フォローしていない状態にする」要求であり、拒む理由がない。
func (f *Follow) Unfollow(ctx context.Context, actor *domain.User, handle string) (*domain.FollowState, error) {
	target, err := f.resolveTarget(ctx, actor, handle)
	if err != nil {
		return nil, err
	}
	if target.ID == actor.ID {
		return nil, domain.ErrCannotFollowSelf
	}

	if err := f.follows.Unfollow(ctx, actor.ID, target.ID); err != nil {
		return nil, err
	}
	return f.state(ctx, actor.ID, target.ID)
}

// resolveTarget は操作の相手を引く。見えない相手は 404 にする。
//
// **相手が自分をブロックしている場合も 404。** BLOCKED_USER を返すと
// ブロックされた事実が分かり、BR-10 に反する。退会済み・存在しない識別名と
// 区別がつかないようにする。
func (f *Follow) resolveTarget(ctx context.Context, actor *domain.User, handle string) (*domain.User, error) {
	target, err := f.users.FindByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	// 退会済みは「無い」と同じ扱いにする。外部キーは行の存在しか見ないため、
	// ここで確かめないと退会した利用者をフォローできてしまう。
	if target.Status == domain.UserDeleted {
		return nil, domain.ErrNotFound
	}

	blockedByTarget, err := f.follows.IsBlocked(ctx, target.ID, actor.ID)
	if err != nil {
		return nil, err
	}
	if blockedByTarget {
		return nil, domain.ErrNotFound
	}
	return target, nil
}

// state は操作後の状態を返す。
func (f *Follow) state(ctx context.Context, actorID, targetID int64) (*domain.FollowState, error) {
	following, err := f.follows.IsFollowing(ctx, actorID, targetID)
	if err != nil {
		return nil, err
	}
	count, err := f.follows.CountFollowers(ctx, targetID)
	if err != nil {
		return nil, err
	}
	return &domain.FollowState{Following: following, FollowersCount: count}, nil
}
