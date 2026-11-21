package entities

import (
	"time"

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

type InvoiceState struct {
	ID                vo.InvoiceID
	CompanyID         vo.CompanyID
	OrderID           vo.OrderID
	SupplierID        vo.SupplierID
	SupplierInvoiceNo string    // เลขที่ใบแจ้งหนี้ / ใบกำกับภาษีของผู้ขาย (ไม่ซ้ำต่อผู้ขาย)
	InvoiceDate       time.Time // วันที่ในใบกำกับภาษี
	Date              time.Time // วันที่ลงบัญชี
	DueDate           time.Time // วันที่ลงบัญชี + เครดิตเทอมตามใบสั่งซื้อ
	Description       string
	Vat               services.VatTerms
	Lines             []InvoiceLine
	Totals            services.Amounts
	Status            vo.DocStatus
	Posting           Posting
	Cancellation      *Cancellation
}

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
	s.DueDate = s.Date.AddDate(0, 0, paymentTermDays)
	s.Totals = services.ZeroAmounts()
	for _, l := range s.Lines {
		s.Totals = s.Totals.Add(l.Amounts)
	}
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.InvoiceID](), vo.DocPosted, nil
	return &Invoice{s: s}, nil
}

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

// Journal: Dr GR/IR (ที่ล้าง) ± ผลต่างราคา + Dr ภาษีซื้อ — Cr เจ้าหนี้ (ยอดรวม VAT)
func (i *Invoice) Journal(grir, inputVat, ap vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	for _, l := range i.s.Lines {
		j.Debit(grir, l.Cleared)
		if l.VarianceAccountID != nil {
			j.Debit(*l.VarianceAccountID, l.Variance())
		}
	}
	j.Debit(inputVat, i.s.Totals.Vat)
	j.Credit(ap, i.s.Totals.Gross)
	return j.Lines()
}

func (i *Invoice) SetEntry(id vo.EntryID, no string) {
	i.s.Posting.EntryID, i.s.Posting.EntryNo = &id, no
}

func (i *Invoice) Cancel(by vo.UserID, at time.Time) error {
	if i.s.Status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	i.s.Status, i.s.Cancellation = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}
