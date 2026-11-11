package usecases

import (
	"context"
	"errors"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	"apexis/backend/internal/modules/identity/domain/events"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

type AuthUseCases struct{ base }

// Register สร้างบัญชีผู้ใช้
// hash รหัสผ่าน (ใช้ CPU นาน) ก่อนเปิด transaction เพื่อให้ transaction สั้นที่สุด
func (uc *AuthUseCases) Register(ctx context.Context, in dto.RegisterUserInput) (dto.User, error) {
	email, err := vo.NewEmail(in.Email)
	if err != nil {
		return dto.User{}, err
	}
	if err := vo.ValidatePassword(in.Password); err != nil {
		return dto.User{}, err
	}
	hash, err := uc.deps.Hasher.Hash(in.Password)
	if err != nil {
		return dto.User{}, err
	}
	user, err := entities.NewUser(email, in.Name, hash)
	if err != nil {
		return dto.User{}, err
	}
	err = uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return uc.deps.Users.Create(ctx, user)
	})
	if err != nil {
		return dto.User{}, err
	}
	uc.publish(ctx, events.UserRegistered{UserID: user.ID(), Email: user.Email(), At: uc.now()})
	return dto.FromUser(user), nil
}

// Login ตรวจ email/password แล้วออก access token — ไม่บอกว่าผิดที่ email หรือ password
// เป็นการอ่านอย่างเดียว จึงไม่เปิด transaction
func (uc *AuthUseCases) Login(ctx context.Context, in dto.LoginInput) (dto.AuthToken, error) {
	email, err := vo.NewEmail(in.Email)
	if err != nil {
		return dto.AuthToken{}, domain.ErrInvalidCredentials
	}
	user, err := uc.deps.Users.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		// เสียเวลาใกล้เคียงกรณีพบ user เพื่อลด timing attack ที่ใช้เดาว่ามี email นี้
		_, _ = uc.deps.Hasher.Hash(in.Password)
		return dto.AuthToken{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return dto.AuthToken{}, err
	}
	if err := uc.deps.Hasher.Compare(user.PasswordHash(), in.Password); err != nil {
		return dto.AuthToken{}, domain.ErrInvalidCredentials
	}
	if !user.IsActive() {
		return dto.AuthToken{}, domain.ErrUserInactive
	}
	token, exp, err := uc.deps.Tokens.Issue(user.ID())
	if err != nil {
		return dto.AuthToken{}, err
	}
	return dto.AuthToken{AccessToken: token, ExpiresAt: exp, User: dto.FromUser(user)}, nil
}

// ChangePassword: hash รหัสใหม่ก่อนเปิด transaction แล้ว load -> ตรวจรหัสเดิม -> update ใน transaction เดียว
func (uc *AuthUseCases) ChangePassword(ctx context.Context, in dto.ChangePasswordInput) error {
	if err := vo.ValidatePassword(in.NewPassword); err != nil {
		return err
	}
	hash, err := uc.deps.Hasher.Hash(in.NewPassword)
	if err != nil {
		return err
	}
	return uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := uc.deps.Users.FindByID(ctx, in.UserID)
		if err != nil {
			return err
		}
		if err := uc.deps.Hasher.Compare(user.PasswordHash(), in.OldPassword); err != nil {
			return domain.ErrInvalidCredentials
		}
		if err := user.ChangePasswordHash(hash); err != nil {
			return err
		}
		return uc.deps.Users.Update(ctx, user)
	})
}

func (uc *AuthUseCases) GetMe(ctx context.Context, userID vo.UserID) (dto.User, error) {
	u, err := uc.deps.Users.FindByID(ctx, userID)
	if err != nil {
		return dto.User{}, err
	}
	return dto.FromUser(u), nil
}
