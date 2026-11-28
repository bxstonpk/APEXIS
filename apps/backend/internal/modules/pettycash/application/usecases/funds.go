package usecases

import (
	"context"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type FundUseCases struct{ base }

func (uc *FundUseCases) List(ctx context.Context, a dto.Actor, f dto.FundFilter) (listing.Result[dto.Fund], error) {
	return read(ctx, uc.base, a, vo.PermFundRead, func() (listing.Result[dto.Fund], error) {
		res, err := uc.funds.List(ctx, a.CompanyID, f)
		if err != nil {
			return res, err
		}
		return res, uc.fill(ctx, a.CompanyID, res.Items)
	})
}

func (uc *FundUseCases) Get(ctx context.Context, a dto.Actor, id vo.FundID) (dto.Fund, error) {
	return read(ctx, uc.base, a, vo.PermFundRead, func() (dto.Fund, error) { return uc.get(ctx, a.CompanyID, id) })
}

func (uc *FundUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.FundID) (dto.Fund, error) {
	f, err := uc.funds.Get(ctx, companyID, id)
	if err != nil {
		return f, err
	}
	items := []dto.Fund{f}
	if err := uc.fill(ctx, companyID, items); err != nil {
		return f, err
	}
	return items[0], nil
}

// fill เติมชื่อผู้ถือเงิน + รหัส / ชื่อบัญชี (2 query ไม่ว่ากี่กองทุน)
func (uc *FundUseCases) fill(ctx context.Context, companyID vo.CompanyID, items []dto.Fund) error {
	users := make([]*dto.Ref, len(items))
	accounts := make([]*dto.Ref, len(items))
	for i := range items {
		users[i], accounts[i] = &items[i].Custodian, &items[i].GLAccount
	}
	if err := uc.userRefs(ctx, companyID, users); err != nil {
		return err
	}
	return uc.accountRefs(ctx, companyID, accounts)
}

// Create: ผู้ถือเงินต้องเป็นสมาชิก active, บัญชี (ไม่ระบุ = เงินสดย่อยของผังมาตรฐาน) ต้องลงรายการได้
func (uc *FundUseCases) Create(ctx context.Context, in dto.FundInput) (dto.Fund, error) {
	var id vo.FundID
	err := uc.writeWith(ctx, in.Actor, vo.PermFundManage, func(ctx context.Context, dec int) error {
		d, err := uc.data(ctx, in)
		if err != nil {
			return err
		}
		f, err := entities.NewFund(d, dec)
		if err != nil {
			return err
		}
		id = f.ID()
		return uc.funds.Create(ctx, in.CompanyID, f)
	})
	if err != nil {
		return dto.Fund{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Update: วงเงินแก้ได้จนกว่าการเงินจ่ายใบขอตั้งวงเงิน
func (uc *FundUseCases) Update(ctx context.Context, in dto.FundInput) (dto.Fund, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermFundManage, func(ctx context.Context, dec int) error {
		f, err := uc.funds.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		d, err := uc.data(ctx, in)
		if err != nil {
			return err
		}
		bal, err := uc.funds.Balances(ctx, in.CompanyID, []vo.FundID{f.ID()}, nil)
		if err != nil {
			return err
		}
		if err := f.Update(d, bal[f.ID()].Funded, dec); err != nil {
			return err
		}
		return uc.funds.Update(ctx, in.CompanyID, f)
	})
	if err != nil {
		return dto.Fund{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *FundUseCases) data(ctx context.Context, in dto.FundInput) (entities.FundData, error) {
	names, err := uc.members.Names(ctx, in.CompanyID, []vo.UserID{in.CustodianID})
	if err != nil {
		return entities.FundData{}, err
	}
	if _, ok := names[in.CustodianID]; !ok {
		return entities.FundData{}, domain.ErrCustodianNotMember
	}
	gl := in.GLAccountID
	if gl == nil {
		found, err := uc.ledger.AccountsByCode(ctx, in.CompanyID, []string{pettyCashCode})
		if err != nil {
			return entities.FundData{}, err
		}
		id, ok := found[pettyCashCode]
		if !ok {
			return entities.FundData{}, wrap(domain.ErrAccountInvalid, "gl_account_id is required")
		}
		gl = &id
	} else if err := uc.checkAccounts(ctx, in.CompanyID, []vo.AccountID{*gl}); err != nil {
		return entities.FundData{}, err
	}
	return entities.FundData{Code: in.Code, Name: in.Name, CustodianID: in.CustodianID, GLAccountID: *gl,
		ImprestAmount: in.ImprestAmount, Status: in.Status}, nil
}
