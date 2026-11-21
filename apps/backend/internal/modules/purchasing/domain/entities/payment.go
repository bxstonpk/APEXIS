package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// PaymentLine คือการตัดจ่ายใบแจ้งหนี้หนึ่งใบ — Amount คือยอดที่ตัดหนี้ (รวม VAT, ก่อนหักภาษี)
// ภาษีหัก ณ ที่จ่ายเก็บอัตรา / ฐาน / ยอด ณ วันจ่ายไว้ในบรรทัด
type PaymentLine struct {
	InvoiceID vo.InvoiceID
	Amount    vo.Money
	WhtTypeID *vo.WhtTypeID
	WhtRate   vo.Money // %
	WhtBase   vo.Money // ยอดก่อน VAT ตามสัดส่วนที่จ่าย
	Wht       vo.Money
}

// PaymentMethod คือช่องทางจ่าย (เงินสด / ธนาคาร จากทะเบียนของ cashbank) — ลงบัญชีที่ทะเบียนผูกไว้
type PaymentMethod struct {
	CashAccountID vo.CashAccountID
	GLAccountID   vo.AccountID
	Amount        vo.Money
	Reference     string // เลขเช็ค / เลขอ้างอิงการโอน
}

type PaymentTotals struct {
	Amount vo.Money // ยอดตัดหนี้รวม
	Wht    vo.Money // ภาษีหัก ณ ที่จ่ายรวม
	Paid   vo.Money // จ่ายจริง = Amount - Wht
}

type PaymentState struct {
	ID           vo.PaymentID
	CompanyID    vo.CompanyID
	SupplierID   vo.SupplierID
	TaxType      vo.TaxType // ของผู้ขาย ณ วันจ่าย (กำหนดแบบ ภ.ง.ด.3 / 53)
	Date         time.Time
	Description  string
	Lines        []PaymentLine
	Methods      []PaymentMethod
	Totals       PaymentTotals
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// Payment คือใบจ่ายชำระเจ้าหนี้ (ผู้ขาย 1 ราย) — ตัดได้หลายใบแจ้งหนี้, จ่ายได้หลายช่องทาง
//
// กฎ: ใบแจ้งหนี้ไม่ซ้ำ, ยอด > 0, ทะเบียนเงินไม่ซ้ำ, ผลรวมช่องทางจ่าย = ยอดตัดหนี้ - ภาษีหัก ณ ที่จ่าย
type Payment struct{ s PaymentState }

func NewPayment(s PaymentState) (*Payment, error) {
	var err error
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalidPayment}, args...)...)
	}
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if len(s.Lines) == 0 || len(s.Lines) > MaxLines {
		return nil, bad("a payment must have 1 to %d invoices", MaxLines)
	}
	if len(s.Methods) == 0 || len(s.Methods) > 20 {
		return nil, bad("a payment must have 1 to 20 payment methods")
	}
	t := PaymentTotals{Amount: decimal.Zero, Wht: decimal.Zero, Paid: decimal.Zero}
	invoices := map[vo.InvoiceID]bool{}
	for i, l := range s.Lines {
		switch {
		case invoices[l.InvoiceID]:
			return nil, bad("line %d: invoice appears more than once", i+1)
		case !l.Amount.IsPositive() || !numeric.InRange(l.Amount):
			return nil, bad("line %d: amount must be positive", i+1)
		case l.Wht.GreaterThan(l.Amount):
			return nil, bad("line %d: withholding tax exceeds the amount", i+1)
		}
		invoices[l.InvoiceID] = true
		t.Amount, t.Wht = t.Amount.Add(l.Amount), t.Wht.Add(l.Wht)
	}
	accounts := map[vo.CashAccountID]bool{}
	for i := range s.Methods {
		m := &s.Methods[i]
		if accounts[m.CashAccountID] {
			return nil, bad("method %d: cash / bank account appears more than once", i+1)
		}
		if !m.Amount.IsPositive() || !numeric.InRange(m.Amount) {
			return nil, bad("method %d: amount must be positive", i+1)
		}
		if m.Reference, err = vo.OptionalText(m.Reference, 50); err != nil {
			return nil, err
		}
		accounts[m.CashAccountID] = true
		t.Paid = t.Paid.Add(m.Amount)
	}
	if net := t.Amount.Sub(t.Wht); !t.Paid.Equal(net) {
		return nil, bad("payment methods total %s must equal amount %s minus withholding tax %s (= %s)", t.Paid, t.Amount, t.Wht, net)
	}
	s.Totals = t
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.PaymentID](), vo.DocPosted, nil
	return &Payment{s: s}, nil
}

func RehydratePayment(s PaymentState) *Payment { return &Payment{s: s} }

func (p *Payment) ID() vo.PaymentID    { return p.s.ID }
func (p *Payment) State() PaymentState { return p.s }

func (p *Payment) SetEntry(id vo.EntryID, no string) {
	p.s.Posting.EntryID, p.s.Posting.EntryNo = &id, no
}

// Journal: Dr เจ้าหนี้ (ยอดตัดหนี้) / Cr บัญชีของช่องทางจ่าย + Cr ภาษีหัก ณ ที่จ่ายค้างจ่าย
// รวมยอดตามผังบัญชี (ช่องทางที่ผูกบัญชีเดียวกัน = 1 บรรทัด)
func (p *Payment) Journal(ap, wht vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	j.Debit(ap, p.s.Totals.Amount)
	for _, m := range p.s.Methods {
		j.Credit(m.GLAccountID, m.Amount)
	}
	j.Credit(wht, p.s.Totals.Wht)
	return j.Lines()
}

func (p *Payment) Cancel(by vo.UserID, at time.Time) error {
	if p.s.Status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	p.s.Status, p.s.Cancellation = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}
