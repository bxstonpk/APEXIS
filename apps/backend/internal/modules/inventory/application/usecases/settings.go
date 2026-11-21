package usecases

import (
	"context"
	"slices"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// SettingsUseCases: ค่าตั้งบัญชีสินค้า (บัญชีค่าเริ่มต้น, ต้นทุนขาย, สมุดรายวัน) และระบบบัญชีตามปีบัญชี
type SettingsUseCases struct {
	base
	settings repositories.SettingsRepository
}

func (uc *SettingsUseCases) Get(ctx context.Context, a dto.Actor) (dto.Settings, error) {
	return read(ctx, uc.base, a, vo.PermSettingsRead, func() (dto.Settings, error) {
		s, systems, err := uc.settings.Get(ctx, a.CompanyID, repositories.NoLock)
		return settingsDTO(s, systems), err
	})
}

// Update แทนที่ค่าตั้งทั้งก้อน — บัญชีต้องลงรายการได้, สมุดรายวันต้อง active
func (uc *SettingsUseCases) Update(ctx context.Context, in dto.Create[entities.Settings]) (dto.Settings, error) {
	err := uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error {
		if err := uc.checkAccounts(ctx, in.CompanyID, in.Data.AccountIDs()); err != nil {
			return err
		}
		if j := in.Data.JournalID; j != nil {
			ok, err := uc.ledger.JournalUsable(ctx, in.CompanyID, *j)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrJournalInvalid
			}
		}
		return uc.settings.Save(ctx, in.CompanyID, in.Data)
	})
	if err != nil {
		return dto.Settings{}, err
	}
	return uc.Get(ctx, in.Actor)
}

type SetSystemInput struct {
	dto.Actor
	FromYear int
	System   vo.AccountingSystem
}

// SetSystem: ใช้ระบบบัญชีตั้งแต่ปีบัญชี FromYear — ปีนั้นและปีถัดไปต้องยังไม่มีเอกสารสต็อกที่ผ่านแล้ว
// lock แถวค่าตั้งไว้ กันการลงบัญชีซื้อ (ที่อ่านระบบแบบ FOR SHARE) ระหว่างเปลี่ยน
func (uc *SettingsUseCases) SetSystem(ctx context.Context, in SetSystemInput) (dto.Settings, error) {
	err := uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error {
		if _, _, err := uc.settings.Get(ctx, in.CompanyID, repositories.ForUpdate); err != nil {
			return err
		}
		posted, err := uc.settings.HasPostedFrom(ctx, in.CompanyID, in.FromYear)
		if err != nil {
			return err
		}
		c, err := entities.NewSystemChange(in.FromYear, in.System, posted)
		if err != nil {
			return err
		}
		return uc.settings.SaveSystem(ctx, in.CompanyID, c)
	})
	if err != nil {
		return dto.Settings{}, err
	}
	return uc.Get(ctx, in.Actor)
}

func settingsDTO(s entities.Settings, systems []entities.SystemChange) dto.Settings {
	out := dto.Settings{
		Defaults:      accountsDTO(s.Defaults),
		CogsAccountID: optString(s.CogsAccountID),
		JournalID:     optString(s.JournalID),
		Systems:       make([]dto.SystemSetting, len(systems)),
	}
	slices.SortFunc(systems, func(a, b entities.SystemChange) int { return a.FromYear - b.FromYear })
	for i, c := range systems {
		out.Systems[i] = dto.SystemSetting{FromYear: c.FromYear, System: string(c.System)}
	}
	return out
}

func accountsDTO(g vo.GLAccounts) dto.Accounts {
	return dto.Accounts{PurchaseAccountID: optString(g.Purchase), InventoryAccountID: optString(g.Inventory)}
}

func optString[T interface{ String() string }](v *T) *string {
	if v == nil {
		return nil
	}
	s := (*v).String()
	return &s
}
