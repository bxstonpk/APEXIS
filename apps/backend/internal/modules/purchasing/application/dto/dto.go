// Package dto คือข้อมูลขาเข้า (Input / Filter) และขาออก (read model) ของ use case purchasing
package dto

import (
	"time"

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
}

type ReceiptInput struct {
	OrderID     vo.OrderID
	Date        time.Time
	WarehouseID *vo.WarehouseID
	Reference   string
	Description string
	Lines       []LineQty
}

type InvoiceInput struct {
	OrderID           vo.OrderID
	SupplierInvoiceNo string
	InvoiceDate       time.Time
	Date              time.Time
	Description       string
	Lines             []LineQty
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
	Search     string
	Statuses   []string
	SupplierID *vo.SupplierID
	OrderID    *vo.OrderID // ใบรับ / ใบแจ้งหนี้ของใบสั่งซื้อนี้
	ItemID     *vo.ItemID  // เฉพาะใบสั่งซื้อ: มีบรรทัดของสินค้านี้
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
	Search     string // เลขที่ใบสั่งซื้อ / รหัส / ชื่อผู้ขาย
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

// ---------- Output (read model) ----------

type Ref struct {
	ID   string
	Code string
	Name string
}

type Settings struct {
	WhtPnd3AccountID       *string
	WhtPnd53AccountID      *string
	VatRate                vo.Money
	JournalID              *string
	GRIRAccountID          *string
	APAccountID            *string
	InputVatAccountID      *string
	PriceVarianceAccountID *string
}

type Supplier struct {
	ID              string
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	PaymentTermDays int
	VatRegistered   bool
	TaxType         string
	Status          string
}

type Totals struct {
	Net   vo.Money
	Vat   vo.Money
	Gross vo.Money
}

type OrderSummary struct {
	ID               string
	OrderNo          string // ว่างจนกว่าจะอนุมัติ
	Date             time.Time
	DeliveryDate     *time.Time
	Supplier         Ref
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
	Paid              vo.Money
	Outstanding       vo.Money
}

type InvoiceLine struct {
	OrderLineNo       int
	Item              Ref
	Quantity          vo.Quantity
	UnitPrice         vo.Money
	Amounts           Totals
	Cleared           vo.Money
	Variance          vo.Money
	VarianceAccountID string
}

type Invoice struct {
	InvoiceSummary
	Description      string
	VatRate          vo.Money
	PriceIncludesVat bool
	CreatedBy        string
	Lines            []InvoiceLine
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

// ---------- ภาษีหัก ณ ที่จ่าย / ใบจ่ายชำระ ----------

// WhtTypeFilter: GET /withholding-tax-types?q=&status=&sort=code|name&page=&page_size=
type WhtTypeFilter struct {
	Search   string
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

type WhtType struct {
	ID     string
	Code   string
	Name   string
	Rate   vo.Money
	Status string
}

type PaymentLineInput struct {
	InvoiceID vo.InvoiceID
	Amount    vo.Money
	WhtTypeID *vo.WhtTypeID
}

type PaymentMethodInput struct {
	CashAccountID vo.CashAccountID
	Amount        vo.Money
	Reference     string
}

type PaymentInput struct {
	SupplierID  vo.SupplierID
	Date        time.Time
	Description string
	Lines       []PaymentLineInput
	Methods     []PaymentMethodInput
}

type PaymentSummary struct {
	ID          string
	DocumentNo  string
	Date        time.Time
	Supplier    Ref
	TaxForm     string // pnd3 / pnd53
	Amount      vo.Money
	Wht         vo.Money
	Paid        vo.Money
	Status      string
	EntryID     *string
	EntryNo     string
	CancelledAt *time.Time
}

type PaymentLine struct {
	Invoice Ref // Code = เลขที่ใบแจ้งหนี้ (PI...), Name = เลขที่ใบแจ้งหนี้ของผู้ขาย
	Amount  vo.Money
	WhtType *Ref
	WhtRate vo.Money
	WhtBase vo.Money
	Wht     vo.Money
}

type PaymentMethod struct {
	CashAccountID string
	GLAccountID   string
	Amount        vo.Money
	Reference     string
}

type Payment struct {
	PaymentSummary
	Description string
	CreatedBy   string
	Lines       []PaymentLine
	Methods     []PaymentMethod
}

var (
	WithholdingSortKeys    = []string{"date", "document_no", "supplier_code"}
	DefaultWithholdingSort = []listing.Sort{{Key: "date"}, {Key: "document_no"}}
)

// WithholdingFilter: book=withholding-tax — ภาษีหัก ณ ที่จ่ายตามวันที่จ่าย (start_date / end_date)
type WithholdingFilter struct {
	Start, End time.Time
	Form       string // pnd3 / pnd53 / "" = ทั้งหมด
	SupplierID *vo.SupplierID
	Sort       []listing.Sort
	Page       listing.Page
}

type WithholdingRow struct {
	Date       time.Time
	PaymentID  string
	DocumentNo string
	Supplier   Ref
	TaxID      string
	BranchNo   string
	TaxForm    string
	WhtType    Ref
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
