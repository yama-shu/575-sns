package domain

import "context"

// Verdict は五七五の判定結果（基本設計 05）。
//
// **文字列のまま扱わない。** prosody が値を増やしたとき、
// 変換の時点で気づけるようにするためである。
type Verdict string

const (
	// VerdictTeikei は定型（5/7/5 ちょうど）。投稿できる。
	VerdictTeikei Verdict = "teikei"
	// VerdictKyoyo は許容（字余り・字足らずが許容範囲内）。投稿できる。
	VerdictKyoyo Verdict = "kyoyo"
	// VerdictHacho は破調。投稿できない。
	VerdictHacho Verdict = "hacho"
	// VerdictUnknown は読みを確定できず判定できない。投稿できない。
	//
	// **破調と区別する。** 読めなかっただけで「五七五になっていません」と
	// 伝えるのは誤りである。利用者は正しく詠んでいるかもしれない。
	VerdictUnknown Verdict = "unknown"
)

// Postable は判定結果が投稿を許すかを返す。
func (v Verdict) Postable() bool {
	return v == VerdictTeikei || v == VerdictKyoyo
}

// Valid は prosody が返しうる値かを返す。
func (v Verdict) Valid() bool {
	switch v {
	case VerdictTeikei, VerdictKyoyo, VerdictHacho, VerdictUnknown:
		return true
	}
	return false
}

// Reason は判定できなかった理由（基本設計 05）。
type Reason string

const (
	// ReasonTooFewMora は総モーラ数が少なすぎる。
	ReasonTooFewMora Reason = "TOO_FEW_MORA"
	// ReasonTooManyMora は総モーラ数が多すぎる。
	ReasonTooManyMora Reason = "TOO_MANY_MORA"
	// ReasonNoValidSplit はモーラ数は範囲内だが、許容範囲に収まる区切りが見つからない。
	ReasonNoValidSplit Reason = "NO_VALID_SPLIT"
	// ReasonReadingUnavailable は読みを取得できない語が含まれる。
	ReasonReadingUnavailable Reason = "READING_UNAVAILABLE"
)

// Valid は prosody が返しうる値かを返す。
func (r Reason) Valid() bool {
	switch r {
	case ReasonTooFewMora, ReasonTooManyMora, ReasonNoValidSplit, ReasonReadingUnavailable:
		return true
	}
	return false
}

// Segment は上五・中七・下五のいずれか1句。
type Segment struct {
	Text    string
	Reading string
	Mora    int
	// Expected は規定のモーラ数（5 / 7 / 5）。
	Expected int
	// Diff は規定との差。字余りが正、字足らずが負。
	Diff int
}

// Analysis は判定の結果。
//
// **破調・unknown もエラーではない。** 判定を求められて判定を返している。
type Analysis struct {
	Verdict Verdict
	// NormalizedText は正規化後の本文。
	//
	// 区切り位置はこの文字列上の位置である。元の入力を保存すると
	// 全角空白の圧縮などで位置がずれるため、保存するのはこちらである。
	NormalizedText string
	Reading        string
	TotalMora      int
	// Segments は teikei / kyoyo のときだけ入る。
	// 五七五に区切れない場合は区切りが定義できないため nil になる。
	Segments []Segment
	// Reason は hacho / unknown のときだけ入る。
	Reason Reason
	// Unreadable は unknown のとき、読めなかった語。
	Unreadable []string
}

// Analyzer は本文を判定する。実装は infra 層（prosody クライアント）に置く。
//
// usecase はこのインターフェースだけを知り、HTTP や JSON の形を知らない。
type Analyzer interface {
	Analyze(ctx context.Context, text string) (*Analysis, error)
}
