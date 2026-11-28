package usecases

import (
	"context"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/domain/entities"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

// SettingsUseCases คือค่าตั้งการผลิตของบริษัท
type SettingsUseCases struct{ base }

// Get: ค่าตั้ง 1 + ชื่อบัญชี 1 + สมุด 1 + คลัง 1 query
func (uc *SettingsUseCases) Get(ctx context.Context, a dto.Actor) (dto.Settings, error) {
	return read(ctx, uc.base, a, vo.PermSettingsRead, func() (dto.Settings, error) {
		s, err := uc.settings.Get(ctx, a.CompanyID)
		if err != nil {
			return dto.Settings{}, err
		}
		return uc.view(ctx, a.CompanyID, s)
	})
}

// Update แทนที่ค่าตั้งทั้งก้อน: บัญชีต้องลงรายการได้, สมุดต้อง active, คลังต้องใช้งานอยู่
func (uc *SettingsUseCases) Update(ctx context.Context, in dto.SettingsInput) (dto.Settings, error) {
	s := in.Settings
	err := uc.writeWith(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context, _ entities.Decimals) error {
		if ids := s.AccountIDs(); len(ids) > 0 {
			bad, err := uc.ledger.InvalidAccounts(ctx, in.CompanyID, ids)
			if err != nil {
				return err
			}
			if len(bad) > 0 {
				return domain.ErrAccountInvalid
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
		if s.DefaultWarehouseID != nil {
			if err := uc.activeWarehouse(ctx, in.CompanyID, *s.DefaultWarehouseID); err != nil {
				return err
			}
		}
		return uc.settings.Save(ctx, in.CompanyID, s)
	})
	if err != nil {
		return dto.Settings{}, err
	}
	return uc.view(ctx, in.CompanyID, s)
}

const (
	codeGeneralJournal = "GEN"
	codeWip            = "1105050000" // งานระหว่างผลิต
	codeApplied        = "5065000000" // ค่าแรงและค่าใช้จ่ายการผลิตปันเข้างาน
)

// Provision ตั้งค่าเริ่มต้นของบริษัทผลิตที่เพิ่งสร้างจากผังบัญชีมาตรฐาน (ประเภทอื่นไม่ทำอะไร)
// เรียกจาก hook ตอนสร้างบริษัท — ไม่ตรวจสิทธิ์, เข้าร่วม transaction ของผู้เรียก; บัญชีที่ไม่มีในผังจะไม่ตั้ง
// query: บัญชีตามรหัส 1 + สมุดตามรหัส 1 + UPSERT ค่าตั้ง 1
func (uc *SettingsUseCases) Provision(ctx context.Context, companyID vo.CompanyID, bt integration.BusinessType) error {
	if bt != integration.BusinessManufacturing {
		return nil
	}
	return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		accounts, err := uc.ledger.AccountsByCode(ctx, companyID, []string{codeWip, codeApplied})
		if err != nil {
			return err
		}
		journals, err := uc.ledger.JournalsByCode(ctx, companyID, []string{codeGeneralJournal})
		if err != nil {
			return err
		}
		s := entities.Settings{
			JournalID:           pick(journals, codeGeneralJournal),
			WipAccountID:        pick(accounts, codeWip),
			ConversionAccountID: pick(accounts, codeApplied),
		}
		return uc.settings.Save(ctx, companyID, s)
	})
}

func pick[T any](found map[string]T, code string) *T {
	if id, ok := found[code]; ok {
		return &id
	}
	return nil
}

func (uc *SettingsUseCases) view(ctx context.Context, companyID vo.CompanyID, s entities.Settings) (dto.Settings, error) {
	out := dto.Settings{Journal: optRef(s.JournalID), Wip: optRef(s.WipAccountID), Conversion: optRef(s.ConversionAccountID),
		Warehouse: optRef(s.DefaultWarehouseID)}
	accounts, err := uc.ledger.AccountRefs(ctx, companyID, s.AccountIDs())
	if err != nil {
		return out, err
	}
	for _, p := range []struct {
		id  *vo.AccountID
		ref *dto.Ref
	}{{s.WipAccountID, out.Wip}, {s.ConversionAccountID, out.Conversion}} {
		if p.id != nil {
			r := accounts[*p.id]
			p.ref.Code, p.ref.Name = r.Code, r.Name
		}
	}
	if s.JournalID != nil {
		journals, err := uc.ledger.JournalRefs(ctx, companyID, []vo.JournalID{*s.JournalID})
		if err != nil {
			return out, err
		}
		r := journals[*s.JournalID]
		out.Journal.Code, out.Journal.Name = r.Code, r.Name
	}
	if s.DefaultWarehouseID != nil {
		found, err := uc.inventory.Warehouses(ctx, companyID, []vo.WarehouseID{*s.DefaultWarehouseID})
		if err != nil {
			return out, err
		}
		w := found[*s.DefaultWarehouseID]
		out.Warehouse.Code, out.Warehouse.Name = w.Code, w.Name
	}
	return out, nil
}
