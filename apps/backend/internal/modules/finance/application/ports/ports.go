// Package ports คือ interface ที่ application ของ finance ต้องการจากภายนอก
// (ทุกตัวเข้าร่วม transaction ใน ctx — รับ / จ่ายเงินทั้งก้อนอยู่ใน transaction เดียว)
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/finance/domain/services"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
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
	Active         bool
	AmountDecimals int
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
	InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID) ([]vo.GLAccountID, error)
	JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error)
	// AccountsByCode / JournalsByCode: id ตามรหัสในผังบัญชีมาตรฐาน (ตั้งค่าเริ่มต้นตอนสร้างบริษัท) — รหัสที่ไม่พบ / ลงรายการไม่ได้ไม่อยู่ในผล
	AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.GLAccountID, error)
	JournalsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error)
	Post(ctx context.Context, e LedgerEntry) (vo.EntryID, string, error)
	Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error
	// Balances: ยอด (เดบิต - เครดิต) สะสมถึง asOf ของบัญชีแยกประเภท — เรียกครั้งเดียวทุกบัญชี
	Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID, asOf time.Time) (map[vo.GLAccountID]vo.Money, error)
}

// ---------- ลูกหนี้ / เจ้าหนี้ (sales / purchasing) ----------

// SettlementDoc คือเอกสารของ finance ที่ตัดยอดใบแจ้งหนี้
type SettlementDoc struct {
	ID   uuid.UUID
	No   string
	Date time.Time
}

type Settlement struct {
	InvoiceID vo.InvoiceID
	Amount    vo.Money
}

// Counterparty คือลูกค้า / ผู้ขาย ณ วันเอกสาร
type Counterparty struct {
	ID       uuid.UUID
	Code     string
	Name     string
	TaxID    string
	BranchNo string
	Address  string
	TaxType  vo.TaxType // ผู้ขายเท่านั้น
}

type SettledInvoice struct {
	ID         vo.InvoiceID
	DocumentNo string
	PartnerRef string
	Net        vo.Money
	Gross      vo.Money
}

// SettlementResult: คู่ค้า + บัญชีคุม (ลูกหนี้ / เจ้าหนี้) + ใบแจ้งหนี้ตามลำดับที่ส่ง
type SettlementResult struct {
	Party            Counterparty
	ControlAccountID vo.GLAccountID
	Invoices         []SettledInvoice
}

type OpenInvoice struct {
	ID          vo.InvoiceID
	DocumentNo  string
	PartnerRef  string
	Date        time.Time
	DueDate     time.Time
	Net         vo.Money
	Gross       vo.Money
	Outstanding vo.Money
}

// Settlements: ตัดยอด / คืนยอดใบแจ้งหนี้ของ sales (ลูกหนี้) หรือ purchasing (เจ้าหนี้)
// error กฎของ module ปลายทางถูกห่อเป็น integration.ErrRejected
type Settlements interface {
	Settle(ctx context.Context, companyID vo.CompanyID, partyID uuid.UUID, doc SettlementDoc, lines []Settlement) (SettlementResult, error)
	Unsettle(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error
	Open(ctx context.Context, companyID vo.CompanyID, partyID uuid.UUID) ([]OpenInvoice, error)
}

// ---------- ภาษีมูลค่าเพิ่ม (ภ.พ.30) ----------

// VatTotals: ยอดรวมเอกสารภาษีของช่วงวันที่ (ใบกำกับหักใบลดหนี้แล้ว)
type VatTotals struct {
	Base      vo.Money
	Tax       vo.Money
	Documents int
}

// Vat: ยอดภาษีขาย (sales) / ภาษีซื้อ (purchasing) ของช่วงวันที่ — ใช้ทำสรุป ภ.พ.30
type Vat interface {
	OutputVat(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (VatTotals, error)
	InputVat(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (VatTotals, error)
}
