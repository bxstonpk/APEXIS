package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- ค่าตั้งขาย ----------

type SettingsUseCases struct{ base }

func (uc *SettingsUseCases) Get(ctx context.Context, a dto.Actor) (dto.Settings, error) {
	return read(ctx, uc.base, a, vo.PermSettingsRead, func() (dto.Settings, error) {
		s, err := uc.settings.Get(ctx, a.CompanyID)
		return settingsDTO(s), err
	})
}

// Update แทนที่ทั้งก้อน — มีผลกับเอกสารที่สร้างหลังจากนี้ (เอกสารเดิมเก็บเงื่อนไข VAT ไว้แล้ว)
func (uc *SettingsUseCases) Update(ctx context.Context, in dto.Create[entities.Settings]) (dto.Settings, error) {
	s, err := entities.NewSettings(in.Data)
	if err != nil {
		return dto.Settings{}, err
	}
	err = uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error {
		if ids := s.AccountIDs(); len(ids) > 0 {
			bad, err := uc.ledger.InvalidAccounts(ctx, in.CompanyID, ids)
			if err != nil {
				return err
			}
			if len(bad) > 0 {
				return fmt.Errorf("%w: %s", domain.ErrAccountInvalid, bad[0])
			}
		}
		if s.JournalID != nil {
			ok, err := uc.ledger.JournalUsable(ctx, in.CompanyID, *s.JournalID)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrJournalInvalid
			}
		}
		return uc.settings.Save(ctx, in.CompanyID, s)
	})
	if err != nil {
		return dto.Settings{}, err
	}
	return uc.Get(ctx, in.Actor)
}

func settingsDTO(s entities.Settings) dto.Settings {
	return dto.Settings{
		VatRegistered: s.VatRegistered, VatRate: s.VatRate, JournalID: optString(s.JournalID), ARAccountID: optString(s.ARAccountID),
		OutputVatAccountID: optString(s.OutputVatAccountID), RevenueAccountID: optString(s.RevenueAccountID),
		CogsAccountID: optString(s.CogsAccountID), WhtAccountID: optString(s.WhtAccountID),
	}
}

// ---------- ลูกค้า ----------

type CustomerUseCases struct {
	base
	repo repositories.CustomerRepository
}

func (uc *CustomerUseCases) Get(ctx context.Context, in dto.ByID[vo.CustomerID]) (dto.Customer, error) {
	return read(ctx, uc.base, in.Actor, vo.PermCustomerRead, func() (dto.Customer, error) { return uc.repo.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *CustomerUseCases) List(ctx context.Context, in dto.List[dto.CustomerFilter]) (listing.Result[dto.Customer], error) {
	return read(ctx, uc.base, in.Actor, vo.PermCustomerRead, func() (listing.Result[dto.Customer], error) {
		return uc.repo.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *CustomerUseCases) Create(ctx context.Context, in dto.Create[entities.CustomerData]) (dto.Customer, error) {
	var id vo.CustomerID
	err := uc.writeWith(ctx, in.Actor, vo.PermCustomerManage, func(ctx context.Context, s ports.CompanySettings) error {
		c, err := entities.NewCustomer(in.CompanyID, in.Data, s.AmountDecimals)
		if err != nil {
			return err
		}
		id = c.ID()
		return uc.repo.Create(ctx, c)
	})
	if err != nil {
		return dto.Customer{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, id)
}

func (uc *CustomerUseCases) Update(ctx context.Context, in dto.Update[vo.CustomerID, entities.CustomerData]) (dto.Customer, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermCustomerManage, func(ctx context.Context, s ports.CompanySettings) error {
		c, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := c.Update(in.Data, s.AmountDecimals); err != nil {
			return err
		}
		return uc.repo.Update(ctx, c)
	})
	if err != nil {
		return dto.Customer{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, in.ID)
}

// Delete: ลูกค้าที่มีเอกสารแล้วลบไม่ได้ (ปิดใช้งานแทน)
func (uc *CustomerUseCases) Delete(ctx context.Context, in dto.ByID[vo.CustomerID]) error {
	return uc.write(ctx, in.Actor, vo.PermCustomerManage, func(ctx context.Context) error {
		if _, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID); err != nil {
			return err
		}
		used, err := uc.repo.InUse(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := entities.EnsureUnused(used); err != nil {
			return err
		}
		return uc.repo.Delete(ctx, in.CompanyID, in.ID)
	})
}
