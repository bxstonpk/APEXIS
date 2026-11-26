// Package dto คือ input / filter / read model ของ sales
package dto

import (
	"apexis/backend/internal/platform/thaiaddress"
	"time"

	"github.com/google/uuid"

	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- Input ----------

type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

type Create[D any] struct {
	Actor
	Data D
}

type Update[ID any, D any] struct {
	Actor
	ID   ID
	Data D
}

type ByID[ID any] struct {
	Actor
	ID ID
}

type List[F any] struct {
	Actor
	Filter F
}

type ShipmentInput struct {
	OrderLineNo int
	Quantity    vo.Quantity
}

// SaleLineInput: บรรทัดเอกสารขายที่ออกตรง (ไม่อ้างใบสั่งขาย)
type SaleLineInput struct {
	ItemID      vo.ItemID
	Description string
	Quantity    vo.Quantity
	UnitPrice   vo.Money
}

// DepositInput: มัดจำที่หักในเอกสารขาย — Amount = ฐานก่อน VAT
type DepositInput struct {
	DepositID vo.InvoiceID
	Amount    vo.Money
}

// SaleInput: ขายสินค้า / ขายบริการ — อ้างใบสั่งขาย (OrderID + OrderLines) หรือออกตรง (CustomerID + Lines)
// VatRate nil = ตามค่าตั้งขาย; WarehouseID บังคับเมื่อมีสินค้า stock
type SaleInput struct {
	Kind             vo.InvoiceKind
	OrderID          *vo.OrderID
	OrderLines       []ShipmentInput
	CustomerID       vo.CustomerID
	CustomerBranchNo string
	PriceIncludesVat bool
	VatRate          *vo.Money
	Lines            []SaleLineInput
	Date             time.Time
	WarehouseID      *vo.WarehouseID
	Reference        string
	Description      string
	Deposits         []DepositInput
}

// DepositDocInput: ใบรับเงินมัดจำ — Amount ตาม PriceIncludesVat (รวม / ไม่รวม VAT)
type DepositDocInput struct {
	CustomerID       vo.CustomerID
	CustomerBranchNo string
	OrderID          *vo.OrderID
	Date             time.Time
	Description      string
	Reference        string
	Amount           vo.Money
	PriceIncludesVat bool
	VatRate          *vo.Money
}

// TaxInvoiceInput: ออกใบกำกับภาษีของใบแจ้งหนี้บริการก่อนรับเงิน
type TaxInvoiceInput struct {
	InvoiceID vo.InvoiceID
	Date      time.Time
}

// BillingNoteInput: ใบวางบิล
type BillingNoteInput struct {
	CustomerID  vo.CustomerID
	Date        time.Time
	DueDate     time.Time
	Description string
	InvoiceIDs  []vo.InvoiceID
}

// CreditNoteInput: ใบลดหนี้อ้างใบแจ้งหนี้ — WarehouseID บังคับเมื่อรับคืนสินค้า stock
type CreditNoteInput struct {
	InvoiceID   vo.InvoiceID
	Date        time.Time
	Kind        vo.CreditNoteKind
	Reason      string
	Description string
	WarehouseID *vo.WarehouseID
	Lines       []CreditNoteLineInput
}

// CreditNoteLineInput: return ใช้ Quantity (+ UnitPrice ไม่ส่ง = ราคาในใบ); allowance ใช้ Amount
type CreditNoteLineInput struct {
	InvoiceLineNo int
	Quantity      vo.Quantity
	UnitPrice     *vo.Money
	Amount        vo.Money
}

// ---------- Filter ----------

var (
	CustomerSortKeys    = []string{"code", "name"}
	DefaultCustomerSort = []listing.Sort{{Key: "code"}}
	DocumentSortKeys    = []string{"date", "document_no", "created_at"}
	DefaultDocumentSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "created_at", Desc: true}}
)

type CustomerFilter struct {
	Search   string
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

// DocumentFilter: ?q=&status=&customer_id=&order_id=&item_id=&start_date=&end_date=&sort=&page=&page_size=
type DocumentFilter struct {
	Search     string
	Statuses   []string
	CustomerID *vo.CustomerID
	OrderID    *vo.OrderID    // เอกสารขายของใบสั่งขายนี้
	InvoiceID  *vo.InvoiceID  // ใบลดหนี้ของใบแจ้งหนี้นี้
	ItemID     *vo.ItemID     // ใบเสนอราคา / ใบสั่งขาย: มีบรรทัดของสินค้านี้
	Kind       vo.InvoiceKind // เอกสารขาย: ชนิด (ตั้งโดย handler ตาม endpoint)
	StartDate  *time.Time
	EndDate    *time.Time
	Sort       []listing.Sort
	Page       listing.Page
}

var (
	OutstandingSortKeys    = []string{"date", "order_no", "customer_code", "delivery_date"}
	DefaultOutstandingSort = []listing.Sort{{Key: "date"}, {Key: "order_no"}}
	TaxSortKeys            = []string{"date", "document_no", "customer_code"}
	DefaultTaxSort         = []listing.Sort{{Key: "date"}, {Key: "document_no"}}
)

type OutstandingFilter struct {
	Search     string
	CustomerID *vo.CustomerID
	ItemID     *vo.ItemID
	Sort       []listing.Sort
	Page       listing.Page
}

type AgingFilter struct {
	AsOf       time.Time
	CustomerID *vo.CustomerID
	Search     string
	Page       listing.Page
}

// TaxFilter: รายงานภาษีขาย / ภาษีถูกหัก ณ ที่จ่าย ตามช่วงวันที่เอกสาร
type TaxFilter struct {
	Start, End time.Time
	CustomerID *vo.CustomerID
	Sort       []listing.Sort
	Page       listing.Page
}

// ---------- Output ----------

type Ref struct {
	ID   string
	Code string
	Name string
	Type string // สินค้า: service | stock | non_stock
}

type Totals struct {
	Net   vo.Money
	Vat   vo.Money
	Gross vo.Money
}

type Settings struct {
	VatRegistered           bool
	VatRate                 vo.Money
	JournalID               *string
	ARAccountID             *string
	OutputVatAccountID      *string
	RevenueAccountID        *string
	CogsAccountID           *string
	SalesReturnsAccountID   *string
	UndueOutputVatAccountID *string
	DepositAccountID        *string
}

type Customer struct {
	ID              string
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	AddressDetail   thaiaddress.Address
	PaymentTermDays int
	TaxType         string
	CreditLimit     *vo.Money
	Status          string
	Branches        []CustomerBranch // สาขาเพิ่มเติม — เฉพาะ Get (รายการไม่โหลด)
}

type CustomerBranch struct {
	BranchNo      string
	Name          string
	Address       string
	AddressDetail thaiaddress.Address
}

// SalesDocSummary คือหัวของใบเสนอราคา / ใบสั่งขาย
type SalesDocSummary struct {
	ID               string
	DocumentNo       string // ว่างจนกว่าจะส่ง (QT) / อนุมัติ (SO)
	Date             time.Time
	Customer         Ref
	CustomerBranchNo string
	Reference        string
	VatRate          vo.Money
	PriceIncludesVat bool
	Totals           Totals
	Status           string
}

type SalesLine struct {
	LineNo      int
	Item        Ref
	Description string
	Quantity    vo.Quantity
	UnitPrice   vo.Money
	Amounts     Totals
	Delivered   vo.Quantity // เฉพาะใบสั่งขาย
	Invoiced    vo.Quantity
}

type Quotation struct {
	SalesDocSummary
	ValidUntil      time.Time
	Description     string
	PaymentTermDays int
	OrderID         *string
	CreatedBy       string
	Lines           []SalesLine
}

type Order struct {
	SalesDocSummary
	QuotationID     *string
	DeliveryDate    *time.Time
	Description     string
	PaymentTermDays int
	CreatedBy       string
	ApprovedAt      *time.Time
	ClosedAt        *time.Time
	CancelledAt     *time.Time
	Lines           []SalesLine
	Warnings        []string // เช่น เกินวงเงินเครดิต (ตอนอนุมัติ — ไม่บันทึก)
}

// PostedSummary คือหัวของใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ
type PostedSummary struct {
	ID          string
	DocumentNo  string
	Date        time.Time
	Order       Ref // Code = เลขที่ใบสั่งขาย (ไม่มีในใบเสร็จ)
	Customer    Ref
	Status      string
	EntryID     *string
	EntryNo     string
	CancelledAt *time.Time
}

type StockInfo struct {
	WarehouseID     *string
	System          string
	StockDocumentNo string
}

type InvoiceLine struct {
	LineNo           int // เลขบรรทัดของใบแจ้งหนี้ (ใบลดหนี้อ้างด้วย invoice_line_no)
	OrderLineNo      int // 0 = ออกตรง
	Item             Ref
	Description      string
	Service          bool
	Quantity         vo.Quantity
	UnitPrice        vo.Money
	Amounts          Totals
	RevenueAccountID string
	Cost             vo.Money
	Returned         vo.Quantity // รับคืนแล้ว (ใบลดหนี้ที่ยังไม่ยกเลิก)
	Credited         vo.Money    // ยอดก่อน VAT ที่ลดหนี้แล้ว
}

type InvoiceSummary struct {
	PostedSummary
	Kind             string
	DueDate          time.Time
	Totals           Totals
	Received         vo.Money // ยอดรับชำระ (ใบเสร็จ) — ไม่รวมใบลดหนี้ / มัดจำที่หัก
	Credited         vo.Money // ยอดใบลดหนี้ (รวม VAT)
	DepositGross     vo.Money // มัดจำที่หัก (รวม VAT)
	Outstanding      vo.Money // ยอดรวม - มัดจำ - รับชำระ - ลดหนี้
	VatStatus        string   // output | undue | partial
	VatRecognized    vo.Money
	VatUndue         vo.Money
	TaxInvoice       *TaxInvoiceRef
	DepositRemaining *vo.Money // มัดจำ: ฐานที่ยังหักได้
	DepositApplied   *vo.Money
}

type TaxInvoiceRef struct {
	No   string
	Date time.Time
}

type DepositApplied struct {
	Deposit Ref // Code = เลขที่ใบมัดจำ
	Amount  vo.Money
	Vat     vo.Money
}

// ---------- ใบลดหนี้ ----------

type CreditNoteSummary struct {
	PostedSummary // Order = ใบแจ้งหนี้เดิม (Code = เลขที่ใบแจ้งหนี้)
	Kind          string
	VatKind       string // output | undue
	InvoiceKind   string // ชนิดเอกสารที่ลดหนี้: invoice | service_invoice
	Reason        string
	Totals        Totals
}

type CreditNoteLine struct {
	LineNo        int
	InvoiceLineNo int
	Item          Ref
	Quantity      vo.Quantity
	UnitPrice     vo.Money
	Amounts       Totals
	AccountID     string
	Cost          vo.Money
}

// CreditNote: ยอดของใบเดิม / ยอดที่ถูกต้อง / ผลต่าง ตาม ม.86/10 = InvoiceTotals / InvoiceTotals - ลดหนี้สะสม / Totals
type CreditNote struct {
	CreditNoteSummary
	StockInfo
	InvoiceDate      time.Time
	InvoiceTotals    Totals
	CreditedBefore   Totals // ใบลดหนี้ของใบแจ้งหนี้เดียวกันที่ออกก่อนใบนี้ (ยังไม่ยกเลิก)
	BillTo           BillTo
	VatRate          vo.Money
	PriceIncludesVat bool
	Description      string
	CreatedBy        string
	Lines            []CreditNoteLine
}

// VatTotals: ยอดรวมภาษีขายของช่วงวันที่ (ใบแจ้งหนี้ - ใบลดหนี้)
type VatTotals struct {
	Base      vo.Money
	Tax       vo.Money
	Documents int
}

// BillTo คือผู้ซื้อตามใบกำกับภาษี ณ วันออก
type BillTo struct {
	Name     string
	TaxID    string
	BranchNo string
	Address  string
}

type Invoice struct {
	InvoiceSummary
	StockInfo
	BillTo           BillTo
	VatRate          vo.Money
	PriceIncludesVat bool
	Reference        string
	Description      string
	Lines            []InvoiceLine
	DepositsApplied  []DepositApplied
	VatRecognitions  []VatRecognition
}

// VatRecognition: ภาษีขายของใบแจ้งหนี้บริการที่ถึงกำหนดแล้ว — Kind = receipt (ใบรับเงินของ finance) | tax_invoice
type VatRecognition struct {
	Kind     string
	Document Ref // Code = เลขที่
	Date     time.Time
	Base     vo.Money
	Vat      vo.Money
}

// OpenDeposit: มัดจำที่รับเงินครบแล้วและยังหักได้
type OpenDeposit struct {
	ID         string
	DocumentNo string
	Date       time.Time
	VatRate    vo.Money
	Base       vo.Money
	Vat        vo.Money
	Remaining  vo.Money
}

// ---------- ใบวางบิล ----------

type BillingNoteInvoice struct {
	ID          string
	DocumentNo  string
	Kind        string
	Date        time.Time
	DueDate     time.Time
	Gross       vo.Money
	Outstanding vo.Money
}

type BillingNoteSummary struct {
	ID          string
	DocumentNo  string
	Date        time.Time
	DueDate     time.Time
	Customer    Ref
	Status      string // open | paid | cancelled
	Total       vo.Money
	Outstanding vo.Money
	CancelledAt *time.Time
}

type BillingNote struct {
	BillingNoteSummary
	Description string
	CreatedBy   string
	Invoices    []BillingNoteInvoice
}

// UndueVatRow: ใบแจ้งหนี้บริการที่ยังมีภาษีขายยังไม่ถึงกำหนด ณ วันที่
type UndueVatRow struct {
	Invoice       Ref // Code = เลขที่
	Date          time.Time
	Customer      Ref
	Gross         vo.Money
	VatTotal      vo.Money
	VatRecognized vo.Money
	VatUndue      vo.Money
}

type UndueVatReport struct {
	EndDate time.Time
	Rows    listing.Result[UndueVatRow]
	Totals  UndueVatRow
}

type UndueVatFilter struct {
	End        time.Time
	CustomerID *vo.CustomerID
	Page       listing.Page
}

type OutstandingRow struct {
	Order        Ref
	Date         time.Time
	DeliveryDate *time.Time
	Customer     Ref
	LineNo       int
	Item         Ref
	Quantity     vo.Quantity
	Delivered    vo.Quantity
	Invoiced     vo.Quantity
	ToDeliver    vo.Quantity
	ToInvoice    vo.Quantity
}

type AgingRow struct {
	Customer                                          Ref
	NotDue, Days1To30, Days31To60, Days61To90, Over90 vo.Money
	Total                                             vo.Money
}

type Aging struct {
	AsOf   time.Time
	Rows   listing.Result[AgingRow]
	Totals AgingRow
}

// TaxRow: รายงานภาษีขาย — เฉพาะเอกสารที่ภาษีเกิดแล้ว (ใบลดหนี้เป็นยอดติดลบ)
type TaxRow struct {
	Date        time.Time
	Kind        string // invoice | deposit | tax_invoice | receipt | credit_note
	DocumentID  string
	DocumentNo  string
	ReferenceNo string // ใบลดหนี้: ใบกำกับเดิม; ใบกำกับ / ใบรับเงิน: ใบแจ้งหนี้บริการ
	ReferenceID string
	Customer    Ref
	TaxID       string
	BranchNo    string
	TaxType     string
	Base        vo.Money // ภาษีขาย: ยอดก่อน VAT; ถูกหัก: ยอดตัดหนี้
	Tax         vo.Money
}

type TaxReport struct {
	Start, End time.Time
	Rows       listing.Result[TaxRow]
	TotalBase  vo.Money
	TotalTax   vo.Money
}

// ---------- การตัดยอดใบแจ้งหนี้ (เรียกโดย finance) ----------

// SettlementDoc คือเอกสารรับเงินของ finance ที่ตัดยอด
type SettlementDoc struct {
	ID   uuid.UUID
	No   string
	Date time.Time
}

type SettlementLine struct {
	InvoiceID vo.InvoiceID
	Amount    vo.Money
}

// SettledInvoice: VatBase / Vat = ภาษีขายที่ถึงกำหนดจากการรับเงินครั้งนี้ (ขายบริการ ม.78/1)
type SettledInvoice struct {
	ID         vo.InvoiceID
	DocumentNo string
	Kind       vo.InvoiceKind
	Net        vo.Money
	Gross      vo.Money
	VatBase    vo.Money
	Vat        vo.Money
}

// JournalLine: บรรทัดที่ finance ต้องลงเพิ่มในรายการรับเงินเดียวกัน (Dr ภาษีขายยังไม่ถึงกำหนด / Cr ภาษีขาย)
type JournalLine struct {
	AccountID vo.AccountID
	Debit     vo.Money
	Credit    vo.Money
}

// SettlementResult: ลูกค้า ณ วันรับเงิน + บัญชีลูกหนี้ (ค่าตั้งขาย) + ใบแจ้งหนี้ตามลำดับที่ส่งมา
type SettlementParty struct {
	ID       vo.CustomerID
	Code     string
	Name     string
	TaxID    string
	BranchNo string
	Address  string
}

type SettlementResult struct {
	Customer     SettlementParty
	ARAccountID  vo.AccountID
	Invoices     []SettledInvoice
	ExtraJournal []JournalLine
}

type OpenInvoice struct {
	ID          string
	DocumentNo  string
	Kind        string
	Date        time.Time
	DueDate     time.Time
	Net         vo.Money
	Gross       vo.Money
	Outstanding vo.Money
	VatUndue    vo.Money // ขายบริการ: ภาษียังไม่ถึงกำหนดที่เหลือ
	TaxInvoice  bool     // ขายบริการที่ออกใบกำกับภาษีแล้ว
}
