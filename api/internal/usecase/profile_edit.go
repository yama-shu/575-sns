package usecase

import (
	"context"
	"strings"

	"github.com/yama-shu/575-sns/api/internal/domain"
)

// UpdateProfileInput はプロフィールの更新内容。
//
// **ポインタで受ける。** nil は「触れていない」、空文字は「消す」である。
// 値型にすると、この2つを区別できない。自己紹介を消す手段が無くなる。
type UpdateProfileInput struct {
	DisplayName *string
	Bio         *string
}

// UpdateProfile は表示名と自己紹介を更新する（FR-01-03）。
//
// **アイコンは扱わない。** 画像を置く場所が設計されていない（#62）。
func (p *Profile) UpdateProfile(
	ctx context.Context, user *domain.User, in UpdateProfileInput,
) (*domain.User, error) {
	displayName := user.DisplayName
	if in.DisplayName != nil {
		// 前後の空白は落とす。空白だけの表示名を「入力済み」と扱わないため。
		displayName = strings.TrimSpace(*in.DisplayName)
		if err := domain.ValidateDisplayName(displayName); err != nil {
			return nil, err
		}
	}

	bio := user.Bio
	if in.Bio != nil {
		bio = strings.TrimSpace(*in.Bio)
		if err := domain.ValidateBio(bio); err != nil {
			return nil, err
		}
	}

	return p.users.UpdateProfile(ctx, user.ID, displayName, bio)
}
