package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

// Posting คือเลขที่และรายการบัญชีของเอกสารที่ลงบัญชีแล้ว
type Posting struct {
	DocumentNo string
	FiscalYear int
	PeriodID   vo.FiscalPeriodID
	EntryID    *vo.EntryID // nil = ไม่มีรายการบัญชี
	EntryNo    string
	By         vo.UserID
	At         time.Time
}

// Stock คือส่วนตัดสต็อกของเอกสารขายสินค้า / ใบลดหนี้รับคืน
type Stock struct {
	WarehouseID     *vo.WarehouseID
	System          vo.AccountingSystem
	StockDocumentID *vo.StockDocumentID // ใบเบิก / ใบรับของ inventory
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

// ---------- เอกสารขายที่ตั้งลูกหนี้ (ขายสินค้า / ขายบริการ / รับเงินมัดจำ) ----------
//
// จุดเกิดภาษี (ประมวลรัษฎากร):
//   - ขายสินค้า (ม.78): ส่งมอบ — เอกสารขายสินค้าออกใบกำกับภาษีพร้อมส่งของ ภาษีขายเกิดทันที
//     (ออกใบกำกับภาษีทำให้ภาษีของบรรทัดบริการในใบเดียวกันเกิดทันทีด้วย)
//   - ขายบริการ (ม.78/1): รับชำระ — ใบแจ้งหนี้บริการลงภาษีขายยังไม่ถึงกำหนด แล้วโอนเป็นภาษีขายตามสัดส่วนที่รับ
//     หรือทั้งหมดเมื่อออกใบกำกับภาษีก่อนรับเงิน
//   - รับเงินมัดจำ: ออกใบรับเงินมัดจำ / ใบกำกับภาษี ภาษีเกิดตามวันที่ใบ — รับเงินจริงที่ finance แล้วจึงหักในเอกสารขายภายหลัง
//     (ภาษีของเอกสารนั้นลดลงเท่าภาษีของมัดจำ)
//
// เงินเข้าทั้งหมดรับที่ finance (ใบรับเงิน) — sales ออกเอกสารเท่านั้น

// InvoiceLine: OrderLineNo = 0 เมื่อออกตรง (ไม่อ้างใบสั่งขาย); Service = สินค้าชนิดบริการ
type InvoiceLine struct {
	OrderLineNo      int
	ItemID           vo.ItemID
	Description      string
	Service          bool
	Quantity         vo.Quantity
	UnitPrice        vo.Money
	Amounts          services.Amounts
	RevenueAccountID vo.AccountID
	CostLine         // สินค้า stock ที่ตัดสต็อก (ขายสินค้า)
}

// DepositUse คือมัดจำที่หักในเอกสารขาย: Amount = ฐานก่อน VAT, Vat = ภาษีของมัดจำส่วนนั้น (เกิดไปแล้วตอนรับมัดจำ)
type DepositUse struct {
	DepositID vo.InvoiceID
	DepositNo string
	Amount    vo.Money
	Vat       vo.Money
}

func (d DepositUse) Gross() vo.Money { return d.Amount.Add(d.Vat) }

// VatDeferral คือภาษีขายยังไม่ถึงกำหนดของใบแจ้งหนี้บริการ
//
//	Vat / Base: ที่ลงบัญชีภาษีขายยังไม่ถึงกำหนดตอนออกใบ (หักภาษีของมัดจำแล้ว)
//	*Recognized: โอนเป็นภาษีขายแล้ว (รับชำระ / ใบกำกับภาษีก่อนรับเงิน)
//	*Credited: ลดด้วยใบลดหนี้ก่อนภาษีถึงกำหนด
type VatDeferral struct {
	Vat, VatRecognized, VatCredited    vo.Money
	Base, BaseRecognized, BaseCredited vo.Money
}

func (v VatDeferral) RemainingVat() vo.Money { return v.Vat.Sub(v.VatRecognized).Sub(v.VatCredited) }
func (v VatDeferral) RemainingBase() vo.Money {
	return v.Base.Sub(v.BaseRecognized).Sub(v.BaseCredited)
}

func zeroDeferral() VatDeferral {
	z := decimal.Zero
	return VatDeferral{Vat: z, VatRecognized: z, VatCredited: z, Base: z, BaseRecognized: z, BaseCredited: z}
}

// TaxInvoice คือใบกำกับภาษีที่ออกจากใบแจ้งหนี้บริการก่อนรับชำระ
type TaxInvoice struct {
	No      string
	Date    time.Time
	EntryID *vo.EntryID
	EntryNo string
	Base    vo.Money
	Vat     vo.Money
}

type InvoiceState struct {
	ID           vo.InvoiceID
	CompanyID    vo.CompanyID
	Kind         vo.InvoiceKind
	OrderID      *vo.OrderID // nil = ออกตรง ไม่อ้างใบสั่งขาย
	CustomerID   vo.CustomerID
	BillTo       BillTo // ผู้ซื้อตามใบกำกับภาษี ณ วันออก
	Date         time.Time
	DueDate      time.Time // วันที่ + เครดิตเทอม
	Reference    string
	Description  string
	Vat          services.VatTerms
	Stock        Stock
	Lines        []InvoiceLine
	Totals       services.Amounts // มัดจำ: ยอดมัดจำ (ไม่มีบรรทัด)
	Deposits     []DepositUse     // มัดจำที่หักในใบนี้
	Received     vo.Money         // ยอดที่ตัดแล้ว (รับเงิน + ใบลดหนี้ + มัดจำที่หัก)
	Deferral     VatDeferral      // ขายบริการเท่านั้น
	TaxInvoice   *TaxInvoice      // ขายบริการ: ใบกำกับภาษีก่อนรับเงิน
	Applied      vo.Money         // มัดจำ: ฐานที่ถูกหักไปแล้ว
	AppliedVat   vo.Money         // มัดจำ: ภาษีของส่วนที่ถูกหัก
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// BillTo คือชื่อ / เลขผู้เสียภาษี / สาขา / ที่อยู่ของผู้ซื้อ ณ วันออกใบกำกับภาษี (ม.86/4)
// เก็บไว้ในใบ — แก้ทะเบียนลูกค้าภายหลังไม่กระทบใบที่ออกแล้ว
type BillTo struct {
	Name     string
	TaxID    string
	BranchNo string
	Address  string
}

// Invoice คือเอกสารขายที่ตั้งลูกหนี้ — ลงบัญชีทันที (ดู Journal)
type Invoice struct{ s InvoiceState }

// NewInvoice: ยอดรวมจากบรรทัด (มัดจำใช้ Totals ที่ส่งมา), มัดจำที่หัก, ภาษียังไม่ถึงกำหนดของขายบริการ
// มัดจำที่หัก = ตัดยอดลูกหนี้ของใบนี้ตั้งแต่ออก (Received เริ่มที่ยอดมัดจำรวม VAT)
func NewInvoice(s InvoiceState, paymentTermDays int) (*Invoice, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Reference, s.Description, err = headerText(s.Reference, s.Description); err != nil {
		return nil, err
	}
	s.DueDate = s.Date.AddDate(0, 0, paymentTermDays)
	if s.Kind == vo.KindDeposit {
		if len(s.Lines) > 0 || len(s.Deposits) > 0 || !s.Totals.Gross.IsPositive() || s.Totals.Net.IsNegative() || s.Totals.Vat.IsNegative() {
			return nil, fmt.Errorf("%w: deposit amount must be greater than 0", domain.ErrInvalidAmount)
		}
	} else {
		if len(s.Lines) == 0 {
			return nil, fmt.Errorf("%w: at least one line", domain.ErrInvalidLine)
		}
		s.Totals = services.ZeroAmounts()
		for n, l := range s.Lines {
			if s.Kind == vo.KindServiceInvoice && !l.Service {
				return nil, fmt.Errorf("%w: line %d", domain.ErrServiceInvoiceGoodsLine, n+1)
			}
			s.Totals = s.Totals.Add(l.Amounts)
		}
	}
	depBase, depVat := decimal.Zero, decimal.Zero
	for _, d := range s.Deposits {
		depBase, depVat = depBase.Add(d.Amount), depVat.Add(d.Vat)
	}
	if depBase.GreaterThan(s.Totals.Net) || depVat.GreaterThan(s.Totals.Vat) {
		return nil, domain.ErrDepositExceedsSale
	}
	s.Deferral = zeroDeferral()
	if s.Kind == vo.KindServiceInvoice {
		s.Deferral.Vat, s.Deferral.Base = s.Totals.Vat.Sub(depVat), s.Totals.Net.Sub(depBase)
	}
	s.Received = depBase.Add(depVat)
	s.Applied, s.AppliedVat, s.TaxInvoice = decimal.Zero, decimal.Zero, nil
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.InvoiceID](), vo.DocPosted, nil
	return &Invoice{s: s}, nil
}

func RehydrateInvoice(s InvoiceState) *Invoice { return &Invoice{s: s} }

func (i *Invoice) ID() vo.InvoiceID    { return i.s.ID }
func (i *Invoice) State() InvoiceState { return i.s }
func (i *Invoice) SetEntry(id vo.EntryID, no string) {
	i.s.Posting.EntryID, i.s.Posting.EntryNo = &id, no
}

// Shipments: บรรทัดที่อ้างใบสั่งขาย (ออกตรง = ไม่มี)
func (i *Invoice) Shipments() []Shipment {
	var out []Shipment
	for _, l := range i.s.Lines {
		if l.OrderLineNo > 0 {
			out = append(out, Shipment{LineNo: l.OrderLineNo, Quantity: l.Quantity})
		}
	}
	return out
}

// DepositTotals: ยอดมัดจำที่หัก (ฐาน, ภาษี)
func (i *Invoice) DepositTotals() (vo.Money, vo.Money) {
	b, v := decimal.Zero, decimal.Zero
	for _, d := range i.s.Deposits {
		b, v = b.Add(d.Amount), v.Add(d.Vat)
	}
	return b, v
}

// JournalAccounts คือบัญชีที่ใช้ลงรายการเอกสารขาย
type JournalAccounts struct {
	AR, OutputVat, UndueVat, Deposit, Cogs vo.AccountID
}

// Journal:
//
//	มัดจำ: Dr ลูกหนี้ / Cr เงินมัดจำรับ (ฐาน) / Cr ภาษีขาย
//	ขายสินค้า: Dr ลูกหนี้ (หลังหักมัดจำ) + Dr เงินมัดจำรับ (ฐานที่หัก) / Cr รายได้ / Cr ภาษีขาย (หักภาษีของมัดจำ)
//	                       [+ Dr ต้นทุนขาย / Cr สินค้าคงเหลือ เมื่อ perpetual]
//	ขายบริการ: เหมือนขายสินค้า แต่ภาษีเข้า "ภาษีขายยังไม่ถึงกำหนด"
func (i *Invoice) Journal(a JournalAccounts) []services.JournalLine {
	j := services.NewJournal()
	t := i.s.Totals
	if i.s.Kind == vo.KindDeposit {
		j.Debit(a.AR, t.Gross)
		j.Credit(a.Deposit, t.Net)
		j.Credit(a.OutputVat, t.Vat)
		return j.Lines()
	}
	depBase, depVat := i.DepositTotals()
	j.Debit(a.AR, t.Gross.Sub(depBase).Sub(depVat))
	j.Debit(a.Deposit, depBase)
	costs := make([]CostLine, 0, len(i.s.Lines))
	for _, l := range i.s.Lines {
		j.Credit(l.RevenueAccountID, l.Amounts.Net)
		costs = append(costs, l.CostLine)
	}
	vatAccount := a.OutputVat
	if i.s.Kind == vo.KindServiceInvoice {
		vatAccount = a.UndueVat
	}
	j.Credit(vatAccount, t.Vat.Sub(depVat))
	costJournal(j, i.s.Stock.System, a.Cogs, costs)
	return j.Lines()
}

// Receivable: ยอดลูกหนี้ของใบ (รวม VAT, หักมัดจำแล้ว)
func (i *Invoice) Receivable() vo.Money {
	b, v := i.DepositTotals()
	return i.s.Totals.Gross.Sub(b).Sub(v)
}

func (i *Invoice) Outstanding() vo.Money { return i.s.Totals.Gross.Sub(i.s.Received) }

// Receive: เอกสารต้อง posted ของลูกค้าเดียวกัน และรับไม่เกินยอดค้าง
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

// VatDue: ขายบริการที่ยังไม่ออกใบกำกับภาษีและยังมีภาษียังไม่ถึงกำหนด
func (i *Invoice) VatDue() bool {
	return i.s.Kind == vo.KindServiceInvoice && i.s.TaxInvoice == nil && i.s.Deferral.RemainingVat().IsPositive()
}

// Recognize: ภาษีที่ถึงกำหนดจากการรับชำระ amount (เรียกก่อน Receive)
// สัดส่วน = amount / ยอดค้างก่อนรับ ของภาษี (และฐาน) ที่ยังไม่ถึงกำหนด — รับครบยอดค้าง = ส่วนที่เหลือทั้งหมด (ผลรวมตรงเสมอ)
func (i *Invoice) Recognize(amount vo.Money, amountDecimals int) (base, vat vo.Money) {
	if !i.VatDue() || !amount.IsPositive() {
		return decimal.Zero, decimal.Zero
	}
	d := &i.s.Deferral
	remVat, remBase := d.RemainingVat(), d.RemainingBase()
	out := i.Outstanding()
	if amount.GreaterThanOrEqual(out) || !out.IsPositive() {
		vat, base = remVat, remBase
	} else {
		dec := int32(amountDecimals)
		vat = remVat.Mul(amount).Div(out).Round(dec)
		base = remBase.Mul(amount).Div(out).Round(dec)
	}
	d.VatRecognized, d.BaseRecognized = d.VatRecognized.Add(vat), d.BaseRecognized.Add(base)
	return base, vat
}

func (i *Invoice) Unrecognize(base, vat vo.Money) {
	d := &i.s.Deferral
	d.VatRecognized, d.BaseRecognized = d.VatRecognized.Sub(vat), d.BaseRecognized.Sub(base)
}

// IssueTaxInvoice: ใบกำกับภาษีก่อนรับเงิน — ภาษีที่ยังไม่ถึงกำหนดทั้งหมดถึงกำหนด ณ วันที่ใบกำกับ
func (i *Invoice) IssueTaxInvoice(no string, date time.Time) (base, vat vo.Money, err error) {
	switch {
	case i.s.Kind != vo.KindServiceInvoice:
		return base, vat, domain.ErrNotServiceInvoice
	case i.s.Status != vo.DocPosted:
		return base, vat, domain.ErrAlreadyCancelled
	case i.s.TaxInvoice != nil:
		return base, vat, domain.ErrTaxInvoiceIssued
	case !vo.IsDateOnly(date) || date.Before(i.s.Date):
		return base, vat, domain.ErrInvalidDate
	case !i.s.Deferral.RemainingVat().IsPositive():
		return base, vat, domain.ErrNoUndueVat
	}
	d := &i.s.Deferral
	base, vat = d.RemainingBase(), d.RemainingVat()
	d.VatRecognized, d.BaseRecognized = d.VatRecognized.Add(vat), d.BaseRecognized.Add(base)
	i.s.TaxInvoice = &TaxInvoice{No: no, Date: date, Base: base, Vat: vat}
	return base, vat, nil
}

func (i *Invoice) SetTaxInvoiceEntry(id vo.EntryID, no string) {
	if i.s.TaxInvoice != nil {
		i.s.TaxInvoice.EntryID, i.s.TaxInvoice.EntryNo = &id, no
	}
}

// CreditVatKind: ใบลดหนี้ของใบนี้ลดภาษีขายยังไม่ถึงกำหนด (ขายบริการที่ยังไม่ออกใบกำกับ) หรือภาษีขาย
func (i *Invoice) CreditVatKind() vo.VatKind {
	if i.s.Kind == vo.KindServiceInvoice && i.s.TaxInvoice == nil {
		return vo.VatUndue
	}
	return vo.VatOutput
}

// CreditUndue: ใบลดหนี้ก่อนภาษีถึงกำหนด — ลดภาษี / ฐานที่ยังไม่ถึงกำหนด (ไม่เกินที่เหลือ)
func (i *Invoice) CreditUndue(base, vat vo.Money) error {
	d := &i.s.Deferral
	if vat.GreaterThan(d.RemainingVat()) || base.GreaterThan(d.RemainingBase()) {
		return fmt.Errorf("%w: %s (undue vat left %s)", domain.ErrOverCredit, i.s.Posting.DocumentNo, d.RemainingVat())
	}
	d.VatCredited, d.BaseCredited = d.VatCredited.Add(vat), d.BaseCredited.Add(base)
	return nil
}

func (i *Invoice) UncreditUndue(base, vat vo.Money) {
	d := &i.s.Deferral
	d.VatCredited, d.BaseCredited = d.VatCredited.Sub(vat), d.BaseCredited.Sub(base)
}

// ---------- มัดจำ ----------

func (i *Invoice) DepositRemaining() vo.Money { return i.s.Totals.Net.Sub(i.s.Applied) }

// ApplyDeposit: หักมัดจำ amount (ฐานก่อน VAT) — ต้องเป็นมัดจำ posted ของลูกค้าเดียวกันที่รับเงินครบแล้ว (finance) ไม่เกินที่เหลือ
// ภาษีตามสัดส่วน; หักครบ = ภาษีที่เหลือทั้งหมด
func (i *Invoice) ApplyDeposit(customerID vo.CustomerID, amount vo.Money, amountDecimals int) (vo.Money, error) {
	if i.s.Kind != vo.KindDeposit || i.s.Status != vo.DocPosted || i.s.CustomerID != customerID {
		return decimal.Zero, fmt.Errorf("%w: %s", domain.ErrDepositNotUsable, i.s.Posting.DocumentNo)
	}
	if i.Outstanding().IsPositive() {
		return decimal.Zero, fmt.Errorf("%w: %s", domain.ErrDepositNotPaid, i.s.Posting.DocumentNo)
	}
	rem := i.DepositRemaining()
	if !amount.IsPositive() || amount.GreaterThan(rem) {
		return decimal.Zero, fmt.Errorf("%w: %s (remaining %s)", domain.ErrDepositOverApplied, i.s.Posting.DocumentNo, rem)
	}
	var vat vo.Money
	if amount.Equal(rem) {
		vat = i.s.Totals.Vat.Sub(i.s.AppliedVat)
	} else {
		vat = i.s.Totals.Vat.Mul(amount).Div(i.s.Totals.Net).Round(int32(amountDecimals))
	}
	i.s.Applied, i.s.AppliedVat = i.s.Applied.Add(amount), i.s.AppliedVat.Add(vat)
	return vat, nil
}

func (i *Invoice) UnapplyDeposit(amount, vat vo.Money) {
	i.s.Applied, i.s.AppliedVat = i.s.Applied.Sub(amount), i.s.AppliedVat.Sub(vat)
}

// Cancel: ต้องไม่มีรับเงิน / ใบลดหนี้ (ยอดที่ตัดเหลือเฉพาะมัดจำที่หัก); มัดจำต้องยังไม่ถูกหัก
func (i *Invoice) Cancel(by vo.UserID, at time.Time) error {
	if i.s.Status == vo.DocPosted {
		b, v := i.DepositTotals()
		if !i.s.Received.Equal(b.Add(v)) {
			return domain.ErrInvoicePaid
		}
		if i.s.Kind == vo.KindDeposit && i.s.Applied.IsPositive() {
			return domain.ErrDepositApplied
		}
	}
	return cancel(&i.s.Status, &i.s.Cancellation, by, at)
}
