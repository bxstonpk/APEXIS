// Package ports คือ interface ที่ application ของ sales ต้องการจากภายนอก (ทุกตัวเข้าร่วม transaction ใน ctx)
package ports

import (
	"context"
	"time"

	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
)

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

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
	DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error)
}

// Item คือสินค้าจาก inventory พร้อมบัญชีที่ใช้จริง (nil = ไม่ได้ตั้ง)
type Item struct {
	ID                 vo.ItemID
	Code               string
	Name               string
	Stock              bool
	Active             bool
	InventoryAccountID *vo.AccountID
	RevenueAccountID   *vo.AccountID
}

type StockLine struct {
	ItemID   vo.ItemID
	Quantity vo.Quantity
}

// StockDocument: LineValues = ต้นทุน FIFO ที่ตัดต่อบรรทัด
type StockDocument struct {
	DocumentID vo.StockDocumentID
	DocumentNo string
	LineValues []vo.Money
}

// Inventory: error กฎของ inventory (เช่น สต็อกไม่พอ) ถูกห่อเป็น integration.ErrRejected
type Inventory interface {
	Items(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) (map[vo.ItemID]Item, error)
	// Context: ระบบบัญชีของปีบัญชีที่ date อยู่ + สินค้า (lock ค่าตั้งของ inventory จน commit)
	Context(ctx context.Context, companyID vo.CompanyID, date time.Time, ids []vo.ItemID) (vo.AccountingSystem, map[vo.ItemID]Item, error)
	PostIssue(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []StockLine) (StockDocument, error)
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

type Ledger interface {
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error)
	JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error)
	Post(ctx context.Context, e LedgerEntry) (vo.EntryID, string, error)
	Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error
}

type CashAccount struct {
	ID          vo.CashAccountID
	Code        string
	GLAccountID vo.AccountID
	Active      bool
}

type CashBank interface {
	CashAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.CashAccountID) (map[vo.CashAccountID]CashAccount, error)
}
