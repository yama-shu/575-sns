package domain

import (
	"context"
	"time"
)

// PendingReport は運営が判断するための1件（S-13）。
//
// **投稿の本文と投稿者を含める。** 運営は投稿を見なければ判断できない。
// 通報だけを返して1件ずつ投稿を引くと N+1 になる。
type PendingReport struct {
	Report   *Report
	Post     *Post
	Author   *User
	Reporter *User
}

// PendingReportQuery は通報一覧の取得条件。
type PendingReportQuery struct {
	// Cursor はこの ID より後を取得する。0 なら先頭から。
	//
	// **タイムラインと向きが逆である。** 通報は古い順に処理するため、
	// カーソルも「これより後」を指す。
	Cursor int64
	Limit  *int
}

// Validate は取得条件を検証し、未指定の項目に既定値を入れる。
func (q *PendingReportQuery) Validate() error {
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
func (q PendingReportQuery) EffectiveLimit() int {
	if q.Limit == nil {
		return DefaultTimelineLimit
	}
	return *q.Limit
}

// PendingReports は1ページぶんの通報一覧。
type PendingReports struct {
	Items []PendingReport
	// NextCursor は次のページの起点。続きが無ければ 0。
	NextCursor int64
}

// AdminRepository は運営の操作。
type AdminRepository interface {
	// PendingReports は未対応の通報を古い順に返す。
	PendingReports(ctx context.Context, q PendingReportQuery) ([]PendingReport, error)
	// Resolve は投稿を非表示にし、その投稿への未対応の通報をすべて対応済みにする。
	//
	// **1トランザクションで行う。** 分けると「投稿は消えたが通報が未対応のまま」
	// 「通報は閉じたが投稿が見えたまま」が生じる。
	//
	// 基本設計 02 §4 の「運営は投稿単位で対応し、その投稿に紐づく未対応の通報を
	// まとめて処理する」に従う。
	//
	// 対象の通報が未対応でなければ ErrAlreadyHandled を返す。
	Resolve(ctx context.Context, reportID, adminID int64, now time.Time) error
	// Reject は通報を却下する。**投稿は変えない。**
	//
	// 同じ投稿への未対応の通報もまとめて却下する。
	Reject(ctx context.Context, reportID, adminID int64, now time.Time) error
}
