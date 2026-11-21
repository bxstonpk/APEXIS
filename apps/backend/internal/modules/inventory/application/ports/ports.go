// Package ports คือ interface ที่ application ของ inventory ต้องการจากโลกภายนอก
package ports

import (
	"context"
	"time"

	"apexis/backend/internal/modules/inventory/domain/services"
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
	Start, End time.Time
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

// Ledger คือสิ่งที่ inventory ต้องการจาก ledger module (เข้าร่วม transaction ใน ctx)
// error กฎบัญชีของ ledger ถูกห่อเป็น integration.ErrRejected
type Ledger interface {
	// InvalidAccounts คืนบัญชีที่ใช้ลงรายการไม่ได้ (ไม่พบ / บัญชีคุม / inactive)
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error)
	JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error)
	// Balances: ยอด (เดบิต - เครดิต) สะสมถึง asOf
	Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID, asOf time.Time) (map[vo.AccountID]vo.Money, error)
	Post(ctx context.Context, e LedgerEntry) (vo.EntryID, string, error)
	Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error
}

type LedgerEntry struct {
	CompanyID   vo.CompanyID
	UserID      vo.UserID
	JournalID   vo.JournalID
	Date        time.Time
	Description string
	Reference   string
	Lines       []services.AdjustmentLine
}
