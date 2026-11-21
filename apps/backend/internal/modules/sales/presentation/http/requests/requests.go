// Package requests คือ JSON / URL query ขาเข้าของ sales และการแปลงเป็น input ของ use case
// presentation validation: field ที่ต้องมี, UUID / วันที่ / ตัวเลข / enum — กฎธุรกิจอยู่ใน domain / use case
package requests

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/domain/entities"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

var ErrValidation = errors.New("validation failed")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

func Invalid(msg string) error { return invalid("%s", msg) }

func required(fields map[string]string) error {
	var missing []string
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return invalid("%s is required", strings.Join(missing, ", "))
}

func id[T ~[16]byte](field, raw string) (T, error) {
	v, err := vo.ParseID[T](raw)
	if err != nil {
		return T{}, invalid("%s must be a UUID", field)
	}
	return v, nil
}

func optionalID[T ~[16]byte](field string, raw *string) (*T, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	v, err := id[T](field, *raw)
	return &v, err
}

func date(field, raw string) (time.Time, error) {
	d, err := listing.ParseDate(raw)
	if err != nil {
		return time.Time{}, invalid("%s must be YYYY-MM-DD", field)
	}
	return d, nil
}

func optionalDate(field string, raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	d, err := date(field, *raw)
	return &d, err
}

func status(raw string, isCreate bool) (vo.Status, error) {
	if raw == "" {
		if isCreate {
			return vo.StatusActive, nil
		}
		return "", invalid("status is required")
	}
	s, err := vo.ParseStatus(raw)
	if err != nil {
		return "", invalid("status has an invalid value %q", raw)
	}
	return s, nil
}

// Decimal รับตัวเลขเป็น string หรือ number — แปลงแบบไม่ผ่าน float
type Decimal string

func (d *Decimal) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		*d = Decimal(bytes.Trim(b, `"`))
	} else {
		*d = Decimal(b)
	}
	return nil
}

func (d *Decimal) parse(field string) (*decimal.Decimal, error) {
	if d == nil || *d == "" || *d == "null" {
		return nil, nil
	}
	v, ok := numeric.Parse(strings.TrimSpace(string(*d)))
	if !ok {
		return nil, invalid("%s must be a decimal number (up to 18 integer digits and 6 decimal places)", field)
	}
	return &v, nil
}

func (d *Decimal) required(field string) (decimal.Decimal, error) {
	v, err := d.parse(field)
	if err != nil {
		return decimal.Zero, err
	}
	if v == nil {
		return decimal.Zero, invalid("%s is required", field)
	}
	return *v, nil
}

func (d *Decimal) orZero(field string) (decimal.Decimal, error) {
	v, err := d.parse(field)
	if err != nil || v == nil {
		return decimal.Zero, err
	}
	return *v, nil
}

// ---------- ค่าตั้ง / ลูกค้า ----------

// Settings: PUT /sales-settings — แทนที่ทั้งก้อน
type Settings struct {
	VatRegistered      *bool    `json:"vat_registered"`
	VatRate            *Decimal `json:"vat_rate"`
	JournalID          *string  `json:"journal_id"`
	ARAccountID        *string  `json:"ar_account_id"`
	OutputVatAccountID *string  `json:"output_vat_account_id"`
	RevenueAccountID   *string  `json:"revenue_account_id"`
	CogsAccountID      *string  `json:"cogs_account_id"`
	WhtAccountID       *string  `json:"wht_account_id"`
}

func (r Settings) ToSettings() (entities.Settings, error) {
	if r.VatRegistered == nil {
		return entities.Settings{}, invalid("vat_registered is required")
	}
	rate, err := r.VatRate.orZero("vat_rate")
	if err != nil {
		return entities.Settings{}, err
	}
	out := entities.Settings{VatRegistered: *r.VatRegistered, VatRate: rate}
	if out.JournalID, err = optionalID[vo.JournalID]("journal_id", r.JournalID); err != nil {
		return entities.Settings{}, err
	}
	for _, f := range []struct {
		name string
		raw  *string
		dst  **vo.AccountID
	}{
		{"ar_account_id", r.ARAccountID, &out.ARAccountID}, {"output_vat_account_id", r.OutputVatAccountID, &out.OutputVatAccountID},
		{"revenue_account_id", r.RevenueAccountID, &out.RevenueAccountID}, {"cogs_account_id", r.CogsAccountID, &out.CogsAccountID},
		{"wht_account_id", r.WhtAccountID, &out.WhtAccountID},
	} {
		if *f.dst, err = optionalID[vo.AccountID](f.name, f.raw); err != nil {
			return entities.Settings{}, err
		}
	}
	return out, nil
}

// Customer: POST / PUT /customers — credit_limit ไม่ส่ง / null = ไม่จำกัด
type Customer struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	TaxID           string   `json:"tax_id"`
	BranchNo        string   `json:"branch_no"`
	Address         string   `json:"address"`
	PaymentTermDays int      `json:"payment_term_days"`
	TaxType         string   `json:"tax_type"`
	CreditLimit     *Decimal `json:"credit_limit"`
	Status          string   `json:"status"`
}

func (r Customer) ToData(isCreate bool) (entities.CustomerData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name, "tax_type": r.TaxType}); err != nil {
		return entities.CustomerData{}, err
	}
	limit, err := r.CreditLimit.parse("credit_limit")
	if err != nil {
		return entities.CustomerData{}, err
	}
	st, err := status(r.Status, isCreate)
	if err != nil {
		return entities.CustomerData{}, err
	}
	return entities.CustomerData{
		Code: r.Code, Name: r.Name, TaxID: r.TaxID, BranchNo: r.BranchNo, Address: r.Address, PaymentTermDays: r.PaymentTermDays,
		TaxType: vo.TaxType(r.TaxType), CreditLimit: limit, Status: st,
	}, nil
}

// ---------- ใบเสนอราคา / ใบสั่งขาย ----------

type Line struct {
	ItemID      string   `json:"item_id"`
	Description string   `json:"description"`
	Quantity    *Decimal `json:"quantity"`
	UnitPrice   *Decimal `json:"unit_price"`
}

func lines(in []Line) ([]entities.LineData, error) {
	out := make([]entities.LineData, len(in))
	for i, l := range in {
		n := i + 1
		if l.ItemID == "" {
			return nil, invalid("lines[%d].item_id is required", n)
		}
		item, err := id[vo.ItemID](fmt.Sprintf("lines[%d].item_id", n), l.ItemID)
		if err != nil {
			return nil, err
		}
		qty, err := l.Quantity.required(fmt.Sprintf("lines[%d].quantity", n))
		if err != nil {
			return nil, err
		}
		price, err := l.UnitPrice.required(fmt.Sprintf("lines[%d].unit_price", n))
		if err != nil {
			return nil, err
		}
		out[i] = entities.LineData{ItemID: item, Description: l.Description, Quantity: qty, UnitPrice: price}
	}
	return out, nil
}

// Quotation: POST / PUT /quotations
type Quotation struct {
	CustomerID       string `json:"customer_id"`
	Date             string `json:"date"`
	ValidUntil       string `json:"valid_until"`
	Reference        string `json:"reference"`
	Description      string `json:"description"`
	PriceIncludesVat bool   `json:"price_includes_vat"`
	Lines            []Line `json:"lines"`
}

func (r Quotation) ToData() (entities.QuotationData, error) {
	if err := required(map[string]string{"customer_id": r.CustomerID, "date": r.Date, "valid_until": r.ValidUntil}); err != nil {
		return entities.QuotationData{}, err
	}
	customer, err := id[vo.CustomerID]("customer_id", r.CustomerID)
	if err != nil {
		return entities.QuotationData{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return entities.QuotationData{}, err
	}
	valid, err := date("valid_until", r.ValidUntil)
	if err != nil {
		return entities.QuotationData{}, err
	}
	ls, err := lines(r.Lines)
	if err != nil {
		return entities.QuotationData{}, err
	}
	return entities.QuotationData{
		CustomerID: customer, Date: d, ValidUntil: valid, Reference: r.Reference, Description: r.Description,
		PriceIncludesVat: r.PriceIncludesVat, Lines: ls,
	}, nil
}

// Order: POST / PUT /sales-orders — POST ส่ง quotation_id ได้ (ไม่ต้องส่งลูกค้า / บรรทัด: คัดลอกจากใบเสนอราคา)
type Order struct {
	QuotationID      *string `json:"quotation_id"`
	CustomerID       string  `json:"customer_id"`
	Date             string  `json:"date"`
	DeliveryDate     *string `json:"delivery_date"`
	Reference        string  `json:"reference"`
	Description      string  `json:"description"`
	PriceIncludesVat bool    `json:"price_includes_vat"`
	Lines            []Line  `json:"lines"`
}

func (r Order) ToData(allowQuotation bool) (entities.OrderData, error) {
	if err := required(map[string]string{"date": r.Date}); err != nil {
		return entities.OrderData{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return entities.OrderData{}, err
	}
	delivery, err := optionalDate("delivery_date", r.DeliveryDate)
	if err != nil {
		return entities.OrderData{}, err
	}
	out := entities.OrderData{Date: d, DeliveryDate: delivery, Reference: r.Reference, Description: r.Description, PriceIncludesVat: r.PriceIncludesVat}
	if r.QuotationID != nil && *r.QuotationID != "" {
		if !allowQuotation {
			return entities.OrderData{}, invalid("quotation_id can only be set when creating a sales order")
		}
		if out.QuotationID, err = optionalID[vo.QuotationID]("quotation_id", r.QuotationID); err != nil {
			return entities.OrderData{}, err
		}
		if len(r.Lines) > 0 || r.CustomerID != "" {
			return entities.OrderData{}, invalid("customer_id and lines come from the quotation (do not send them with quotation_id)")
		}
		return out, nil
	}
	if err := required(map[string]string{"customer_id": r.CustomerID}); err != nil {
		return entities.OrderData{}, err
	}
	if out.CustomerID, err = id[vo.CustomerID]("customer_id", r.CustomerID); err != nil {
		return entities.OrderData{}, err
	}
	out.Lines, err = lines(r.Lines)
	return out, err
}

// Status: PATCH เอกสาร — {"status": "..."}
type Status struct {
	Status string `json:"status"`
}

// ---------- ส่งของ / ออกบิล / รับเงิน ----------

type Shipment struct {
	OrderLineNo int      `json:"order_line_no"`
	Quantity    *Decimal `json:"quantity"`
}

func shipments(in []Shipment) ([]dto.ShipmentInput, error) {
	out := make([]dto.ShipmentInput, len(in))
	for i, l := range in {
		if l.OrderLineNo < 1 {
			return nil, invalid("lines[%d].order_line_no is required", i+1)
		}
		q, err := l.Quantity.required(fmt.Sprintf("lines[%d].quantity", i+1))
		if err != nil {
			return nil, err
		}
		out[i] = dto.ShipmentInput{OrderLineNo: l.OrderLineNo, Quantity: q}
	}
	return out, nil
}

// Delivery: POST /deliveries — warehouse_id บังคับเมื่อมีสินค้า stock
type Delivery struct {
	OrderID     string     `json:"order_id"`
	Date        string     `json:"date"`
	WarehouseID *string    `json:"warehouse_id"`
	Reference   string     `json:"reference"`
	Description string     `json:"description"`
	Lines       []Shipment `json:"lines"`
}

func (r Delivery) ToInput() (dto.DeliveryInput, error) {
	if err := required(map[string]string{"order_id": r.OrderID, "date": r.Date}); err != nil {
		return dto.DeliveryInput{}, err
	}
	order, err := id[vo.OrderID]("order_id", r.OrderID)
	if err != nil {
		return dto.DeliveryInput{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return dto.DeliveryInput{}, err
	}
	wh, err := optionalID[vo.WarehouseID]("warehouse_id", r.WarehouseID)
	if err != nil {
		return dto.DeliveryInput{}, err
	}
	ls, err := shipments(r.Lines)
	return dto.DeliveryInput{OrderID: order, Date: d, WarehouseID: wh, Reference: r.Reference, Description: r.Description, Lines: ls}, err
}

// Invoice: POST /sales-invoices — with_delivery = ใบส่งของ / ใบกำกับภาษี (ตัดสต็อกพร้อมออกบิล)
type Invoice struct {
	OrderID      string     `json:"order_id"`
	Date         string     `json:"date"`
	WithDelivery bool       `json:"with_delivery"`
	WarehouseID  *string    `json:"warehouse_id"`
	Reference    string     `json:"reference"`
	Description  string     `json:"description"`
	Lines        []Shipment `json:"lines"`
}

func (r Invoice) ToInput() (dto.InvoiceInput, error) {
	in, err := Delivery{OrderID: r.OrderID, Date: r.Date, WarehouseID: r.WarehouseID, Reference: r.Reference, Description: r.Description, Lines: r.Lines}.ToInput()
	if err != nil {
		return dto.InvoiceInput{}, err
	}
	if !r.WithDelivery && in.WarehouseID != nil {
		return dto.InvoiceInput{}, invalid("warehouse_id is only used with with_delivery")
	}
	return dto.InvoiceInput{OrderID: in.OrderID, Date: in.Date, WithDelivery: r.WithDelivery, WarehouseID: in.WarehouseID,
		Reference: in.Reference, Description: in.Description, Lines: in.Lines}, nil
}

type ReceiptLine struct {
	InvoiceID string   `json:"invoice_id"`
	Amount    *Decimal `json:"amount"` // ยอดตัดหนี้ (รวม VAT)
	Wht       *Decimal `json:"wht"`    // ภาษีที่ลูกค้าหัก ณ ที่จ่าย (ไม่ส่ง = 0)
}

type ReceiptMethod struct {
	CashAccountID string   `json:"cash_account_id"`
	Amount        *Decimal `json:"amount"`
	Reference     string   `json:"reference"`
}

// Receipt: POST /customer-receipts — ยอดรวม methods = ยอดรวม lines.amount - lines.wht
type Receipt struct {
	CustomerID  string          `json:"customer_id"`
	Date        string          `json:"date"`
	Description string          `json:"description"`
	Lines       []ReceiptLine   `json:"lines"`
	Methods     []ReceiptMethod `json:"methods"`
}

func (r Receipt) ToInput() (dto.ReceiptInput, error) {
	if err := required(map[string]string{"customer_id": r.CustomerID, "date": r.Date}); err != nil {
		return dto.ReceiptInput{}, err
	}
	customer, err := id[vo.CustomerID]("customer_id", r.CustomerID)
	if err != nil {
		return dto.ReceiptInput{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return dto.ReceiptInput{}, err
	}
	out := dto.ReceiptInput{CustomerID: customer, Date: d, Description: r.Description,
		Lines: make([]dto.ReceiptLineInput, len(r.Lines)), Methods: make([]dto.ReceiptMethodInput, len(r.Methods))}
	for i, l := range r.Lines {
		n := i + 1
		inv, err := id[vo.InvoiceID](fmt.Sprintf("lines[%d].invoice_id", n), l.InvoiceID)
		if err != nil {
			return dto.ReceiptInput{}, err
		}
		amount, err := l.Amount.required(fmt.Sprintf("lines[%d].amount", n))
		if err != nil {
			return dto.ReceiptInput{}, err
		}
		wht, err := l.Wht.orZero(fmt.Sprintf("lines[%d].wht", n))
		if err != nil {
			return dto.ReceiptInput{}, err
		}
		out.Lines[i] = dto.ReceiptLineInput{InvoiceID: inv, Amount: amount, Wht: wht}
	}
	for i, m := range r.Methods {
		n := i + 1
		acc, err := id[vo.CashAccountID](fmt.Sprintf("methods[%d].cash_account_id", n), m.CashAccountID)
		if err != nil {
			return dto.ReceiptInput{}, err
		}
		amount, err := m.Amount.required(fmt.Sprintf("methods[%d].amount", n))
		if err != nil {
			return dto.ReceiptInput{}, err
		}
		out.Methods[i] = dto.ReceiptMethodInput{CashAccountID: acc, Amount: amount, Reference: m.Reference}
	}
	return out, nil
}

// ---------- URL query ----------

type query struct {
	v   url.Values
	err error
}

func parse[T any](q *query, fn func() (T, error)) T {
	var zero T
	if q.err != nil {
		return zero
	}
	v, err := fn()
	if err != nil {
		q.err = err
	}
	return v
}

func optionalIDParam[T ~[16]byte](q *query, key string) *T {
	return parse(q, func() (*T, error) { return listing.ParseOptional(q.v, key, vo.ParseID[T]) })
}

func strs[T ~string](in []T, err error) ([]string, error) {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out, err
}

// GET /customers?q=&status=&sort=code|name&page=&page_size=
func ParseCustomerFilter(v url.Values) (dto.CustomerFilter, error) {
	q := &query{v: v}
	f := dto.CustomerFilter{
		Search:   parse(q, func() (string, error) { return listing.ParseSearch(v) }),
		Statuses: parse(q, func() ([]vo.Status, error) { return listing.ParseList(v, "status", vo.ParseStatus) }),
		Sort: parse(q, func() ([]listing.Sort, error) {
			return listing.ParseSort(v, dto.CustomerSortKeys, dto.DefaultCustomerSort)
		}),
		Page: parse(q, func() (listing.Page, error) { return listing.ParsePage(v) }),
	}
	return f, q.err
}

// DocKind กำหนดค่า status ที่รับของรายการเอกสาร
type DocKind int

const (
	KindQuotation DocKind = iota
	KindOrder
	KindPosted
)

// ParseDocumentFilter: ?q=&status=&customer_id=&order_id=&item_id=&start_date=&end_date=&sort=date|document_no|created_at&page=&page_size=
func ParseDocumentFilter(v url.Values, kind DocKind) (dto.DocumentFilter, error) {
	q := &query{v: v}
	f := dto.DocumentFilter{
		Search: parse(q, func() (string, error) { return listing.ParseSearch(v) }),
		Statuses: parse(q, func() ([]string, error) {
			switch kind {
			case KindQuotation:
				return strs(listing.ParseList(v, "status", vo.ParseQuotationStatus))
			case KindOrder:
				return strs(listing.ParseList(v, "status", vo.ParseOrderStatus))
			default:
				return strs(listing.ParseList(v, "status", vo.ParseDocStatus))
			}
		}),
		CustomerID: optionalIDParam[vo.CustomerID](q, "customer_id"),
		OrderID:    optionalIDParam[vo.OrderID](q, "order_id"),
		ItemID:     optionalIDParam[vo.ItemID](q, "item_id"),
		Sort: parse(q, func() ([]listing.Sort, error) {
			return listing.ParseSort(v, dto.DocumentSortKeys, dto.DefaultDocumentSort)
		}),
		Page: parse(q, func() (listing.Page, error) { return listing.ParsePage(v) }),
	}
	if q.err == nil {
		f.StartDate, f.EndDate, q.err = listing.ParseDateRange(v, "start_date", "end_date")
	}
	return f, q.err
}

// GET /reports?book=outstanding-sales-orders&customer_id=&item_id=&q=&sort=&page=&page_size=
func ParseOutstanding(v url.Values) (dto.OutstandingFilter, error) {
	q := &query{v: v}
	f := dto.OutstandingFilter{
		Search:     parse(q, func() (string, error) { return listing.ParseSearch(v) }),
		CustomerID: optionalIDParam[vo.CustomerID](q, "customer_id"),
		ItemID:     optionalIDParam[vo.ItemID](q, "item_id"),
		Sort: parse(q, func() ([]listing.Sort, error) {
			return listing.ParseSort(v, dto.OutstandingSortKeys, dto.DefaultOutstandingSort)
		}),
		Page: parse(q, func() (listing.Page, error) { return listing.ParsePage(v) }),
	}
	return f, q.err
}

func requiredDate(v url.Values, key string) (time.Time, error) {
	raw := v.Get(key)
	t, err := listing.ParseDate(raw)
	if raw == "" || err != nil {
		return time.Time{}, fmt.Errorf("%w: %s is required (YYYY-MM-DD)", listing.ErrInvalidQuery, key)
	}
	return t, nil
}

// GET /reports?book=ar-aging&end_date=&customer_id=&q=&page=&page_size=
func ParseAging(v url.Values) (dto.AgingFilter, error) {
	q := &query{v: v}
	f := dto.AgingFilter{
		Search:     parse(q, func() (string, error) { return listing.ParseSearch(v) }),
		CustomerID: optionalIDParam[vo.CustomerID](q, "customer_id"),
		Page:       parse(q, func() (listing.Page, error) { return listing.ParsePage(v) }),
	}
	if q.err != nil {
		return f, q.err
	}
	var err error
	f.AsOf, err = requiredDate(v, "end_date")
	return f, err
}

// GET /reports?book=output-vat | withholding-tax-received &start_date=&end_date=&customer_id=&sort=&page=&page_size=
func ParseTax(v url.Values) (dto.TaxFilter, error) {
	q := &query{v: v}
	f := dto.TaxFilter{
		CustomerID: optionalIDParam[vo.CustomerID](q, "customer_id"),
		Sort:       parse(q, func() ([]listing.Sort, error) { return listing.ParseSort(v, dto.TaxSortKeys, dto.DefaultTaxSort) }),
		Page:       parse(q, func() (listing.Page, error) { return listing.ParsePage(v) }),
	}
	if q.err != nil {
		return f, q.err
	}
	var err error
	if f.Start, err = requiredDate(v, "start_date"); err != nil {
		return f, err
	}
	if f.End, err = requiredDate(v, "end_date"); err != nil {
		return f, err
	}
	if f.Start.After(f.End) {
		return f, fmt.Errorf("%w: start_date must not be after end_date", listing.ErrInvalidQuery)
	}
	return f, nil
}
