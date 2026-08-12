package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// RelationListRepository はフォロー中・フォロワー・ブロック中の一覧。
type RelationListRepository struct {
	pool *pgxpool.Pool
}

// NewRelationListRepository をつくる。
func NewRelationListRepository(pool *pgxpool.Pool) *RelationListRepository {
	return &RelationListRepository{pool: pool}
}

// relationListColumns は一覧に出す項目。
//
// **プロフィールを引かない。** 投稿数などの数え上げが人数ぶん走る。
//
// `following` は EXISTS で同じクエリの中で解決する。1人ずつ問い合わせると、
// 50人の一覧で 51 回のクエリになる（N+1）。`follows` の主キーがこの検索に効く。
const relationListColumns = `
	u.id, u.handle, u.display_name, COALESCE(u.bio, ''), COALESCE(u.avatar_url, ''), u.status,
	CASE WHEN $2::bigint IS NULL THEN false ELSE EXISTS (
		SELECT 1 FROM follows f2 WHERE f2.follower_id = $2 AND f2.followee_id = u.id
	) END AS following`

// visibleToViewer は一覧から外す相手の条件。
//
// **閲覧者をブロックしている相手を外す。** 残すと、一覧に並ぶのに開くと 404 に
// なる相手が出る（#58 でプロフィールを 404 にしたため）。
//
// 利用停止・退会も外す。同じ理由である。
const visibleToViewer = `
	u.status = 'active'
	AND ($2::bigint IS NULL OR NOT EXISTS (
		SELECT 1 FROM blocks b WHERE b.blocker_id = u.id AND b.blocked_id = $2
	))`

// listQueries は関係ごとのクエリ。
//
// 並びは**相手の利用者 ID の降順**である。follows と blocks に id 列は無く、
// created_at は同時刻の並びが定まらない（基本設計 03 §5）。
//
// **実行計画のテストはこの定数を使う。** テスト側に書き写すと、
// 実装を変えたときにテストが古いクエリを検査し続ける（#41 で判明）。
var listQueries = map[domain.RelationListKind]string{
	domain.RelationFollowing: `
		SELECT ` + relationListColumns + `
		FROM follows f
		JOIN users u ON u.id = f.followee_id
		WHERE f.follower_id = $1
		  AND ($3::bigint = 0 OR u.id < $3)
		  AND ` + visibleToViewer + `
		ORDER BY u.id DESC
		LIMIT $4`,

	domain.RelationFollowers: `
		SELECT ` + relationListColumns + `
		FROM follows f
		JOIN users u ON u.id = f.follower_id
		WHERE f.followee_id = $1
		  AND ($3::bigint = 0 OR u.id < $3)
		  AND ` + visibleToViewer + `
		ORDER BY u.id DESC
		LIMIT $4`,

	domain.RelationBlocking: `
		SELECT ` + relationListColumns + `
		FROM blocks b
		JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		  AND ($3::bigint = 0 OR u.id < $3)
		  AND ` + visibleToViewer + `
		ORDER BY u.id DESC
		LIMIT $4`,
}

// List は一覧を返す。
func (r *RelationListRepository) List(
	ctx context.Context, q domain.RelationListQuery,
) ([]domain.RelationListItem, error) {
	query, ok := listQueries[q.Kind]
	if !ok {
		return nil, fmt.Errorf("一覧の種類が不正です: %s", q.Kind)
	}

	rows, err := r.pool.Query(ctx, query, q.OwnerID, q.ViewerID, q.Cursor, q.EffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("一覧を取得できません: %w", err)
	}
	return scanRelationList(rows)
}

func scanRelationList(rows pgx.Rows) ([]domain.RelationListItem, error) {
	defer rows.Close()

	items := []domain.RelationListItem{}
	for rows.Next() {
		var user domain.User
		var following bool
		if err := rows.Scan(
			&user.ID, &user.Handle, &user.DisplayName, &user.Bio, &user.AvatarURL, &user.Status,
			&following,
		); err != nil {
			return nil, fmt.Errorf("一覧を読み取れません: %w", err)
		}
		items = append(items, domain.RelationListItem{User: &user, Following: following})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("一覧を読み取れません: %w", err)
	}
	return items, nil
}
