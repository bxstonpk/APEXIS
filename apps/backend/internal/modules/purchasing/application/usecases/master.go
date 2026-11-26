package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- ค่าตั้งจัดซื้อ ----------

type SettingsUseCases struct{ base }

func (uc *SettingsUseCases) Get(ctx context.Context, a dto.Actor) (dto.Settings, error) {
	return read(ctx, uc.base, a, vo.PermSettingsRead, func() (dto.Settings, error) {
		s, err := uc.settings.Get(ctx, a.CompanyID)
		return settingsDTO(s), err
	})
}

// Update แทนที่ค่าตั้งทั้งก้อน — บัญชีต้องลงรายการได้, สมุดรายวันต้อง active
// (อัตรา VAT ใหม่มีผลกับใบสั่งซื้อที่สร้างหลังจากนี้เท่านั้น)
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
		VatRate: s.VatRate, JournalID: optString(s.JournalID), GRIRAccountID: optString(s.GRIRAccountID),
		APAccountID: optString(s.APAccountID), InputVatAccountID: optString(s.InputVatAccountID),
		PriceVarianceAccountID: optString(s.PriceVarianceAccountID), PurchaseReturnsAccountID: optString(s.PurchaseReturnsAccountID),
	}
}

func optString[T interface{ String() string }](v *T) *string {
	if v == nil {
		return nil
	}
	s := (*v).String()
	return &s
}

// ---------- ผู้ขาย ----------

type SupplierUseCases struct {
	base
	repo repositories.SupplierRepository
}

func (uc *SupplierUseCases) Get(ctx context.Context, in dto.ByID[vo.SupplierID]) (dto.Supplier, error) {
	return read(ctx, uc.base, in.Actor, vo.PermSupplierRead, func() (dto.Supplier, error) { return uc.repo.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *SupplierUseCases) List(ctx context.Context, in dto.List[dto.SupplierFilter]) (listing.Result[dto.Supplier], error) {
	return read(ctx, uc.base, in.Actor, vo.PermSupplierRead, func() (listing.Result[dto.Supplier], error) {
		return uc.repo.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *SupplierUseCases) Create(ctx context.Context, in dto.Create[entities.SupplierData]) (dto.Supplier, error) {
	s, err := entities.NewSupplier(in.CompanyID, in.Data)
	if err != nil {
		return dto.Supplier{}, err
	}
	if err := uc.write(ctx, in.Actor, vo.PermSupplierManage, func(ctx context.Context) error {
		if err := uc.ensureTaxIDFree(ctx, in.CompanyID, s, nil); err != nil {
			return err
		}
		return uc.repo.Create(ctx, s)
	}); err != nil {
		return dto.Supplier{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, s.ID())
}

// Update: แก้สถานะจด VAT ได้ — ใบสั่งซื้อเดิมยังใช้เงื่อนไขที่เก็บไว้ตอนสร้าง
func (uc *SupplierUseCases) Update(ctx context.Context, in dto.Update[vo.SupplierID, entities.SupplierData]) (dto.Supplier, error) {
	err := uc.write(ctx, in.Actor, vo.PermSupplierManage, func(ctx context.Context) error {
		s, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := s.Update(in.Data); err != nil {
			return err
		}
		if err := uc.ensureTaxIDFree(ctx, in.CompanyID, s, &in.ID); err != nil {
			return err
		}
		return uc.repo.Update(ctx, s)
	})
	if err != nil {
		return dto.Supplier{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, in.ID)
}

// Delete: ผู้ขายที่มีใบสั่งซื้อแล้วลบไม่ได้ (ปิดใช้งานแทน)
func (uc *SupplierUseCases) Delete(ctx context.Context, in dto.ByID[vo.SupplierID]) error {
	return uc.write(ctx, in.Actor, vo.PermSupplierManage, func(ctx context.Context) error {
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

// ensureTaxIDFree: เลขผู้เสียภาษี 1 เลข = ผู้ขาย 1 รหัส — ซ้ำ = แจ้งรหัสเดิมให้ไปเพิ่มสาขาที่รหัสนั้น
// (ตรวจก่อนเพื่อข้อความที่ชัด; unique index ใน DB กันกรณีบันทึกพร้อมกัน)
func (uc *SupplierUseCases) ensureTaxIDFree(ctx context.Context, companyID vo.CompanyID, s *entities.Supplier, except *vo.SupplierID) error {
	code, err := uc.repo.TaxIDOwner(ctx, companyID, s.Data().TaxID, except)
	if err != nil {
		return err
	}
	if code != "" {
		return fmt.Errorf("%w: supplier %s", domain.ErrTaxIDExists, code)
	}
	return nil
}
