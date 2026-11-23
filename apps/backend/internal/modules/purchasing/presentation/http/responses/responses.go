// Package responses คือรูปแบบ JSON ขาออกของ purchasing
//
//   - วันที่ "YYYY-MM-DD" (ค.ศ.)
//   - จำนวน / มูลค่า / อัตรา VAT เป็น string ทศนิยมตามค่าจริง — frontend จัดรูปแบบตามทศนิยมที่บริษัทกำหนด
package responses

import (
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/platform/httpx"
)

func date(t time.Time) string { return t.Format(time.DateOnly) }

func optDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := date(*t)
	return &s
}

func str(d decimal.Decimal) string { return d.String() }

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func ref(r dto.Ref) Ref { return Ref{ID: r.ID, Code: r.Code, Name: r.Name} }

type Totals struct {
	Net   string `json:"net"`
	Vat   string `json:"vat"`
	Gross string `json:"gross"`
}

func totals(t dto.Totals) Totals {
	return Totals{Net: str(t.Net), Vat: str(t.Vat), Gross: str(t.Gross)}
}

// ---------- ค่าตั้ง / ผู้ขาย ----------

type Settings struct {
	VatRate                string  `json:"vat_rate"`
	JournalID              *string `json:"journal_id"`
	GRIRAccountID          *string `json:"grir_account_id"`
	APAccountID            *string `json:"ap_account_id"`
	InputVatAccountID      *string `json:"input_vat_account_id"`
	PriceVarianceAccountID *string `json:"price_variance_account_id"`
}

func FromSettings(s dto.Settings) Settings {
	return Settings{
		VatRate: str(s.VatRate), JournalID: s.JournalID, GRIRAccountID: s.GRIRAccountID, APAccountID: s.APAccountID,
		InputVatAccountID: s.InputVatAccountID, PriceVarianceAccountID: s.PriceVarianceAccountID,
	}
}

type Supplier struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	TaxID           string `json:"tax_id"`
	BranchNo        string `json:"branch_no"`
	Address         string `json:"address"`
	PaymentTermDays int    `json:"payment_term_days"`
	VatRegistered   bool   `json:"vat_registered"`
	TaxType         string `json:"tax_type"`
	Status          string `json:"status"`
}

func FromSupplier(s dto.Supplier) Supplier {
	return Supplier{
		ID: s.ID, Code: s.Code, Name: s.Name, TaxID: s.TaxID, BranchNo: s.BranchNo, Address: s.Address,
		PaymentTermDays: s.PaymentTermDays, VatRegistered: s.VatRegistered, TaxType: s.TaxType, Status: s.Status,
	}
}

// ---------- ใบสั่งซื้อ ----------

type OrderSummary struct {
	ID               string  `json:"id"`
	OrderNo          *string `json:"order_no"` // null จนกว่าจะอนุมัติ
	Date             string  `json:"date"`
	DeliveryDate     *string `json:"delivery_date"`
	Supplier         Ref     `json:"supplier"`
	Reference        string  `json:"reference"`
	VatRate          string  `json:"vat_rate"`
	PriceIncludesVat bool    `json:"price_includes_vat"`
	Totals           Totals  `json:"totals"`
	Status           string  `json:"status"`
}

func FromOrderSummary(o dto.OrderSummary) OrderSummary {
	out := OrderSummary{
		ID: o.ID, Date: date(o.Date), DeliveryDate: optDate(o.DeliveryDate), Supplier: ref(o.Supplier), Reference: o.Reference,
		VatRate: str(o.VatRate), PriceIncludesVat: o.PriceIncludesVat, Totals: totals(o.Totals), Status: o.Status,
	}
	if o.OrderNo != "" {
		no := o.OrderNo
		out.OrderNo = &no
	}
	return out
}

type OrderLine struct {
	LineNo      int    `json:"line_no"`
	Item        Ref    `json:"item"`
	Description string `json:"description"`
	Quantity    string `json:"quantity"`
	UnitPrice   string `json:"unit_price"`
	Amounts     Totals `json:"amounts"`
	Received    string `json:"received_quantity"`
	Invoiced    string `json:"invoiced_quantity"`
}

type Order struct {
	OrderSummary
	Description     string      `json:"description"`
	VatRegistered   bool        `json:"vat_registered"`
	PaymentTermDays int         `json:"payment_term_days"`
	CreatedBy       string      `json:"created_by"`
	ApprovedBy      *string     `json:"approved_by"`
	ApprovedAt      *time.Time  `json:"approved_at"`
	ClosedAt        *time.Time  `json:"closed_at"`
	CancelledAt     *time.Time  `json:"cancelled_at"`
	Lines           []OrderLine `json:"lines"`
}

func FromOrder(o dto.Order) Order {
	out := Order{
		OrderSummary: FromOrderSummary(o.OrderSummary), Description: o.Description, VatRegistered: o.VatRegistered,
		PaymentTermDays: o.PaymentTermDays, CreatedBy: o.CreatedBy, ApprovedBy: o.ApprovedBy, ApprovedAt: o.ApprovedAt,
		ClosedAt: o.ClosedAt, CancelledAt: o.CancelledAt, Lines: make([]OrderLine, len(o.Lines)),
	}
	for i, l := range o.Lines {
		out.Lines[i] = OrderLine{
			LineNo: l.LineNo, Item: ref(l.Item), Description: l.Description, Quantity: str(l.Quantity), UnitPrice: str(l.UnitPrice),
			Amounts: totals(l.Amounts), Received: str(l.Received), Invoiced: str(l.Invoiced),
		}
	}
	return out
}

// ---------- ใบรับสินค้า / ใบแจ้งหนี้ ----------

type DocumentSummary struct {
	ID          string     `json:"id"`
	DocumentNo  string     `json:"document_no"`
	Date        string     `json:"date"`
	Order       OrderRef   `json:"order"`
	Supplier    Ref        `json:"supplier"`
	Status      string     `json:"status"`
	EntryID     *string    `json:"entry_id"` // รายการบัญชีใน ledger (null = มูลค่าเป็นศูนย์)
	EntryNo     string     `json:"entry_no"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

type OrderRef struct {
	ID      string `json:"id"`
	OrderNo string `json:"order_no"`
}

func docSummary(d dto.DocumentSummary) DocumentSummary {
	return DocumentSummary{
		ID: d.ID, DocumentNo: d.DocumentNo, Date: date(d.Date), Order: OrderRef{ID: d.Order.ID, OrderNo: d.Order.Code},
		Supplier: ref(d.Supplier), Status: d.Status, EntryID: d.EntryID, EntryNo: d.EntryNo, CancelledAt: d.CancelledAt,
	}
}

type ReceiptSummary struct {
	DocumentSummary
	WarehouseID     *string `json:"warehouse_id"`
	StockDocumentNo string  `json:"stock_document_no"` // ใบรับเข้าของ inventory (ว่าง = ไม่มีสินค้า stock)
	System          string  `json:"accounting_system"`
	TotalValue      string  `json:"total_value"`
}

func FromReceiptSummary(r dto.ReceiptSummary) ReceiptSummary {
	return ReceiptSummary{
		DocumentSummary: docSummary(r.DocumentSummary), WarehouseID: r.WarehouseID, StockDocumentNo: r.StockDocumentNo,
		System: r.System, TotalValue: str(r.TotalValue),
	}
}

type ReceiptLine struct {
	OrderLineNo int    `json:"order_line_no"`
	Item        Ref    `json:"item"`
	Quantity    string `json:"quantity"`
	UnitCost    string `json:"unit_cost"`
	Value       string `json:"value"`
	AccountID   string `json:"account_id"`
}

type Receipt struct {
	ReceiptSummary
	Reference   string        `json:"reference"`
	Description string        `json:"description"`
	CreatedBy   string        `json:"created_by"`
	Lines       []ReceiptLine `json:"lines"`
}

func FromReceipt(r dto.Receipt) Receipt {
	out := Receipt{
		ReceiptSummary: FromReceiptSummary(r.ReceiptSummary), Reference: r.Reference, Description: r.Description,
		CreatedBy: r.CreatedBy, Lines: make([]ReceiptLine, len(r.Lines)),
	}
	for i, l := range r.Lines {
		out.Lines[i] = ReceiptLine{
			OrderLineNo: l.OrderLineNo, Item: ref(l.Item), Quantity: str(l.Quantity), UnitCost: str(l.UnitCost),
			Value: str(l.Value), AccountID: l.AccountID,
		}
	}
	return out
}

type InvoiceSummary struct {
	DocumentSummary
	SupplierInvoiceNo string `json:"supplier_invoice_no"`
	InvoiceDate       string `json:"invoice_date"`
	DueDate           string `json:"due_date"`
	Totals            Totals `json:"totals"`
	Paid              string `json:"paid_amount"`
	Outstanding       string `json:"outstanding"`
}

func FromInvoiceSummary(i dto.InvoiceSummary) InvoiceSummary {
	return InvoiceSummary{
		DocumentSummary: docSummary(i.DocumentSummary), SupplierInvoiceNo: i.SupplierInvoiceNo,
		InvoiceDate: date(i.InvoiceDate), DueDate: date(i.DueDate), Totals: totals(i.Totals),
		Paid: str(i.Paid), Outstanding: str(i.Outstanding),
	}
}

type InvoiceLine struct {
	OrderLineNo       int     `json:"order_line_no"`
	Item              Ref     `json:"item"`
	Quantity          string  `json:"quantity"`
	UnitPrice         string  `json:"unit_price"`
	Amounts           Totals  `json:"amounts"`
	Cleared           string  `json:"grir_cleared"`
	Variance          string  `json:"price_variance"`
	VarianceAccountID *string `json:"price_variance_account_id"`
}

type Invoice struct {
	InvoiceSummary
	Description      string        `json:"description"`
	VatRate          string        `json:"vat_rate"`
	PriceIncludesVat bool          `json:"price_includes_vat"`
	CreatedBy        string        `json:"created_by"`
	Lines            []InvoiceLine `json:"lines"`
}

func FromInvoice(i dto.Invoice) Invoice {
	out := Invoice{
		InvoiceSummary: FromInvoiceSummary(i.InvoiceSummary), Description: i.Description, VatRate: str(i.VatRate),
		PriceIncludesVat: i.PriceIncludesVat, CreatedBy: i.CreatedBy, Lines: make([]InvoiceLine, len(i.Lines)),
	}
	for n, l := range i.Lines {
		line := InvoiceLine{
			OrderLineNo: l.OrderLineNo, Item: ref(l.Item), Quantity: str(l.Quantity), UnitPrice: str(l.UnitPrice),
			Amounts: totals(l.Amounts), Cleared: str(l.Cleared), Variance: str(l.Variance),
		}
		if l.VarianceAccountID != "" {
			acc := l.VarianceAccountID
			line.VarianceAccountID = &acc
		}
		out.Lines[n] = line
	}
	return out
}

// ---------- รายงาน ----------

type OutstandingRow struct {
	Order        OrderRef `json:"order"`
	Date         string   `json:"date"`
	DeliveryDate *string  `json:"delivery_date"`
	Supplier     Ref      `json:"supplier"`
	LineNo       int      `json:"line_no"`
	Item         Ref      `json:"item"`
	Quantity     string   `json:"quantity"`
	Received     string   `json:"received_quantity"`
	Invoiced     string   `json:"invoiced_quantity"`
	ToReceive    string   `json:"to_receive"`
	ToInvoice    string   `json:"to_invoice"`
}

type Outstanding struct {
	Book string           `json:"book"`
	Rows []OutstandingRow `json:"rows"`
}

type OutstandingBody struct {
	Data Outstanding `json:"data"`
	Meta httpx.Meta  `json:"meta"`
}

func FromOutstanding(book string, r httpx.List[OutstandingRow]) OutstandingBody {
	return OutstandingBody{Data: Outstanding{Book: book, Rows: r.Data}, Meta: r.Meta}
}

func FromOutstandingRow(r dto.OutstandingRow) OutstandingRow {
	return OutstandingRow{
		Order: OrderRef{ID: r.Order.ID, OrderNo: r.Order.Code}, Date: date(r.Date), DeliveryDate: optDate(r.DeliveryDate),
		Supplier: ref(r.Supplier), LineNo: r.LineNo, Item: ref(r.Item), Quantity: str(r.Quantity), Received: str(r.Received),
		Invoiced: str(r.Invoiced), ToReceive: str(r.ToReceive), ToInvoice: str(r.ToInvoice),
	}
}

type AgingAmounts struct {
	NotDue     string `json:"not_due"`
	Days1To30  string `json:"days_1_30"`
	Days31To60 string `json:"days_31_60"`
	Days61To90 string `json:"days_61_90"`
	Over90     string `json:"over_90"`
	Total      string `json:"total"`
}

func agingAmounts(r dto.AgingRow) AgingAmounts {
	return AgingAmounts{
		NotDue: str(r.NotDue), Days1To30: str(r.Days1To30), Days31To60: str(r.Days31To60), Days61To90: str(r.Days61To90),
		Over90: str(r.Over90), Total: str(r.Total),
	}
}

type AgingRow struct {
	Supplier Ref `json:"supplier"`
	AgingAmounts
}

type Aging struct {
	Book   string       `json:"book"`
	AsOf   string       `json:"as_of"`
	Rows   []AgingRow   `json:"rows"`
	Totals AgingAmounts `json:"totals"` // รวมทุกผู้ขายที่ตรงเงื่อนไข (ไม่ใช่แค่หน้านี้)
}

type AgingBody struct {
	Data Aging      `json:"data"`
	Meta httpx.Meta `json:"meta"`
}

func FromAging(book string, a dto.Aging) AgingBody {
	rows := make([]AgingRow, len(a.Rows.Items))
	for i, r := range a.Rows.Items {
		rows[i] = AgingRow{Supplier: ref(r.Supplier), AgingAmounts: agingAmounts(r)}
	}
	return AgingBody{
		Data: Aging{Book: book, AsOf: date(a.AsOf), Rows: rows, Totals: agingAmounts(a.Totals)},
		Meta: httpx.Meta{Page: a.Rows.Page.Number, PageSize: a.Rows.Page.Size, Total: a.Rows.Total},
	}
}
