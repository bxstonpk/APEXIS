package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
)

// Posting คือเลขที่และรายการบัญชีของเอกสารที่ลงบัญชีแล้ว
type Posting struct {
	DocumentNo string // RC2026-000001 / PI2026-000001 — เรียงไม่ขาดตอนต่อปีบัญชี
	FiscalYear int
	PeriodID   vo.FiscalPeriodID
	EntryID    *vo.EntryID // nil = มูลค่าเป็นศูนย์ ไม่มีรายการบัญชี
	EntryNo    string
	By         vo.UserID
	At         time.Time
}

// ---------- ใบรับสินค้า ----------

// ReceiptLine คือบรรทัดที่รับ — มูลค่าก่อน VAT, บัญชีเดบิตตามระบบบัญชี:
// perpetual + สินค้า stock = สินค้าคงเหลือ, อื่นๆ (periodic / ไม่นับสต็อก / บริการ) = บัญชีซื้อ
type ReceiptLine struct {
	OrderLineNo int
	ItemID      vo.ItemID
	Stock       bool
	Quantity    vo.Quantity
	UnitCost    vo.Money // ราคาต่อหน่วยก่อน VAT
	Value       vo.Money
	AccountID   vo.AccountID
}

type ReceiptState struct {
	ID              vo.ReceiptID
	CompanyID       vo.CompanyID
	// Kind: goods = ใบรับสินค้า (รับเข้าคลัง + ลงบัญชี), service = ใบตรวจรับงาน (บันทึกจำนวนที่รับงานเท่านั้น ไม่มีบัญชี / สต็อก)
	Kind            vo.PurchaseKind
	OrderID         vo.OrderID
	SupplierID      vo.SupplierID
	Date            time.Time
	WarehouseID     *vo.WarehouseID // มีเมื่อมีสินค้า stock
	Reference       string
	Description     string
	System          vo.AccountingSystem
	Lines           []ReceiptLine
	StockDocumentID *vo.StockDocumentID // ใบรับเข้าของ inventory
	StockDocumentNo string
	Status          vo.DocStatus
	Posting         Posting
	Cancellation    *Cancellation
}

// Receipt คือใบรับสินค้าตามใบสั่งซื้อ — สร้างแล้วลงบัญชี + รับเข้าคลังทันที, ยกเลิกด้วยการกลับรายการ
type Receipt struct{ s ReceiptState }

func NewReceipt(s ReceiptState) (*Receipt, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Reference, err = vo.OptionalText(s.Reference, 100); err != nil {
		return nil, err
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.ReceiptID](), vo.DocPosted, nil
	return &Receipt{s: s}, nil
}

func RehydrateReceipt(s ReceiptState) *Receipt { return &Receipt{s: s} }

func (r *Receipt) ID() vo.ReceiptID    { return r.s.ID }
func (r *Receipt) State() ReceiptState { return r.s }

// Deliveries คือจำนวน / มูลค่าที่ใบนี้รับเข้าใบสั่งซื้อ (ใช้กลับรายการเมื่อยกเลิก)
func (r *Receipt) Deliveries() []Delivery {
	out := make([]Delivery, len(r.s.Lines))
	for i, l := range r.s.Lines {
		out[i] = Delivery{LineNo: l.OrderLineNo, Quantity: l.Quantity, Value: l.Value}
	}
	return out
}

// Journal: Dr สินค้าคงเหลือ / ซื้อ (ตามบรรทัด) — Cr รับสินค้ายังไม่ได้รับใบแจ้งหนี้ (GR/IR)
func (r *Receipt) Journal(grir vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	for _, l := range r.s.Lines {
		j.Debit(l.AccountID, l.Value)
		j.Credit(grir, l.Value)
	}
	return j.Lines()
}

func (r *Receipt) SetEntry(id vo.EntryID, no string) {
	r.s.Posting.EntryID, r.s.Posting.EntryNo = &id, no
}

func (r *Receipt) Cancel(by vo.UserID, at time.Time) error {
	if r.s.Status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	r.s.Status, r.s.Cancellation = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}

// ---------- ใบแจ้งหนี้ผู้ขาย ----------

// InvoiceLine: ยอดตามใบแจ้งหนี้ (ก่อน VAT / VAT / รวม), มูลค่าที่ล้าง GR/IR และผลต่างราคา
// ผลต่าง (Net - Cleared) ลงบัญชีผลต่างราคา (perpetual + stock) หรือบัญชีซื้อ (อื่นๆ)
type InvoiceLine struct {
	OrderLineNo       int
	ItemID            vo.ItemID
	Quantity          vo.Quantity
	UnitPrice         vo.Money // ราคาตามใบแจ้งหนี้ (รวม / แยก VAT ตามใบสั่งซื้อ)
	Amounts           services.Amounts
	Cleared           vo.Money
	VarianceAccountID *vo.AccountID // nil เมื่อไม่มีผลต่าง
}

func (l InvoiceLine) Variance() vo.Money { return l.Amounts.Net.Sub(l.Cleared) }

// BillFrom คือชื่อ / เลขผู้เสียภาษี / สาขา / ที่อยู่ของผู้ขายตามใบกำกับภาษี ณ วันบันทึก
// เก็บไว้ในใบ — แก้ทะเบียนผู้ขายภายหลังไม่กระทบใบที่บันทึกแล้ว
type BillFrom struct {
	Name     string
	TaxID    string
	BranchNo string
	Address  string
}

type InvoiceState struct {
	ID                vo.InvoiceID
	CompanyID         vo.CompanyID
	OrderID           vo.OrderID
	SupplierID        vo.SupplierID
	BillFrom          BillFrom  // ผู้ขายตามใบกำกับภาษี ณ วันบันทึก
	SupplierInvoiceNo string    // เลขที่ใบแจ้งหนี้ / ใบกำกับภาษีของผู้ขาย (ไม่ซ้ำต่อผู้ขาย)
	InvoiceDate       time.Time // วันที่ในใบกำกับภาษี
	Date              time.Time // วันที่ลงบัญชี
	DueDate           time.Time // วันที่ลงบัญชี + เครดิตเทอมตามใบสั่งซื้อ
	Description       string
	Vat               services.VatTerms
	Lines             []InvoiceLine
	Totals            services.Amounts
	Paid              vo.Money // ยอดที่จ่ายชำระแล้ว (ใบจ่ายที่ยังไม่ยกเลิก)
	// Kind: goods = ซื้อสินค้า (เลขที่ / วันที่ใบแจ้งหนี้ของผู้ขาย = ใบกำกับภาษี), service = ซื้อบริการ
	Kind vo.PurchaseKind
	// TaxInvoiceNo / TaxInvoiceDate: ซื้อบริการที่ผู้ขายออกใบกำกับภาษีมาพร้อมใบแจ้งหนี้ (ใช้สิทธิทันที)
	TaxInvoiceNo   string
	TaxInvoiceDate *time.Time
	// TaxInvoiceReceived: ได้รับใบกำกับภาษีพร้อมใบแจ้งหนี้แล้ว (ใช้สิทธิภาษีซื้อทันที) — ซื้อสินค้า = เสมอ, ซื้อบริการ = เมื่อระบุ TaxInvoiceNo
	// false = ซื้อบริการที่ผู้ขายออกใบกำกับเมื่อได้รับเงิน (ม.78/1) — VAT ลงภาษีซื้อยังไม่ถึงกำหนดจนกว่าจะได้รับใบกำกับ (ม.82/3)
	TaxInvoiceReceived bool
	Deferral           VatDeferral
	Status             vo.DocStatus
	Posting            Posting
	Cancellation       *Cancellation
}

// VatDeferral คือภาษีซื้อยังไม่ถึงกำหนดของใบแจ้งหนี้ที่ยังไม่ได้รับใบกำกับภาษี
//
//	ยังไม่ถึงกำหนดคงเหลือ = VAT ของใบ - ที่ได้รับใบกำกับแล้ว (Recognized) - ที่ลดด้วยใบลดหนี้ (Credited)
type VatDeferral struct {
	VatRecognized  vo.Money
	BaseRecognized vo.Money
	VatCredited    vo.Money
	BaseCredited   vo.Money
}

// VatStatus ของใบแจ้งหนี้ผู้ขาย
const (
	VatStatusNone    = "none"    // ไม่มี VAT
	VatStatusClaimed = "claimed" // ใช้สิทธิภาษีซื้อได้ทั้งใบ (ได้รับใบกำกับครบ)
	VatStatusUndue   = "undue"   // ยังไม่ได้รับใบกำกับเลย
	VatStatusPartial = "partial" // ได้รับใบกำกับบางส่วน
)

// Invoice คือใบแจ้งหนี้ผู้ขายตามใบสั่งซื้อ — สร้างแล้วลงบัญชีทันที (ตั้งเจ้าหนี้), ยกเลิกด้วยการกลับรายการ
type Invoice struct{ s InvoiceState }

func NewInvoice(s InvoiceState, paymentTermDays int) (*Invoice, error) {
	var err error
	if !vo.IsDateOnly(s.Date) || !vo.IsDateOnly(s.InvoiceDate) {
		return nil, domain.ErrInvalidDate
	}
	if s.SupplierInvoiceNo, err = vo.NewName(s.SupplierInvoiceNo, 50); err != nil {
		return nil, domain.ErrInvalidText
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	s.Kind = vo.KindOrDefault(string(s.Kind))
	if s.TaxInvoiceNo, err = vo.OptionalText(s.TaxInvoiceNo, 50); err != nil {
		return nil, domain.ErrInvalidText
	}
	switch {
	case s.Kind != vo.KindService:
		// ซื้อสินค้า: ใบแจ้งหนี้ของผู้ขายคือใบกำกับภาษี
		s.TaxInvoiceReceived, s.TaxInvoiceNo, s.TaxInvoiceDate = true, "", nil
	case s.TaxInvoiceNo != "":
		if s.TaxInvoiceDate == nil || !vo.IsDateOnly(*s.TaxInvoiceDate) {
			return nil, domain.ErrInvalidDate
		}
		s.TaxInvoiceReceived = true
	default:
		s.TaxInvoiceReceived, s.TaxInvoiceDate = false, nil
	}
	s.DueDate = s.Date.AddDate(0, 0, paymentTermDays)
	s.Totals = services.ZeroAmounts()
	for _, l := range s.Lines {
		s.Totals = s.Totals.Add(l.Amounts)
	}
	s.ID, s.Status, s.Cancellation, s.Paid = vo.NewID[vo.InvoiceID](), vo.DocPosted, nil, decimal.Zero
	if s.Totals.Vat.IsZero() {
		s.TaxInvoiceReceived = true // ไม่มี VAT (ผู้ขายไม่จด VAT / อัตรา 0) — ไม่มีอะไรรอใบกำกับ
	}
	s.Deferral = VatDeferral{VatRecognized: decimal.Zero, BaseRecognized: decimal.Zero, VatCredited: decimal.Zero, BaseCredited: decimal.Zero}
	return &Invoice{s: s}, nil
}

func (i *Invoice) Kind() vo.PurchaseKind { return vo.KindOrDefault(string(i.s.Kind)) }

func RehydrateInvoice(s InvoiceState) *Invoice { return &Invoice{s: s} }

func (i *Invoice) ID() vo.InvoiceID    { return i.s.ID }
func (i *Invoice) State() InvoiceState { return i.s }

func (i *Invoice) Deliveries() []Delivery {
	out := make([]Delivery, len(i.s.Lines))
	for n, l := range i.s.Lines {
		out[n] = Delivery{LineNo: l.OrderLineNo, Quantity: l.Quantity, Value: l.Cleared}
	}
	return out
}

// Journal: Dr GR/IR (ที่ล้าง) ± ผลต่างราคา + Dr ภาษีซื้อ (ได้รับใบกำกับแล้ว) หรือภาษีซื้อยังไม่ถึงกำหนด — Cr เจ้าหนี้ (ยอดรวม VAT)
// vatAccount = บัญชีตาม VatAccountKey()
func (i *Invoice) Journal(grir, vatAccount, ap vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	for _, l := range i.s.Lines {
		j.Debit(grir, l.Cleared)
		if l.VarianceAccountID != nil {
			j.Debit(*l.VarianceAccountID, l.Variance())
		}
	}
	j.Debit(vatAccount, i.s.Totals.Vat)
	j.Credit(ap, i.s.Totals.Gross)
	return j.Lines()
}

// VatAccountKey: ค่าตั้งของบัญชีฝั่ง VAT ตอนบันทึกใบ ("" = ไม่มี VAT)
func (i *Invoice) VatAccountKey() string {
	switch {
	case i.s.Totals.Vat.IsZero():
		return ""
	case i.s.TaxInvoiceReceived:
		return "input_vat_account_id"
	default:
		return "undue_input_vat_account_id"
	}
}

// RemainingUndueVat / RemainingUndueBase: ภาษีซื้อ (และฐาน) ที่ยังรอใบกำกับภาษี
func (i *Invoice) RemainingUndueVat() vo.Money {
	if i.s.TaxInvoiceReceived {
		return decimal.Zero
	}
	d := i.s.Deferral
	return i.s.Totals.Vat.Sub(d.VatRecognized).Sub(d.VatCredited)
}

func (i *Invoice) RemainingUndueBase() vo.Money {
	if i.s.TaxInvoiceReceived {
		return decimal.Zero
	}
	d := i.s.Deferral
	return i.s.Totals.Net.Sub(d.BaseRecognized).Sub(d.BaseCredited)
}

func (i *Invoice) VatStatus() string {
	rem := i.RemainingUndueVat()
	switch {
	case i.s.Totals.Vat.IsZero():
		return VatStatusNone
	case !rem.IsPositive():
		return VatStatusClaimed
	case i.s.Deferral.VatRecognized.IsPositive():
		return VatStatusPartial
	default:
		return VatStatusUndue
	}
}

// DefaultRecognition: ภาษีซื้อของใบกำกับที่คาดว่าจะได้รับ เมื่อผู้ใช้ไม่ระบุยอด
//
//	ยอดจ่ายด้วยใบจ่ายเงินที่ยังไม่มีใบกำกับรองรับ × VAT / ยอดรวม (ไม่เกินที่เหลือ)
//	ยังไม่ได้จ่าย หรือใบกำกับเดิมครอบคลุมยอดจ่ายแล้ว (ผู้ขายออกใบกำกับก่อนรับเงิน) = ที่เหลือทั้งหมด
func (i *Invoice) DefaultRecognition(paidByPayments vo.Money, amountDecimals int) vo.Money {
	return SuggestRecognition(i.s.Totals, i.s.Deferral.VatRecognized, i.RemainingUndueVat(), paidByPayments, amountDecimals)
}

// SuggestRecognition คือกฎของ DefaultRecognition จากตัวเลขของใบ (ใช้ได้กับ read model โดยไม่ต้อง rehydrate)
func SuggestRecognition(t services.Amounts, recognized, remaining, paidByPayments vo.Money, amountDecimals int) vo.Money {
	if !remaining.IsPositive() || t.Vat.IsZero() {
		return remaining
	}
	covered := recognized.Mul(t.Gross).Div(t.Vat)
	uncovered := paidByPayments.Sub(covered)
	if !uncovered.IsPositive() {
		return remaining
	}
	vat := uncovered.Mul(t.Vat).Div(t.Gross).Round(int32(amountDecimals))
	if !vat.IsPositive() || vat.GreaterThan(remaining) {
		return remaining
	}
	return vat
}

// RecordTaxInvoice: ได้รับใบกำกับภาษีจากผู้ขาย — ภาษีซื้อยังไม่ถึงกำหนด vat ถึงกำหนด (ใช้สิทธิได้)
// ฐานตามสัดส่วน; ใบที่ทำให้ครบใช้ส่วนที่เหลือทั้งหมด (ผลรวมตรงเสมอ)
func (i *Invoice) RecordTaxInvoice(vat vo.Money, amountDecimals int) (base vo.Money, err error) {
	if i.s.Status != vo.DocPosted {
		return decimal.Zero, domain.ErrInvoiceNotPayable
	}
	rem, remBase := i.RemainingUndueVat(), i.RemainingUndueBase()
	if !rem.IsPositive() {
		return decimal.Zero, domain.ErrNoUndueInputVat
	}
	if !vat.IsPositive() || !vat.Equal(vat.Round(int32(amountDecimals))) {
		return decimal.Zero, domain.ErrInvalidTaxInvoiceVat
	}
	if vat.GreaterThan(rem) {
		return decimal.Zero, fmt.Errorf("%w (remaining %s)", domain.ErrOverRecognition, rem)
	}
	base = remBase
	if !vat.Equal(rem) {
		base = remBase.Mul(vat).Div(rem).Round(int32(amountDecimals))
	}
	d := &i.s.Deferral
	d.VatRecognized, d.BaseRecognized = d.VatRecognized.Add(vat), d.BaseRecognized.Add(base)
	return base, nil
}

// UnrecordTaxInvoice: ยกเลิกใบกำกับที่บันทึกไว้ — กลับเป็นภาษีซื้อยังไม่ถึงกำหนด
func (i *Invoice) UnrecordTaxInvoice(base, vat vo.Money) {
	d := &i.s.Deferral
	d.VatRecognized, d.BaseRecognized = d.VatRecognized.Sub(vat), d.BaseRecognized.Sub(base)
}

// CreditUndue: ใบลดหนี้ของใบที่ยังรอใบกำกับ — VAT ของใบลดหนี้ลดภาษีซื้อยังไม่ถึงกำหนด (true)
// ใบที่ได้รับใบกำกับครบแล้ว = ลดภาษีซื้อตามปกติ (false); ใบลดหนี้ VAT เกินส่วนที่ยังรอ (ได้รับใบกำกับบางส่วน) = ปฏิเสธ
func (i *Invoice) CreditUndue(net, vat vo.Money) (bool, error) {
	rem := i.RemainingUndueVat()
	if !rem.IsPositive() || vat.IsZero() {
		return false, nil
	}
	if vat.GreaterThan(rem) {
		return false, fmt.Errorf("%w (remaining %s)", domain.ErrCreditExceedsUndueVat, rem)
	}
	d := &i.s.Deferral
	d.VatCredited, d.BaseCredited = d.VatCredited.Add(vat), d.BaseCredited.Add(net)
	return true, nil
}

func (i *Invoice) UncreditUndue(net, vat vo.Money) {
	d := &i.s.Deferral
	d.VatCredited, d.BaseCredited = d.VatCredited.Sub(vat), d.BaseCredited.Sub(net)
}

func (i *Invoice) SetEntry(id vo.EntryID, no string) {
	i.s.Posting.EntryID, i.s.Posting.EntryNo = &id, no
}

// Outstanding คือยอดค้างจ่าย (รวม VAT)
func (i *Invoice) Outstanding() vo.Money { return i.s.Totals.Gross.Sub(i.s.Paid) }

// Pay: ใบแจ้งหนี้ต้อง posted ของผู้ขายเดียวกัน และจ่ายไม่เกินยอดค้าง
func (i *Invoice) Pay(supplierID vo.SupplierID, amount vo.Money) error {
	if i.s.Status != vo.DocPosted || i.s.SupplierID != supplierID {
		return fmt.Errorf("%w: %s", domain.ErrInvoiceNotPayable, i.s.Posting.DocumentNo)
	}
	if amount.GreaterThan(i.Outstanding()) {
		return fmt.Errorf("%w: %s (outstanding %s)", domain.ErrOverPayment, i.s.Posting.DocumentNo, i.Outstanding())
	}
	i.s.Paid = i.s.Paid.Add(amount)
	return nil
}

// Unpay กลับยอดจ่าย (ยกเลิกใบจ่าย)
func (i *Invoice) Unpay(amount vo.Money) { i.s.Paid = i.s.Paid.Sub(amount) }

func (i *Invoice) Cancel(by vo.UserID, at time.Time) error {
	if i.s.Status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	if !i.s.Paid.IsZero() {
		return domain.ErrInvoicePaid
	}
	if i.s.Deferral.VatRecognized.IsPositive() {
		return domain.ErrInvoiceHasTaxInvoices
	}
	i.s.Status, i.s.Cancellation = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}
