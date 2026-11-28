// Package requests คือรูปแบบ JSON ขาเข้า / URL query ของ purchasing และการแปลงเป็น input ของ use case
//
// presentation validation: field ที่ต้องมี, รูปแบบ UUID / วันที่ / ตัวเลข / enum
// กฎธุรกิจ (VAT ตามผู้ขาย, รับไม่เกินที่สั่ง, ทศนิยมตามบริษัท ฯลฯ) ตรวจใน domain / use case
package requests

import (
	"apexis/backend/internal/platform/thaiaddress"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

// ErrValidation ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client (HTTP 400)
var ErrValidation = errors.New("validation failed")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

// Invalid: validation error ที่ handler สร้างเอง
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
	if err != nil {
		return nil, err
	}
	return &v, nil
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

// Decimal รับตัวเลขเป็น string ("12.5") หรือ number (12.5) — เก็บข้อความดิบแล้วแปลงแบบไม่ผ่าน float
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

// ---------- ค่าตั้ง / ผู้ขาย ----------

// Settings: PUT /purchasing-settings — แทนที่ทั้งก้อน (บัญชีที่ไม่ส่ง = ไม่ตั้ง)
type Settings struct {
	VatRate                  *Decimal `json:"vat_rate"`
	JournalID                *string  `json:"journal_id"`
	GRIRAccountID            *string  `json:"grir_account_id"`
	APAccountID              *string  `json:"ap_account_id"`
	InputVatAccountID        *string  `json:"input_vat_account_id"`
	PriceVarianceAccountID   *string  `json:"price_variance_account_id"`
	PurchaseReturnsAccountID *string  `json:"purchase_returns_account_id"`
	UndueInputVatAccountID   *string  `json:"undue_input_vat_account_id"`
	ServiceExpenseAccountID  *string  `json:"service_expense_account_id"`
}

func (r Settings) ToSettings() (entities.Settings, error) {
	rate, err := r.VatRate.required("vat_rate")
	if err != nil {
		return entities.Settings{}, err
	}
	out := entities.Settings{VatRate: rate}
	if out.JournalID, err = optionalID[vo.JournalID]("journal_id", r.JournalID); err != nil {
		return entities.Settings{}, err
	}
	for _, f := range []struct {
		name string
		raw  *string
		dst  **vo.AccountID
	}{
		{"grir_account_id", r.GRIRAccountID, &out.GRIRAccountID},
		{"ap_account_id", r.APAccountID, &out.APAccountID},
		{"input_vat_account_id", r.InputVatAccountID, &out.InputVatAccountID},
		{"price_variance_account_id", r.PriceVarianceAccountID, &out.PriceVarianceAccountID},
		{"purchase_returns_account_id", r.PurchaseReturnsAccountID, &out.PurchaseReturnsAccountID},
		{"undue_input_vat_account_id", r.UndueInputVatAccountID, &out.UndueInputVatAccountID},
		{"service_expense_account_id", r.ServiceExpenseAccountID, &out.ServiceExpenseAccountID},
	} {
		if *f.dst, err = optionalID[vo.AccountID](f.name, f.raw); err != nil {
			return entities.Settings{}, err
		}
	}
	return out, nil
}

// Supplier: POST / PUT /suppliers
type Supplier struct {
	Code            string            `json:"code"`
	Name            string            `json:"name"`
	TaxID           string            `json:"tax_id"`
	BranchNo        string            `json:"branch_no"`
	AddressDetail   *thaiaddress.JSON `json:"address_detail"` // ที่อยู่ตามแบบสรรพากร (ไม่ส่ง = ว่าง)
	PaymentTermDays int               `json:"payment_term_days"`
	VatRegistered   *bool             `json:"vat_registered"`
	TaxType         string            `json:"tax_type"` // individual (ภ.ง.ด.3) / juristic (ภ.ง.ด.53)
	Status          string            `json:"status"`
	Branches        []Branch          `json:"branches"` // สาขาเพิ่มเติม (แทนที่ทั้งชุด; ไม่ส่ง = ไม่มี)
}

func (r Supplier) ToData(isCreate bool) (entities.SupplierData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name, "tax_type": r.TaxType}); err != nil {
		return entities.SupplierData{}, err
	}
	if r.VatRegistered == nil {
		return entities.SupplierData{}, invalid("vat_registered is required")
	}
	st, err := status(r.Status, isCreate)
	if err != nil {
		return entities.SupplierData{}, err
	}
	return entities.SupplierData{
		Code: r.Code, Name: r.Name, TaxID: r.TaxID, BranchNo: r.BranchNo, AddressDetail: r.AddressDetail.Address(),
		PaymentTermDays: r.PaymentTermDays, VatRegistered: *r.VatRegistered, TaxType: vo.TaxType(r.TaxType), Status: st,
		Branches: branches(r.Branches),
	}, nil
}

// Branch คือสาขาเพิ่มเติมของผู้ขาย
type Branch struct {
	BranchNo      string            `json:"branch_no"`
	Name          string            `json:"name"`
	AddressDetail *thaiaddress.JSON `json:"address_detail"`
}

func branches(in []Branch) []entities.SupplierBranch {
	out := make([]entities.SupplierBranch, len(in))
	for i, b := range in {
		out[i] = entities.SupplierBranch{BranchNo: b.BranchNo, Name: b.Name, AddressDetail: b.AddressDetail.Address()}
	}
	return out
}

// ---------- ใบสั่งซื้อ ----------

type OrderLine struct {
	ItemID      string   `json:"item_id"`
	Description string   `json:"description"`
	Quantity    *Decimal `json:"quantity"`
	UnitPrice   *Decimal `json:"unit_price"`
}

// Order: POST / PUT /purchase-orders — price_includes_vat เลือกได้เฉพาะผู้ขายที่จด VAT
type Order struct {
	SupplierID       string      `json:"supplier_id"`
	SupplierBranchNo string      `json:"supplier_branch_no"` // ว่าง = สาขาหลัก
	Date             string      `json:"date"`
	DeliveryDate     *string     `json:"delivery_date"`
	Reference        string      `json:"reference"`
	Description      string      `json:"description"`
	PriceIncludesVat bool        `json:"price_includes_vat"`
	Lines            []OrderLine `json:"lines"`
}

func (r Order) ToData() (entities.OrderData, error) {
	if err := required(map[string]string{"supplier_id": r.SupplierID, "date": r.Date}); err != nil {
		return entities.OrderData{}, err
	}
	supplier, err := id[vo.SupplierID]("supplier_id", r.SupplierID)
	if err != nil {
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
	out := entities.OrderData{
		SupplierID: supplier, SupplierBranchNo: r.SupplierBranchNo, Date: d, DeliveryDate: delivery, Reference: r.Reference, Description: r.Description,
		PriceIncludesVat: r.PriceIncludesVat, Lines: make([]entities.OrderLineData, len(r.Lines)),
	}
	for i, l := range r.Lines {
		n := i + 1
		if l.ItemID == "" {
			return entities.OrderData{}, invalid("lines[%d].item_id is required", n)
		}
		item, err := id[vo.ItemID](fmt.Sprintf("lines[%d].item_id", n), l.ItemID)
		if err != nil {
			return entities.OrderData{}, err
		}
		qty, err := l.Quantity.required(fmt.Sprintf("lines[%d].quantity", n))
		if err != nil {
			return entities.OrderData{}, err
		}
		price, err := l.UnitPrice.required(fmt.Sprintf("lines[%d].unit_price", n))
		if err != nil {
			return entities.OrderData{}, err
		}
		out.Lines[i] = entities.OrderLineData{ItemID: item, Description: l.Description, Quantity: qty, UnitPrice: price}
	}
	return out, nil
}

// Status: PATCH เอกสาร — {"status": "..."}
type Status struct {
	Status string `json:"status"`
}

// ---------- ใบรับสินค้า / ใบแจ้งหนี้ ----------

type LineQty struct {
	OrderLineNo int      `json:"order_line_no"`
	Quantity    *Decimal `json:"quantity"`
	UnitPrice   *Decimal `json:"unit_price"` // เฉพาะใบแจ้งหนี้: ไม่ส่ง = ราคาตามใบสั่งซื้อ
	AccountID   *string  `json:"account_id"` // เฉพาะซื้อบริการ: บัญชีค่าใช้จ่ายของบรรทัด (ไม่ส่ง = ค่าตั้ง)
}

func lineQty(lines []LineQty, allowPrice bool) ([]dto.LineQty, error) {
	out := make([]dto.LineQty, len(lines))
	for i, l := range lines {
		n := i + 1
		if l.OrderLineNo < 1 {
			return nil, invalid("lines[%d].order_line_no is required", n)
		}
		qty, err := l.Quantity.required(fmt.Sprintf("lines[%d].quantity", n))
		if err != nil {
			return nil, err
		}
		if !allowPrice && l.UnitPrice != nil {
			return nil, invalid("lines[%d].unit_price is not allowed (receipt cost follows the purchase order)", n)
		}
		price, err := l.UnitPrice.parse(fmt.Sprintf("lines[%d].unit_price", n))
		if err != nil {
			return nil, err
		}
		if !allowPrice && l.AccountID != nil {
			return nil, invalid("lines[%d].account_id is not allowed on a receipt", n)
		}
		acc, err := optionalID[vo.AccountID](fmt.Sprintf("lines[%d].account_id", n), l.AccountID)
		if err != nil {
			return nil, err
		}
		out[i] = dto.LineQty{OrderLineNo: l.OrderLineNo, Quantity: qty, UnitPrice: price, AccountID: acc}
	}
	return out, nil
}

// Receipt: POST /goods-receipts — warehouse_id ต้องมีเมื่อมีสินค้า stock
type Receipt struct {
	OrderID     string    `json:"order_id"`
	Date        string    `json:"date"`
	WarehouseID *string   `json:"warehouse_id"`
	Reference   string    `json:"reference"`
	Description string    `json:"description"`
	Lines       []LineQty `json:"lines"`
}

func (r Receipt) ToInput() (dto.ReceiptInput, error) {
	if err := required(map[string]string{"order_id": r.OrderID, "date": r.Date}); err != nil {
		return dto.ReceiptInput{}, err
	}
	order, err := id[vo.OrderID]("order_id", r.OrderID)
	if err != nil {
		return dto.ReceiptInput{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return dto.ReceiptInput{}, err
	}
	wh, err := optionalID[vo.WarehouseID]("warehouse_id", r.WarehouseID)
	if err != nil {
		return dto.ReceiptInput{}, err
	}
	lines, err := lineQty(r.Lines, false)
	if err != nil {
		return dto.ReceiptInput{}, err
	}
	return dto.ReceiptInput{OrderID: order, Date: d, WarehouseID: wh, Reference: r.Reference, Description: r.Description, Lines: lines}, nil
}

// Invoice: POST /supplier-invoices (ซื้อสินค้า) | /supplier-service-invoices (ซื้อบริการ)
//
//	ตามใบสั่ง: order_id + lines[{order_line_no, quantity, unit_price?, account_id?}]
//	ซื้อตรง (ไม่ส่ง order_id): supplier_id + lines[{item_id, description?, quantity, unit_price, account_id?}] (+ warehouse_id เมื่อมีสินค้า stock)
//	invoice_date (วันที่ในใบแจ้งหนี้ / ใบกำกับของผู้ขาย) ไม่ส่ง = วันที่ลงบัญชี
//	ซื้อบริการ: tax_invoice_no (+ tax_invoice_date ไม่ส่ง = invoice_date) = ผู้ขายออกใบกำกับมาพร้อมใบแจ้งหนี้ (ใช้สิทธิภาษีซื้อทันที)
type Invoice struct {
	OrderID           string          `json:"order_id"`
	SupplierInvoiceNo string          `json:"supplier_invoice_no"`
	InvoiceDate       string          `json:"invoice_date"`
	Date              string          `json:"date"`
	Description       string          `json:"description"`
	Lines             json.RawMessage `json:"lines"`
	TaxInvoiceNo      string          `json:"tax_invoice_no"`
	TaxInvoiceDate    string          `json:"tax_invoice_date"`
	// ซื้อตรง
	SupplierID       string  `json:"supplier_id"`
	SupplierBranchNo string  `json:"supplier_branch_no"`
	WarehouseID      *string `json:"warehouse_id"`
	PriceIncludesVat bool    `json:"price_includes_vat"`
	Reference        string  `json:"reference"`
}

// Direct: ไม่ส่ง order_id = ซื้อตรง
func (r Invoice) Direct() bool { return r.OrderID == "" }

func (r Invoice) dates() (time.Time, time.Time, *time.Time, error) {
	d, err := date("date", r.Date)
	if err != nil {
		return time.Time{}, time.Time{}, nil, err
	}
	invDate := d
	if r.InvoiceDate != "" {
		if invDate, err = date("invoice_date", r.InvoiceDate); err != nil {
			return time.Time{}, time.Time{}, nil, err
		}
	}
	var taxDate *time.Time
	if r.TaxInvoiceDate != "" {
		t, err := date("tax_invoice_date", r.TaxInvoiceDate)
		if err != nil {
			return time.Time{}, time.Time{}, nil, err
		}
		taxDate = &t
	}
	return d, invDate, taxDate, nil
}

func (r Invoice) ToInput() (dto.InvoiceInput, error) {
	if err := required(map[string]string{"order_id": r.OrderID, "date": r.Date, "supplier_invoice_no": r.SupplierInvoiceNo}); err != nil {
		return dto.InvoiceInput{}, err
	}
	order, err := id[vo.OrderID]("order_id", r.OrderID)
	if err != nil {
		return dto.InvoiceInput{}, err
	}
	d, invDate, taxDate, err := r.dates()
	if err != nil {
		return dto.InvoiceInput{}, err
	}
	var raw []LineQty
	if len(r.Lines) > 0 {
		if err := json.Unmarshal(r.Lines, &raw); err != nil {
			return dto.InvoiceInput{}, invalid("lines must be an array of {order_line_no, quantity, unit_price?, account_id?}")
		}
	}
	lines, err := lineQty(raw, true)
	if err != nil {
		return dto.InvoiceInput{}, err
	}
	return dto.InvoiceInput{
		OrderID: order, SupplierInvoiceNo: r.SupplierInvoiceNo, InvoiceDate: invDate, Date: d,
		Description: r.Description, Lines: lines, TaxInvoiceNo: r.TaxInvoiceNo, TaxInvoiceDate: taxDate,
	}, nil
}

// DirectLine: บรรทัดของซื้อตรง
type DirectLine struct {
	ItemID      string   `json:"item_id"`
	Description string   `json:"description"`
	Quantity    *Decimal `json:"quantity"`
	UnitPrice   *Decimal `json:"unit_price"`
	AccountID   *string  `json:"account_id"`
}

func (r Invoice) ToDirect() (dto.DirectInvoiceInput, error) {
	if err := required(map[string]string{"supplier_id": r.SupplierID, "date": r.Date, "supplier_invoice_no": r.SupplierInvoiceNo}); err != nil {
		return dto.DirectInvoiceInput{}, err
	}
	supplier, err := id[vo.SupplierID]("supplier_id", r.SupplierID)
	if err != nil {
		return dto.DirectInvoiceInput{}, err
	}
	d, invDate, taxDate, err := r.dates()
	if err != nil {
		return dto.DirectInvoiceInput{}, err
	}
	wh, err := optionalID[vo.WarehouseID]("warehouse_id", r.WarehouseID)
	if err != nil {
		return dto.DirectInvoiceInput{}, err
	}
	var raw []DirectLine
	if len(r.Lines) > 0 {
		if err := json.Unmarshal(r.Lines, &raw); err != nil {
			return dto.DirectInvoiceInput{}, invalid("lines must be an array of {item_id, quantity, unit_price, description?, account_id?}")
		}
	}
	out := dto.DirectInvoiceInput{
		SupplierID: supplier, SupplierBranchNo: r.SupplierBranchNo, SupplierInvoiceNo: r.SupplierInvoiceNo, InvoiceDate: invDate, Date: d,
		WarehouseID: wh, PriceIncludesVat: r.PriceIncludesVat, Reference: r.Reference, Description: r.Description,
		Lines: make([]dto.DirectLine, len(raw)), TaxInvoiceNo: r.TaxInvoiceNo, TaxInvoiceDate: taxDate,
	}
	for i, l := range raw {
		n := i + 1
		if l.ItemID == "" {
			return dto.DirectInvoiceInput{}, invalid("lines[%d].item_id is required", n)
		}
		item, err := id[vo.ItemID](fmt.Sprintf("lines[%d].item_id", n), l.ItemID)
		if err != nil {
			return dto.DirectInvoiceInput{}, err
		}
		qty, err := l.Quantity.required(fmt.Sprintf("lines[%d].quantity", n))
		if err != nil {
			return dto.DirectInvoiceInput{}, err
		}
		price, err := l.UnitPrice.required(fmt.Sprintf("lines[%d].unit_price", n))
		if err != nil {
			return dto.DirectInvoiceInput{}, err
		}
		acc, err := optionalID[vo.AccountID](fmt.Sprintf("lines[%d].account_id", n), l.AccountID)
		if err != nil {
			return dto.DirectInvoiceInput{}, err
		}
		out.Lines[i] = dto.DirectLine{ItemID: item, Description: l.Description, Quantity: qty, UnitPrice: price, AccountID: acc}
	}
	return out, nil
}

// TaxInvoice: POST /supplier-service-invoices/{id}/tax-invoices — ใบกำกับภาษีที่ได้รับจากผู้ขาย
// vat ไม่ส่ง = ตามสัดส่วนที่จ่ายแล้ว (ยังไม่จ่าย = ที่รอทั้งหมด); date (วันที่ลงบัญชี) ไม่ส่ง = tax_invoice_date
type TaxInvoice struct {
	TaxInvoiceNo   string   `json:"tax_invoice_no"`
	TaxInvoiceDate string   `json:"tax_invoice_date"`
	Date           string   `json:"date"`
	Vat            *Decimal `json:"vat"`
}

func (r TaxInvoice) ToInput(invoiceID vo.InvoiceID) (dto.TaxInvoiceInput, error) {
	if err := required(map[string]string{"tax_invoice_no": r.TaxInvoiceNo, "tax_invoice_date": r.TaxInvoiceDate}); err != nil {
		return dto.TaxInvoiceInput{}, err
	}
	td, err := date("tax_invoice_date", r.TaxInvoiceDate)
	if err != nil {
		return dto.TaxInvoiceInput{}, err
	}
	out := dto.TaxInvoiceInput{InvoiceID: invoiceID, TaxInvoiceNo: r.TaxInvoiceNo, TaxInvoiceDate: td}
	if r.Date != "" {
		d, err := date("date", r.Date)
		if err != nil {
			return dto.TaxInvoiceInput{}, err
		}
		out.Date = &d
	}
	if out.Vat, err = r.Vat.parse("vat"); err != nil {
		return dto.TaxInvoiceInput{}, err
	}
	return out, nil
}

// ---------- ใบลดหนี้ผู้ขาย ----------

// CreditNoteLine: ส่งคืนสินค้า = quantity (+ unit_price ไม่ส่ง = ราคาตามใบแจ้งหนี้); ลดราคา = amount (รวม / แยก VAT ตามใบแจ้งหนี้)
type CreditNoteLine struct {
	InvoiceLineNo int      `json:"invoice_line_no"`
	Quantity      *Decimal `json:"quantity"`
	UnitPrice     *Decimal `json:"unit_price"`
	Amount        *Decimal `json:"amount"`
}

// CreditNote: POST /supplier-credit-notes — credit_note_date (วันที่ในใบลดหนี้ของผู้ขาย) ไม่ส่ง = วันที่ลงบัญชี
type CreditNote struct {
	InvoiceID            string           `json:"invoice_id"`
	SupplierCreditNoteNo string           `json:"supplier_credit_note_no"`
	CreditNoteDate       string           `json:"credit_note_date"`
	Date                 string           `json:"date"`
	Kind                 string           `json:"kind"`
	Reason               string           `json:"reason"`
	Description          string           `json:"description"`
	WarehouseID          *string          `json:"warehouse_id"`
	Lines                []CreditNoteLine `json:"lines"`
}

func (r CreditNote) ToInput() (dto.CreditNoteInput, error) {
	if err := required(map[string]string{"invoice_id": r.InvoiceID, "date": r.Date, "supplier_credit_note_no": r.SupplierCreditNoteNo,
		"kind": r.Kind, "reason": r.Reason}); err != nil {
		return dto.CreditNoteInput{}, err
	}
	inv, err := id[vo.InvoiceID]("invoice_id", r.InvoiceID)
	if err != nil {
		return dto.CreditNoteInput{}, err
	}
	kind, err := vo.ParseCreditNoteKind(r.Kind)
	if err != nil {
		return dto.CreditNoteInput{}, invalid("kind must be return or allowance")
	}
	d, err := date("date", r.Date)
	if err != nil {
		return dto.CreditNoteInput{}, err
	}
	cnDate := d
	if r.CreditNoteDate != "" {
		if cnDate, err = date("credit_note_date", r.CreditNoteDate); err != nil {
			return dto.CreditNoteInput{}, err
		}
	}
	wh, err := optionalID[vo.WarehouseID]("warehouse_id", r.WarehouseID)
	if err != nil {
		return dto.CreditNoteInput{}, err
	}
	if len(r.Lines) == 0 {
		return dto.CreditNoteInput{}, invalid("lines must have at least one line")
	}
	lines := make([]dto.CreditNoteLineInput, len(r.Lines))
	for i, l := range r.Lines {
		n := i + 1
		if l.InvoiceLineNo < 1 {
			return dto.CreditNoteInput{}, invalid("lines[%d].invoice_line_no is required", n)
		}
		out := dto.CreditNoteLineInput{InvoiceLineNo: l.InvoiceLineNo}
		if kind == vo.CreditReturn {
			if l.Amount != nil {
				return dto.CreditNoteInput{}, invalid("lines[%d].amount is not allowed for kind return (use quantity / unit_price)", n)
			}
			q, err := l.Quantity.required(fmt.Sprintf("lines[%d].quantity", n))
			if err != nil {
				return dto.CreditNoteInput{}, err
			}
			out.Quantity = &q
			if out.UnitPrice, err = l.UnitPrice.parse(fmt.Sprintf("lines[%d].unit_price", n)); err != nil {
				return dto.CreditNoteInput{}, err
			}
		} else {
			if l.Quantity != nil || l.UnitPrice != nil {
				return dto.CreditNoteInput{}, invalid("lines[%d].quantity / unit_price are not allowed for kind allowance (use amount)", n)
			}
			a, err := l.Amount.required(fmt.Sprintf("lines[%d].amount", n))
			if err != nil {
				return dto.CreditNoteInput{}, err
			}
			out.Amount = &a
		}
		lines[i] = out
	}
	return dto.CreditNoteInput{
		InvoiceID: inv, SupplierCreditNoteNo: r.SupplierCreditNoteNo, CreditNoteDate: cnDate, Date: d, Kind: kind,
		Reason: r.Reason, Description: r.Description, WarehouseID: wh, Lines: lines,
	}, nil
}
