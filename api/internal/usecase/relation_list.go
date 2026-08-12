package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// RelationList は関係の一覧の業務ロジック（S-05 / S-06 / S-11）。
type RelationList struct {
	users   domain.UserRepository
	lists   domain.RelationListRepository
	blocks  domain.BlockRepository
	follows domain.FollowRepository
}

// NewRelationList をつくる。
func NewRelationList(
	users domain.UserRepository,
	lists domain.RelationListRepository,
	blocks domain.BlockRepository,
	follows domain.FollowRepository,
) *RelationList {
	return &RelationList{users: users, lists: lists, blocks: blocks, follows: follows}
}

// OfUser は handle の利用者のフォロー中・フォロワー一覧を返す。
//
// ログインは不要。見えない相手は 404 とする（#58 と同じ扱い）。
func (r *RelationList) OfUser(
	ctx context.Context, handle string, q domain.RelationListQuery,
) (*domain.RelationList, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	target, err := r.resolve(ctx, handle, q.ViewerID)
	if err != nil {
		return nil, err
	}
	q.OwnerID = target.ID
	return r.list(ctx, q)
}

// Blocking は本人がブロックしている相手の一覧を返す（S-11）。
//
// **本人だけが見られる。** 誰をブロックしたかは他人に見せない。
func (r *RelationList) Blocking(
	ctx context.Context, viewer *domain.User, q domain.RelationListQuery,
) (*domain.RelationList, error) {
	if viewer == nil {
		return nil, domain.ErrUnauthenticated
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	q.Kind = domain.RelationBlocking
	q.OwnerID = viewer.ID
	q.ViewerID = &viewer.ID
	return r.list(ctx, q)
}

func (r *RelationList) list(
	ctx context.Context, q domain.RelationListQuery,
) (*domain.RelationList, error) {
	items, err := r.lists.List(ctx, q)
	if err != nil {
		return nil, err
	}

	list := &domain.RelationList{Items: items}
	// **limit に満たなければ続きが無い。** 常にカーソルを返すと、
	// クライアントが空のページを1回余計に取りに行く。
	if len(items) == q.EffectiveLimit() {
		list.NextCursor = items[len(items)-1].User.ID
	}
	return list, nil
}

// resolve は対象の利用者を引く。見えない相手は ErrNotFound を返す。
//
// **理由を区別しない**（BR-10）。#58 の Profile.resolve と同じ判断である。
func (r *RelationList) resolve(
	ctx context.Context, handle string, viewerID *int64,
) (*domain.User, error) {
	target, err := r.users.FindByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	if target.Status != domain.UserActive {
		return nil, domain.ErrNotFound
	}
	if viewerID == nil || *viewerID == target.ID {
		return target, nil
	}

	blockedByTarget, err := r.blocks.IsBlocked(ctx, target.ID, *viewerID)
	if err != nil {
		return nil, err
	}
	if blockedByTarget {
		return nil, domain.ErrNotFound
	}
	return target, nil
}
