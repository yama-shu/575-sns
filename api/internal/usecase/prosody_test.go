package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// fakeAnalyzer は判定エンジンの偽物。
//
// **prosody を起動しない。** domain.Analyzer を満たすだけの実装を差し込む
// （詳細設計 02 §2）。呼び出し回数も記録し、
// 「入力を検証せずに下流を叩く」実装を検出できるようにする。
type fakeAnalyzer struct {
	result  *domain.Analysis
	err     error
	calls   int
	gotText string
}

func (f *fakeAnalyzer) Analyze(_ context.Context, text string) (*domain.Analysis, error) {
	f.calls++
	f.gotText = text
	return f.result, f.err
}

func teikei() *domain.Analysis {
	return &domain.Analysis{
		Verdict:        domain.VerdictTeikei,
		NormalizedText: "今日もまた会議のための会議かな",
		Reading:        "キョウモマタカイギノタメノカイギカナ",
		TotalMora:      17,
		Segments: []domain.Segment{
			{Text: "今日もまた", Reading: "キョウモマタ", Mora: 5, Expected: 5},
			{Text: "会議のための", Reading: "カイギノタメノ", Mora: 7, Expected: 7},
			{Text: "会議かな", Reading: "カイギカナ", Mora: 5, Expected: 5},
		},
	}
}

func TestCheckReturnsAnalysis(t *testing.T) {
	analyzer := &fakeAnalyzer{result: teikei()}
	p := usecase.NewProsody(analyzer)

	got, err := p.Check(context.Background(), "今日もまた会議のための会議かな")
	if err != nil {
		t.Fatalf("判定できない: %v", err)
	}
	if got.Verdict != domain.VerdictTeikei {
		t.Errorf("判定結果が違う: %v", got.Verdict)
	}
	if analyzer.gotText != "今日もまた会議のための会議かな" {
		t.Errorf("本文が加工されて渡っている: %q", analyzer.gotText)
	}
}

// 破調は結果として返る。エラーにしない。
func TestCheckReturnsHachoAsResultNotError(t *testing.T) {
	analyzer := &fakeAnalyzer{result: &domain.Analysis{
		Verdict:   domain.VerdictHacho,
		Reason:    domain.ReasonTooFewMora,
		TotalMora: 8,
	}}
	p := usecase.NewProsody(analyzer)

	got, err := p.Check(context.Background(), "今日は疲れた")
	if err != nil {
		t.Fatalf("破調がエラーとして返された: %v", err)
	}
	if got.Verdict != domain.VerdictHacho || got.Reason != domain.ReasonTooFewMora {
		t.Errorf("判定結果が違う: %+v", got)
	}
}

// 入力の検証に落ちたら下流を呼ばないこと。
// 呼ぶと、明らかに無駄な負荷を prosody にかける。
func TestCheckRejectsInvalidBodyWithoutCallingAnalyzer(t *testing.T) {
	tests := map[string]string{
		"空":      "",
		"空白のみ":   "   ",
		"全角空白のみ": "　　",
		"改行のみ":   "\n\n",
		"長すぎる":   strings.Repeat("あ", domain.BodyMaxLength+1),
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			analyzer := &fakeAnalyzer{result: teikei()}
			p := usecase.NewProsody(analyzer)

			_, err := p.Check(context.Background(), body)
			var appErr *domain.Error
			if !errors.As(err, &appErr) || appErr.Code != domain.CodeValidationFailed {
				t.Fatalf("VALIDATION_FAILED を期待したが %v", err)
			}
			if analyzer.calls != 0 {
				t.Errorf("検証に落ちたのに下流を %d 回呼んだ", analyzer.calls)
			}
		})
	}
}

// 上限ちょうどは通ること。境界を固定する。
func TestCheckAcceptsBodyAtMaxLength(t *testing.T) {
	analyzer := &fakeAnalyzer{result: teikei()}
	p := usecase.NewProsody(analyzer)

	if _, err := p.Check(context.Background(), strings.Repeat("あ", domain.BodyMaxLength)); err != nil {
		t.Errorf("上限ちょうどが弾かれた: %v", err)
	}
}

// 文字数はルーン単位で数えること。
// バイト数で数えると、日本語が上限の 1/3 で弾かれる。
func TestCheckCountsLengthInRunesNotBytes(t *testing.T) {
	analyzer := &fakeAnalyzer{result: teikei()}
	p := usecase.NewProsody(analyzer)

	// 90 文字（UTF-8 で 270 バイト）。上限 100 文字を超えないため通る。
	if _, err := p.Check(context.Background(), strings.Repeat("あ", 90)); err != nil {
		t.Errorf("バイト数で数えている: %v", err)
	}
}

// 判定エンジンのエラーはそのまま伝えること。
// ここで握りつぶすと、利用者に 200 で「判定できません」を返すことになる。
func TestCheckPropagatesAnalyzerError(t *testing.T) {
	analyzer := &fakeAnalyzer{err: domain.ErrProsodyUnavailable}
	p := usecase.NewProsody(analyzer)

	_, err := p.Check(context.Background(), "今日もまた会議のための会議かな")
	if !errors.Is(err, domain.ErrProsodyUnavailable) {
		t.Errorf("PROSODY_UNAVAILABLE を期待したが %v", err)
	}
}
