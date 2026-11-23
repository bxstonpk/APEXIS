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
	ID                string
	DocumentNo        string
	SupplierInvoiceNo string
	Date              time.Time
	DueDate           time.Time
	Net               vo.Money
	Gross             vo.Money
	Outstanding       vo.Money
}
