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

// PaymentInput: kind = petty_cash ใช้ PettyCashRequestID (บรรทัดมาจากคำขอของเงินสดย่อย — Lines ไม่ใช้)
type PaymentInput struct {
	Kind               vo.PaymentKind
	SupplierID         *uuid.UUID // kind = supplier
	PettyCashRequestID *uuid.UUID // kind = petty_cash
	Payee              PartyInput // kind = other
	Date               time.Time
	Description        string
	Lines              []PaymentLineInput
	Methods            []MethodInput
}

// PettyCashRequest คือคำขอเงินสดย่อยที่รอการเงินจ่าย (เลือกในหน้าจ่ายเงิน)
type PettyCashRequest struct {
	ID         string
	DocumentNo string
	FundCode   string
	FundName   string
	Kind       string
	Date       time.Time
	Amount     vo.Money
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
	// WhtCertificateNo: เลขที่หนังสือรับรอง 50 ทวิ (ใบจ่ายที่มีภาษีหักเท่านั้น, ใบรับ = nil)
	WhtCertificateNo *string
}

type ReceiptLine struct {
	Invoice     *Ref // Code = เลขที่ใบแจ้งหนี้
	InvoiceKind string
	AccountID   *string
	Description string
	Amount      vo.Money
	Wht         vo.Money
	VatBase     vo.Money // ภาษีขายที่ถึงกำหนดด้วยใบรับเงินนี้ (ขายบริการ)
	Vat         vo.Money
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
	WhtType     *WhtTypeRef
	WhtRate     vo.Money
	WhtBase     vo.Money
	Wht         vo.Money
}

type Payment struct {
	DocumentSummary
	CreatedBy string
	Lines     []PaymentLine
	Methods   []Method
	// WhtFormSeq: ลำดับที่ในใบแนบ ภ.ง.ด.3 / 53 ของเดือน (คำนวณ, 0 = ไม่มีภาษีหัก / ยกเลิกแล้ว) — พิมพ์ใน 50 ทวิ
	WhtFormSeq int
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

// WhtTypeRef: ประเภทภาษีที่อ้างในบรรทัดใบจ่าย / รายงาน — IncomeType ใช้เลือกช่องใน 50 ทวิ
type WhtTypeRef struct {
	ID         string
	Code       string
	Name       string
	IncomeType string
}

type WhtType struct {
	ID         string
	Code       string
	Name       string
	Rate       vo.Money
	IncomeType string
	Status     string
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
	Kind        string
	VatUndue    vo.Money
	TaxInvoice  bool
}

// WithholdingRow: ภาษีหัก ณ ที่จ่าย 1 บรรทัด (ใบจ่าย) หรือภาษีถูกหัก 1 ใบรับ
type WithholdingRow struct {
	Date       time.Time
	DocumentID string
	DocumentNo string
	Party      Party
	TaxForm    string
	WhtType    *WhtTypeRef
	// CertificateNo: เลขที่หนังสือรับรอง 50 ทวิ ของใบจ่าย ("" = ภาษีถูกหัก / ไม่มี)
	CertificateNo string
	Rate          vo.Money
	Base          vo.Money
	Wht           vo.Money
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

// ---------- dashboard ----------

// DashboardCashAccount คือทะเบียนเงินที่ใช้งาน + บัญชีแยกประเภทที่ผูก (ยอดคงเหลือมาจาก ledger)
type DashboardCashAccount struct {
	Account     Ref
	Type        string
	GLAccountID vo.GLAccountID
	Balance     vo.Money
}

// FlowTotals คือยอดเงินเข้า-ออกของช่วง
type FlowTotals struct {
	Received      vo.Money // เงินที่ได้รับจริง (หลังภาษีถูกหัก)
	Paid          vo.Money // เงินที่จ่ายจริง (หลังภาษีหัก ณ ที่จ่าย)
	TransferCount int64
	WhtDeducted   vo.Money // ภาษีที่เราหักไว้จากใบจ่าย (ต้องนำส่ง ภ.ง.ด.3 / 53)
	WhtSuffered   vo.Money // ภาษีที่ลูกค้าหักจากใบรับ
}

type MonthlyFlow struct {
	Month    time.Time // วันที่ 1 ของเดือน
	Received vo.Money
	Paid     vo.Money
}

type RecentDocument struct {
	Kind       string // receipt | payment | transfer
	ID         string
	DocumentNo string
	Date       time.Time
	Party      string
	Amount     vo.Money
}

type FinanceDashboard struct {
	Start        time.Time
	End          time.Time
	CashAccounts []DashboardCashAccount
	CashTotal    vo.Money
	Flows        FlowTotals
	Monthly      []MonthlyFlow
	Recent       []RecentDocument
}

// ---------- สรุป ภ.พ.30 ----------

// VatSummaryFilter: ช่วงวันที่ (ปกติ = 1 เดือนภาษี)
type VatSummaryFilter struct {
	Start, End time.Time
}

type VatTotals struct {
	Base      vo.Money
	Tax       vo.Money
	Documents int
}

// VatSummary: ภาษีขาย - ภาษีซื้อ = ภาษีสุทธิ (บวก = ต้องชำระ, ลบ = ชำระเกิน / ขอคืน)
type VatSummary struct {
	Start, End time.Time
	Output     VatTotals
	Input      VatTotals
	NetTax     vo.Money
	Payable    vo.Money
	Refundable vo.Money
}
