package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// ErrorBody は基本設計 05 のエラー形式。
//
//	{"error": {"code": "...", "message": "...", "details": {...}}}
//
// **code で分岐させる。** message は文言が変わりうるため、
// これで分岐するとクライアントが壊れる。
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail はエラーの中身。
type ErrorDetail struct {
	Code    domain.ErrorCode `json:"code"`
	Message string           `json:"message"`
	Details map[string]any   `json:"details,omitempty"`
}

// statusOf はエラーコードに対応する HTTP ステータス（基本設計 05）。
var statusOf = map[domain.ErrorCode]int{
	domain.CodeValidationFailed:      http.StatusBadRequest,
	domain.CodeUnauthenticated:       http.StatusUnauthorized,
	domain.CodeInvalidCredentials:    http.StatusUnauthorized,
	domain.CodeForbidden:             http.StatusForbidden,
	domain.CodeAccountSuspended:      http.StatusForbidden,
	domain.CodeNotFound:              http.StatusNotFound,
	domain.CodeHandleTaken:           http.StatusConflict,
	domain.CodeEmailTaken:            http.StatusConflict,
	domain.CodeCannotFollowSelf:      http.StatusUnprocessableEntity,
	domain.CodeCannotBlockSelf:       http.StatusUnprocessableEntity,
	domain.CodeCannotReportSelf:      http.StatusUnprocessableEntity,
	domain.CodeAlreadyReported:       http.StatusConflict,
	domain.CodeAlreadyHandled:        http.StatusConflict,
	domain.CodeBlockedUser:           http.StatusUnprocessableEntity,
	domain.CodeProsodyHacho:          http.StatusUnprocessableEntity,
	domain.CodeProsodyUnknownReading: http.StatusUnprocessableEntity,
	// 異常なエラー（詳細設計 03 §2）。
	domain.CodeProsodyUnavailable: http.StatusServiceUnavailable,
	domain.CodeUpstreamTimeout:    http.StatusGatewayTimeout,
}

// logLevelOf はエラーコードに対応するログレベル（詳細設計 03 §2）。
//
// 利用者の入力ミスで通知が飛ぶと、通知が意味を失う。
// FORBIDDEN だけ WARN にするのは、攻撃の兆候でありうるためである。
func logLevelOf(code domain.ErrorCode) slog.Level {
	// システム起因のものは ERROR。継続していれば人に届く必要がある。
	if code.IsAbnormal() {
		return slog.LevelError
	}
	if code == domain.CodeForbidden {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

// Respond はエラーを基本設計 05 の形式で返す。
//
// domain.Error は「正常なエラー」として 4xx で返す。
// それ以外は「想定外のエラー」として 500 を返し、**内部情報を含めない**。
func Respond(c echo.Context, err error) error {
	var appErr *domain.Error
	if errors.As(err, &appErr) {
		status, ok := statusOf[appErr.Code]
		if !ok {
			status = http.StatusBadRequest
		}
		body := ErrorBody{Error: ErrorDetail{
			Code: appErr.Code, Message: appErr.Message, Details: appErr.Details,
		}}
		if appErr.Field != "" {
			if body.Error.Details == nil {
				body.Error.Details = map[string]any{}
			}
			body.Error.Details["field"] = appErr.Field
		}
		slog.Log(c.Request().Context(), logLevelOf(appErr.Code), "リクエストを処理できませんでした",
			"event", "request_rejected",
			"error_code", string(appErr.Code),
			"path", c.Path(),
			"status", status,
		)
		return c.JSON(status, body)
	}

	// 想定外のエラー。回復手段が定義されていないため、
	// 処理を中断し、原因を追える情報を残し、人に知らせる（詳細設計 03 §1）。
	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	slog.ErrorContext(c.Request().Context(), "想定外のエラーが発生しました",
		"event", "internal_error",
		"error_code", "INTERNAL_ERROR",
		"path", c.Path(),
		"error_detail", err.Error(),
	)
	// **スタックトレース・SQL 文・内部のホスト名を返さない。**
	// 攻撃者にシステム構造の手がかりを与える。代わりに request_id を返し、
	// 問い合わせがあればログを引けるようにする。
	return c.JSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
		Code:    "INTERNAL_ERROR",
		Message: "エラーが発生しました",
		Details: map[string]any{"request_id": requestID},
	}})
}
