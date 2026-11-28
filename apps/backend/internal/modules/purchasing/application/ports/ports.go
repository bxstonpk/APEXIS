// Package ports คือ interface ที่ application ของ purchasing ต้องการจากโลกภายนอก
// (ทุกตัวเข้าร่วม transaction ใน ctx — รับสินค้า / แจ้งหนี้ทั้งก้อนอยู่ใน transaction เดียว)
package ports

import (
	"context"
	"time"

	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
)

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Authorizer คืน domain.ErrForbidden เมื่อไม่มีสิทธิ์
type Authorizer interface {
	Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error
}

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

// Organization: PeriodAt คืน domain.ErrNoFiscalPeriod ถ้าไม่มีงวด
type Organization interface {
	CompanySettings(ctx context.Context, companyID vo.CompanyID) (CompanySettings, error)
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (Period, error)
	// DocumentFormat: รูปแบบเลขที่ที่บริษัทตั้งสำหรับชนิดเอกสาร
	DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error)
}

// Item คือสินค้าจาก inventory พร้อมบัญชีที่ใช้จริง (สินค้า -> หมวด -> ค่าเริ่มต้น; nil = ไม่ได้ตั้ง)
type Item struct {
	ID                 vo.ItemID
	Code               string
	Name               string
	Stock              bool
	Active             bool
	PurchaseAccountID  *vo.AccountID
	InventoryAccountID *vo.AccountID
}

type StockLine struct {
	ItemID   vo.ItemID
	Quantity vo.Quantity
	UnitCost vo.Money
}

type StockReceipt struct {
	DocumentID vo.StockDocumentID
	DocumentNo string
	LineValues []vo.Money
}

// Inventory: error กฎของ inventory ถูกห่อเป็น integration.ErrRejected
type Inventory interface {
	// Items: สินค้าที่ไม่พบจะไม่อยู่ใน map
	Items(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) (map[vo.ItemID]Item, error)
	// PurchaseContext: ระบบบัญชีของปีบัญชีที่ date อยู่ + สินค้า (lock ค่าตั้งของ inventory จน commit)
	PurchaseContext(ctx context.Context, companyID vo.CompanyID, date time.Time, ids []vo.ItemID) (vo.AccountingSystem, map[vo.ItemID]Item, error)
	PostReceipt(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []StockLine) (StockReceipt, error)
	// PostIssue ตัดสต็อกออก (FIFO, ห้ามติดลบ) — LineValues = ต้นทุน FIFO ต่อบรรทัด (UnitCost ของ lines ไม่ใช้)
	PostIssue(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []StockLine) (StockReceipt, error)
	CancelDocument(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.StockDocumentID) error
}

type LedgerEntry struct {
	CompanyID   vo.CompanyID
	UserID      vo.UserID
	JournalID   vo.JournalID
	Date        time.Time
	Description string
	Reference   string
	Lines       []services.JournalLine
}

// Ledger: error กฎบัญชีถูกห่อเป็น integration.ErrRejected
type Ledger interface {
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error)
	JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error)
	// AccountsByCode / JournalsByCode: id ตามรหัสในผังบัญชีมาตรฐาน (ตั้งค่าเริ่มต้นตอนสร้างบริษัท) — รหัสที่ไม่พบ / ลงรายการไม่ได้ไม่อยู่ในผล
	AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error)
	JournalsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error)
	Post(ctx context.Context, e LedgerEntry) (vo.EntryID, string, error)
	Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error
}
