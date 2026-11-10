package commands

import (
	"context"
	"errors"

	"account/backend/internal/modules/identity/application/dto"
	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/events"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type RegisterUser struct {
	Email    string
	Name     string
	Password string
}

func (h *Handler) RegisterUser(ctx context.Context, cmd RegisterUser) (dto.User, error) {
	email, err := vo.NewEmail(cmd.Email)
	if err != nil {
		return dto.User{}, err
	}
	if err := vo.ValidatePassword(cmd.Password); err != nil {
		return dto.User{}, err
	}
	hash, err := h.hasher.Hash(cmd.Password)
	if err != nil {
		return dto.User{}, err
	}
	user, err := entities.NewUser(email, cmd.Name, hash)
	if err != nil {
		return dto.User{}, err
	}
	if err := h.users.Create(ctx, user); err != nil {
		return dto.User{}, err
	}
	h.publish(ctx, events.UserRegistered{UserID: user.ID(), Email: user.Email(), At: h.now()})
	return dto.FromUser(user), nil
}

type Login struct {
	Email    string
	Password string
}

// Login ตรวจ email/password แล้วออก access token — ไม่บอกว่าผิดที่ email หรือ password
func (h *Handler) Login(ctx context.Context, cmd Login) (dto.AuthToken, error) {
	email, err := vo.NewEmail(cmd.Email)
	if err != nil {
		return dto.AuthToken{}, domain.ErrInvalidCredentials
	}
	user, err := h.users.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		// เสียเวลาใกล้เคียงกรณีพบ user เพื่อลด timing attack ที่ใช้เดาว่ามี email นี้
		_, _ = h.hasher.Hash(cmd.Password)
		return dto.AuthToken{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return dto.AuthToken{}, err
	}
	if err := h.hasher.Compare(user.PasswordHash(), cmd.Password); err != nil {
		return dto.AuthToken{}, domain.ErrInvalidCredentials
	}
	if !user.IsActive() {
		return dto.AuthToken{}, domain.ErrUserInactive
	}
	token, exp, err := h.tokens.Issue(user.ID())
	if err != nil {
		return dto.AuthToken{}, err
	}
	return dto.AuthToken{AccessToken: token, ExpiresAt: exp, User: dto.FromUser(user)}, nil
}

type ChangePassword struct {
	UserID      vo.UserID
	OldPassword string
	NewPassword string
}

func (h *Handler) ChangePassword(ctx context.Context, cmd ChangePassword) error {
	if err := vo.ValidatePassword(cmd.NewPassword); err != nil {
		return err
	}
	user, err := h.users.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if err := h.hasher.Compare(user.PasswordHash(), cmd.OldPassword); err != nil {
		return domain.ErrInvalidCredentials
	}
	hash, err := h.hasher.Hash(cmd.NewPassword)
	if err != nil {
		return err
	}
	if err := user.ChangePasswordHash(hash); err != nil {
		return err
	}
	return h.users.Update(ctx, user)
}
