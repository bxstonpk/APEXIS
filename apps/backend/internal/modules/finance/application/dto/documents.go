package dto

import (
	"time"

	"github.com/google/uuid"

	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- input ----------

type MethodInput struct {
	CashAccountID vo.CashAccountID
	Amount        vo.Money
	Reference     string
}

// PartyInput: คู่ค้าที่ผู้ใช้ระบุเอง (รับ / จ่ายเงินอื่น) — ลูกค้า / ผู้ขายใช้ข้อมูลจาก sales / purchasing
type PartyInput struct {
	Name     string
	TaxID    string
	BranchNo string
	Address  string
	TaxType  vo.TaxType
}

type ReceiptLineInput struct {
	InvoiceID   *vo.InvoiceID
	AccountID   *vo.GLAccountID
	Description string
	Amount      vo.Money
	Wht         vo.Money
}

type ReceiptInput struct {
	Kind        vo.ReceiptKind
	CustomerID  *uuid.UUID // kind = customer
	Payer       PartyInput // kind = other
	Date        time.Time
	Description string
	Lines       []ReceiptLineInput
	Methods     []MethodInput
}

type PaymentLineInput struct {
	InvoiceID   *vo.InvoiceID
	AccountID   *vo.GLAccountID
	Description string
	Amount      vo.Money
	WhtTypeID   *vo.WhtTypeID
}

type PaymentInput struct {
	Kind        vo.PaymentKind
	SupplierID  *uuid.UUID // kind = supplier
	Payee       PartyInput // kind = other
	Date        time.Time
	Description string
	Lines       []PaymentLineInput
	Methods     []MethodInput
}

type TransferInput struct {
	Date        time.Time
	FromID      vo.CashAccountID
	ToID        vo.CashAccountID
	Amount      vo.Money
	Reference   string
	Description string
}

// ---------- filter ----------

var (
	DocumentSortKeys    = []string{"date", "document_no"}
	DefaultDocumentSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "document_no", Desc: true}}
)

// DocumentFilter: GET /receipts | /payments | /cash-transfers?q=&status=&kind=&party_id=&cash_account_id=&start_date=&end_date=&sort=&page=
type DocumentFilter struct {
	Search        string
	Statuses      []vo.DocStatus
	Kinds         []string
	PartyID       *uuid.UUID
	CashAccountID *vo.CashAccountID
	StartDate     *time.Time
	EndDate       *time.Time
	Sort          []listing.Sort
	Page          listing.Page
}

type WhtTypeFilter struct {
	Search   string
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

var (
	WithholdingSortKeys    = []string{"date", "document_no", "party_name"}
	DefaultWithholdingSort = []listing.Sort{{Key: "date"}, {Key: "document_no"}}
)

// WithholdingFilter: book=withholding-tax / withholding-tax-received ในช่วงวันที่เอกสาร
type WithholdingFilter struct {
	Start, End time.Time
	Form       string // pnd3 / pnd53 / "" (เฉพาะ withholding-tax)
	PartyID    *uuid.UUID
	Sort       []listing.Sort
	Page       listing.Page
}

// ---------- output ----------

type Ref struct {
	ID   string
	Code string
	Name string
}

type Party struct {
	ID       *string
	Code     string
	Name     string
	TaxID    string
	BranchNo string
	Address  string
	TaxType  string
}

type Method struct {
	CashAccount Ref
	GLAccountID string
	Amount      vo.Money
	Reference   string
}

// DocumentSummary คือส่วนร่วมของใบรับ / ใบจ่ายในรายการ
type DocumentSummary struct {
	ID          string
	DocumentNo  string
	Date        time.Time
	Kind        string
	Party       Party
	Amount      vo.Money // ยอดรวม (ตัดหนี้)
	Wht         vo.Money
	Net         vo.Money // รับ / จ่ายจริง
	Status      string
	EntryID     *string
	EntryNo     string
	CancelledAt *time.Time
	Description string
}

type ReceiptLine struct {
	Invoice     *Ref // Code = เลขที่ใบแจ้งหนี้
	AccountID   *string
	Description string
	Amount      vo.Money
	Wht         vo.Money
}

type Receipt struct {
	DocumentSummary
	CreatedBy string
	Lines     []ReceiptLine
	Methods   []Method
}

type PaymentLine struct {
	Invoice     *Ref // Code = เลขที่ใบแจ้งหนี้, Name = เลขที่ของผู้ขาย
	AccountID   *string
	Description string
	Amount      vo.Money
	WhtType     *Ref
	WhtRate     vo.Money
	WhtBase     vo.Money
	Wht         vo.Money
}

type Payment struct {
	DocumentSummary
	CreatedBy string
	Lines     []PaymentLine
	Methods   []Method
}

type Transfer struct {
	ID          string
	DocumentNo  string
	Date        time.Time
	From        Ref
	To          Ref
	Amount      vo.Money
	Reference   string
	Description string
	Status      string
	EntryID     *string
	EntryNo     string
	CancelledAt *time.Time
}

type WhtType struct {
	ID     string
	Code   string
	Name   string
	Rate   vo.Money
	Status string
}

type Settings struct {
	ReceiptJournalID     *string
	PaymentJournalID     *string
	TransferJournalID    *string
	WhtReceivedAccountID *string
	WhtPnd3AccountID     *string
	WhtPnd53AccountID    *string
}

type OpenInvoice struct {
	ID          string
	DocumentNo  string
	PartnerRef  string
	Date        time.Time
	DueDate     time.Time
	Net         vo.Money
	Gross       vo.Money
	Outstanding vo.Money
}

// WithholdingRow: ภาษีหัก ณ ที่จ่าย 1 บรรทัด (ใบจ่าย) หรือภาษีถูกหัก 1 ใบรับ
type WithholdingRow struct {
	Date       time.Time
	DocumentID string
	DocumentNo string
	Party      Party
	TaxForm    string
	WhtType    *Ref
	Rate       vo.Money
	Base       vo.Money
	Wht        vo.Money
}

type WithholdingTotals struct {
	Base vo.Money
	Wht  vo.Money
}

type Withholding struct {
	Start, End time.Time
	Rows       listing.Result[WithholdingRow]
	Totals     WithholdingTotals
}
