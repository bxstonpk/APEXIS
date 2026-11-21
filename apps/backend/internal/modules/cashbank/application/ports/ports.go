// Package ports คือ interface ที่ application ของ cashbank ต้องการจากภายนอก
package ports

import (
	"context"

	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
)

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Authorizer คืน domain.ErrForbidden เมื่อไม่มีสิทธิ์
type Authorizer interface {
	Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error
}

type Organization interface {
	CompanyActive(ctx context.Context, companyID vo.CompanyID) (bool, error)
}

type Ledger interface {
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID) ([]vo.GLAccountID, error)
}
