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

type DeliveryInput struct {
	OrderID     vo.OrderID
	Date        time.Time
	WarehouseID *vo.WarehouseID
	Reference   string
	Description string
	Lines       []ShipmentInput
}

// InvoiceInput: WithDelivery = ใบส่งของ / ใบกำกับภาษี (ตัดสต็อกพร้อมออกบิล; ต้องมี WarehouseID เมื่อมีสินค้า stock)
type InvoiceInput struct {
	OrderID      vo.OrderID
	Date         time.Time
	WithDelivery bool
	WarehouseID  *vo.WarehouseID
	Reference    string
	Description  string
	Lines        []ShipmentInput
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
	OrderID    *vo.OrderID // ใบส่งของ / ใบแจ้งหนี้ของใบสั่งขายนี้
	ItemID     *vo.ItemID  // ใบเสนอราคา / ใบสั่งขาย: มีบรรทัดของสินค้านี้
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
}

type Totals struct {
	Net   vo.Money
	Vat   vo.Money
	Gross vo.Money
}

type Settings struct {
	VatRegistered      bool
	VatRate            vo.Money
	JournalID          *string
	ARAccountID        *string
	OutputVatAccountID *string
	RevenueAccountID   *string
	CogsAccountID      *string
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

type DeliveryLine struct {
	OrderLineNo int
	Item        Ref
	Quantity    vo.Quantity
	Cost        vo.Money
}

type Delivery struct {
	PostedSummary
	StockInfo
	TotalCost   vo.Money
	Reference   string
	Description string
	Lines       []DeliveryLine
}

type InvoiceLine struct {
	OrderLineNo      int
	Item             Ref
	Quantity         vo.Quantity
	UnitPrice        vo.Money
	Amounts          Totals
	RevenueAccountID string
	Cost             vo.Money
}

type InvoiceSummary struct {
	PostedSummary
	WithDelivery bool
	DueDate      time.Time
	Totals       Totals
	Received     vo.Money
	Outstanding  vo.Money
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

// TaxRow: รายงานภาษีขาย (ใบแจ้งหนี้) / ภาษีถูกหัก ณ ที่จ่าย (ใบเสร็จ)
type TaxRow struct {
	Date       time.Time
	DocumentID string
	DocumentNo string
	Customer   Ref
	TaxID      string
	BranchNo   string
	TaxType    string
	Base       vo.Money // ภาษีขาย: ยอดก่อน VAT; ถูกหัก: ยอดตัดหนี้
	Tax        vo.Money
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

type SettledInvoice struct {
	ID         vo.InvoiceID
	DocumentNo string
	Net        vo.Money
	Gross      vo.Money
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
	Customer    SettlementParty
	ARAccountID vo.AccountID
	Invoices    []SettledInvoice
}

type OpenInvoice struct {
	ID          string
	DocumentNo  string
	Date        time.Time
	DueDate     time.Time
	Net         vo.Money
	Gross       vo.Money
	Outstanding vo.Money
}
