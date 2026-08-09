package prosody

import (
	"fmt"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// 本ファイルは prosody の JSON をそのまま写した型を持つ。
//
// **この形を上位へ漏らさない。** usecase は domain の型だけを知る
// （詳細設計 02 §2）。prosody が形を変えたとき、影響をここで止める。
//
// 契約は prosody/openapi.json。

type analyzeRequest struct {
	Text string `json:"text"`
}

type analyzeResponse struct {
	Verdict        string            `json:"verdict"`
	NormalizedText string            `json:"normalized_text"`
	Reading        *string           `json:"reading"`
	TotalMora      *int              `json:"total_mora"`
	Segments       []segmentResponse `json:"segments"`
	Reason         *string           `json:"reason"`
	Unreadable     []string          `json:"unreadable"`
}

type segmentResponse struct {
	Text     string `json:"text"`
	Reading  string `json:"reading"`
	Mora     int    `json:"mora"`
	Expected int    `json:"expected"`
	Diff     int    `json:"diff"`
}

// toDomain は domain の型へ変換する。
//
// 知らない verdict / reason を error にするのは、
// prosody が値を増やしたときに**黙って通さない**ためである。
// 通してしまうと、投稿可否の判断を誤ったまま保存する。
func (r analyzeResponse) toDomain() (*domain.Analysis, error) {
	verdict := domain.Verdict(r.Verdict)
	if !verdict.Valid() {
		return nil, fmt.Errorf("prosody が未知の判定を返しました: verdict=%q", r.Verdict)
	}

	var reason domain.Reason
	if r.Reason != nil {
		reason = domain.Reason(*r.Reason)
		if !reason.Valid() {
			return nil, fmt.Errorf("prosody が未知の理由を返しました: reason=%q", *r.Reason)
		}
	}

	analysis := &domain.Analysis{
		Verdict:        verdict,
		NormalizedText: r.NormalizedText,
		Reason:         reason,
		Unreadable:     r.Unreadable,
	}
	if r.Reading != nil {
		analysis.Reading = *r.Reading
	}
	if r.TotalMora != nil {
		analysis.TotalMora = *r.TotalMora
	}
	for _, s := range r.Segments {
		analysis.Segments = append(analysis.Segments, domain.Segment{
			Text:     s.Text,
			Reading:  s.Reading,
			Mora:     s.Mora,
			Expected: s.Expected,
			Diff:     s.Diff,
		})
	}
	return analysis, nil
}
