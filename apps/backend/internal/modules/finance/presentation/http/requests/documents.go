package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// Decimal รับจำนวนเงินได้ทั้ง string ("1234.5" — แนะนำ) และ number
type Decimal struct {
	v   decimal.Decimal
	set bool
}

func (d *Decimal) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		return nil
	}
	var s string
	if len(b) > 0 && b[0] == '"' {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
	} else {
		s = string(b)
	}
	v, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return fmt.Errorf("invalid number %q", s)
	}
	d.v, d.set = v, true
	return nil
}

func (d *Decimal) required(field string) (decimal.Decimal, error) {
	if d == nil || !d.set {
		return decimal.Zero, invalid("%s is required", field)
	}
	return d.v, nil
}

func (d *Decimal) orZero() decimal.Decimal {
	if d == nil || !d.set {
		return decimal.Zero
	}
	return d.v
}

func date(field, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, invalid("%s is required", field)
	}
	t, err := listing.ParseDate(raw)
	if err != nil {
		return time.Time{}, invalid("%s must be YYYY-MM-DD", field)
	}
	return t, nil
}

func optID[T ~[16]byte](field string, raw *string) (*T, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := vo.ParseID[T](*raw)
	if err != nil {
		return nil, invalid("%s must be a UUID", field)
	}
	return &id, nil
}

func reqID[T ~[16]byte](field, raw string) (T, error) {
	if raw == "" {
		var zero T
		return zero, invalid("%s is required", field)
	}
	id, err := vo.ParseID[T](raw)
	if err != nil {
		return id, invalid("%s must be a UUID", field)
	}
	return id, nil
}

// ---------- ช่องทาง / คู่ค้า ----------

type Method struct {
	CashAccountID string   `json:"cash_account_id"`
	Amount        *Decimal `json:"amount"`
	Reference     string   `json:"reference"` // เลขเช็ค / เลขอ้างอิงการโอน
}

func methods(in []Method) ([]dto.MethodInput, error) {
	out := make([]dto.MethodInput, len(in))
	for i, m := range in {
		n := i + 1
		acc, err := reqID[vo.CashAccountID](fmt.Sprintf("methods[%d].cash_account_id", n), m.CashAccountID)
		if err != nil {
			return nil, err
		}
		amount, err := m.Amount.required(fmt.Sprintf("methods[%d].amount", n))
		if err != nil {
			return nil, err
		}
		out[i] = dto.MethodInput{CashAccountID: acc, Amount: amount, Reference: m.Reference}
	}
	return out, nil
}

// Party: ผู้จ่าย / ผู้รับเงินที่ระบุเอง (รับ / จ่ายเงินอื่น) — tax_type: individual / juristic (ยื่น ภ.ง.ด.3 / 53)
type Party struct {
	Name     string `json:"name"`
	TaxID    string `json:"tax_id"`
	BranchNo string `json:"branch_no"`
	Address  string `json:"address"`
	TaxType  string `json:"tax_type"`
}

func (p *Party) input() (dto.PartyInput, error) {
	if p == nil {
		return dto.PartyInput{}, nil
	}
	out := dto.PartyInput{Name: p.Name, TaxID: p.TaxID, BranchNo: p.BranchNo, Address: p.Address}
	if p.TaxType != "" {
		t, err := vo.ParseTaxType(p.TaxType)
		if err != nil {
			return out, invalid("payee.tax_type must be individual or juristic")
		}
		out.TaxType = t
	}
	return out, nil
}

func partyID(field string, raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil || id == uuid.Nil {
		return nil, invalid("%s must be a UUID", field)
	}
	return &id, nil
}

// ---------- ใบรับเงิน ----------

type ReceiptLine struct {
	InvoiceID   *string  `json:"invoice_id"` // รับชำระหนี้
	AccountID   *string  `json:"account_id"` // รับเงินอื่น
	Description string   `json:"description"`
	Amount      *Decimal `json:"amount"`
	Wht         *Decimal `json:"wht"` // ภาษีที่ลูกค้าหัก ณ ที่จ่าย (ไม่ส่ง = 0)
}

// Receipt: POST /receipts — kind: customer (customer_id + lines.invoice_id) / other (payer + lines.account_id)
// ยอดรวม methods = ยอดรวม lines.amount - lines.wht
type Receipt struct {
	Kind        string        `json:"kind"`
	CustomerID  *string       `json:"customer_id"`
	Payer       *Party        `json:"payer"`
	Date        string        `json:"date"`
	Description string        `json:"description"`
	Lines       []ReceiptLine `json:"lines"`
	Methods     []Method      `json:"methods"`
}

func (r Receipt) ToInput() (dto.ReceiptInput, error) {
	kind, err := vo.ParseReceiptKind(r.Kind)
	if err != nil {
		return dto.ReceiptInput{}, invalid("kind must be customer or other")
	}
	out := dto.ReceiptInput{Kind: kind, Description: r.Description}
	if out.Date, err = date("date", r.Date); err != nil {
		return out, err
	}
	if out.CustomerID, err = partyID("customer_id", r.CustomerID); err != nil {
		return out, err
	}
	if kind == vo.ReceiptCustomer && out.CustomerID == nil {
		return out, invalid("customer_id is required for kind customer")
	}
	if out.Payer, err = r.Payer.input(); err != nil {
		return out, err
	}
	out.Lines = make([]dto.ReceiptLineInput, len(r.Lines))
	for i, l := range r.Lines {
		n := i + 1
		line := dto.ReceiptLineInput{Description: l.Description, Wht: l.Wht.orZero()}
		if line.InvoiceID, err = optID[vo.InvoiceID](fmt.Sprintf("lines[%d].invoice_id", n), l.InvoiceID); err != nil {
			return out, err
		}
		if line.AccountID, err = optID[vo.GLAccountID](fmt.Sprintf("lines[%d].account_id", n), l.AccountID); err != nil {
			return out, err
		}
		if line.Amount, err = l.Amount.required(fmt.Sprintf("lines[%d].amount", n)); err != nil {
			return out, err
		}
		out.Lines[i] = line
	}
	out.Methods, err = methods(r.Methods)
	return out, err
}

// ---------- ใบสำคัญจ่าย ----------

type PaymentLine struct {
	InvoiceID   *string  `json:"invoice_id"` // จ่ายชำระหนี้
	AccountID   *string  `json:"account_id"` // จ่ายเงินอื่น
	Description string   `json:"description"`
	Amount      *Decimal `json:"amount"`
	WhtTypeID   *string  `json:"wht_type_id"` // ไม่ส่ง = ไม่หักภาษี
}

// Payment: POST /payments — kind: supplier (supplier_id + lines.invoice_id) / other (payee + lines.account_id)
type Payment struct {
	Kind        string        `json:"kind"`
	SupplierID  *string       `json:"supplier_id"`
	Payee       *Party        `json:"payee"`
	Date        string        `json:"date"`
	Description string        `json:"description"`
	Lines       []PaymentLine `json:"lines"`
	Methods     []Method      `json:"methods"`
}

func (r Payment) ToInput() (dto.PaymentInput, error) {
	kind, err := vo.ParsePaymentKind(r.Kind)
	if err != nil {
		return dto.PaymentInput{}, invalid("kind must be supplier or other")
	}
	out := dto.PaymentInput{Kind: kind, Description: r.Description}
	if out.Date, err = date("date", r.Date); err != nil {
		return out, err
	}
	if out.SupplierID, err = partyID("supplier_id", r.SupplierID); err != nil {
		return out, err
	}
	if kind == vo.PaymentSupplier && out.SupplierID == nil {
		return out, invalid("supplier_id is required for kind supplier")
	}
	if out.Payee, err = r.Payee.input(); err != nil {
		return out, err
	}
	out.Lines = make([]dto.PaymentLineInput, len(r.Lines))
	for i, l := range r.Lines {
		n := i + 1
		line := dto.PaymentLineInput{Description: l.Description}
		if line.InvoiceID, err = optID[vo.InvoiceID](fmt.Sprintf("lines[%d].invoice_id", n), l.InvoiceID); err != nil {
			return out, err
		}
		if line.AccountID, err = optID[vo.GLAccountID](fmt.Sprintf("lines[%d].account_id", n), l.AccountID); err != nil {
			return out, err
		}
		if line.WhtTypeID, err = optID[vo.WhtTypeID](fmt.Sprintf("lines[%d].wht_type_id", n), l.WhtTypeID); err != nil {
			return out, err
		}
		if line.Amount, err = l.Amount.required(fmt.Sprintf("lines[%d].amount", n)); err != nil {
			return out, err
		}
		out.Lines[i] = line
	}
	out.Methods, err = methods(r.Methods)
	return out, err
}

// ---------- โอนเงิน ----------

// Transfer: POST /cash-transfers
type Transfer struct {
	Date              string   `json:"date"`
	FromCashAccountID string   `json:"from_cash_account_id"`
	ToCashAccountID   string   `json:"to_cash_account_id"`
	Amount            *Decimal `json:"amount"`
	Reference         string   `json:"reference"`
	Description       string   `json:"description"`
}

func (r Transfer) ToInput() (dto.TransferInput, error) {
	var out dto.TransferInput
	var err error
	if out.Date, err = date("date", r.Date); err != nil {
		return out, err
	}
	if out.FromID, err = reqID[vo.CashAccountID]("from_cash_account_id", r.FromCashAccountID); err != nil {
		return out, err
	}
	if out.ToID, err = reqID[vo.CashAccountID]("to_cash_account_id", r.ToCashAccountID); err != nil {
		return out, err
	}
	if out.Amount, err = r.Amount.required("amount"); err != nil {
		return out, err
	}
	out.Reference, out.Description = r.Reference, r.Description
	return out, nil
}

// Cancel: PATCH /receipts|payments|cash-transfers/{id} {"status":"cancelled"}
type Cancel struct {
	Status string `json:"status"`
}

func (r Cancel) Validate() error {
	if r.Status != string(vo.DocCancelled) {
		return invalid(`status can only be changed to "cancelled"`)
	}
	return nil
}

// ---------- ค่าตั้ง / ประเภทภาษี ----------

// Settings: PUT /finance-settings (ทุก field ไม่บังคับ — null = ไม่ได้ตั้ง)
type Settings struct {
	ReceiptJournalID     *string `json:"receipt_journal_id"`
	PaymentJournalID     *string `json:"payment_journal_id"`
	TransferJournalID    *string `json:"transfer_journal_id"`
	WhtReceivedAccountID *string `json:"wht_received_account_id"`
	WhtPnd3AccountID     *string `json:"wht_pnd3_account_id"`
	WhtPnd53AccountID    *string `json:"wht_pnd53_account_id"`
}

func (r Settings) ToSettings() (entities.Settings, error) {
	var out entities.Settings
	var err error
	for _, j := range []struct {
		name string
		raw  *string
		dst  **vo.JournalID
	}{{"receipt_journal_id", r.ReceiptJournalID, &out.ReceiptJournalID}, {"payment_journal_id", r.PaymentJournalID, &out.PaymentJournalID},
		{"transfer_journal_id", r.TransferJournalID, &out.TransferJournalID}} {
		if *j.dst, err = optID[vo.JournalID](j.name, j.raw); err != nil {
			return out, err
		}
	}
	for _, a := range []struct {
		name string
		raw  *string
		dst  **vo.GLAccountID
	}{{"wht_received_account_id", r.WhtReceivedAccountID, &out.WhtReceivedAccountID},
		{"wht_pnd3_account_id", r.WhtPnd3AccountID, &out.WhtPnd3AccountID}, {"wht_pnd53_account_id", r.WhtPnd53AccountID, &out.WhtPnd53AccountID}} {
		if *a.dst, err = optID[vo.GLAccountID](a.name, a.raw); err != nil {
			return out, err
		}
	}
	return out, nil
}

// WhtType: POST / PUT /withholding-tax-types — rate เป็น % เช่น "3"
type WhtType struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Rate   *Decimal `json:"rate"`
	Status string   `json:"status"`
}

func (r WhtType) ToData(isCreate bool) (entities.WhtTypeData, error) {
	if strings.TrimSpace(r.Code) == "" || strings.TrimSpace(r.Name) == "" {
		return entities.WhtTypeData{}, invalid("code, name is required")
	}
	rate, err := r.Rate.required("rate")
	if err != nil {
		return entities.WhtTypeData{}, err
	}
	st := vo.Status(r.Status)
	if r.Status == "" {
		if !isCreate {
			return entities.WhtTypeData{}, invalid("status is required")
		}
		st = vo.StatusActive
	}
	return entities.WhtTypeData{Code: r.Code, Name: r.Name, Rate: rate, Status: st}, nil
}

// ---------- URL query ----------

func optDate(v url.Values, key string) (*time.Time, error) {
	raw := v.Get(key)
	if raw == "" {
		return nil, nil
	}
	t, err := listing.ParseDate(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s has an invalid value %q", listing.ErrInvalidQuery, key, raw)
	}
	return &t, nil
}

func optUUIDParam(v url.Values, key string) (*uuid.UUID, error) {
	raw := v.Get(key)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return nil, fmt.Errorf("%w: %s must be a UUID", listing.ErrInvalidQuery, key)
	}
	return &id, nil
}

// ParseDocumentFilter: ?q=&status=posted,cancelled&kind=&party_id=&cash_account_id=&start_date=&end_date=&sort=date|document_no&page=&page_size=
func ParseDocumentFilter(v url.Values, kinds []string) (dto.DocumentFilter, error) {
	var f dto.DocumentFilter
	var err error
	if f.Search, err = listing.ParseSearch(v); err != nil {
		return f, err
	}
	if f.Statuses, err = listing.ParseList(v, "status", vo.ParseDocStatus); err != nil {
		return f, err
	}
	if f.Kinds, err = listing.ParseList(v, "kind", func(raw string) (string, error) {
		for _, k := range kinds {
			if k == raw {
				return raw, nil
			}
		}
		return "", fmt.Errorf("kind must be one of %s", strings.Join(kinds, ", "))
	}); err != nil {
		return f, err
	}
	if f.PartyID, err = optUUIDParam(v, "party_id"); err != nil {
		return f, err
	}
	if cash, err := optUUIDParam(v, "cash_account_id"); err != nil {
		return f, err
	} else if cash != nil {
		id := vo.CashAccountID(*cash)
		f.CashAccountID = &id
	}
	if f.StartDate, err = optDate(v, "start_date"); err != nil {
		return f, err
	}
	if f.EndDate, err = optDate(v, "end_date"); err != nil {
		return f, err
	}
	if f.Sort, err = listing.ParseSort(v, dto.DocumentSortKeys, dto.DefaultDocumentSort); err != nil {
		return f, err
	}
	f.Page, err = listing.ParsePage(v)
	return f, err
}

// ParseWhtTypeFilter: GET /withholding-tax-types?q=&status=&sort=code|name&page=&page_size=
func ParseWhtTypeFilter(v url.Values) (dto.WhtTypeFilter, error) {
	var f dto.WhtTypeFilter
	var err error
	if f.Search, err = listing.ParseSearch(v); err != nil {
		return f, err
	}
	if f.Statuses, err = listing.ParseList(v, "status", vo.ParseStatus); err != nil {
		return f, err
	}
	if f.Sort, err = listing.ParseSort(v, []string{"code", "name"}, []listing.Sort{{Key: "code"}}); err != nil {
		return f, err
	}
	f.Page, err = listing.ParsePage(v)
	return f, err
}

// ParseDateRange: start_date / end_date ต้องมีทั้งคู่ (YYYY-MM-DD) และ start <= end — รายงาน / dashboard
func ParseDateRange(v url.Values) (time.Time, time.Time, error) {
	var out [2]time.Time
	for i, key := range []string{"start_date", "end_date"} {
		raw := v.Get(key)
		t, err := listing.ParseDate(raw)
		if raw == "" || err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: %s is required (YYYY-MM-DD)", listing.ErrInvalidQuery, key)
		}
		out[i] = t
	}
	if out[0].After(out[1]) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: start_date must not be after end_date", listing.ErrInvalidQuery)
	}
	return out[0], out[1], nil
}

// ParseWithholding: GET /reports?book=withholding-tax|withholding-tax-received&start_date=&end_date=&form=pnd3|pnd53&party_id=&sort=&page=
func ParseWithholding(v url.Values) (dto.WithholdingFilter, error) {
	var f dto.WithholdingFilter
	var err error
	switch form := v.Get("form"); form {
	case "", "pnd3", "pnd53":
		f.Form = form
	default:
		return f, fmt.Errorf("%w: form must be pnd3 or pnd53", listing.ErrInvalidQuery)
	}
	if f.Start, f.End, err = ParseDateRange(v); err != nil {
		return f, err
	}
	if f.PartyID, err = optUUIDParam(v, "party_id"); err != nil {
		return f, err
	}
	if f.Sort, err = listing.ParseSort(v, dto.WithholdingSortKeys, dto.DefaultWithholdingSort); err != nil {
		return f, err
	}
	f.Page, err = listing.ParsePage(v)
	return f, err
}
