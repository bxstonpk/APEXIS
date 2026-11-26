// Package responses คือ JSON ขาออกของ sales — วันที่ "YYYY-MM-DD", ตัวเลขเป็นข้อความทศนิยมตามค่าจริง
package responses

import (
	"apexis/backend/internal/platform/thaiaddress"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/listing"
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

func optStr(d *decimal.Decimal) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

func optNo(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

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

func meta[T any](r listing.Result[T]) httpx.Meta {
	return httpx.Meta{Page: r.Page.Number, PageSize: r.Page.Size, Total: r.Total}
}

// ---------- ค่าตั้ง / ลูกค้า ----------

type Settings struct {
	VatRegistered         bool    `json:"vat_registered"`
	VatRate               string  `json:"vat_rate"`
	JournalID             *string `json:"journal_id"`
	ARAccountID           *string `json:"ar_account_id"`
	OutputVatAccountID    *string `json:"output_vat_account_id"`
	RevenueAccountID      *string `json:"revenue_account_id"`
	CogsAccountID         *string `json:"cogs_account_id"`
	SalesReturnsAccountID *string `json:"sales_returns_account_id"`
}

func FromSettings(s dto.Settings) Settings {
	return Settings{
		VatRegistered: s.VatRegistered, VatRate: str(s.VatRate), JournalID: s.JournalID, ARAccountID: s.ARAccountID,
		OutputVatAccountID: s.OutputVatAccountID, RevenueAccountID: s.RevenueAccountID, CogsAccountID: s.CogsAccountID,
		SalesReturnsAccountID: s.SalesReturnsAccountID,
	}
}

type Customer struct {
	ID              string           `json:"id"`
	Code            string           `json:"code"`
	Name            string           `json:"name"`
	TaxID           string           `json:"tax_id"`
	BranchNo        string           `json:"branch_no"`
	Address         string           `json:"address"`
	AddressDetail   thaiaddress.JSON `json:"address_detail"`
	PaymentTermDays int              `json:"payment_term_days"`
	TaxType         string           `json:"tax_type"`
	CreditLimit     *string          `json:"credit_limit"`
	Status          string           `json:"status"`
	Branches        []Branch         `json:"branches,omitempty"` // เฉพาะ GET รายตัว
}

func FromCustomer(c dto.Customer) Customer {
	return Customer{
		ID: c.ID, Code: c.Code, Name: c.Name, TaxID: c.TaxID, BranchNo: c.BranchNo, Address: c.Address, AddressDetail: thaiaddress.ToJSON(c.AddressDetail),
		PaymentTermDays: c.PaymentTermDays, TaxType: c.TaxType, CreditLimit: optStr(c.CreditLimit), Status: c.Status,
		Branches: branchList(c.Branches),
	}
}

type Branch struct {
	BranchNo      string           `json:"branch_no"`
	Name          string           `json:"name"`
	Address       string           `json:"address"`
	AddressDetail thaiaddress.JSON `json:"address_detail"`
}

func branchList(in []dto.CustomerBranch) []Branch {
	if in == nil {
		return nil
	}
	out := make([]Branch, len(in))
	for i, b := range in {
		out[i] = Branch{BranchNo: b.BranchNo, Name: b.Name, Address: b.Address, AddressDetail: thaiaddress.ToJSON(b.AddressDetail)}
	}
	return out
}

// ---------- ใบเสนอราคา / ใบสั่งขาย ----------

type SalesDocSummary struct {
	ID               string  `json:"id"`
	DocumentNo       *string `json:"document_no"` // null จนกว่าจะส่ง (QT) / อนุมัติ (SO)
	Date             string  `json:"date"`
	Customer         Ref     `json:"customer"`
	CustomerBranchNo string  `json:"customer_branch_no"`
	Reference        string  `json:"reference"`
	VatRate          string  `json:"vat_rate"`
	PriceIncludesVat bool    `json:"price_includes_vat"`
	Totals           Totals  `json:"totals"`
	Status           string  `json:"status"`
}

func FromSalesDocSummary(s dto.SalesDocSummary) SalesDocSummary {
	return SalesDocSummary{
		ID: s.ID, DocumentNo: optNo(s.DocumentNo), Date: date(s.Date), Customer: ref(s.Customer), CustomerBranchNo: s.CustomerBranchNo, Reference: s.Reference,
		VatRate: str(s.VatRate), PriceIncludesVat: s.PriceIncludesVat, Totals: totals(s.Totals), Status: s.Status,
	}
}

type SalesLine struct {
	LineNo      int    `json:"line_no"`
	Item        Ref    `json:"item"`
	Description string `json:"description"`
	Quantity    string `json:"quantity"`
	UnitPrice   string `json:"unit_price"`
	Amounts     Totals `json:"amounts"`
}

type OrderLine struct {
	SalesLine
	Delivered string `json:"delivered_quantity"`
	Invoiced  string `json:"invoiced_quantity"`
}

func salesLine(l dto.SalesLine) SalesLine {
	return SalesLine{LineNo: l.LineNo, Item: ref(l.Item), Description: l.Description, Quantity: str(l.Quantity), UnitPrice: str(l.UnitPrice), Amounts: totals(l.Amounts)}
}

type Quotation struct {
	SalesDocSummary
	ValidUntil      string      `json:"valid_until"`
	Description     string      `json:"description"`
	PaymentTermDays int         `json:"payment_term_days"`
	OrderID         *string     `json:"order_id"`
	CreatedBy       string      `json:"created_by"`
	Lines           []SalesLine `json:"lines"`
}

func FromQuotation(q dto.Quotation) Quotation {
	out := Quotation{
		SalesDocSummary: FromSalesDocSummary(q.SalesDocSummary), ValidUntil: date(q.ValidUntil), Description: q.Description,
		PaymentTermDays: q.PaymentTermDays, OrderID: q.OrderID, CreatedBy: q.CreatedBy, Lines: make([]SalesLine, len(q.Lines)),
	}
	for i, l := range q.Lines {
		out.Lines[i] = salesLine(l)
	}
	return out
}

type Order struct {
	SalesDocSummary
	QuotationID     *string     `json:"quotation_id"`
	DeliveryDate    *string     `json:"delivery_date"`
	Description     string      `json:"description"`
	PaymentTermDays int         `json:"payment_term_days"`
	CreatedBy       string      `json:"created_by"`
	ApprovedAt      *time.Time  `json:"approved_at"`
	ClosedAt        *time.Time  `json:"closed_at"`
	CancelledAt     *time.Time  `json:"cancelled_at"`
	Lines           []OrderLine `json:"lines"`
	Warnings        []string    `json:"warnings"` // เช่น เกินวงเงินเครดิต (ตอนอนุมัติ)
}

func FromOrder(o dto.Order) Order {
	out := Order{
		SalesDocSummary: FromSalesDocSummary(o.SalesDocSummary), QuotationID: o.QuotationID, DeliveryDate: optDate(o.DeliveryDate),
		Description: o.Description, PaymentTermDays: o.PaymentTermDays, CreatedBy: o.CreatedBy, ApprovedAt: o.ApprovedAt,
		ClosedAt: o.ClosedAt, CancelledAt: o.CancelledAt, Lines: make([]OrderLine, len(o.Lines)), Warnings: o.Warnings,
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	for i, l := range o.Lines {
		out.Lines[i] = OrderLine{SalesLine: salesLine(l), Delivered: str(l.Delivered), Invoiced: str(l.Invoiced)}
	}
	return out
}

// ---------- ใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ ----------

type OrderRef struct {
	ID      string `json:"id"`
	OrderNo string `json:"order_no"`
}

type PostedSummary struct {
	ID          string     `json:"id"`
	DocumentNo  string     `json:"document_no"`
	Date        string     `json:"date"`
	Order       *OrderRef  `json:"order,omitempty"`
	Customer    Ref        `json:"customer"`
	Status      string     `json:"status"`
	EntryID     *string    `json:"entry_id"`
	EntryNo     string     `json:"entry_no"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

func FromPostedSummary(p dto.PostedSummary) PostedSummary {
	out := PostedSummary{
		ID: p.ID, DocumentNo: p.DocumentNo, Date: date(p.Date), Customer: ref(p.Customer), Status: p.Status,
		EntryID: p.EntryID, EntryNo: p.EntryNo, CancelledAt: p.CancelledAt,
	}
	if p.Order.ID != "" {
		out.Order = &OrderRef{ID: p.Order.ID, OrderNo: p.Order.Code}
	}
	return out
}

type StockInfo struct {
	WarehouseID     *string `json:"warehouse_id"`
	System          string  `json:"accounting_system"`
	StockDocumentNo string  `json:"stock_document_no"` // ใบเบิกของ inventory (ว่าง = ไม่มีสินค้า stock)
}

func stockInfo(s dto.StockInfo) StockInfo {
	return StockInfo{WarehouseID: s.WarehouseID, System: s.System, StockDocumentNo: s.StockDocumentNo}
}

type DeliveryLine struct {
	OrderLineNo int    `json:"order_line_no"`
	Item        Ref    `json:"item"`
	Quantity    string `json:"quantity"`
	Cost        string `json:"cost"`
}

type Delivery struct {
	PostedSummary
	StockInfo
	TotalCost   string         `json:"total_cost"`
	Reference   string         `json:"reference"`
	Description string         `json:"description"`
	Lines       []DeliveryLine `json:"lines"`
}

func FromDelivery(d dto.Delivery) Delivery {
	out := Delivery{
		PostedSummary: FromPostedSummary(d.PostedSummary), StockInfo: stockInfo(d.StockInfo), TotalCost: str(d.TotalCost),
		Reference: d.Reference, Description: d.Description, Lines: make([]DeliveryLine, len(d.Lines)),
	}
	for i, l := range d.Lines {
		out.Lines[i] = DeliveryLine{OrderLineNo: l.OrderLineNo, Item: ref(l.Item), Quantity: str(l.Quantity), Cost: str(l.Cost)}
	}
	return out
}

type InvoiceSummary struct {
	PostedSummary
	WithDelivery bool   `json:"with_delivery"`
	DueDate      string `json:"due_date"`
	Totals       Totals `json:"totals"`
	Received     string `json:"received_amount"` // รับชำระ (ใบเสร็จ) — ไม่รวมใบลดหนี้
	Credited     string `json:"credited_amount"` // ใบลดหนี้ที่ยังไม่ยกเลิก (รวม VAT)
	Outstanding  string `json:"outstanding"`     // ยอดรวม - รับชำระ - ลดหนี้
}

func FromInvoiceSummary(i dto.InvoiceSummary) InvoiceSummary {
	return InvoiceSummary{
		PostedSummary: FromPostedSummary(i.PostedSummary), WithDelivery: i.WithDelivery, DueDate: date(i.DueDate),
		Totals: totals(i.Totals), Received: str(i.Received), Credited: str(i.Credited), Outstanding: str(i.Outstanding),
	}
}

type InvoiceLine struct {
	LineNo           int    `json:"line_no"` // ใบลดหนี้อ้างด้วย invoice_line_no
	OrderLineNo      int    `json:"order_line_no"`
	Item             Ref    `json:"item"`
	Quantity         string `json:"quantity"`
	UnitPrice        string `json:"unit_price"`
	Amounts          Totals `json:"amounts"`
	RevenueAccountID string `json:"revenue_account_id"`
	Cost             string `json:"cost"`
	Returned         string `json:"returned_quantity"` // รับคืนแล้ว (ใบลดหนี้ที่ยังไม่ยกเลิก)
	Credited         string `json:"credited_amount"`   // ยอดก่อน VAT ที่ลดหนี้แล้ว
}

// BillTo คือผู้ซื้อตามใบกำกับภาษี ณ วันออก (ชื่อ / เลขผู้เสียภาษี / สาขา / ที่อยู่)
type BillTo struct {
	Name     string `json:"name"`
	TaxID    string `json:"tax_id"`
	BranchNo string `json:"branch_no"`
	Address  string `json:"address"`
}

type Invoice struct {
	InvoiceSummary
	StockInfo
	BillTo           BillTo        `json:"bill_to"`
	VatRate          string        `json:"vat_rate"`
	PriceIncludesVat bool          `json:"price_includes_vat"`
	Reference        string        `json:"reference"`
	Description      string        `json:"description"`
	Lines            []InvoiceLine `json:"lines"`
}

func FromInvoice(i dto.Invoice) Invoice {
	out := Invoice{
		InvoiceSummary: FromInvoiceSummary(i.InvoiceSummary), StockInfo: stockInfo(i.StockInfo), VatRate: str(i.VatRate),
		PriceIncludesVat: i.PriceIncludesVat, Reference: i.Reference, Description: i.Description, Lines: make([]InvoiceLine, len(i.Lines)),
		BillTo: BillTo{Name: i.BillTo.Name, TaxID: i.BillTo.TaxID, BranchNo: i.BillTo.BranchNo, Address: i.BillTo.Address},
	}
	for n, l := range i.Lines {
		out.Lines[n] = InvoiceLine{
			LineNo: l.LineNo, OrderLineNo: l.OrderLineNo, Item: ref(l.Item), Quantity: str(l.Quantity), UnitPrice: str(l.UnitPrice),
			Amounts: totals(l.Amounts), RevenueAccountID: l.RevenueAccountID, Cost: str(l.Cost),
			Returned: str(l.Returned), Credited: str(l.Credited),
		}
	}
	return out
}

// ---------- ใบลดหนี้ ----------

type InvoiceRef struct {
	ID         string `json:"id"`
	DocumentNo string `json:"document_no"`
}

type CreditNoteSummary struct {
	ID          string     `json:"id"`
	DocumentNo  string     `json:"document_no"`
	Date        string     `json:"date"`
	Invoice     InvoiceRef `json:"invoice"`
	Customer    Ref        `json:"customer"`
	Kind        string     `json:"kind"`
	Reason      string     `json:"reason"`
	Totals      Totals     `json:"totals"`
	Status      string     `json:"status"`
	EntryID     *string    `json:"entry_id"`
	EntryNo     string     `json:"entry_no"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

func FromCreditNoteSummary(c dto.CreditNoteSummary) CreditNoteSummary {
	return CreditNoteSummary{
		ID: c.ID, DocumentNo: c.DocumentNo, Date: date(c.Date), Invoice: InvoiceRef{ID: c.Order.ID, DocumentNo: c.Order.Code},
		Customer: ref(c.Customer), Kind: c.Kind, Reason: c.Reason, Totals: totals(c.Totals), Status: c.Status,
		EntryID: c.EntryID, EntryNo: c.EntryNo, CancelledAt: c.CancelledAt,
	}
}

type CreditNoteLine struct {
	LineNo        int    `json:"line_no"`
	InvoiceLineNo int    `json:"invoice_line_no"`
	Item          Ref    `json:"item"`
	Quantity      string `json:"quantity"`   // allowance = 0
	UnitPrice     string `json:"unit_price"` // allowance = ยอดที่ลด
	Amounts       Totals `json:"amounts"`
	AccountID     string `json:"account_id"`
	Cost          string `json:"cost"` // ต้นทุนที่รับคืนเข้าคลัง
}

// CreditNote: original_totals = ใบกำกับภาษีเดิม, credited_before = ใบลดหนี้ก่อนหน้า, correct_totals = มูลค่าที่ถูกต้อง (ม.86/10)
type CreditNote struct {
	CreditNoteSummary
	StockInfo
	InvoiceDate      string           `json:"invoice_date"`
	OriginalTotals   Totals           `json:"original_totals"`
	CreditedBefore   Totals           `json:"credited_before"`
	CorrectTotals    Totals           `json:"correct_totals"`
	BillTo           BillTo           `json:"bill_to"`
	VatRate          string           `json:"vat_rate"`
	PriceIncludesVat bool             `json:"price_includes_vat"`
	Description      string           `json:"description"`
	CreatedBy        string           `json:"created_by"`
	Lines            []CreditNoteLine `json:"lines"`
}

func FromCreditNote(c dto.CreditNote) CreditNote {
	o, b, t := c.InvoiceTotals, c.CreditedBefore, c.Totals
	out := CreditNote{
		CreditNoteSummary: FromCreditNoteSummary(c.CreditNoteSummary), StockInfo: stockInfo(c.StockInfo), InvoiceDate: date(c.InvoiceDate),
		OriginalTotals: totals(o), CreditedBefore: totals(b),
		CorrectTotals: totals(dto.Totals{Net: o.Net.Sub(b.Net).Sub(t.Net), Vat: o.Vat.Sub(b.Vat).Sub(t.Vat), Gross: o.Gross.Sub(b.Gross).Sub(t.Gross)}),
		BillTo:        BillTo{Name: c.BillTo.Name, TaxID: c.BillTo.TaxID, BranchNo: c.BillTo.BranchNo, Address: c.BillTo.Address},
		VatRate:       str(c.VatRate), PriceIncludesVat: c.PriceIncludesVat, Description: c.Description, CreatedBy: c.CreatedBy,
		Lines: make([]CreditNoteLine, len(c.Lines)),
	}
	for i, l := range c.Lines {
		out.Lines[i] = CreditNoteLine{LineNo: i + 1, InvoiceLineNo: l.InvoiceLineNo, Item: ref(l.Item), Quantity: str(l.Quantity),
			UnitPrice: str(l.UnitPrice), Amounts: totals(l.Amounts), AccountID: l.AccountID, Cost: str(l.Cost)}
	}
	return out
}

// ---------- รายงาน ----------

type OutstandingRow struct {
	Order        OrderRef `json:"order"`
	Date         string   `json:"date"`
	DeliveryDate *string  `json:"delivery_date"`
	Customer     Ref      `json:"customer"`
	LineNo       int      `json:"line_no"`
	Item         Ref      `json:"item"`
	Quantity     string   `json:"quantity"`
	Delivered    string   `json:"delivered_quantity"`
	Invoiced     string   `json:"invoiced_quantity"`
	ToDeliver    string   `json:"to_deliver"`
	ToInvoice    string   `json:"to_invoice"`
}

type ReportBody[T any] struct {
	Data T          `json:"data"`
	Meta httpx.Meta `json:"meta"`
}

type Outstanding struct {
	Book string           `json:"book"`
	Rows []OutstandingRow `json:"rows"`
}

func FromOutstanding(book string, r listing.Result[dto.OutstandingRow]) ReportBody[Outstanding] {
	rows := make([]OutstandingRow, len(r.Items))
	for i, x := range r.Items {
		rows[i] = OutstandingRow{
			Order: OrderRef{ID: x.Order.ID, OrderNo: x.Order.Code}, Date: date(x.Date), DeliveryDate: optDate(x.DeliveryDate),
			Customer: ref(x.Customer), LineNo: x.LineNo, Item: ref(x.Item), Quantity: str(x.Quantity), Delivered: str(x.Delivered),
			Invoiced: str(x.Invoiced), ToDeliver: str(x.ToDeliver), ToInvoice: str(x.ToInvoice),
		}
	}
	return ReportBody[Outstanding]{Data: Outstanding{Book: book, Rows: rows}, Meta: meta(r)}
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
	return AgingAmounts{NotDue: str(r.NotDue), Days1To30: str(r.Days1To30), Days31To60: str(r.Days31To60),
		Days61To90: str(r.Days61To90), Over90: str(r.Over90), Total: str(r.Total)}
}

type AgingRow struct {
	Customer Ref `json:"customer"`
	AgingAmounts
}

type Aging struct {
	Book   string       `json:"book"`
	AsOf   string       `json:"as_of"`
	Rows   []AgingRow   `json:"rows"`
	Totals AgingAmounts `json:"totals"`
}

func FromAging(book string, a dto.Aging) ReportBody[Aging] {
	rows := make([]AgingRow, len(a.Rows.Items))
	for i, r := range a.Rows.Items {
		rows[i] = AgingRow{Customer: ref(r.Customer), AgingAmounts: agingAmounts(r)}
	}
	return ReportBody[Aging]{Data: Aging{Book: book, AsOf: date(a.AsOf), Rows: rows, Totals: agingAmounts(a.Totals)}, Meta: meta(a.Rows)}
}

type TaxRow struct {
	Date        string `json:"date"`
	Kind        string `json:"kind"` // invoice | credit_note (ยอดติดลบ)
	DocumentID  string `json:"document_id"`
	DocumentNo  string `json:"document_no"`
	ReferenceNo string `json:"reference_no"` // ใบลดหนี้: เลขที่ใบกำกับภาษีเดิม
	Customer    Ref    `json:"customer"`
	TaxID       string `json:"tax_id"`
	BranchNo    string `json:"branch_no"`
	TaxType     string `json:"tax_type"`
	Base        string `json:"base"`
	Tax         string `json:"tax"`
}

type TaxReport struct {
	Book      string   `json:"book"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Rows      []TaxRow `json:"rows"`
	TotalBase string   `json:"total_base"`
	TotalTax  string   `json:"total_tax"`
}

func FromTax(book string, t dto.TaxReport) ReportBody[TaxReport] {
	rows := make([]TaxRow, len(t.Rows.Items))
	for i, r := range t.Rows.Items {
		rows[i] = TaxRow{Date: date(r.Date), Kind: r.Kind, DocumentID: r.DocumentID, DocumentNo: r.DocumentNo, ReferenceNo: r.ReferenceNo, Customer: ref(r.Customer),
			TaxID: r.TaxID, BranchNo: r.BranchNo, TaxType: r.TaxType, Base: str(r.Base), Tax: str(r.Tax)}
	}
	return ReportBody[TaxReport]{Data: TaxReport{Book: book, StartDate: date(t.Start), EndDate: date(t.End), Rows: rows,
		TotalBase: str(t.TotalBase), TotalTax: str(t.TotalTax)}, Meta: meta(t.Rows)}
}

// ---------- ภาพรวม (dashboard) ----------

type MonthNet struct {
	Month string `json:"month"`
	Net   string `json:"net"`
}

type Aging5 struct {
	NotDue     string `json:"not_due"`
	Days1To30  string `json:"days_1_30"`
	Days31To60 string `json:"days_31_60"`
	Days61To90 string `json:"days_61_90"`
	Over90     string `json:"over_90"`
}

type Dashboard struct {
	StartDate    string     `json:"start_date"`
	EndDate      string     `json:"end_date"`
	Sales        Comparison `json:"sales"`
	InvoiceCount int64      `json:"invoice_count"`
	Monthly      []MonthNet `json:"monthly"`
	// FiscalYear: ปีบัญชีของกราฟ (null = ไม่มีปีบัญชีครอบคลุม end_date — กราฟเป็น 12 เดือนล่าสุด)
	FiscalYear *FiscalYear `json:"fiscal_year"`
	// PreviousYear: ปีบัญชีก่อนหน้า เดือนต่อเดือนกับ monthly (null = ไม่มียอด)
	PreviousYear []MonthNet `json:"previous_year"`
	Quotations   struct {
		OpenCount  int64  `json:"open_count"`
		OpenAmount string `json:"open_amount"`
	} `json:"quotations"`
	Orders struct {
		DraftCount     int64 `json:"draft_count"`
		ToDeliverCount int64 `json:"to_deliver_count"`
		ToInvoiceCount int64 `json:"to_invoice_count"`
	} `json:"orders"`
	Receivables struct {
		Outstanding string `json:"outstanding"`
		Overdue     string `json:"overdue"`
		Aging       Aging5 `json:"aging"`
	} `json:"receivables"`
	TopCustomers []TopCustomer `json:"top_customers"`
}

type Comparison struct {
	Current  string `json:"current"`
	Previous string `json:"previous"`
}

type TopCustomer struct {
	Customer Ref    `json:"customer"`
	Net      string `json:"net"`
}

type FiscalYear struct {
	Year      int    `json:"year"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func FromDashboard(d dto.Dashboard) Dashboard {
	out := Dashboard{
		StartDate: date(d.Period.Start), EndDate: date(d.Period.End),
		Sales:        Comparison{Current: str(d.SalesCurrent), Previous: str(d.SalesPrevious)},
		InvoiceCount: d.InvoiceCount, Monthly: make([]MonthNet, len(d.Monthly)), TopCustomers: make([]TopCustomer, len(d.TopCustomers)),
	}
	for i, m := range d.Monthly {
		out.Monthly[i] = MonthNet{Month: m.Month, Net: str(m.Net)}
	}
	if p := d.Period; p.FiscalYear > 0 {
		out.FiscalYear = &FiscalYear{Year: p.FiscalYear, StartDate: date(p.TrendStart()), EndDate: date(p.TrendEnd())}
	}
	for _, m := range d.PreviousYear {
		out.PreviousYear = append(out.PreviousYear, MonthNet{Month: m.Month, Net: str(m.Net)})
	}
	out.Quotations.OpenCount, out.Quotations.OpenAmount = d.QuotationOpenCount, str(d.QuotationOpenAmount)
	out.Orders.DraftCount, out.Orders.ToDeliverCount, out.Orders.ToInvoiceCount = d.OrderDraftCount, d.OrderToDeliverCount, d.OrderToInvoiceCount
	r := d.Receivables
	out.Receivables.Outstanding, out.Receivables.Overdue = str(r.Outstanding), str(r.Overdue)
	out.Receivables.Aging = Aging5{NotDue: str(r.NotDue), Days1To30: str(r.Days1To30), Days31To60: str(r.Days31To60), Days61To90: str(r.Days61To90), Over90: str(r.Over90)}
	for i, c := range d.TopCustomers {
		out.TopCustomers[i] = TopCustomer{Customer: ref(c.Customer), Net: str(c.Net)}
	}
	return out
}
