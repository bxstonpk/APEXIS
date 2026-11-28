// Package dto คือข้อมูลขาเข้า (Input / Filter) และขาออก (read model) ของ use case purchasing
package dto

import (
	"apexis/backend/internal/platform/thaiaddress"
	"time"

	"github.com/google/uuid"

	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
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
	// Kind: เอกสารที่แยกสาย (ใบสั่ง / ใบรับ / ใบแจ้งหนี้) — สายของ endpoint ("" = สินค้า); เอกสารต่างสาย = ไม่พบ
	Kind vo.PurchaseKind
}

type List[F any] struct {
	Actor
	Filter F
}

// LineQty คือจำนวนที่รับ / แจ้งหนี้ของบรรทัดใบสั่งซื้อ (UnitPrice เฉพาะใบแจ้งหนี้: nil = ราคาตามใบสั่งซื้อ)
type LineQty struct {
	OrderLineNo int
	Quantity    vo.Quantity
	UnitPrice   *vo.Money
	AccountID   *vo.AccountID // ซื้อบริการ: บัญชีค่าใช้จ่ายของบรรทัด (nil = ค่าตั้ง service_expense_account_id -> บัญชีซื้อของสินค้า)
}

type ReceiptInput struct {
	Kind        vo.PurchaseKind // goods = ใบรับสินค้า, service = ใบตรวจรับงาน
	OrderID     vo.OrderID
	Date        time.Time
	WarehouseID *vo.WarehouseID
	Reference   string
	Description string
	Lines       []LineQty
}

type InvoiceInput struct {
	Kind              vo.PurchaseKind // goods = ซื้อสินค้า, service = ซื้อบริการ
	OrderID           vo.OrderID
	SupplierInvoiceNo string
	InvoiceDate       time.Time
	Date              time.Time
	Description       string
	Lines             []LineQty
	// ซื้อบริการ: ใบกำกับภาษีที่ผู้ขายออกมาพร้อมใบแจ้งหนี้ (ว่าง = ยังไม่ได้รับ — ภาษีซื้อยังไม่ถึงกำหนด)
	TaxInvoiceNo   string
	TaxInvoiceDate *time.Time
}

// DirectLine คือบรรทัดของซื้อตรงไม่ผ่านใบสั่ง
type DirectLine struct {
	ItemID      vo.ItemID
	Description string
	Quantity    vo.Quantity
	UnitPrice   vo.Money
	AccountID   *vo.AccountID // ซื้อบริการเท่านั้น
}

// DirectInvoiceInput: ซื้อสินค้า / ซื้อบริการตรงไม่ผ่านใบสั่ง — ระบบออกใบสั่ง (อนุมัติ) + ใบรับ / ใบตรวจรับงาน + ใบแจ้งหนี้ใน transaction เดียว
type DirectInvoiceInput struct {
	Kind              vo.PurchaseKind
	SupplierID        vo.SupplierID
	SupplierBranchNo  string
	SupplierInvoiceNo string
	InvoiceDate       time.Time
	Date              time.Time
	WarehouseID       *vo.WarehouseID // ซื้อสินค้าที่มีสินค้า stock
	PriceIncludesVat  bool
	Reference         string
	Description       string
	Lines             []DirectLine
	TaxInvoiceNo      string
	TaxInvoiceDate    *time.Time
}

// CreditNoteLineInput: บรรทัดใบลดหนี้ อ้าง line_no ของใบแจ้งหนี้เดิม
// ส่งคืนสินค้า: Quantity (+ UnitPrice ไม่ส่ง = ราคาตามใบแจ้งหนี้); ลดราคา: Amount (รวม / แยก VAT ตามใบแจ้งหนี้)
type CreditNoteLineInput struct {
	InvoiceLineNo int
	Quantity      *vo.Quantity
	UnitPrice     *vo.Money
	Amount        *vo.Money
}

type CreditNoteInput struct {
	InvoiceID            vo.InvoiceID
	SupplierCreditNoteNo string
	CreditNoteDate       time.Time // วันที่ในใบลดหนี้ของผู้ขาย
	Date                 time.Time // วันที่ลงบัญชี
	Kind                 vo.CreditNoteKind
	Reason               string
	Description          string
	WarehouseID          *vo.WarehouseID
	Lines                []CreditNoteLineInput
}

// ---------- Filter (จาก URL query) ----------

var (
	SupplierSortKeys    = []string{"code", "name"}
	DefaultSupplierSort = []listing.Sort{{Key: "code"}}
	OrderSortKeys       = []string{"date", "order_no", "created_at"}
	DefaultOrderSort    = []listing.Sort{{Key: "date", Desc: true}, {Key: "created_at", Desc: true}}
	DocumentSortKeys    = []string{"date", "document_no"}
	DefaultDocumentSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "document_no", Desc: true}}
)

// PurchaseDocumentFilter: GET /purchase-documents?type=&status=&supplier_id=&q=&start_date=&end_date=&sort=&page=&page_size=
// Types ว่าง = ทุกชนิดที่ผู้ใช้มีสิทธิ์อ่าน; Statuses รวมสถานะของใบสั่ง (draft / approved / closed) และเอกสาร (posted) — cancelled ใช้ร่วมกัน
type PurchaseDocumentFilter struct {
	Types      []vo.PurchaseDocType
	Search     string
	Statuses   []string
	SupplierID *vo.SupplierID
	StartDate  *time.Time
	EndDate    *time.Time
	Sort       []listing.Sort
	Page       listing.Page
}

// PurchaseDocument คือแถวของรายการรวม "เอกสารซื้อ"
//
//	Reference: ใบแจ้งหนี้ = เลขที่ใบแจ้งหนี้ของผู้ขาย, ใบสั่ง / ใบรับ = อ้างอิงที่กรอก
//	Total: ยอดรวม (รวม VAT) — nil สำหรับใบรับสินค้า / ใบตรวจรับงาน
//	Outstanding: ยอดค้างจ่าย — เฉพาะใบแจ้งหนี้
//	VatStatus: ภาษีซื้อของใบแจ้งหนี้ (none / claimed / undue / partial) — ว่างสำหรับเอกสารอื่น
type PurchaseDocument struct {
	Type        vo.PurchaseDocType
	ID          string
	DocumentNo  string
	Date        time.Time
	Supplier    Ref
	Status      string
	Reference   string
	Total       *vo.Money
	Outstanding *vo.Money
	VatStatus   string
}

// SupplierFilter: GET /suppliers?q=&status=&vat_registered=&sort=&page=&page_size=
type SupplierFilter struct {
	Search        string
	Statuses      []vo.Status
	VatRegistered *bool
	Sort          []listing.Sort
	Page          listing.Page
}

// DocumentFilter: GET /purchase-orders | /goods-receipts | /supplier-invoices
// ?q=&status=&supplier_id=&order_id=&item_id=&start_date=&end_date=&sort=&page=&page_size=
type DocumentFilter struct {
	Kind       vo.PurchaseKind // ใบสั่ง / ใบรับ / ใบแจ้งหนี้: สายของ endpoint ("" = ทุกสาย — ใบลดหนี้)
	Search     string
	Statuses   []string
	SupplierID *vo.SupplierID
	OrderID    *vo.OrderID   // ใบรับ / ใบแจ้งหนี้ของใบสั่งซื้อนี้
	ItemID     *vo.ItemID    // เฉพาะใบสั่งซื้อ: มีบรรทัดของสินค้านี้
	InvoiceID  *vo.InvoiceID // เฉพาะใบลดหนี้: ของใบแจ้งหนี้นี้
	StartDate  *time.Time
	EndDate    *time.Time
	Sort       []listing.Sort
	Page       listing.Page
}

var (
	OutstandingSortKeys    = []string{"date", "order_no", "supplier_code", "delivery_date"}
	DefaultOutstandingSort = []listing.Sort{{Key: "date"}, {Key: "order_no"}}
)

// OutstandingFilter: book=outstanding-purchase-orders — บรรทัดใบสั่งซื้อ (อนุมัติแล้ว / ปิด) ที่ยังรับหรือแจ้งหนี้ไม่ครบ
type OutstandingFilter struct {
	Kind       vo.PurchaseKind // goods = outstanding-purchase-orders, service = outstanding-work-orders
	Search     string          // เลขที่ใบสั่งซื้อ / รหัส / ชื่อผู้ขาย
	SupplierID *vo.SupplierID
	ItemID     *vo.ItemID
	Sort       []listing.Sort
	Page       listing.Page
}

// AgingFilter: book=ap-aging — ยอดเจ้าหนี้ค้างจ่าย ณ วันที่ (end_date) แยกตามอายุนับจากวันครบกำหนด
type AgingFilter struct {
	AsOf       time.Time
	SupplierID *vo.SupplierID
	Search     string
	Page       listing.Page
}

var (
	InputVatSortKeys    = []string{"date", "document_no", "supplier_name"}
	DefaultInputVatSort = []listing.Sort{{Key: "date"}, {Key: "document_no"}}
)

// InputVatFilter: book=input-vat — ใบแจ้งหนี้ผู้ขาย / ใบลดหนี้ที่มี VAT ตามวันที่ลงบัญชี (เดือนที่ใช้สิทธิ์ภาษีซื้อ)
type InputVatFilter struct {
	Start, End time.Time
	SupplierID *vo.SupplierID
	Sort       []listing.Sort
	Page       listing.Page
}

// ---------- Output (read model) ----------

type Ref struct {
	ID   string
	Code string
	Name string
}

type Settings struct {
	VatRate                  vo.Money
	JournalID                *string
	GRIRAccountID            *string
	APAccountID              *string
	InputVatAccountID        *string
	PriceVarianceAccountID   *string
	PurchaseReturnsAccountID *string
	UndueInputVatAccountID   *string
	ServiceExpenseAccountID  *string
}

type Supplier struct {
	ID              string
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	AddressDetail   thaiaddress.Address
	PaymentTermDays int
	VatRegistered   bool
	TaxType         string
	Status          string
	Branches        []SupplierBranch // สาขาเพิ่มเติม — เฉพาะ Get (รายการไม่โหลด)
}

type SupplierBranch struct {
	BranchNo      string
	Name          string
	Address       string
	AddressDetail thaiaddress.Address
}

type Totals struct {
	Net   vo.Money
	Vat   vo.Money
	Gross vo.Money
}

type OrderSummary struct {
	Kind             string // goods = ใบสั่งซื้อ, service = ใบสั่งจ้าง
	ID               string
	OrderNo          string // ว่างจนกว่าจะอนุมัติ
	Date             time.Time
	DeliveryDate     *time.Time
	Supplier         Ref
	SupplierBranchNo string
	Reference        string
	VatRate          vo.Money
	PriceIncludesVat bool
	Totals           Totals
	Status           string
}

type OrderLine struct {
	LineNo      int
	Item        Ref
	Description string
	Quantity    vo.Quantity
	UnitPrice   vo.Money
	Amounts     Totals
	Received    vo.Quantity
	Invoiced    vo.Quantity
}

type Order struct {
	OrderSummary
	Description     string
	VatRegistered   bool
	PaymentTermDays int
	CreatedBy       string
	ApprovedBy      *string
	ApprovedAt      *time.Time
	ClosedAt        *time.Time
	CancelledAt     *time.Time
	Lines           []OrderLine
}

type DocumentSummary struct {
	Kind        string // ใบรับ / ใบแจ้งหนี้: goods | service; ใบลดหนี้: สายของใบแจ้งหนี้เดิม
	ID          string
	DocumentNo  string
	Date        time.Time
	Order       Ref // Code = เลขที่ใบสั่งซื้อ
	Supplier    Ref
	Status      string
	EntryID     *string
	EntryNo     string
	CancelledAt *time.Time
}

type ReceiptSummary struct {
	DocumentSummary
	WarehouseID     *string
	StockDocumentNo string
	System          string
	TotalValue      vo.Money
}

type ReceiptLine struct {
	OrderLineNo int
	Item        Ref
	Quantity    vo.Quantity
	UnitCost    vo.Money
	Value       vo.Money
	AccountID   string
}

type Receipt struct {
	ReceiptSummary
	Reference   string
	Description string
	CreatedBy   string
	Lines       []ReceiptLine
}

type InvoiceSummary struct {
	DocumentSummary
	SupplierInvoiceNo string
	InvoiceDate       time.Time
	DueDate           time.Time
	Totals            Totals
	Paid              vo.Money // ยอดจ่ายด้วยใบจ่ายเงิน (ไม่รวมใบลดหนี้)
	Credited          vo.Money // ยอดลดหนี้ (รวม VAT) ของใบลดหนี้ที่ยังไม่ยกเลิก
	Outstanding       vo.Money
	// ภาษีซื้อ: TaxInvoiceReceived = ได้รับใบกำกับพร้อมใบแจ้งหนี้; VatStatus none | claimed | undue | partial
	TaxInvoiceReceived bool
	VatStatus          string
	VatRecognized      vo.Money // ภาษีซื้อที่ได้รับใบกำกับภายหลังแล้ว
	VatUndue           vo.Money // ภาษีซื้อที่ยังรอใบกำกับ
	// ซื้อบริการที่ได้รับใบกำกับพร้อมใบแจ้งหนี้
	TaxInvoiceNo   string
	TaxInvoiceDate *time.Time
}

// TaxInvoiceRecord คือใบกำกับภาษีที่ได้รับจากผู้ขายภายหลังใบแจ้งหนี้ (แถวของ purchasing_tax_invoices)
type TaxInvoiceRecord struct {
	ID             uuid.UUID
	CompanyID      vo.CompanyID
	InvoiceID      vo.InvoiceID
	SupplierID     vo.SupplierID
	TaxInvoiceNo   string
	TaxInvoiceDate time.Time
	Date           time.Time // วันที่ลงบัญชี (ใช้สิทธิภาษีซื้อในเดือนภาษีของวันนี้)
	Base           vo.Money
	Vat            vo.Money
	EntryID        *vo.EntryID
	EntryNo        string
	Status         vo.DocStatus
	CreatedBy      vo.UserID
	CreatedAt      time.Time
	CancelledAt    *time.Time
}

// TaxInvoiceInput: POST /supplier-invoices/{id}/tax-invoices — Vat nil = ค่าตั้งต้นตามยอดจ่าย; Date nil = วันที่ในใบกำกับ
type TaxInvoiceInput struct {
	InvoiceID      vo.InvoiceID
	TaxInvoiceNo   string
	TaxInvoiceDate time.Time
	Date           *time.Time
	Vat            *vo.Money
}

// TaxInvoiceRef: PATCH /supplier-invoices/{id}/tax-invoices/{taxInvoiceID}
type TaxInvoiceRef struct {
	InvoiceID vo.InvoiceID
	ID        uuid.UUID
}

// UndueInputVatFilter: book=undue-input-vat — end_date (บังคับ), supplier_id, page, page_size
type UndueInputVatFilter struct {
	End        time.Time
	SupplierID *vo.SupplierID
	Page       listing.Page
}

// UndueInputVatRow: ใบแจ้งหนี้ผู้ขายที่ยังรอใบกำกับภาษี ณ วันที่
type UndueInputVatRow struct {
	Invoice       Ref // Code = เลขที่ในระบบ, Name = เลขที่ใบแจ้งหนี้ของผู้ขาย
	Date          time.Time
	Supplier      Ref
	Gross         vo.Money
	VatTotal      vo.Money
	VatRecognized vo.Money
	VatCredited   vo.Money
	VatUndue      vo.Money
}

type UndueInputVat struct {
	End    time.Time
	Rows   listing.Result[UndueInputVatRow]
	Totals UndueInputVatRow
}

type InvoiceLine struct {
	LineNo            int
	OrderLineNo       int
	Item              Ref
	Quantity          vo.Quantity
	UnitPrice         vo.Money
	Amounts           Totals
	Cleared           vo.Money
	Variance          vo.Money
	VarianceAccountID string
	Returned          vo.Quantity // ส่งคืนแล้ว (ใบลดหนี้ที่ยังไม่ยกเลิก)
	CreditedNet       vo.Money    // ลดหนี้แล้ว (ก่อน VAT)
}

// LineCredit: ยอดที่ลดหนี้ไปแล้วของบรรทัดใบแจ้งหนี้ (ใบลดหนี้ที่ยังไม่ยกเลิก)
type LineCredit struct {
	Returned vo.Quantity
	Net      vo.Money
}

// ---------- ใบลดหนี้ผู้ขาย ----------

type CreditNoteSummary struct {
	DocumentSummary
	Invoice              Ref // Code = เลขที่ใบแจ้งหนี้ในระบบ, Name = เลขที่ใบแจ้งหนี้ของผู้ขาย
	Kind                 string
	VatKind              string // output | undue
	SupplierCreditNoteNo string
	CreditNoteDate       time.Time
	Reason               string
	Totals               Totals
}

type CreditNoteLine struct {
	LineNo             int
	InvoiceLineNo      int
	Item               Ref
	Quantity           vo.Quantity
	UnitPrice          vo.Money
	Amounts            Totals
	AccountID          string
	Stock              bool
	Cost               vo.Money
	InventoryAccountID *string
	Variance           vo.Money
}

type CreditNote struct {
	CreditNoteSummary
	BillFrom         BillFrom
	Description      string
	VatRate          vo.Money
	PriceIncludesVat bool
	WarehouseID      *string
	System           string
	StockDocumentNo  string
	CreatedBy        string
	Lines            []CreditNoteLine
}

// InputVatRow: แถวรายงานภาษีซื้อ — ใบลดหนี้เป็นยอดติดลบ
type InputVatRow struct {
	Date           time.Time
	DocumentID     string
	DocumentNo     string
	Kind           string // invoice | tax_invoice (ใบกำกับที่ได้รับภายหลัง) | credit_note
	TaxInvoiceNo   string
	TaxInvoiceDate time.Time
	ReferenceNo    string // ใบลดหนี้: เลขที่ใบแจ้งหนี้ของผู้ขายเดิม
	Supplier       Ref
	TaxID          string
	BranchNo       string
	Base           vo.Money
	Tax            vo.Money
}

type InputVat struct {
	Start, End time.Time
	Rows       listing.Result[InputVatRow]
	TotalBase  vo.Money
	TotalTax   vo.Money
}

// VatTotals: ยอดรวมภาษีซื้อของช่วงวันที่ (หักใบลดหนี้)
type VatTotals struct {
	Base      vo.Money
	Tax       vo.Money
	Documents int
}

// BillFrom คือผู้ขายตามใบกำกับภาษี ณ วันบันทึก
type BillFrom struct {
	Name     string
	TaxID    string
	BranchNo string
	Address  string
}

type Invoice struct {
	InvoiceSummary
	BillFrom         BillFrom
	Description      string
	VatRate          vo.Money
	PriceIncludesVat bool
	CreatedBy        string
	Lines            []InvoiceLine
	TaxInvoices      []TaxInvoiceRecord // ใบกำกับที่ได้รับภายหลัง (รวมที่ยกเลิก เรียงวันที่)
	DefaultVat       vo.Money           // ภาษีซื้อตั้งต้นของใบกำกับถัดไป (0 = ไม่มีที่รอ)
}

type OutstandingRow struct {
	Order        Ref // Code = เลขที่ใบสั่งซื้อ
	Date         time.Time
	DeliveryDate *time.Time
	Supplier     Ref
	LineNo       int
	Item         Ref
	Quantity     vo.Quantity
	Received     vo.Quantity
	Invoiced     vo.Quantity
	ToReceive    vo.Quantity
	ToInvoice    vo.Quantity // รับแล้วยังไม่แจ้งหนี้
}

// AgingRow: ยอดค้างต่อผู้ขาย แยกตามจำนวนวันที่เลยกำหนดชำระ ณ วันที่รายงาน
type AgingRow struct {
	Supplier                                  Ref
	NotDue                                    vo.Money
	Days1To30, Days31To60, Days61To90, Over90 vo.Money
	Total                                     vo.Money
}

type Aging struct {
	AsOf   time.Time
	Rows   listing.Result[AgingRow]
	Totals AgingRow // รวมทุกผู้ขายที่ตรงเงื่อนไข (Supplier ว่าง)
}

// ---------- การตัดยอดใบแจ้งหนี้ (เรียกโดย finance) ----------

// SettlementDoc คือเอกสารจ่ายเงินของ finance ที่ตัดยอด
type SettlementDoc struct {
	ID   uuid.UUID
	No   string
	Date time.Time
}

type SettlementLine struct {
	InvoiceID vo.InvoiceID
	Amount    vo.Money
}

type SettledInvoice struct {
	ID                vo.InvoiceID
	DocumentNo        string
	SupplierInvoiceNo string
	Net               vo.Money
	Gross             vo.Money
}

type SettlementParty struct {
	ID       vo.SupplierID
	Code     string
	Name     string
	TaxID    string
	BranchNo string
	Address  string
	TaxType  vo.TaxType
}

// SettlementResult: ผู้ขาย ณ วันจ่าย + บัญชีเจ้าหนี้ (ค่าตั้งจัดซื้อ) + ใบแจ้งหนี้ตามลำดับที่ส่งมา
type SettlementResult struct {
	Supplier    SettlementParty
	APAccountID vo.AccountID
	Invoices    []SettledInvoice
}

type OpenInvoice struct {
	Kind              string // goods = ซื้อสินค้า, service = ซื้อบริการ
	ID                string
	DocumentNo        string
	SupplierInvoiceNo string
	Date              time.Time
	DueDate           time.Time
	Net               vo.Money
	Gross             vo.Money
	Outstanding       vo.Money
}
