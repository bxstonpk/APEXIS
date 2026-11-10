package commands

import (
	"context"

	"account/backend/internal/modules/identity/application/dto"
	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/events"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type CreateCompany struct {
	ActorID vo.UserID
	Code    string
	Name    string
	TaxID   string
}

// CreateCompany สร้างบริษัท + role "owner" + ให้ผู้สร้างเป็น owner ใน transaction เดียว
//
// หมายเหตุ: ตอนนี้ user ที่ active ทุกคนสร้างบริษัทได้
// ถ้าต้องการจำกัดเฉพาะ platform admin ให้เพิ่มการตรวจตรงนี้
func (h *Handler) CreateCompany(ctx context.Context, cmd CreateCompany) (dto.Company, error) {
	now := h.now()
	company, err := entities.NewCompany(cmd.Code, cmd.Name, cmd.TaxID)
	if err != nil {
		return dto.Company{}, err
	}
	owner, err := entities.NewOwnerRole(company.ID())
	if err != nil {
		return dto.Company{}, err
	}
	membership, err := entities.NewMembership(cmd.ActorID, company.ID(), []*entities.Role{owner})
	if err != nil {
		return dto.Company{}, err
	}

	err = h.tx.WithinTx(ctx, func(ctx context.Context) error {
		user, err := h.users.FindByID(ctx, cmd.ActorID)
		if err != nil {
			return err
		}
		if !user.IsActive() {
			return domain.ErrUserInactive
		}
		if err := h.companies.Create(ctx, company); err != nil {
			return err
		}
		if err := h.roles.Create(ctx, owner); err != nil {
			return err
		}
		return h.memberships.Create(ctx, membership)
	})
	if err != nil {
		return dto.Company{}, err
	}
	h.publish(ctx, events.CompanyCreated{CompanyID: company.ID(), OwnerID: cmd.ActorID, At: now})
	return dto.FromCompany(company), nil
}

type UpdateCompany struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	Name      string
	TaxID     string
}

func (h *Handler) UpdateCompany(ctx context.Context, cmd UpdateCompany) (dto.Company, error) {
	var company *entities.Company
	err := h.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := h.access.Authorize(ctx, cmd.ActorID, cmd.CompanyID, vo.PermCompanyUpdate); err != nil {
			return err
		}
		c, err := h.companies.FindByID(ctx, cmd.CompanyID)
		if err != nil {
			return err
		}
		if err := c.UpdateProfile(cmd.Name, cmd.TaxID); err != nil {
			return err
		}
		company = c
		return h.companies.Update(ctx, c)
	})
	if err != nil {
		return dto.Company{}, err
	}
	return dto.FromCompany(company), nil
}
