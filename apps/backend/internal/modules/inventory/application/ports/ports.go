// Package ports คือ interface ที่ application ของ inventory ต้องการจากโลกภายนอก
package ports

import (
	"context"
	"time"

	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// TransactionManager: repository ทุกตัว (รวมถึง module อื่นที่เรียกผ่าน ctx) ใช้ transaction เดียวกัน
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Authorizer ตรวจสิทธิ์ผ่าน identity — คืน domain.ErrForbidden เมื่อไม่มีสิทธิ์
type Authorizer interface {
	Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error
}

// CompanySettings: สถานะบริษัท และจำนวนทศนิยมของเงิน / จำนวนสินค้าที่บริษัทกำหนด
type CompanySettings struct {
	Active           bool
	AmountDecimals   int
	QuantityDecimals int
}

type Period struct {
	ID         vo.FiscalPeriodID
	FiscalYear int
	Open       bool
}

// Organization คือสิ่งที่ inventory ต้องการจาก organization module (เข้าร่วม transaction ใน ctx)
type Organization interface {
	CompanySettings(ctx context.Context, companyID vo.CompanyID) (CompanySettings, error)
	// PeriodAt คืน domain.ErrNoFiscalPeriod ถ้าไม่มีงวด; ใน transaction จะ lock แถวงวดจน commit
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (Period, error)
	// BranchExists: สาขามีอยู่ (ยังไม่ถูกลบ) ในบริษัท
	BranchExists(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) (bool, error)
}
