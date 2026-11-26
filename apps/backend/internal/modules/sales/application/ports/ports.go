// Package ports คือ interface ที่ application ของ sales ต้องการจากภายนอก (ทุกตัวเข้าร่วม transaction ใน ctx)
package ports

import (
	"context"
	"time"

	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/integration"
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
	YearStart  time.Time // วันแรกของปีบัญชี
	Open       bool
}

// Organization: PeriodAt คืน domain.ErrNoFiscalPeriod ถ้าไม่มีงวด
type Organization interface {
	CompanySettings(ctx context.Context, companyID vo.CompanyID) (CompanySettings, error)
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (Period, error)
	DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error)
	// BusinessType: ประเภทธุรกิจของบริษัท (กำหนดว่าออกขายสินค้า หรือขายบริการ)
	BusinessType(ctx context.Context, companyID vo.CompanyID) (integration.BusinessType, error)
}

// Item คือสินค้าจาก inventory พร้อมบัญชีที่ใช้จริง (nil = ไม่ได้ตั้ง)
type Item struct {
	ID                 vo.ItemID
	Code               string
	Name               string
	Stock              bool
	Service            bool // ชนิดบริการ (ภาษีเกิดเมื่อรับชำระ ม.78/1)
	Active             bool
	InventoryAccountID *vo.AccountID
	RevenueAccountID   *vo.AccountID
}

type StockLine struct {
	ItemID   vo.ItemID
	Quantity vo.Quantity
}

// ReceiptLine: บรรทัดรับคืนเข้าคลัง — UnitCost = ต้นทุนเดิมต่อหน่วย (ก่อน VAT)
type ReceiptLine struct {
	ItemID   vo.ItemID
	Quantity vo.Quantity
	UnitCost vo.Money
}

// StockDocument: LineValues = ต้นทุน FIFO ที่ตัดต่อบรรทัด / มูลค่าที่รับเข้าต่อบรรทัด
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
	// PostReceipt: ใบรับเข้า (รับคืนสินค้าจากใบลดหนี้) ผ่านทันที
	PostReceipt(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []ReceiptLine) (StockDocument, error)
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
	// AccountsByCode / JournalsByCode: id ตามรหัสในผังบัญชีมาตรฐาน (ตั้งค่าเริ่มต้นตอนสร้างบริษัท) — รหัสที่ไม่พบ / ลงรายการไม่ได้ไม่อยู่ในผล
	AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error)
	JournalsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error)
	Post(ctx context.Context, e LedgerEntry) (vo.EntryID, string, error)
	Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error
}
