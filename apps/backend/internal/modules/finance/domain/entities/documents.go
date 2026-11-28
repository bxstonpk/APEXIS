package entities

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/finance/domain/errors"
	"apexis/backend/internal/modules/finance/domain/services"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// MaxLines คือจำนวนบรรทัดสูงสุดต่อเอกสาร
const MaxLines = 500

// MaxMethods คือจำนวนช่องทางรับ / จ่ายเงินสูงสุดต่อเอกสาร
const MaxMethods = 20

// Party คือคู่ค้า ณ วันเอกสาร (เก็บติดเอกสาร — รายงานภาษีใช้ข้อมูลนี้ไม่ต้องอ่านข้าม module)
// ID = ลูกค้า / ผู้ขาย (nil = รับ / จ่ายเงินอื่นที่ระบุชื่อเอง)
type Party struct {
	ID       *uuid.UUID
	Code     string
	Name     string
	TaxID    string
	BranchNo string
	Address  string
	TaxType  vo.TaxType // ผู้รับเงิน (จ่ายเงิน) — กำหนดแบบ ภ.ง.ด.3 / 53
}

// Method คือช่องทางรับ / จ่ายเงิน (ทะเบียนเงินสด / ธนาคาร) — ลงบัญชีตามบัญชีที่ทะเบียนผูกไว้ ณ วันเอกสาร
type Method struct {
	CashAccountID vo.CashAccountID
	GLAccountID   vo.GLAccountID
	Amount        vo.Money
	Reference     string // เลขเช็ค / เลขอ้างอิงการโอน
}

// Posting คือข้อมูลการลงบัญชีของเอกสาร
type Posting struct {
	DocumentNo string
	FiscalYear int
	PeriodID   vo.FiscalPeriodID
	EntryID    *vo.EntryID
	EntryNo    string
	By         vo.UserID
	At         time.Time
}

type Cancellation struct {
	By vo.UserID
	At time.Time
}

func cancel(status *vo.DocStatus, c **Cancellation, by vo.UserID, at time.Time) error {
	if *status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	*status, *c = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}

// checkMethods: ช่องทาง 1-20, ทะเบียนไม่ซ้ำ, ยอด > 0 — คืนยอดรวม
func checkMethods(methods []Method, money func(vo.Money) bool, bad func(string, ...any) error) (vo.Money, error) {
	if len(methods) == 0 || len(methods) > MaxMethods {
		return decimal.Zero, bad("must have 1 to %d payment methods", MaxMethods)
	}
	total := decimal.Zero
	seen := map[vo.CashAccountID]bool{}
	for i := range methods {
		m := &methods[i]
		switch {
		case seen[m.CashAccountID]:
			return decimal.Zero, bad("method %d: cash / bank account appears more than once", i+1)
		case !m.Amount.IsPositive() || !money(m.Amount):
			return decimal.Zero, bad("method %d: amount must be positive", i+1)
		}
		ref, err := vo.OptionalText(m.Reference, 50)
		if err != nil {
			return decimal.Zero, err
		}
		m.Reference = ref
		seen[m.CashAccountID] = true
		total = total.Add(m.Amount)
	}
	return total, nil
}

func normalizeParty(p *Party) error {
	var err error
	if p.Name, err = vo.OptionalText(p.Name, 200); err != nil {
		return err
	}
	if p.TaxID, err = vo.OptionalText(p.TaxID, 20); err != nil {
		return err
	}
	if p.BranchNo, err = vo.OptionalText(p.BranchNo, 5); err != nil {
		return err
	}
	if p.Address, err = vo.OptionalText(p.Address, 500); err != nil {
		return err
	}
	return nil
}

// ---------- ใบรับเงิน ----------

// ReceiptLine: รับชำระหนี้ = InvoiceID (ตัดยอดใบแจ้งหนี้ขาย, Wht = ภาษีที่ลูกค้าหักไว้)
// รับเงินอื่น = AccountID (เครดิตบัญชีนั้น, ไม่มีภาษีถูกหัก)
// ReceiptLine: InvoiceKind / VatBase / Vat มาจาก sales ตอนตัดยอด — Vat > 0 = ภาษีขายของใบแจ้งหนี้บริการถึงกำหนดด้วยใบรับเงินนี้
// (ใบรับเงินเป็น "ใบเสร็จรับเงิน / ใบกำกับภาษี")
type ReceiptLine struct {
	InvoiceID   *vo.InvoiceID
	InvoiceNo   string
	InvoiceKind string
	AccountID   *vo.GLAccountID
	Description string
	Amount      vo.Money
	Wht         vo.Money
	VatBase     vo.Money
	Vat         vo.Money
}

type ReceiptTotals struct {
	Amount   vo.Money // ตัดหนี้ / รับรวม
	Wht      vo.Money // ถูกหัก ณ ที่จ่ายรวม
	Received vo.Money // รับจริง = Amount - Wht
}

type ReceiptState struct {
	ID           vo.ReceiptID
	CompanyID    vo.CompanyID
	Kind         vo.ReceiptKind
	Party        Party
	Date         time.Time
	Description  string
	Lines        []ReceiptLine
	Methods      []Method
	Totals       ReceiptTotals
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// Receipt คือใบรับเงิน — รับชำระหนี้ลูกค้า 1 ราย (หลายใบแจ้งหนี้) หรือรับเงินอื่น (หลายบัญชี), รับได้หลายช่องทาง
//
// กฎ: บรรทัดตามชนิด (customer = ใบแจ้งหนี้ไม่ซ้ำ, other = ระบุบัญชี + ไม่มีภาษีถูกหัก), ยอด > 0,
// รับเงินอื่นต้องระบุชื่อผู้จ่าย, ผลรวมช่องทาง = ยอดรวม - ภาษีถูกหัก
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
	if _, err = vo.ParseReceiptKind(string(s.Kind)); err != nil {
		return nil, err
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if err = normalizeParty(&s.Party); err != nil {
		return nil, err
	}
	if s.Kind == vo.ReceiptOther && s.Party.Name == "" {
		return nil, bad("payer name is required")
	}
	if len(s.Lines) == 0 || len(s.Lines) > MaxLines {
		return nil, bad("must have 1 to %d lines", MaxLines)
	}
	t := ReceiptTotals{Amount: decimal.Zero, Wht: decimal.Zero}
	invoices := map[vo.InvoiceID]bool{}
	for i := range s.Lines {
		l := &s.Lines[i]
		if l.Description, err = vo.OptionalText(l.Description, 500); err != nil {
			return nil, err
		}
		switch {
		case s.Kind == vo.ReceiptCustomer && (l.InvoiceID == nil || l.AccountID != nil):
			return nil, bad("line %d: invoice_id is required (and account_id is not used) for a customer receipt", i+1)
		case s.Kind == vo.ReceiptOther && (l.AccountID == nil || l.InvoiceID != nil):
			return nil, bad("line %d: account_id is required (and invoice_id is not used) for other receipts", i+1)
		case s.Kind == vo.ReceiptOther && !l.Wht.IsZero():
			return nil, bad("line %d: withholding tax applies to customer receipts only", i+1)
		case l.InvoiceID != nil && invoices[*l.InvoiceID]:
			return nil, bad("line %d: invoice appears more than once", i+1)
		case !l.Amount.IsPositive() || !money(l.Amount):
			return nil, bad("line %d: amount must be positive with at most %d decimal places", i+1, amountDecimals)
		case l.Wht.IsNegative() || !money(l.Wht) || l.Wht.GreaterThan(l.Amount):
			return nil, bad("line %d: withholding tax must be zero to amount with at most %d decimal places", i+1, amountDecimals)
		}
		if l.InvoiceID != nil {
			invoices[*l.InvoiceID] = true
		}
		t.Amount, t.Wht = t.Amount.Add(l.Amount), t.Wht.Add(l.Wht)
	}
	if t.Received, err = checkMethods(s.Methods, money, bad); err != nil {
		return nil, err
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

// Journal: Dr เงินสด / ธนาคาร (ตามทะเบียน) + Dr ภาษีถูกหัก ณ ที่จ่าย / Cr ลูกหนี้ (customer) หรือ Cr บัญชีของบรรทัด (other)
func (r *Receipt) Journal(ar, whtReceived vo.GLAccountID) []services.JournalLine {
	j := services.NewJournal()
	for _, m := range r.s.Methods {
		j.Debit(m.GLAccountID, m.Amount)
	}
	j.Debit(whtReceived, r.s.Totals.Wht)
	if r.s.Kind == vo.ReceiptCustomer {
		j.Credit(ar, r.s.Totals.Amount)
	} else {
		for _, l := range r.s.Lines {
			j.Credit(*l.AccountID, l.Amount)
		}
	}
	return j.Lines()
}

func (r *Receipt) Cancel(by vo.UserID, at time.Time) error {
	return cancel(&r.s.Status, &r.s.Cancellation, by, at)
}

// ---------- ใบสำคัญจ่าย ----------

// PaymentLine: จ่ายชำระหนี้ = InvoiceID (ตัดยอดใบแจ้งหนี้ซื้อ, ฐานภาษีตามสัดส่วนก่อน VAT)
// จ่ายเงินอื่น = AccountID (เดบิตบัญชีนั้น, ฐานภาษี = ยอดจ่าย) — ภาษีหัก ณ ที่จ่ายเก็บอัตรา / ฐาน / ยอด ณ วันจ่าย
type PaymentLine struct {
	InvoiceID         *vo.InvoiceID
	InvoiceNo         string
	SupplierInvoiceNo string
	AccountID         *vo.GLAccountID
	Description       string
	Amount            vo.Money
	WhtTypeID         *vo.WhtTypeID
	WhtRate           vo.Money // %
	WhtBase           vo.Money
	Wht               vo.Money
}

type PaymentTotals struct {
	Amount vo.Money // ตัดหนี้ / จ่ายรวม
	Wht    vo.Money // ภาษีหัก ณ ที่จ่ายรวม
	Paid   vo.Money // จ่ายจริง = Amount - Wht
}

type PaymentState struct {
	ID           vo.PaymentID
	CompanyID    vo.CompanyID
	Kind         vo.PaymentKind
	Party        Party
	Date         time.Time
	Description  string
	Lines        []PaymentLine
	Methods      []Method
	Totals       PaymentTotals
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
	// WhtCertificateNo: เลขที่หนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ) — ออกเมื่อมีภาษีหัก ("" = ไม่มี), ยกเลิกใบจ่ายแล้วเลขยังคงอยู่ (หนังสือรับรองเป็นโมฆะ)
	WhtCertificateNo string
}

// Payment คือใบสำคัญจ่าย — จ่ายชำระหนี้ผู้ขาย 1 ราย (หลายใบแจ้งหนี้) หรือจ่ายเงินอื่น (หลายบัญชี), จ่ายได้หลายช่องทาง
//
// กฎ: บรรทัดตามชนิด, ยอด > 0, ภาษีไม่เกินยอด, จ่ายเงินอื่นต้องระบุชื่อผู้รับ,
// มีภาษีหัก ณ ที่จ่ายต้องรู้เลขผู้เสียภาษี + ประเภทผู้รับ (ยื่น ภ.ง.ด.), ผลรวมช่องทาง = ยอดรวม - ภาษีหัก ณ ที่จ่าย
type Payment struct{ s PaymentState }

func NewPayment(s PaymentState, amountDecimals int) (*Payment, error) {
	var err error
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalidPayment}, args...)...)
	}
	money := func(v vo.Money) bool { return numeric.InRange(v) && numeric.FitsScale(v, amountDecimals) }
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if _, err = vo.ParsePaymentKind(string(s.Kind)); err != nil {
		return nil, err
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if err = normalizeParty(&s.Party); err != nil {
		return nil, err
	}
	if s.Kind == vo.PaymentOther && s.Party.Name == "" {
		return nil, bad("payee name is required")
	}
	if len(s.Lines) == 0 || len(s.Lines) > MaxLines {
		return nil, bad("must have 1 to %d lines", MaxLines)
	}
	t := PaymentTotals{Amount: decimal.Zero, Wht: decimal.Zero}
	invoices := map[vo.InvoiceID]bool{}
	for i := range s.Lines {
		l := &s.Lines[i]
		if l.Description, err = vo.OptionalText(l.Description, 500); err != nil {
			return nil, err
		}
		switch {
		case s.Kind == vo.PaymentSupplier && (l.InvoiceID == nil || l.AccountID != nil):
			return nil, bad("line %d: invoice_id is required (and account_id is not used) for a supplier payment", i+1)
		case s.Kind == vo.PaymentOther && (l.AccountID == nil || l.InvoiceID != nil):
			return nil, bad("line %d: account_id is required (and invoice_id is not used) for other payments", i+1)
		case l.InvoiceID != nil && invoices[*l.InvoiceID]:
			return nil, bad("line %d: invoice appears more than once", i+1)
		case !l.Amount.IsPositive() || !money(l.Amount):
			return nil, bad("line %d: amount must be positive with at most %d decimal places", i+1, amountDecimals)
		case l.Wht.IsNegative() || l.Wht.GreaterThan(l.Amount):
			return nil, bad("line %d: withholding tax exceeds the amount", i+1)
		}
		if l.InvoiceID != nil {
			invoices[*l.InvoiceID] = true
		}
		t.Amount, t.Wht = t.Amount.Add(l.Amount), t.Wht.Add(l.Wht)
	}
	if t.Wht.IsPositive() {
		if s.Party.TaxID == "" || s.Party.TaxType == "" {
			return nil, domain.ErrPayeeTaxIDRequired
		}
		if _, err := vo.ParseTaxType(string(s.Party.TaxType)); err != nil {
			return nil, err
		}
	}
	if t.Paid, err = checkMethods(s.Methods, money, bad); err != nil {
		return nil, err
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

// Journal: Dr เจ้าหนี้ (supplier) หรือ Dr บัญชีของบรรทัด (other) / Cr เงินสด / ธนาคาร + Cr ภาษีหัก ณ ที่จ่ายค้างจ่าย
func (p *Payment) Journal(ap, whtPayable vo.GLAccountID) []services.JournalLine {
	j := services.NewJournal()
	if p.s.Kind == vo.PaymentSupplier {
		j.Debit(ap, p.s.Totals.Amount)
	} else {
		for _, l := range p.s.Lines {
			j.Debit(*l.AccountID, l.Amount)
		}
	}
	for _, m := range p.s.Methods {
		j.Credit(m.GLAccountID, m.Amount)
	}
	j.Credit(whtPayable, p.s.Totals.Wht)
	return j.Lines()
}

func (p *Payment) Cancel(by vo.UserID, at time.Time) error {
	return cancel(&p.s.Status, &p.s.Cancellation, by, at)
}

// ---------- โอนเงินระหว่างบัญชี ----------

type TransferState struct {
	ID           vo.TransferID
	CompanyID    vo.CompanyID
	Date         time.Time
	Description  string
	Reference    string
	From         Method // ทะเบียนต้นทาง (Amount = ยอดโอน)
	To           Method // ทะเบียนปลายทาง
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// Transfer คือการโอนเงินระหว่างทะเบียนเงินสด / ธนาคาร (เช่น ฝากเงินสดเข้าธนาคาร, โอนระหว่างบัญชี)
// กฎ: ต้นทางกับปลายทางต่างกัน, ยอด > 0
type Transfer struct{ s TransferState }

func NewTransfer(s TransferState, amountDecimals int) (*Transfer, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if s.Reference, err = vo.OptionalText(s.Reference, 50); err != nil {
		return nil, err
	}
	switch {
	case s.From.CashAccountID == s.To.CashAccountID:
		return nil, fmt.Errorf("%w: source and destination must be different", domain.ErrInvalidTransfer)
	case !s.From.Amount.IsPositive() || !numeric.InRange(s.From.Amount) || !numeric.FitsScale(s.From.Amount, amountDecimals):
		return nil, fmt.Errorf("%w: amount must be positive with at most %d decimal places", domain.ErrInvalidTransfer, amountDecimals)
	}
	s.To.Amount = s.From.Amount
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.TransferID](), vo.DocPosted, nil
	return &Transfer{s: s}, nil
}

func RehydrateTransfer(s TransferState) *Transfer { return &Transfer{s: s} }

func (t *Transfer) ID() vo.TransferID    { return t.s.ID }
func (t *Transfer) State() TransferState { return t.s }
func (t *Transfer) SetEntry(id vo.EntryID, no string) {
	t.s.Posting.EntryID, t.s.Posting.EntryNo = &id, no
}

// Journal: Dr บัญชีปลายทาง / Cr บัญชีต้นทาง (ทะเบียนที่ผูกบัญชีเดียวกัน = ไม่มีรายการ)
func (t *Transfer) Journal() []services.JournalLine {
	j := services.NewJournal()
	j.Debit(t.s.To.GLAccountID, t.s.From.Amount)
	j.Credit(t.s.From.GLAccountID, t.s.From.Amount)
	return j.Lines()
}

func (t *Transfer) Cancel(by vo.UserID, at time.Time) error {
	return cancel(&t.s.Status, &t.s.Cancellation, by, at)
}
