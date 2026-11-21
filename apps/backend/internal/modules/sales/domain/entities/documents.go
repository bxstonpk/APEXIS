package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// Posting คือเลขที่และรายการบัญชีของเอกสารที่ลงบัญชีแล้ว
type Posting struct {
	DocumentNo string
	FiscalYear int
	PeriodID   vo.FiscalPeriodID
	EntryID    *vo.EntryID // nil = ไม่มีรายการบัญชี (เช่น ส่งของระบบ periodic)
	EntryNo    string
	By         vo.UserID
	At         time.Time
}

// Stock คือส่วนตัดสต็อกของเอกสาร (ใบส่งของ / ใบส่งของ-ใบกำกับภาษี)
type Stock struct {
	WarehouseID     *vo.WarehouseID
	System          vo.AccountingSystem
	StockDocumentID *vo.StockDocumentID // ใบเบิกของ inventory
	StockDocumentNo string
}

// CostLine: ต้นทุน FIFO ของบรรทัดที่ตัดสต็อก — perpetual ลง Dr ต้นทุนขาย / Cr สินค้าคงเหลือ
type CostLine struct {
	Stock              bool
	Cost               vo.Money
	InventoryAccountID *vo.AccountID // perpetual + stock เท่านั้น
}

func costJournal(j *services.Journal, system vo.AccountingSystem, cogs vo.AccountID, lines []CostLine) {
	if system != vo.Perpetual {
		return
	}
	for _, l := range lines {
		if l.Stock && l.InventoryAccountID != nil {
			j.Debit(cogs, l.Cost)
			j.Credit(*l.InventoryAccountID, l.Cost)
		}
	}
}

func cancel(status *vo.DocStatus, c **Cancellation, by vo.UserID, at time.Time) error {
	if *status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	*status, *c = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}

// ---------- ใบส่งของ ----------

type DeliveryLine struct {
	OrderLineNo int
	ItemID      vo.ItemID
	Quantity    vo.Quantity
	CostLine
}

type DeliveryState struct {
	ID           vo.DeliveryID
	CompanyID    vo.CompanyID
	OrderID      vo.OrderID
	CustomerID   vo.CustomerID
	Date         time.Time
	Reference    string
	Description  string
	Stock        Stock
	Lines        []DeliveryLine
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// Delivery คือใบส่งของ (แยกจากใบแจ้งหนี้) — ตัดสต็อก FIFO + ต้นทุนขาย (perpetual) ทันที
type Delivery struct{ s DeliveryState }

func NewDelivery(s DeliveryState) (*Delivery, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Reference, s.Description, err = headerText(s.Reference, s.Description); err != nil {
		return nil, err
	}
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.DeliveryID](), vo.DocPosted, nil
	return &Delivery{s: s}, nil
}

func RehydrateDelivery(s DeliveryState) *Delivery { return &Delivery{s: s} }

func (d *Delivery) ID() vo.DeliveryID    { return d.s.ID }
func (d *Delivery) State() DeliveryState { return d.s }
func (d *Delivery) SetEntry(id vo.EntryID, no string) {
	d.s.Posting.EntryID, d.s.Posting.EntryNo = &id, no
}

func (d *Delivery) Shipments() []Shipment {
	out := make([]Shipment, len(d.s.Lines))
	for i, l := range d.s.Lines {
		out[i] = Shipment{LineNo: l.OrderLineNo, Quantity: l.Quantity}
	}
	return out
}

// Journal: Dr ต้นทุนขาย / Cr สินค้าคงเหลือ (perpetual, สินค้า stock) — periodic ไม่มีรายการ
func (d *Delivery) Journal(cogs vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	lines := make([]CostLine, len(d.s.Lines))
	for i, l := range d.s.Lines {
		lines[i] = l.CostLine
	}
	costJournal(j, d.s.Stock.System, cogs, lines)
	return j.Lines()
}

func (d *Delivery) Cancel(by vo.UserID, at time.Time) error {
	return cancel(&d.s.Status, &d.s.Cancellation, by, at)
}

// ---------- ใบแจ้งหนี้ / ใบกำกับภาษี ----------

type InvoiceLine struct {
	OrderLineNo      int
	ItemID           vo.ItemID
	Quantity         vo.Quantity
	UnitPrice        vo.Money
	Amounts          services.Amounts
	RevenueAccountID vo.AccountID
	CostLine         // เฉพาะใบส่งของ / ใบกำกับภาษี (WithDelivery)
}

type InvoiceState struct {
	ID           vo.InvoiceID
	CompanyID    vo.CompanyID
	OrderID      vo.OrderID
	CustomerID   vo.CustomerID
	Date         time.Time
	DueDate      time.Time // วันที่ + เครดิตเทอมตามใบสั่งขาย
	Reference    string
	Description  string
	Vat          services.VatTerms
	WithDelivery bool  // ใบส่งของ / ใบกำกับภาษี (ส่งของพร้อมออกบิล)
	Stock        Stock // เฉพาะ WithDelivery
	Lines        []InvoiceLine
	Totals       services.Amounts
	Received     vo.Money // ยอดรับชำระแล้ว (ใบเสร็จที่ยังไม่ยกเลิก)
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// Invoice คือใบแจ้งหนี้ / ใบกำกับภาษี — ลงบัญชีทันที: Dr ลูกหนี้ / Cr รายได้ + ภาษีขาย
// (แบบรวมใบเพิ่ม Dr ต้นทุนขาย / Cr สินค้าคงเหลือ ในรายการเดียวกัน)
type Invoice struct{ s InvoiceState }

func NewInvoice(s InvoiceState, paymentTermDays int) (*Invoice, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Reference, s.Description, err = headerText(s.Reference, s.Description); err != nil {
		return nil, err
	}
	s.DueDate = s.Date.AddDate(0, 0, paymentTermDays)
	s.Totals = services.ZeroAmounts()
	for _, l := range s.Lines {
		s.Totals = s.Totals.Add(l.Amounts)
	}
	s.ID, s.Status, s.Cancellation, s.Received = vo.NewID[vo.InvoiceID](), vo.DocPosted, nil, decimal.Zero
	return &Invoice{s: s}, nil
}

func RehydrateInvoice(s InvoiceState) *Invoice { return &Invoice{s: s} }

func (i *Invoice) ID() vo.InvoiceID    { return i.s.ID }
func (i *Invoice) State() InvoiceState { return i.s }
func (i *Invoice) SetEntry(id vo.EntryID, no string) {
	i.s.Posting.EntryID, i.s.Posting.EntryNo = &id, no
}

func (i *Invoice) Shipments() []Shipment {
	out := make([]Shipment, len(i.s.Lines))
	for n, l := range i.s.Lines {
		out[n] = Shipment{LineNo: l.OrderLineNo, Quantity: l.Quantity}
	}
	return out
}

// Journal: Dr ลูกหนี้ (รวม VAT) / Cr รายได้ (ตามบัญชีของบรรทัด) + Cr ภาษีขาย [+ ต้นทุนขาย / สินค้าคงเหลือ]
func (i *Invoice) Journal(ar, outputVat, cogs vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	j.Debit(ar, i.s.Totals.Gross)
	costs := make([]CostLine, 0, len(i.s.Lines))
	for _, l := range i.s.Lines {
		j.Credit(l.RevenueAccountID, l.Amounts.Net)
		costs = append(costs, l.CostLine)
	}
	j.Credit(outputVat, i.s.Totals.Vat)
	if i.s.WithDelivery {
		costJournal(j, i.s.Stock.System, cogs, costs)
	}
	return j.Lines()
}

func (i *Invoice) Outstanding() vo.Money { return i.s.Totals.Gross.Sub(i.s.Received) }

// Receive: ใบแจ้งหนี้ต้อง posted ของลูกค้าเดียวกัน และรับไม่เกินยอดค้าง
func (i *Invoice) Receive(customerID vo.CustomerID, amount vo.Money) error {
	if i.s.Status != vo.DocPosted || i.s.CustomerID != customerID {
		return fmt.Errorf("%w: %s", domain.ErrInvoiceNotPayable, i.s.Posting.DocumentNo)
	}
	if amount.GreaterThan(i.Outstanding()) {
		return fmt.Errorf("%w: %s (outstanding %s)", domain.ErrOverReceipt, i.s.Posting.DocumentNo, i.Outstanding())
	}
	i.s.Received = i.s.Received.Add(amount)
	return nil
}

func (i *Invoice) Unreceive(amount vo.Money) { i.s.Received = i.s.Received.Sub(amount) }

func (i *Invoice) Cancel(by vo.UserID, at time.Time) error {
	if i.s.Status == vo.DocPosted && !i.s.Received.IsZero() {
		return domain.ErrInvoicePaid
	}
	return cancel(&i.s.Status, &i.s.Cancellation, by, at)
}

// ---------- ใบเสร็จรับเงิน ----------

// ReceiptLine: ยอดที่ตัดหนี้ (รวม VAT) และภาษีที่ลูกค้าหัก ณ ที่จ่าย
type ReceiptLine struct {
	InvoiceID vo.InvoiceID
	Amount    vo.Money
	Wht       vo.Money
}

type ReceiptMethod struct {
	CashAccountID vo.CashAccountID
	GLAccountID   vo.AccountID
	Amount        vo.Money
	Reference     string
}

type ReceiptTotals struct {
	Amount   vo.Money // ตัดหนี้รวม
	Wht      vo.Money // ถูกหัก ณ ที่จ่ายรวม
	Received vo.Money // รับจริง = Amount - Wht
}

type ReceiptState struct {
	ID           vo.ReceiptID
	CompanyID    vo.CompanyID
	CustomerID   vo.CustomerID
	Date         time.Time
	Description  string
	Lines        []ReceiptLine
	Methods      []ReceiptMethod
	Totals       ReceiptTotals
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// Receipt คือใบเสร็จรับเงิน (ลูกค้า 1 ราย) — ตัดหลายใบแจ้งหนี้, รับหลายช่องทาง; รวมช่องทาง = ตัดหนี้ - ถูกหัก ณ ที่จ่าย
type Receipt struct{ s ReceiptState }

func NewReceipt(s ReceiptState, amountDecimals int) (*Receipt, error) {
	var err error
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalidReceipt}, args...)...)
	}
	money := func(v vo.Money) bool { return numeric.InRange(v) && numeric.FitsScale(v, amountDecimals) }
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if len(s.Lines) == 0 || len(s.Lines) > MaxLines {
		return nil, bad("a receipt must have 1 to %d invoices", MaxLines)
	}
	if len(s.Methods) == 0 || len(s.Methods) > 20 {
		return nil, bad("a receipt must have 1 to 20 payment methods")
	}
	t := ReceiptTotals{Amount: decimal.Zero, Wht: decimal.Zero, Received: decimal.Zero}
	invoices := map[vo.InvoiceID]bool{}
	for i, l := range s.Lines {
		switch {
		case invoices[l.InvoiceID]:
			return nil, bad("line %d: invoice appears more than once", i+1)
		case !l.Amount.IsPositive() || !money(l.Amount):
			return nil, bad("line %d: amount must be positive with at most %d decimal places", i+1, amountDecimals)
		case l.Wht.IsNegative() || !money(l.Wht) || l.Wht.GreaterThan(l.Amount):
			return nil, bad("line %d: withholding tax must be zero to amount with at most %d decimal places", i+1, amountDecimals)
		}
		invoices[l.InvoiceID] = true
		t.Amount, t.Wht = t.Amount.Add(l.Amount), t.Wht.Add(l.Wht)
	}
	accounts := map[vo.CashAccountID]bool{}
	for i := range s.Methods {
		m := &s.Methods[i]
		switch {
		case accounts[m.CashAccountID]:
			return nil, bad("method %d: cash / bank account appears more than once", i+1)
		case !m.Amount.IsPositive() || !money(m.Amount):
			return nil, bad("method %d: amount must be positive with at most %d decimal places", i+1, amountDecimals)
		}
		if m.Reference, err = vo.OptionalText(m.Reference, 50); err != nil {
			return nil, err
		}
		accounts[m.CashAccountID] = true
		t.Received = t.Received.Add(m.Amount)
	}
	if net := t.Amount.Sub(t.Wht); !t.Received.Equal(net) {
		return nil, bad("payment methods total %s must equal amount %s minus withholding tax %s (= %s)", t.Received, t.Amount, t.Wht, net)
	}
	s.Totals = t
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.ReceiptID](), vo.DocPosted, nil
	return &Receipt{s: s}, nil
}

func RehydrateReceipt(s ReceiptState) *Receipt { return &Receipt{s: s} }

func (r *Receipt) ID() vo.ReceiptID    { return r.s.ID }
func (r *Receipt) State() ReceiptState { return r.s }
func (r *Receipt) SetEntry(id vo.EntryID, no string) {
	r.s.Posting.EntryID, r.s.Posting.EntryNo = &id, no
}

// Journal: Dr เงินสด / ธนาคาร (ตามทะเบียน) + Dr ภาษีถูกหัก ณ ที่จ่าย / Cr ลูกหนี้
func (r *Receipt) Journal(ar, wht vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	for _, m := range r.s.Methods {
		j.Debit(m.GLAccountID, m.Amount)
	}
	j.Debit(wht, r.s.Totals.Wht)
	j.Credit(ar, r.s.Totals.Amount)
	return j.Lines()
}

func (r *Receipt) Cancel(by vo.UserID, at time.Time) error {
	return cancel(&r.s.Status, &r.s.Cancellation, by, at)
}
