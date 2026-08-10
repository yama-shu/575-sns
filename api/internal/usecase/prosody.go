package usecase

import (
	"context"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

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
	if err := validatePostBody(body); err != nil {
		return nil, err
	}
	return p.analyzer.Analyze(ctx, body)
}
