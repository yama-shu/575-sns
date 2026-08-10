package postgres

// 実行計画のテストが**実物のクエリ**を検査できるようにする。
//
// テスト側にクエリを書き写すと、実装を変えたときにテストが
// 古いクエリを検査し続ける（#41 で実際に起きた）。
// 本ファイルはテストのビルドにしか含まれない。
const (
	PublicTimelineQueryForTest = publicTimelineQuery
	HomeTimelineQueryForTest   = homeTimelineQuery
)
