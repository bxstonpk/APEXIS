package entities

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

// MaxRequestVouchers คือจำนวนใบสำคัญจ่ายสูงสุดต่อคำขอเบิกชดเชย
const MaxRequestVouchers = 200

// Payment คือใบสำคัญจ่ายของการเงินที่จ่ายคำขอนี้
type Payment struct {
	DocumentID uuid.UUID
	DocumentNo string
	Date       time.Time
}

type ReplenishmentState struct {
	ID          vo.ReplenishmentID
	CompanyID   vo.CompanyID
	DocumentNo  string
	FundID      vo.FundID
	Kind        vo.RequestKind
	Date        time.Time
	Description string
	Amount      vo.Money // establish / increase = วงเงิน; replenish = ผลรวมยอดรวม (รวม VAT) ของใบสำคัญจ่าย
	VoucherIDs  []vo.VoucherID
	Status      vo.RequestStatus
	Payment     *Payment
	CreatedBy   vo.UserID
	CreatedAt   time.Time
	CancelledAt *time.Time
}

// Replenishment คือใบขอเบิกชดเชย / ขอตั้งวงเงินเงินสดย่อย — การเงินยืนยันจ่ายเงินแล้วเปลี่ยนเป็น paid
type Replenishment struct{ s ReplenishmentState }

// NewFundingRequest: ขอตั้งวงเงิน (establish) / เพิ่มวงเงิน (increase) — ยอด > 0
func NewFundingRequest(s ReplenishmentState, amountDecimals int) (*Replenishment, error) {
	if s.Kind != vo.RequestEstablish && s.Kind != vo.RequestIncrease {
		return nil, domain.ErrInvalidKind
	}
	if !s.Amount.IsPositive() || !vo.FitsCompany(s.Amount, amountDecimals) {
		return nil, fmt.Errorf("%w: amount must be positive with at most %d decimal places", domain.ErrInvalidAmount, amountDecimals)
	}
	s.VoucherIDs = nil
	return newRequest(s)
}

// NewReplenishRequest: เบิกชดเชยตามใบสำคัญจ่ายที่อนุมัติแล้ว (ยอด = ผลรวมยอดรวมของใบสำคัญจ่าย)
func NewReplenishRequest(s ReplenishmentState, vouchers []*Voucher) (*Replenishment, error) {
	if len(vouchers) == 0 || len(vouchers) > MaxRequestVouchers {
		return nil, fmt.Errorf("%w: select 1 to %d approved vouchers", domain.ErrInvalidLine, MaxRequestVouchers)
	}
	s.Kind, s.Amount, s.VoucherIDs = vo.RequestReplenish, decimal.Zero, make([]vo.VoucherID, len(vouchers))
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	s.ID = vo.NewID[vo.ReplenishmentID]()
	for i, v := range vouchers {
		if v.State().Date.After(s.Date) {
			return nil, domain.ErrDateBeforeVouchers
		}
		if err := v.AttachTo(s.FundID, s.ID); err != nil {
			return nil, err
		}
		s.VoucherIDs[i], s.Amount = v.ID(), s.Amount.Add(v.State().Totals.Gross)
	}
	return newRequest(s)
}

func newRequest(s ReplenishmentState) (*Replenishment, error) {
	var err error
	if !vo.IsDateOnly(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if s.ID == (vo.ReplenishmentID{}) {
		s.ID = vo.NewID[vo.ReplenishmentID]()
	}
	s.Status, s.Payment, s.CancelledAt = vo.RequestRequested, nil, nil
	return &Replenishment{s: s}, nil
}

func RehydrateReplenishment(s ReplenishmentState) *Replenishment { return &Replenishment{s: s} }

func (r *Replenishment) ID() vo.ReplenishmentID    { return r.s.ID }
func (r *Replenishment) State() ReplenishmentState { return r.s }

// Cancel: ยกเลิกคำขอที่การเงินยังไม่จ่าย
func (r *Replenishment) Cancel(at time.Time) error {
	switch r.s.Status {
	case vo.RequestCancelled:
		return domain.ErrAlreadyCancelled
	case vo.RequestPaid:
		return domain.ErrRequestPaid
	}
	r.s.Status, r.s.CancelledAt = vo.RequestCancelled, &at
	return nil
}

// Pay: การเงินจ่ายเงินแล้ว (ใบสำคัญจ่ายของการเงิน)
func (r *Replenishment) Pay(p Payment) error {
	if r.s.Status != vo.RequestRequested {
		return domain.ErrRequestNotRequested
	}
	r.s.Status, r.s.Payment = vo.RequestPaid, &p
	return nil
}

// Unpay: การเงินยกเลิกใบจ่าย — คำขอกลับมาเปิดให้จ่ายใหม่
func (r *Replenishment) Unpay() error {
	if r.s.Status != vo.RequestPaid {
		return domain.ErrRequestNotRequested
	}
	r.s.Status, r.s.Payment = vo.RequestRequested, nil
	return nil
}

// JournalLine คือบรรทัดเดบิตที่การเงินลงเมื่อจ่ายคำขอ (เครดิต = เงินสด / ธนาคารตามช่องทางจ่ายของการเงิน)
type JournalLine struct {
	AccountID   vo.AccountID
	Description string
	Amount      vo.Money
}

// Debits คือบรรทัดเดบิตของการจ่ายคำขอ:
//   - ตั้ง / เพิ่มวงเงิน: Dr บัญชีเงินสดย่อยของกองทุน
//   - เบิกชดเชย: Dr บัญชีของบรรทัดใบสำคัญจ่าย (ยอดก่อน VAT รวมตามบัญชี) + Dr ภาษีซื้อ (ใบที่มีใบกำกับภาษี)
//
// เรียงตามลำดับที่พบในใบสำคัญจ่าย — ภาษีซื้ออยู่ท้ายสุด
func (r *Replenishment) Debits(fundAccount, inputVat vo.AccountID, vouchers []*Voucher, describe func(string) string) []JournalLine {
	if r.s.Kind != vo.RequestReplenish {
		return []JournalLine{{AccountID: fundAccount, Description: describe(string(r.s.Kind)), Amount: r.s.Amount}}
	}
	sums := map[vo.AccountID]vo.Money{}
	var order []vo.AccountID
	vat := decimal.Zero
	for _, v := range vouchers {
		st := v.State()
		for _, l := range st.Lines {
			if _, ok := sums[l.AccountID]; !ok {
				order = append(order, l.AccountID)
				sums[l.AccountID] = decimal.Zero
			}
			sums[l.AccountID] = sums[l.AccountID].Add(l.Net)
		}
		vat = vat.Add(st.Totals.Vat)
	}
	out := make([]JournalLine, 0, len(order)+1)
	for _, id := range order {
		if sums[id].IsPositive() {
			out = append(out, JournalLine{AccountID: id, Description: describe("expense"), Amount: sums[id]})
		}
	}
	if vat.IsPositive() {
		out = append(out, JournalLine{AccountID: inputVat, Description: describe("input_vat"), Amount: vat})
	}
	return out
}
