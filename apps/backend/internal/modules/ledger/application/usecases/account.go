package usecases

import (
	"context"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// AccountUseCases จัดการผังบัญชี
type AccountUseCases struct {
	base
	accounts repositories.AccountRepository
}

func (uc *AccountUseCases) Get(ctx context.Context, in dto.ByID[vo.AccountID]) (dto.Account, error) {
	return read(ctx, uc.base, in.Actor, vo.PermAccountRead, func() (dto.Account, error) {
		return uc.accounts.Get(ctx, in.CompanyID, in.ID)
	})
}

func (uc *AccountUseCases) List(ctx context.Context, in dto.List[dto.AccountFilter]) (listing.Result[dto.Account], error) {
	return read(ctx, uc.base, in.Actor, vo.PermAccountRead, func() (listing.Result[dto.Account], error) {
		return uc.accounts.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *AccountUseCases) Create(ctx context.Context, in dto.Create[entities.AccountData]) (dto.Account, error) {
	var id vo.AccountID
	err := uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		parent, err := uc.parent(ctx, in.CompanyID, in.Data.ParentID)
		if err != nil {
			return err
		}
		a, err := entities.NewAccount(in.CompanyID, in.Data, parent)
		if err != nil {
			return err
		}
		id = a.ID()
		return uc.accounts.Create(ctx, a)
	})
	if err != nil {
		return dto.Account{}, err
	}
	return uc.accounts.Get(ctx, in.CompanyID, id)
}

// Update: โหลดบัญชี + parent + การใช้งาน -> ตรวจลูปลำดับชั้น -> กฎใน entity -> บันทึก (transaction เดียว)
func (uc *AccountUseCases) Update(ctx context.Context, in dto.Update[vo.AccountID, entities.AccountData]) (dto.Account, error) {
	err := uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		a, err := uc.accounts.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		parent, err := uc.parent(ctx, in.CompanyID, in.Data.ParentID)
		if err != nil {
			return err
		}
		if parent != nil {
			chain, err := uc.accounts.Ancestry(ctx, in.CompanyID, parent.ID())
			if err != nil {
				return err
			}
			if err := services.EnsureNoCycle(a.ID(), chain); err != nil {
				return err
			}
		}
		usage, err := uc.accounts.Usage(ctx, in.CompanyID, a.ID())
		if err != nil {
			return err
		}
		if err := a.Update(in.Data, parent, usage); err != nil {
			return err
		}
		return uc.accounts.Update(ctx, a)
	})
	if err != nil {
		return dto.Account{}, err
	}
	return uc.accounts.Get(ctx, in.CompanyID, in.ID)
}

// Delete: soft delete ได้เมื่อไม่มีบัญชีย่อยและไม่เคยถูกใช้ในรายการ
func (uc *AccountUseCases) Delete(ctx context.Context, in dto.ByID[vo.AccountID]) error {
	return uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		a, err := uc.accounts.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		usage, err := uc.accounts.Usage(ctx, in.CompanyID, a.ID())
		if err != nil {
			return err
		}
		if err := a.EnsureDeletable(usage); err != nil {
			return err
		}
		return uc.accounts.Delete(ctx, in.CompanyID, in.ID)
	})
}

func (uc *AccountUseCases) parent(ctx context.Context, companyID vo.CompanyID, id *vo.AccountID) (*entities.Account, error) {
	if id == nil {
		return nil, nil
	}
	return uc.accounts.FindByID(ctx, companyID, *id) // ไม่พบ = domain.ErrAccountNotFound
}
