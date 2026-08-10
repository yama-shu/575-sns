package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// SetCurrentUserForTest は認証済みの利用者を載せる。
//
// テストでミドルウェアを通さずにハンドラを直接呼ぶために使う。
// `currentUserKey` を公開する代わりにここへ閉じ込め、
// **本番のコードから利用者を差し替えられないようにする**
// （本ファイルはテストのビルドにしか含まれない）。
func SetCurrentUserForTest(c echo.Context, user *domain.User) {
	c.Set(currentUserKey, user)
}
