package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/application/ports/repositories"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// FinanceUseCases คือสิ่งที่การเงินเรียก (public API ของ module) — ไม่ตรวจสิทธิ์ของเงินสดย่อย
// (การเงินตรวจสิทธิ์ใบจ่ายเองแล้ว) และทำงานใน transaction ของใบจ่ายของการเงิน
type FinanceUseCases struct {
	base
	reports repositories.ReportRepository
}

// PaidRequest คือผลของการจ่ายคำขอ: กองทุน + บรรทัดเดบิตที่การเงินลงบัญชี (เครดิต = ช่องทางจ่ายของการเงิน)
type PaidRequest struct {
	DocumentNo string
	FundCode   string
	FundName   string
	Kind       vo.RequestKind
	Amount     vo.Money
	Debits     []entities.JournalLine
}

func (uc *FinanceUseCases) Open(ctx context.Context, companyID vo.CompanyID) ([]dto.OpenRequest, error) {
	return uc.requests.Open(ctx, companyID)
}

// Pay: การเงินจ่ายคำขอด้วยใบสำคัญจ่าย doc — lock คำขอ + กองทุน -> paid (+ ใบสำคัญจ่ายเป็นชดเชยแล้ว, + วงเงินเมื่อเพิ่มวงเงิน)
// และคืนบรรทัดเดบิต: ตั้ง / เพิ่มวงเงิน = Dr เงินสดย่อย; เบิกชดเชย = Dr ค่าใช้จ่ายตามบัญชี + Dr ภาษีซื้อ
func (uc *FinanceUseCases) Pay(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID, doc entities.Payment) (PaidRequest, error) {
	var out PaidRequest
	r, err := uc.requests.FindForUpdate(ctx, companyID, id)
	if err != nil {
		return out, err
	}
	st := r.State()
	f, err := uc.funds.FindForUpdate(ctx, companyID, st.FundID)
	if err != nil {
		return out, err
	}
	if st.Kind == vo.RequestEstablish {
		bal, err := uc.funds.Balances(ctx, companyID, []vo.FundID{f.ID()}, nil)
		if err != nil {
			return out, err
		}
		if bal[f.ID()].Funded {
			return out, domain.ErrFundAlreadyFunded
		}
	}
	if doc.Date.Before(st.Date) {
		return out, domain.ErrPaymentBeforeRequest
	}
	if err := r.Pay(doc); err != nil {
		return out, err
	}
	var vouchers []*entities.Voucher
	inputVat := vo.AccountID{}
	if st.Kind == vo.RequestReplenish {
		if vouchers, err = uc.vouchers.ByReplenishmentForUpdate(ctx, companyID, r.ID()); err != nil {
			return out, err
		}
		for _, v := range vouchers {
			if v.State().Totals.Vat.IsPositive() {
				s, err := uc.settings.Get(ctx, companyID)
				if err != nil {
					return out, err
				}
				if inputVat, err = s.RequireInputVat(); err != nil {
					return out, err
				}
				break
			}
		}
		for _, v := range vouchers {
			v.MarkReplenished()
		}
		if err := uc.vouchers.UpdateMany(ctx, vouchers); err != nil {
			return out, err
		}
	}
	if st.Kind == vo.RequestIncrease {
		f.AddImprest(st.Amount)
		if err := uc.funds.Update(ctx, companyID, f); err != nil {
			return out, err
		}
	}
	if err := uc.requests.Update(ctx, r); err != nil {
		return out, err
	}
	fd := f.Data()
	describe := func(part string) string {
		label := map[string]string{"establish": "Petty cash fund", "increase": "Petty cash increase", "expense": "Petty cash", "input_vat": "Input VAT"}[part]
		return label + " " + st.DocumentNo + " / " + fd.Code
	}
	return PaidRequest{DocumentNo: st.DocumentNo, FundCode: fd.Code, FundName: fd.Name, Kind: st.Kind, Amount: st.Amount,
		Debits: r.Debits(fd.GLAccountID, inputVat, vouchers, describe)}, nil
}

// Unpay: การเงินยกเลิกใบจ่าย documentID — คำขอกลับเป็นรอจ่าย, ใบสำคัญจ่ายกลับเป็นอนุมัติแล้ว (ยังอยู่ในคำขอ),
// เพิ่มวงเงินถูกหักคืน; ตั้งวงเงินยกเลิกไม่ได้ถ้ามีใบสำคัญจ่ายอนุมัติแล้ว (เงินคงเหลือจะติดลบ)
func (uc *FinanceUseCases) Unpay(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	r, err := uc.requests.FindByPaymentForUpdate(ctx, companyID, documentID)
	if err != nil {
		return err
	}
	st := r.State()
	f, err := uc.funds.FindForUpdate(ctx, companyID, st.FundID)
	if err != nil {
		return err
	}
	if err := r.Unpay(); err != nil {
		return err
	}
	switch st.Kind {
	case vo.RequestReplenish:
		vouchers, err := uc.vouchers.ByReplenishmentForUpdate(ctx, companyID, r.ID())
		if err != nil {
			return err
		}
		for _, v := range vouchers {
			v.Unreplenish()
		}
		if err := uc.vouchers.UpdateMany(ctx, vouchers); err != nil {
			return err
		}
	case vo.RequestIncrease:
		f.AddImprest(st.Amount.Neg())
		if err := uc.funds.Update(ctx, companyID, f); err != nil {
			return err
		}
	}
	if err := uc.requests.Update(ctx, r); err != nil {
		return err
	}
	bal, err := uc.funds.Balances(ctx, companyID, []vo.FundID{f.ID()}, nil)
	if err != nil {
		return err
	}
	if bal[f.ID()].CashOnHand.IsNegative() {
		return domain.ErrInsufficientCash
	}
	return nil
}

// VatTotals: ภาษีซื้อของใบสำคัญจ่ายที่ชดเชยแล้ว (ตามวันที่ใบกำกับภาษี) — finance ใช้ทำ ภ.พ.30
func (uc *FinanceUseCases) VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error) {
	return uc.reports.VatTotals(ctx, companyID, start, end)
}

// VatRowsLimit: จำนวนแถวสูงสุดของภาษีซื้อเงินสดย่อย 1 เดือนที่ finance พิมพ์ประกอบ ภ.พ.30 (ไม่แบ่งหน้า)
const VatRowsLimit = 20000

// VatRows: แถวภาษีซื้อของใบสำคัญจ่ายที่ชดเชยแล้ว (ชุดเดียวกับ VatTotals / book=input-vat) — finance พิมพ์รายงานภาษีซื้อ
func (uc *FinanceUseCases) VatRows(ctx context.Context, companyID vo.CompanyID, start, end time.Time) ([]dto.InputVatRow, error) {
	rep, err := uc.reports.InputVat(ctx, companyID, dto.InputVatFilter{DateRange: dto.DateRange{Start: start, End: end},
		Page: listing.Page{Number: 1, Size: VatRowsLimit}})
	return rep.Rows.Items, err
}
