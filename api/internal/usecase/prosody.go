package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// BodyMaxLength は判定にかける本文の最大文字数。
//
// 五七五は最大でも 18 モーラであり、漢字仮名交じりでも 100 文字を超えない。
// 上限を設けるのは、長大な入力で prosody の CPU を消費させないためである
// （判定は CPU バウンド。NFR-03-03）。
const BodyMaxLength = 140

// Prosody は判定の業務ロジック。
//
// **HTTP を知らない。** domain.Analyzer に対して操作するだけであり、
// 単体テストで prosody を起動する必要がない（詳細設計 02 §2）。
type Prosody struct {
	analyzer domain.Analyzer
}

// NewProsody は判定のユースケースをつくる。
func NewProsody(analyzer domain.Analyzer) *Prosody {
	return &Prosody{analyzer: analyzer}
}

// Check は本文を判定する。何も保存しない。
//
// 破調・unknown はエラーではなく結果として返る。
// 判定を求められて判定を返しているためである。
func (p *Prosody) Check(ctx context.Context, body string) (*domain.Analysis, error) {
	if err := validateBody(body); err != nil {
		return nil, err
	}
	return p.analyzer.Analyze(ctx, body)
}

func validateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return domain.NewValidationError("body", "本文を入力してください")
	}
	// 文字数はルーン単位で数える。バイト数で数えると日本語が 1/3 で弾かれる。
	if utf8.RuneCountInString(body) > BodyMaxLength {
		return domain.NewValidationError("body", "本文が長すぎます")
	}
	return nil
}
