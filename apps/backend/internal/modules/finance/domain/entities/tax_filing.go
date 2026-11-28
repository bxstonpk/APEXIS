package entities

import (
	"fmt"
	"time"

	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

// TaxFilingData คือข้อมูลการยื่นแบบภาษี 1 แบบ / 1 เดือน (บันทึกการยื่น — ไม่ใช่การจ่ายเงิน)
//
//   - Amount: ภาษีที่ยื่นชำระตามแบบ (>= 0)
//   - Surcharge / Penalty: เงินเพิ่ม / เบี้ยปรับ (ยื่นเกินกำหนด, >= 0)
//   - CarryForward: ภาษีชำระเกินที่ยกไปเดือนถัดไป (ภ.พ.30 เท่านั้น, >= 0) — เดือนถัดไปอ่านเป็น "ภาษีชำระเกินยกมา"
//   - Reference: เลขที่ใบเสร็จ / เลขอ้างอิงการยื่นของกรมสรรพากร
//   - PaymentID: ใบจ่ายของการเงินที่จ่ายภาษีนี้ (ไม่บังคับ — การจ่ายเงินทำที่หน้าจ่ายเงิน)
type TaxFilingData struct {
	Form         vo.TaxForm
	Month        vo.TaxMonth
	FiledDate    time.Time
	Amount       vo.Money
	Surcharge    vo.Money
	Penalty      vo.Money
	CarryForward vo.Money
	Reference    string
	Note         string
	PaymentID    *vo.PaymentID
}

type TaxFilingState struct {
	ID          vo.TaxFilingID
	CompanyID   vo.CompanyID
	Data        TaxFilingData
	Status      vo.TaxFilingStatus
	CreatedBy   vo.UserID
	CreatedAt   time.Time
	CancelledBy *vo.UserID
	CancelledAt *time.Time
}

// TaxFiling: 1 แบบ / 1 เดือนมีบันทึกที่ยังไม่ยกเลิกได้ 1 รายการ (unique index ในฐานข้อมูลเป็นตัวกันสุดท้าย)
type TaxFiling struct{ s TaxFilingState }

func NewTaxFiling(companyID vo.CompanyID, d TaxFilingData, by vo.UserID, at time.Time, amountDecimals int) (*TaxFiling, error) {
	if !vo.IsDateOnly(d.FiledDate) {
		return nil, domain.ErrInvalidDate
	}
	if d.Month.IsZero() {
		return nil, domain.ErrInvalidMonth
	}
	if d.FiledDate.Before(d.Month.Start()) {
		return nil, fmt.Errorf("%w: filed_date must not be before the tax month", domain.ErrInvalidTaxFiling)
	}
	for name, v := range map[string]*vo.Money{"amount": &d.Amount, "surcharge": &d.Surcharge, "penalty": &d.Penalty, "carry_forward": &d.CarryForward} {
		if v.IsNegative() || !v.Equal(v.Round(int32(amountDecimals))) {
			return nil, fmt.Errorf("%w: %s must be zero or more with at most %d decimal places", domain.ErrInvalidTaxFiling, name, amountDecimals)
		}
	}
	if d.Form != vo.FormPp30 && d.CarryForward.IsPositive() {
		return nil, fmt.Errorf("%w: carry_forward is for pp30 only", domain.ErrInvalidTaxFiling)
	}
	if d.Form == vo.FormPp30 && d.Amount.IsPositive() && d.CarryForward.IsPositive() {
		return nil, fmt.Errorf("%w: pp30 has either a tax amount or an overpaid amount carried forward", domain.ErrInvalidTaxFiling)
	}
	var err error
	if d.Reference, err = vo.OptionalText(d.Reference, 100); err != nil {
		return nil, err
	}
	if d.Note, err = vo.OptionalText(d.Note, 500); err != nil {
		return nil, err
	}
	return &TaxFiling{s: TaxFilingState{ID: vo.NewID[vo.TaxFilingID](), CompanyID: companyID, Data: d, Status: vo.FilingFiled,
		CreatedBy: by, CreatedAt: at}}, nil
}

func RehydrateTaxFiling(s TaxFilingState) *TaxFiling { return &TaxFiling{s: s} }

func (f *TaxFiling) ID() vo.TaxFilingID    { return f.s.ID }
func (f *TaxFiling) State() TaxFilingState { return f.s }

// Total: ภาษี + เงินเพิ่ม + เบี้ยปรับ
func (f *TaxFiling) Total() vo.Money {
	return f.s.Data.Amount.Add(f.s.Data.Surcharge).Add(f.s.Data.Penalty)
}

// Cancel: ยกเลิกบันทึก (เช่น บันทึกผิด / ยื่นเพิ่มเติม) — ยื่นใหม่ของเดือนเดิมได้
func (f *TaxFiling) Cancel(by vo.UserID, at time.Time) error {
	if f.s.Status == vo.FilingCancelled {
		return domain.ErrAlreadyCancelled
	}
	f.s.Status, f.s.CancelledBy, f.s.CancelledAt = vo.FilingCancelled, &by, &at
	return nil
}
