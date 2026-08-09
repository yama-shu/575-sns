package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/usecase"
)

// Prosody は判定のハンドラ。
type Prosody struct {
	prosody *usecase.Prosody
}

// NewProsody をつくる。
func NewProsody(prosody *usecase.Prosody) *Prosody {
	return &Prosody{prosody: prosody}
}

type checkRequest struct {
	Body string `json:"body"`
}

// checkResponse は基本設計 05 の判定レスポンス。
//
// prosody の応答をほぼそのまま転送する。
// `normalized_text` を返さないのは、投稿時にサーバー側で保存するものであり、
// 入力中のプレビューに必要ないためである。
type checkResponse struct {
	Verdict   domain.Verdict `json:"verdict"`
	Reading   string         `json:"reading,omitempty"`
	TotalMora *int           `json:"total_mora"`
	// Segments は五七五に区切れない場合 null になる。
	// 区切れないため区切りが定義できない。
	Segments []segmentResponse `json:"segments"`
	Reason   domain.Reason     `json:"reason,omitempty"`
	// Unreadable は unknown のとき、読めなかった語。
	//
	// **これを返さないと利用者が直せない。** 「五七五になっていません」とだけ
	// 言われても、正しく詠んでいる利用者には直しようがない。
	Unreadable []string `json:"unreadable,omitempty"`
}

type segmentResponse struct {
	Text     string `json:"text"`
	Reading  string `json:"reading"`
	Mora     int    `json:"mora"`
	Expected int    `json:"expected"`
	Diff     int    `json:"diff"`
}

// Check は POST /api/v1/prosody/check。入力中の判定に使う。何も保存しない。
//
// **破調でも 200 を返す。** 判定を求められて判定を返しているため、
// エラーではない（基本設計 05）。
func (h *Prosody) Check(c echo.Context) error {
	var req checkRequest
	if err := c.Bind(&req); err != nil {
		return Respond(c, domain.NewValidationError("body", "リクエストの形式が不正です"))
	}

	analysis, err := h.prosody.Check(c.Request().Context(), req.Body)
	if err != nil {
		return Respond(c, err)
	}
	return c.JSON(http.StatusOK, toCheckResponse(analysis))
}

func toCheckResponse(a *domain.Analysis) checkResponse {
	res := checkResponse{
		Verdict:    a.Verdict,
		Reading:    a.Reading,
		Reason:     a.Reason,
		Unreadable: a.Unreadable,
	}
	// unknown は読みを確定できていないため、モーラ数を数えられていない。
	// 0 を返すと「0 モーラだった」と読めてしまうため null にする。
	if a.Verdict != domain.VerdictUnknown {
		total := a.TotalMora
		res.TotalMora = &total
	}
	for _, s := range a.Segments {
		res.Segments = append(res.Segments, segmentResponse{
			Text:     s.Text,
			Reading:  s.Reading,
			Mora:     s.Mora,
			Expected: s.Expected,
			Diff:     s.Diff,
		})
	}
	return res
}
