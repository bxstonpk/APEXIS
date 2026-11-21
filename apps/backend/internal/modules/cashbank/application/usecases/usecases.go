// Package usecases คือ business operation ของ cashbank (ทะเบียนเงินสด / บัญชีธนาคาร)
package usecases

import (
	"context"

	"apexis/backend/internal/modules/cashbank/application/dto"
	"apexis/backend/internal/modules/cashbank/application/ports"
	"apexis/backend/internal/modules/cashbank/application/ports/repositories"
	"apexis/backend/internal/modules/cashbank/domain/entities"
	domain "apexis/backend/internal/modules/cashbank/domain/errors"
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type Deps struct {
	Tx       ports.TransactionManager
	Auth     ports.Authorizer
	Org      ports.Organization
	Ledger   ports.Ledger
	Accounts repositories.CashAccountRepository
}

type UseCases struct {
	Accounts *AccountUseCases
}

func New(d Deps) *UseCases {
	return &UseCases{Accounts: &AccountUseCases{d: d}}
}

type AccountUseCases struct{ d Deps }

func (uc *AccountUseCases) authorize(ctx context.Context, a dto.Actor, p vo.Permission) error {
	return uc.d.Auth.Authorize(ctx, a.UserID, a.CompanyID, p)
}

// write: transaction -> สิทธิ์ -> บริษัทต้อง active -> fn
func (uc *AccountUseCases) write(ctx context.Context, a dto.Actor, fn func(ctx context.Context) error) error {
	return uc.d.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.authorize(ctx, a, vo.PermAccountManage); err != nil {
			return err
		}
		active, err := uc.d.Org.CompanyActive(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if !active {
			return domain.ErrCompanyInactive
		}
		return fn(ctx)
	})
}

func (uc *AccountUseCases) Get(ctx context.Context, in dto.ByID) (dto.CashAccount, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermAccountRead); err != nil {
		return dto.CashAccount{}, err
	}
	return uc.d.Accounts.Get(ctx, in.CompanyID, in.ID)
}

func (uc *AccountUseCases) List(ctx context.Context, a dto.Actor, f dto.Filter) (listing.Result[dto.CashAccount], error) {
	if err := uc.authorize(ctx, a, vo.PermAccountRead); err != nil {
		return listing.Result[dto.CashAccount]{}, err
	}
	return uc.d.Accounts.List(ctx, a.CompanyID, f)
}

func (uc *AccountUseCases) Create(ctx context.Context, a dto.Actor, d entities.CashAccountData) (dto.CashAccount, error) {
	acc, err := entities.NewCashAccount(a.CompanyID, d)
	if err != nil {
		return dto.CashAccount{}, err
	}
	err = uc.write(ctx, a, func(ctx context.Context) error {
		if err := uc.checkGL(ctx, a.CompanyID, d.GLAccountID); err != nil {
			return err
		}
		return uc.d.Accounts.Create(ctx, acc)
	})
	if err != nil {
		return dto.CashAccount{}, err
	}
	return uc.d.Accounts.Get(ctx, a.CompanyID, acc.ID())
}

func (uc *AccountUseCases) Update(ctx context.Context, in dto.ByID, d entities.CashAccountData) (dto.CashAccount, error) {
	err := uc.write(ctx, in.Actor, func(ctx context.Context) error {
		acc, err := uc.d.Accounts.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := acc.Update(d); err != nil {
			return err
		}
		if err := uc.checkGL(ctx, in.CompanyID, d.GLAccountID); err != nil {
			return err
		}
		return uc.d.Accounts.Update(ctx, acc)
	})
	if err != nil {
		return dto.CashAccount{}, err
	}
	return uc.d.Accounts.Get(ctx, in.CompanyID, in.ID)
}

// Delete: soft delete — เอกสารรับ / จ่ายเดิมยังอ้าง id ได้ (แนะนำปิดใช้งานแทนเมื่อเคยใช้แล้ว)
func (uc *AccountUseCases) Delete(ctx context.Context, in dto.ByID) error {
	return uc.write(ctx, in.Actor, func(ctx context.Context) error {
		return uc.d.Accounts.Delete(ctx, in.CompanyID, in.ID)
	})
}

// Lookup: สำหรับ module อื่น (ไม่ตรวจสิทธิ์) — ทะเบียนที่ไม่พบจะไม่อยู่ในผลลัพธ์
func (uc *AccountUseCases) Lookup(ctx context.Context, companyID vo.CompanyID, ids []vo.CashAccountID) ([]*entities.CashAccount, error) {
	return uc.d.Accounts.FindByIDs(ctx, companyID, ids)
}

func (uc *AccountUseCases) checkGL(ctx context.Context, companyID vo.CompanyID, id vo.GLAccountID) error {
	bad, err := uc.d.Ledger.InvalidAccounts(ctx, companyID, []vo.GLAccountID{id})
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return domain.ErrAccountInvalid
	}
	return nil
}
