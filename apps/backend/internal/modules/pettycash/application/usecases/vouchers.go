package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type VoucherUseCases struct{ base }

func (uc *VoucherUseCases) List(ctx context.Context, a dto.Actor, f dto.VoucherFilter) (listing.Result[dto.VoucherSummary], error) {
	return read(ctx, uc.base, a, vo.PermVoucherRead, func() (listing.Result[dto.VoucherSummary], error) {
		return uc.vouchers.List(ctx, a.CompanyID, f)
	})
}

func (uc *VoucherUseCases) Get(ctx context.Context, a dto.Actor, id vo.VoucherID) (dto.Voucher, error) {
	return read(ctx, uc.base, a, vo.PermVoucherRead, func() (dto.Voucher, error) { return uc.get(ctx, a.CompanyID, id) })
}

func (uc *VoucherUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.VoucherID) (dto.Voucher, error) {
	v, err := uc.vouchers.Get(ctx, companyID, id)
	if err != nil {
		return v, err
	}
	accounts := make([]*dto.Ref, len(v.Lines))
	for i := range v.Lines {
		accounts[i] = &v.Lines[i].Account
	}
	if err := uc.accountRefs(ctx, companyID, accounts); err != nil {
		return v, err
	}
	return v, uc.userRefs(ctx, companyID, []*dto.Ref{&v.CreatedBy, v.ApprovedBy})
}

// Create บันทึกร่าง + ออกเลขที่ (กองทุนต้องใช้งานอยู่, บัญชีของบรรทัดต้องลงรายการได้)
func (uc *VoucherUseCases) Create(ctx context.Context, in dto.VoucherInput) (dto.Voucher, error) {
	var id vo.VoucherID
	err := uc.writeWith(ctx, in.Actor, vo.PermVoucherManage, func(ctx context.Context, dec int) error {
		if err := uc.check(ctx, in); err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocVoucher, in.Data.Date)
		if err != nil {
			return err
		}
		v, err := entities.NewVoucher(in.CompanyID, no, in.Data, in.UserID, dec)
		if err != nil {
			return err
		}
		id = v.ID()
		return uc.vouchers.Create(ctx, v)
	})
	if err != nil {
		return dto.Voucher{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

func (uc *VoucherUseCases) Update(ctx context.Context, in dto.VoucherInput) (dto.Voucher, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermVoucherManage, func(ctx context.Context, dec int) error {
		v, err := uc.vouchers.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if v.State().Status != vo.VoucherDraft {
			return domain.ErrVoucherNotDraft
		}
		if err := uc.check(ctx, in); err != nil {
			return err
		}
		if _, err := uc.org.PeriodAt(ctx, in.CompanyID, in.Data.Date); err != nil {
			return err
		}
		if err := v.Update(in.Data, dec); err != nil {
			return err
		}
		return uc.vouchers.Update(ctx, v)
	})
	if err != nil {
		return dto.Voucher{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *VoucherUseCases) check(ctx context.Context, in dto.VoucherInput) error {
	f, err := uc.funds.FindForUpdate(ctx, in.CompanyID, in.Data.FundID)
	if err != nil {
		return err
	}
	if !f.IsActive() {
		return domain.ErrFundInactive
	}
	ids := make([]vo.AccountID, 0, len(in.Data.Lines))
	for _, l := range in.Data.Lines {
		ids = append(ids, l.AccountID)
	}
	return uc.checkAccounts(ctx, in.CompanyID, ids)
}

// Approve: lock กองทุน -> ต้องตั้งวงเงินแล้ว + เงินคงเหลือพอ -> อนุมัติ (เงินออกจากกองทุน)
func (uc *VoucherUseCases) Approve(ctx context.Context, in dto.ByVoucher) (dto.Voucher, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermVoucherApprove, func(ctx context.Context, _ int) error {
		v, err := uc.vouchers.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		st := v.State()
		f, err := uc.funds.FindForUpdate(ctx, in.CompanyID, st.FundID)
		if err != nil {
			return err
		}
		if !f.IsActive() {
			return domain.ErrFundInactive
		}
		bal, err := uc.funds.Balances(ctx, in.CompanyID, []vo.FundID{f.ID()}, nil)
		if err != nil {
			return err
		}
		b := bal[f.ID()]
		if !b.Funded {
			return domain.ErrFundNotFunded
		}
		if b.CashOnHand.LessThan(st.Totals.Gross) {
			return fmt.Errorf("%w: cash on hand %s, voucher %s", domain.ErrInsufficientCash, b.CashOnHand, st.Totals.Gross)
		}
		if err := v.Approve(in.UserID, uc.now()); err != nil {
			return err
		}
		return uc.vouchers.Update(ctx, v)
	})
	if err != nil {
		return dto.Voucher{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

// Cancel: ร่าง (สิทธิ์จัดการ) / อนุมัติแล้วที่ยังไม่อยู่ในคำขอ (สิทธิ์อนุมัติ)
func (uc *VoucherUseCases) Cancel(ctx context.Context, in dto.ByVoucher) (dto.Voucher, error) {
	err := uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.auth.Authorize(ctx, in.UserID, in.CompanyID, vo.PermVoucherRead); err != nil {
			return err
		}
		v, err := uc.vouchers.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		perm := vo.PermVoucherManage
		if v.State().Status != vo.VoucherDraft {
			perm = vo.PermVoucherApprove
		}
		return uc.writeWith(ctx, in.Actor, perm, func(ctx context.Context, _ int) error {
			if err := v.Cancel(uc.now()); err != nil {
				return err
			}
			return uc.vouchers.Update(ctx, v)
		})
	})
	if err != nil {
		return dto.Voucher{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}
