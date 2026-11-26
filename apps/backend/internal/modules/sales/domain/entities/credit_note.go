package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

// ---------- ใบลดหนี้ (ม.86/10) ----------

// CreditNoteLine คือบรรทัดของใบลดหนี้ อ้างบรรทัดของใบแจ้งหนี้
//
//	return:    Quantity > 0 ที่ราคาไม่เกินราคาในใบแจ้งหนี้ — สินค้า stock รับคืนเข้าคลังที่ต้นทุนเดิม (Cost)
//	allowance: Quantity = 0, ยอดลด = Amounts (ไม่มีสินค้าคืน)
type CreditNoteLine struct {
	InvoiceLineNo int
	ItemID        vo.ItemID
	Quantity      vo.Quantity
	UnitPrice     vo.Money
	Amounts       services.Amounts
	AccountID     vo.AccountID // บัญชีที่เดบิต: รับคืนสินค้าและส่วนลดจ่าย (ค่าตั้ง) หรือบัญชีรายได้ของบรรทัดเดิม
	CostLine                   // ต้นทุนที่รับคืนเข้าคลัง (return + สินค้า stock)
}

type CreditNoteState struct {
	ID           vo.CreditNoteID
	CompanyID    vo.CompanyID
	InvoiceID    vo.InvoiceID
	InvoiceNo    string // เลขที่ใบกำกับภาษีเดิม (คำอธิบายรายการบัญชี)
	CustomerID   vo.CustomerID
	BillTo       BillTo // ผู้ซื้อตามใบกำกับภาษีเดิม
	Date         time.Time
	Kind         vo.CreditNoteKind
	Reason       string // เหตุที่ลดหนี้ (ต้องพิมพ์ในใบลดหนี้)
	Description  string
	Vat          services.VatTerms // ตามใบแจ้งหนี้เดิม
	VatKind      vo.VatKind        // output = ใบลดหนี้ / ใบกำกับภาษี; undue = ลดภาษีขายยังไม่ถึงกำหนด (ขายบริการที่ยังไม่ออกใบกำกับ)
	Stock        Stock             // ใบรับคืนของ inventory (return + สินค้า stock)
	Lines        []CreditNoteLine
	Totals       services.Amounts
	Status       vo.DocStatus
	Posting      Posting
	Cancellation *Cancellation
}

// CreditNote คือใบลดหนี้ — ลงบัญชีทันที: Dr รับคืน / ส่วนลด + ภาษีขาย / Cr ลูกหนี้
// (perpetual + สินค้าคืนเข้าคลัง: Dr สินค้าคงเหลือ / Cr ต้นทุนขาย ที่ต้นทุนเดิม)
type CreditNote struct{ s CreditNoteState }

func NewCreditNote(s CreditNoteState, invoiceDate time.Time) (*CreditNote, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Date.Before(invoiceDate) {
		return nil, domain.ErrCreditBeforeInvoice
	}
	if s.Reason, err = vo.NewName(s.Reason, 500); err != nil {
		return nil, domain.ErrReasonRequired
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if len(s.Lines) == 0 {
		return nil, fmt.Errorf("%w: at least one line", domain.ErrInvalidLine)
	}
	s.Totals = services.ZeroAmounts()
	for i, l := range s.Lines {
		if !l.Amounts.Gross.IsPositive() {
			return nil, fmt.Errorf("%w: line %d amount must be greater than 0", domain.ErrInvalidAmount, i+1)
		}
		s.Totals = s.Totals.Add(l.Amounts)
	}
	if s.VatKind == "" {
		s.VatKind = vo.VatOutput
	}
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.CreditNoteID](), vo.DocPosted, nil
	return &CreditNote{s: s}, nil
}

func RehydrateCreditNote(s CreditNoteState) *CreditNote { return &CreditNote{s: s} }

func (c *CreditNote) ID() vo.CreditNoteID    { return c.s.ID }
func (c *CreditNote) State() CreditNoteState { return c.s }
func (c *CreditNote) SetEntry(id vo.EntryID, no string) {
	c.s.Posting.EntryID, c.s.Posting.EntryNo = &id, no
}

// Journal: Dr บัญชีของบรรทัด (ก่อน VAT) + Dr ภาษีขาย (หรือภาษีขายยังไม่ถึงกำหนด ตาม VatKind — ผู้เรียกส่งบัญชีมา) / Cr ลูกหนี้ (รวม VAT) [+ Dr สินค้าคงเหลือ / Cr ต้นทุนขาย]
func (c *CreditNote) Journal(ar, outputVat, cogs vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	costs := make([]CostLine, 0, len(c.s.Lines))
	for _, l := range c.s.Lines {
		j.Debit(l.AccountID, l.Amounts.Net)
		costs = append(costs, l.CostLine)
	}
	j.Debit(outputVat, c.s.Totals.Vat)
	j.Credit(ar, c.s.Totals.Gross)
	if c.s.Stock.System == vo.Perpetual {
		for _, l := range costs {
			if l.Stock && l.InventoryAccountID != nil {
				j.Debit(*l.InventoryAccountID, l.Cost)
				j.Credit(cogs, l.Cost)
			}
		}
	}
	return j.Lines()
}

func (c *CreditNote) Cancel(by vo.UserID, at time.Time) error {
	return cancel(&c.s.Status, &c.s.Cancellation, by, at)
}

// ---------- กฎของบรรทัดใบลดหนี้เทียบกับใบแจ้งหนี้ ----------

// Credited คือยอดที่ลดหนี้ไปแล้วของบรรทัดใบแจ้งหนี้ (ใบลดหนี้ที่ยังไม่ยกเลิก)
type Credited struct {
	Returned vo.Quantity // จำนวนที่รับคืนแล้ว
	Net      vo.Money    // ยอดก่อน VAT ที่ลดแล้ว
}

// CreditLine: บรรทัดที่ lineNo ของใบแจ้งหนี้ (เริ่ม 1)
func (i *Invoice) CreditLine(lineNo int) (InvoiceLine, error) {
	if lineNo < 1 || lineNo > len(i.s.Lines) {
		return InvoiceLine{}, fmt.Errorf("%w: invoice line %d", domain.ErrInvalidLine, lineNo)
	}
	return i.s.Lines[lineNo-1], nil
}

// CreditAmounts: ยอดลดหนี้ของบรรทัด ตามชนิดใบลดหนี้ + ตรวจไม่เกินที่เหลือของบรรทัดใบแจ้งหนี้
//
//	return:    qty ต้อง > 0 และไม่เกิน (จำนวนในใบ - รับคืนแล้ว); ราคา > 0 และไม่เกินราคาในใบ
//	allowance: amount > 0 (ใช้หลัก VAT เดียวกับราคาในใบ)
//
// ทุกชนิด: ยอดก่อน VAT ที่ลดสะสม ไม่เกินยอดก่อน VAT ของบรรทัด
func (i *Invoice) CreditAmounts(kind vo.CreditNoteKind, lineNo int, qty, price, amount vo.Money, done Credited, amountDecimals int) (services.Amounts, error) {
	l, err := i.CreditLine(lineNo)
	if err != nil {
		return services.Amounts{}, err
	}
	var a services.Amounts
	switch kind {
	case vo.CreditReturn:
		if !qty.IsPositive() || qty.Add(done.Returned).GreaterThan(l.Quantity) {
			return a, fmt.Errorf("%w: invoice line %d (invoiced %s, returned %s)", domain.ErrOverReturn, lineNo, l.Quantity, done.Returned)
		}
		if !price.IsPositive() || price.GreaterThan(l.UnitPrice) {
			return a, fmt.Errorf("%w: invoice line %d (invoice price %s)", domain.ErrCreditPrice, lineNo, l.UnitPrice)
		}
		a = services.LineAmounts(qty, price, i.s.Vat, amountDecimals)
	default:
		if !amount.IsPositive() {
			return a, fmt.Errorf("%w: invoice line %d amount must be greater than 0", domain.ErrInvalidAmount, lineNo)
		}
		a = services.LineAmounts(decimal.NewFromInt(1), amount, i.s.Vat, amountDecimals)
	}
	if a.Net.Add(done.Net).GreaterThan(l.Amounts.Net) {
		return a, fmt.Errorf("%w: invoice line %d (net %s, credited %s)", domain.ErrOverCredit, lineNo, l.Amounts.Net, done.Net)
	}
	return a, nil
}
