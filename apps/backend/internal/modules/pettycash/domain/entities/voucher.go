package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

// MaxVoucherLines คือจำนวนบรรทัดสูงสุดต่อใบสำคัญจ่าย
const MaxVoucherLines = 50

var hundred = decimal.NewFromInt(100)

// Payee คือผู้รับเงิน ณ วันเอกสาร (ร้านค้า / บุคคล) — เลขผู้เสียภาษีใช้ในรายงานภาษีซื้อ
type Payee struct {
	Name     string
	TaxID    string
	BranchNo string
}

// TaxInvoice คือใบกำกับภาษีเต็มรูปที่ได้รับ (บังคับเมื่อมี VAT — ไม่มีใบกำกับขอคืนภาษีซื้อไม่ได้)
type TaxInvoice struct {
	No   string
	Date time.Time
}

// VoucherLine: Amount = ยอดที่กรอก (รวม VAT ถ้า PriceIncludesVat), Net = ยอดก่อน VAT ที่ลงบัญชีค่าใช้จ่าย
type VoucherLine struct {
	AccountID   vo.AccountID
	Description string
	Amount      vo.Money
	Net         vo.Money
}

type Totals struct {
	Net   vo.Money
	Vat   vo.Money
	Gross vo.Money
}

type VoucherData struct {
	FundID           vo.FundID
	Date             time.Time
	Payee            Payee
	Description      string
	Lines            []VoucherLine
	VatRate          vo.Money // 0 = ไม่มี VAT
	PriceIncludesVat bool
	TaxInvoice       *TaxInvoice
}

type VoucherState struct {
	ID         vo.VoucherID
	CompanyID  vo.CompanyID
	DocumentNo string
	VoucherData
	Totals          Totals
	Status          vo.VoucherStatus
	ReplenishmentID *vo.ReplenishmentID // ใบขอเบิกชดเชยที่ยังไม่ยกเลิก (nil = ยังไม่ได้ขอ)
	CreatedBy       vo.UserID
	ApprovedBy      *vo.UserID
	ApprovedAt      *time.Time
	CancelledAt     *time.Time
}

// Voucher คือใบสำคัญจ่ายเงินสดย่อย
//
// กฎ: บรรทัด 1-50, ยอด > 0, ทศนิยมตามบริษัท, ไม่ซ้ำบัญชีไม่บังคับ; มี VAT (อัตรา > 0) ต้องมีเลขที่ / วันที่ใบกำกับภาษี;
// VAT คิดจากยอดรวมทั้งใบ (ราคารวม VAT: ถอด VAT ออก แล้วปันยอดก่อน VAT ให้บรรทัดตามสัดส่วน เศษไว้บรรทัดสุดท้าย)
type Voucher struct{ s VoucherState }

func NewVoucher(companyID vo.CompanyID, no string, d VoucherData, by vo.UserID, amountDecimals int) (*Voucher, error) {
	n, totals, err := normalizeVoucher(d, amountDecimals)
	if err != nil {
		return nil, err
	}
	return &Voucher{s: VoucherState{ID: vo.NewID[vo.VoucherID](), CompanyID: companyID, DocumentNo: no, VoucherData: n,
		Totals: totals, Status: vo.VoucherDraft, CreatedBy: by}}, nil
}

func RehydrateVoucher(s VoucherState) *Voucher { return &Voucher{s: s} }

func (v *Voucher) ID() vo.VoucherID    { return v.s.ID }
func (v *Voucher) State() VoucherState { return v.s }

// Update แก้ร่าง (กองทุนเปลี่ยนได้)
func (v *Voucher) Update(d VoucherData, amountDecimals int) error {
	if v.s.Status != vo.VoucherDraft {
		return domain.ErrVoucherNotDraft
	}
	n, totals, err := normalizeVoucher(d, amountDecimals)
	if err != nil {
		return err
	}
	v.s.VoucherData, v.s.Totals = n, totals
	return nil
}

// Approve: ร่าง -> อนุมัติ (เงินออกจากกองทุน) — use case ตรวจเงินคงเหลือของกองทุนก่อน
func (v *Voucher) Approve(by vo.UserID, at time.Time) error {
	if v.s.Status != vo.VoucherDraft {
		return domain.ErrVoucherNotDraft
	}
	v.s.Status, v.s.ApprovedBy, v.s.ApprovedAt = vo.VoucherApproved, &by, &at
	return nil
}

// Cancel: ร่าง / อนุมัติแล้ว (ยังไม่อยู่ในคำขอเบิกชดเชย) -> ยกเลิก
func (v *Voucher) Cancel(at time.Time) error {
	switch {
	case v.s.Status == vo.VoucherCancelled:
		return domain.ErrAlreadyCancelled
	case v.s.Status == vo.VoucherReplenished:
		return domain.ErrVoucherReplenished
	case v.s.ReplenishmentID != nil:
		return fmt.Errorf("%w: %s", domain.ErrVoucherInRequest, v.s.DocumentNo)
	}
	v.s.Status, v.s.CancelledAt = vo.VoucherCancelled, &at
	return nil
}

// AttachTo: ใส่ในคำขอเบิกชดเชย (ต้องอนุมัติแล้ว, กองทุนเดียวกัน, ยังไม่อยู่ในคำขออื่น)
func (v *Voucher) AttachTo(fundID vo.FundID, id vo.ReplenishmentID) error {
	switch {
	case v.s.Status != vo.VoucherApproved:
		return fmt.Errorf("%w: %s", domain.ErrVoucherNotApproved, v.s.DocumentNo)
	case v.s.FundID != fundID:
		return fmt.Errorf("%w: %s", domain.ErrVoucherOtherFund, v.s.DocumentNo)
	case v.s.ReplenishmentID != nil:
		return fmt.Errorf("%w: %s", domain.ErrVoucherInRequest, v.s.DocumentNo)
	}
	v.s.ReplenishmentID = &id
	return nil
}

// Detach: คำขอถูกยกเลิก — กลับเป็นอนุมัติแล้วที่ยังไม่ได้ขอ
func (v *Voucher) Detach() {
	v.s.ReplenishmentID = nil
	if v.s.Status == vo.VoucherReplenished {
		v.s.Status = vo.VoucherApproved
	}
}

// MarkReplenished: การเงินจ่ายชดเชยแล้ว; Unreplenish: การเงินยกเลิกใบจ่าย (ยังอยู่ในคำขอเดิม)
func (v *Voucher) MarkReplenished() { v.s.Status = vo.VoucherReplenished }
func (v *Voucher) Unreplenish()     { v.s.Status = vo.VoucherApproved }

func normalizeVoucher(d VoucherData, dec int) (VoucherData, Totals, error) {
	var err error
	var t Totals
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{domain.ErrInvalidLine}, args...)...)
	}
	if !vo.IsDateOnly(d.Date) {
		return d, t, domain.ErrInvalidDate
	}
	if d.Payee.Name, err = vo.NewName(d.Payee.Name, 200); err != nil {
		return d, t, err
	}
	if d.Payee.TaxID, err = vo.OptionalText(d.Payee.TaxID, 20); err != nil {
		return d, t, err
	}
	if d.Payee.BranchNo, err = vo.OptionalText(d.Payee.BranchNo, 5); err != nil {
		return d, t, err
	}
	if d.Description, err = vo.OptionalText(d.Description, 500); err != nil {
		return d, t, err
	}
	if d.VatRate.IsNegative() || d.VatRate.GreaterThan(hundred) || !vo.FitsCompany(d.VatRate, 2) {
		return d, t, fmt.Errorf("%w: vat_rate must be 0 to 100", domain.ErrInvalidVatRate)
	}
	if len(d.Lines) == 0 || len(d.Lines) > MaxVoucherLines {
		return d, t, bad("must have 1 to %d lines", MaxVoucherLines)
	}
	sum := decimal.Zero
	for i := range d.Lines {
		l := &d.Lines[i]
		if l.Description, err = vo.OptionalText(l.Description, 300); err != nil {
			return d, t, err
		}
		if !l.Amount.IsPositive() || !vo.FitsCompany(l.Amount, dec) {
			return d, t, bad("line %d: amount must be positive with at most %d decimal places", i+1, dec)
		}
		sum = sum.Add(l.Amount)
	}
	t = VoucherTotals(sum, d.VatRate, d.PriceIncludesVat, dec)
	allocateNet(d.Lines, t.Net, dec)
	if t.Vat.IsPositive() {
		if d.TaxInvoice == nil || !vo.IsDateOnly(d.TaxInvoice.Date) {
			return d, t, domain.ErrTaxInvoiceRequired
		}
		if d.TaxInvoice.No, err = vo.NewName(d.TaxInvoice.No, 50); err != nil {
			return d, t, domain.ErrTaxInvoiceRequired
		}
	} else {
		d.TaxInvoice = nil
	}
	return d, t, nil
}

// VoucherTotals: ยอดก่อน VAT / VAT / รวม จากผลรวมยอดที่กรอก (VAT ปัดตามทศนิยมบริษัท)
func VoucherTotals(sum, rate vo.Money, includesVat bool, dec int) Totals {
	if !rate.IsPositive() {
		return Totals{Net: sum, Vat: decimal.Zero, Gross: sum}
	}
	if includesVat {
		vat := sum.Mul(rate).Div(hundred.Add(rate)).Round(int32(dec))
		return Totals{Net: sum.Sub(vat), Vat: vat, Gross: sum}
	}
	vat := sum.Mul(rate).Div(hundred).Round(int32(dec))
	return Totals{Net: sum, Vat: vat, Gross: sum.Add(vat)}
}

// allocateNet: ยอดก่อน VAT ของแต่ละบรรทัดตามสัดส่วนยอดที่กรอก — เศษไว้บรรทัดสุดท้าย (ผลรวม = net พอดี)
func allocateNet(lines []VoucherLine, net vo.Money, dec int) {
	sum := decimal.Zero
	for _, l := range lines {
		sum = sum.Add(l.Amount)
	}
	if sum.Equal(net) {
		for i := range lines {
			lines[i].Net = lines[i].Amount
		}
		return
	}
	left := net
	for i := range lines {
		if i == len(lines)-1 {
			lines[i].Net = left
			break
		}
		part := net.Mul(lines[i].Amount).Div(sum).Round(int32(dec))
		lines[i].Net, left = part, left.Sub(part)
	}
}
