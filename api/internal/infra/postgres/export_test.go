package postgres

import "github.com/yama-shu/575-sns/api/internal/domain"

// 実行計画のテストが**実物のクエリ**を検査できるようにする。
//
// テスト側にクエリを書き写すと、実装を変えたときにテストが
// 古いクエリを検査し続ける（#41 で実際に起きた）。
// 本ファイルはテストのビルドにしか含まれない。
const (
	PublicTimelineQueryForTest = publicTimelineQuery
	HomeTimelineQueryForTest   = homeTimelineQuery
	UserPostsQueryForTest      = userPostsQuery
	ProfileCountsQueryForTest  = profileCountsQuery
)

// RelationListQueryForTest は関係の一覧のクエリ。実行計画の検査に使う。
func RelationListQueryForTest(kind domain.RelationListKind) string { return listQueries[kind] }
