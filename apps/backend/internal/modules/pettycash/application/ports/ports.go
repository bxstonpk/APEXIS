// Package ports คือ interface ที่ application ของ pettycash ต้องการจากโลกภายนอก
// (ทุกตัวเข้าร่วม transaction ใน ctx — การเงินจ่ายคำขอ + ปรับสถานะใบสำคัญจ่ายอยู่ใน transaction เดียว)
package ports

import (
	"context"
	"time"

	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
)

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Authorizer คืน domain.ErrForbidden เมื่อไม่มีสิทธิ์
type Authorizer interface {
	Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error
}

// Members: ชื่อของผู้ใช้ที่เป็นสมาชิก active ของบริษัท (identity) — id ที่ไม่ใช่สมาชิกไม่อยู่ในผล
type Members interface {
	Names(ctx context.Context, companyID vo.CompanyID, ids []vo.UserID) (map[vo.UserID]string, error)
}

type CompanySettings struct {
	Active         bool
	AmountDecimals int
}

type Period struct {
	FiscalYear int
	Open       bool
}

// Organization: PeriodAt คืน domain.ErrNoFiscalPeriod ถ้าไม่มีงวด
type Organization interface {
	CompanySettings(ctx context.Context, companyID vo.CompanyID) (CompanySettings, error)
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (Period, error)
	DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error)
}

// Ref คือรหัส / ชื่อของบัญชี
type Ref struct {
	Code string
	Name string
}

// Ledger: ตรวจ / อ่านบัญชีเท่านั้น — เงินสดย่อยไม่ลงบัญชีเอง (การเงินลงตอนจ่ายคำขอ)
type Ledger interface {
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error)
	AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error)
	AccountRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) (map[vo.AccountID]Ref, error)
}
