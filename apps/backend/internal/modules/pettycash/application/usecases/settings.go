package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

func wrap(err error, detail string) error { return fmt.Errorf("%w: %s", err, detail) }

// ภาษีซื้อในผังบัญชีมาตรฐาน (ทุกประเภทธุรกิจ) และบัญชีเงินสดย่อย
const (
	inputVatCode  = "1107090000"
	pettyCashCode = "1101010200"
)

type SettingsUseCases struct{ base }

func (uc *SettingsUseCases) Get(ctx context.Context, a dto.Actor) (dto.Settings, error) {
	return read(ctx, uc.base, a, vo.PermFundRead, func() (dto.Settings, error) {
		s, err := uc.settings.Get(ctx, a.CompanyID)
		if err != nil {
			return dto.Settings{}, err
		}
		return uc.toDTO(ctx, a.CompanyID, s)
	})
}

func (uc *SettingsUseCases) Update(ctx context.Context, in dto.SettingsInput) (dto.Settings, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermFundManage, func(ctx context.Context, _ int) error {
		if id := in.Settings.InputVatAccountID; id != nil {
			if err := uc.checkAccounts(ctx, in.CompanyID, []vo.AccountID{*id}); err != nil {
				return err
			}
		}
		return uc.settings.Save(ctx, in.CompanyID, in.Settings)
	})
	if err != nil {
		return dto.Settings{}, err
	}
	return uc.Get(ctx, in.Actor)
}

// Provision: ค่าตั้งเริ่มต้นของบริษัทใหม่ — บัญชีภาษีซื้อจากผังบัญชีมาตรฐาน (ไม่พบ = ไม่ตั้ง)
// เรียกใน transaction ของการสร้างบริษัท (ไม่ตรวจสิทธิ์ — เป็นการตั้งค่าของระบบ)
func (uc *SettingsUseCases) Provision(ctx context.Context, companyID vo.CompanyID) error {
	found, err := uc.ledger.AccountsByCode(ctx, companyID, []string{inputVatCode})
	if err != nil {
		return err
	}
	var s entities.Settings
	if id, ok := found[inputVatCode]; ok {
		s.InputVatAccountID = &id
	}
	return uc.settings.Save(ctx, companyID, s)
}

func (uc *SettingsUseCases) toDTO(ctx context.Context, companyID vo.CompanyID, s entities.Settings) (dto.Settings, error) {
	var out dto.Settings
	if s.InputVatAccountID != nil {
		out.InputVatAccount = &dto.Ref{ID: s.InputVatAccountID.String()}
		if err := uc.accountRefs(ctx, companyID, []*dto.Ref{out.InputVatAccount}); err != nil {
			return out, err
		}
	}
	return out, nil
}
