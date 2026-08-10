package domain

import (
	"context"
	"time"
	"unicode/utf8"
)

// ReportReason は通報の理由（基本設計 03 §2）。
type ReportReason string

const (
	// ReportSpam は宣伝・スパム。
	ReportSpam ReportReason = "spam"
	// ReportHarassment は嫌がらせ。
	ReportHarassment ReportReason = "harassment"
	// ReportInappropriate は不適切な内容。
	ReportInappropriate ReportReason = "inappropriate"
	// ReportOther はその他。
	ReportOther ReportReason = "other"
)

// Valid は受け付ける値かを返す。DB の CHECK 制約と一致させる。
func (r ReportReason) Valid() bool {
	switch r {
	case ReportSpam, ReportHarassment, ReportInappropriate, ReportOther:
		return true
	}
	return false
}

// ReportStatus は通報の状態（基本設計 02 §4）。
type ReportStatus string

const (
	// ReportPending は未対応。
	ReportPending ReportStatus = "pending"
	// ReportResolved は対応済み（運営が投稿を非表示にした）。
	ReportResolved ReportStatus = "resolved"
	// ReportRejected は却下（運営が問題なしと判断した）。
	ReportRejected ReportStatus = "rejected"
)

// ReportCommentMaxLength はコメントの最大文字数。
// DB の `reports.comment VARCHAR(500)` と一致させる。
const ReportCommentMaxLength = 500

// Report は投稿への通報。
type Report struct {
	ID         int64
	ReporterID int64
	PostID     int64
	Reason     ReportReason
	Comment    string
	Status     ReportStatus
	CreatedAt  time.Time
}

// NewReport は通報をつくる。
func NewReport(reporterID, postID int64, reason ReportReason, comment string) (*Report, error) {
	if !reason.Valid() {
		return nil, NewValidationError("reason", "通報理由の指定が不正です")
	}
	if utf8.RuneCountInString(comment) > ReportCommentMaxLength {
		return nil, NewValidationError("comment", "コメントが長すぎます")
	}
	return &Report{
		ReporterID: reporterID,
		PostID:     postID,
		Reason:     reason,
		Comment:    comment,
		Status:     ReportPending,
	}, nil
}

// BlockState はブロック操作の結果。
type BlockState struct {
	Blocked bool
}

// ReportRepository は通報の永続化。
type ReportRepository interface {
	// Create は通報を1件作る。
	//
	// 同一利用者の同一投稿への重複は ErrAlreadyReported を返す。
	// **事前確認ではなく DB の UNIQUE 制約で防ぐ。** 確認と INSERT のあいだに
	// 同じ操作が挟まると、確認だけでは防げない。
	Create(ctx context.Context, report *Report) (*Report, error)
}

// BlockRepository はブロックの永続化。
type BlockRepository interface {
	// Block はブロックし、フォロー関係を双方向に解除する。
	//
	// **1トランザクションで行う。** 分けると「ブロックはできたが
	// フォローが残る」状態が生じ、BR-08 が防ごうとしたものがそのまま起きる。
	Block(ctx context.Context, blockerID, blockedID int64) error
	// Unblock はブロックを解除する。フォロー関係は復活しない。
	Unblock(ctx context.Context, blockerID, blockedID int64) error
	// IsBlocked は blockerID が blockedID をブロックしているか。
	IsBlocked(ctx context.Context, blockerID, blockedID int64) (bool, error)
	// IsBlockedEitherWay は2者のあいだにどちらの向きでもブロックがあるか。
	//
	// 可視性の判定に使う（BR-09）。片方向だけを見ると、
	// ブロックされた側が投稿を読み続けられてしまう。
	IsBlockedEitherWay(ctx context.Context, userA, userB int64) (bool, error)
}
