// Package repositories คือสัญญา persistence ที่ application ของ cashbank ต้องการ
package repositories

import (
	"context"

	"apexis/backend/internal/modules/cashbank/application/dto"
	"apexis/backend/internal/modules/cashbank/domain/entities"
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// CashAccountRepository — ไม่เปิด transaction เอง; Create / Update คืน domain.ErrCodeExists เมื่อรหัสซ้ำ
type CashAccountRepository interface {
	Create(ctx context.Context, a *entities.CashAccount) error
	Update(ctx context.Context, a *entities.CashAccount) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) error
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) (*entities.CashAccount, error)
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.CashAccountID) ([]*entities.CashAccount, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) (dto.CashAccount, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.Filter) (listing.Result[dto.CashAccount], error)
}
