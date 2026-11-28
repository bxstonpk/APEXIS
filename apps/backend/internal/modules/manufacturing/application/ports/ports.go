// Package ports คือ interface ที่ application ของ manufacturing ต้องการจากโลกภายนอก
// (ทุกตัวเข้าร่วม transaction ใน ctx — เบิก / คืน / ปิดงาน / ยกเลิกทั้งก้อนอยู่ใน transaction เดียว)
package ports

import (
	"context"
	"time"

	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/integration"
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
	BusinessType(ctx context.Context, companyID vo.CompanyID) (integration.BusinessType, error)
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (Period, error)
	DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error)
}

// Item คือสินค้าจาก inventory พร้อมบัญชีสินค้าคงเหลือที่ใช้จริง (สินค้า -> หมวด -> ค่าเริ่มต้น; nil = ไม่ได้ตั้ง)
type Item struct {
	ID                 vo.ItemID
	Code               string
	Name               string
	Uom                string
	Stock              bool
	Active             bool
	InventoryAccountID *vo.AccountID
}

type Warehouse struct {
	ID     vo.WarehouseID
	Code   string
	Name   string
	Active bool
}

type StockLine struct {
	ItemID   vo.ItemID
	Quantity vo.Quantity
	UnitCost vo.Money
}

type StockDocument struct {
	DocumentID vo.StockDocumentID
	DocumentNo string
	LineValues []vo.Money
}

// Inventory: error กฎของ inventory (สต็อกไม่พอ, งวดปิด, ของที่รับเข้าถูกใช้แล้ว ฯลฯ) ถูกห่อเป็น integration.ErrRejected
type Inventory interface {
	// Items: สินค้าที่ไม่พบจะไม่อยู่ใน map
	Items(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) (map[vo.ItemID]Item, error)
	// Context: ระบบบัญชีของปีบัญชีที่ date อยู่ + สินค้า (lock ค่าตั้งของ inventory จน commit)
	Context(ctx context.Context, companyID vo.CompanyID, date time.Time, ids []vo.ItemID) (vo.AccountingSystem, map[vo.ItemID]Item, error)
	Warehouses(ctx context.Context, companyID vo.CompanyID, ids []vo.WarehouseID) (map[vo.WarehouseID]Warehouse, error)
	PostReceipt(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []StockLine) (StockDocument, error)
	// PostIssue ตัดสต็อกออก (FIFO, ห้ามติดลบ) — LineValues = ต้นทุน FIFO ต่อบรรทัด (UnitCost ของ lines ไม่ใช้)
	PostIssue(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []StockLine) (StockDocument, error)
	CancelDocument(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.StockDocumentID) error
}

// JournalLine คือบรรทัดรายการบัญชีด้านเดียว
type JournalLine struct {
	AccountID vo.AccountID
	Debit     vo.Money
	Credit    vo.Money
}

type LedgerEntry struct {
	CompanyID   vo.CompanyID
	UserID      vo.UserID
	JournalID   vo.JournalID
	Date        time.Time
	Description string
	Reference   string
	Lines       []JournalLine
}

// Ref คือรหัส / ชื่อของบัญชีหรือสมุดรายวัน
type Ref struct {
	Code string
	Name string
}

// Ledger: error กฎบัญชีถูกห่อเป็น integration.ErrRejected
type Ledger interface {
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error)
	JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error)
	AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error)
	JournalsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error)
	AccountRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) (map[vo.AccountID]Ref, error)
	JournalRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.JournalID) (map[vo.JournalID]Ref, error)
	Post(ctx context.Context, e LedgerEntry) (vo.EntryID, string, error)
	Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error
}
