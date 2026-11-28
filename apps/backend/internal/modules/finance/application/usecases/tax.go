package usecases

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/application/ports"
	"apexis/backend/internal/modules/finance/application/ports/repositories"
	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

// TaxUseCases: แบบยื่นภาษีรายเดือน (ภ.ง.ด.3 / 53 / ภ.พ.30) และทะเบียนการยื่น
//
//   - ดูแบบยื่น / พิมพ์ / ส่งออกไฟล์ = finance.report:read (อ่านอย่างเดียว ไม่ใช้ transaction)
//   - ทะเบียนการยื่น = finance.tax_filing:read / manage — บันทึกการยื่นไม่ใช่การจ่ายเงิน (จ่ายภาษีทำที่หน้าจ่ายเงิน)
//   - ผู้ที่ไม่มีสิทธิ์ทะเบียนการยื่นยังดูแบบยื่นได้ (ข้อมูลการยื่นเป็น nil)
type TaxUseCases struct {
	base
	reports  repositories.ReportRepository
	filings  repositories.TaxFilingRepository
	payments repositories.PaymentRepository
	vat      ports.Vat
}

// canSeeFilings: มีสิทธิ์ดูทะเบียนการยื่นไหม — ไม่มีสิทธิ์ไม่ใช่ error (แค่ไม่แสดง)
func (uc *TaxUseCases) canSeeFilings(ctx context.Context, a dto.Actor) (bool, error) {
	err := uc.authorize(ctx, a, vo.PermTaxFilingRead)
	if errors.Is(err, domain.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}

func (uc *TaxUseCases) activeFiling(ctx context.Context, a dto.Actor, see bool, form vo.TaxForm, m vo.TaxMonth) (*dto.TaxFiling, error) {
	if !see {
		return nil, nil
	}
	f, err := uc.filings.Active(ctx, a.CompanyID, form, m)
	if err != nil || f == nil {
		return nil, err
	}
	d := filingDTO(f)
	return &d, nil
}

// WhtForm: ภ.ง.ด.3 / 53 ของเดือน — ใบแนบทุกบรรทัด + สรุป (จำนวนราย = จำนวนใบจ่าย) + บันทึกการยื่น
// query: บรรทัดใบแนบ 1 + สิทธิ์ทะเบียน 1 + บันทึกการยื่น 1
func (uc *TaxUseCases) WhtForm(ctx context.Context, a dto.Actor, form vo.TaxForm, m vo.TaxMonth) (dto.WhtForm, error) {
	if !form.IsWithholding() {
		return dto.WhtForm{}, domain.ErrInvalidTaxForm
	}
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.WhtForm, error) {
		lines, err := uc.reports.WhtFormLines(ctx, a.CompanyID, form.PayeeTaxType(), m.Start(), m.End())
		if err != nil {
			return dto.WhtForm{}, err
		}
		out := dto.WhtForm{Form: form, Month: m, Lines: lines, Summary: whtSummary(lines)}
		see, err := uc.canSeeFilings(ctx, a)
		if err != nil {
			return dto.WhtForm{}, err
		}
		out.Filing, err = uc.activeFiling(ctx, a, see, form, m)
		return out, err
	})
}

func whtSummary(lines []dto.WhtFormLine) dto.WhtFormSummary {
	s := dto.WhtFormSummary{Lines: len(lines), Base: decimal.Zero, Wht: decimal.Zero}
	for _, l := range lines {
		s.Base, s.Wht = s.Base.Add(l.Base), s.Wht.Add(l.Wht)
		if l.Seq > s.Payees {
			s.Payees = l.Seq
		}
	}
	return s
}

// Pp30: ภ.พ.30 ของเดือน — ภาษีขาย (sales) - ภาษีซื้อ (จัดซื้อ + เงินสดย่อย) - ภาษีชำระเกินยกมา (บันทึกการยื่นเดือนก่อน)
func (uc *TaxUseCases) Pp30(ctx context.Context, a dto.Actor, m vo.TaxMonth) (dto.Pp30, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.Pp30, error) {
		sales, err := uc.vat.OutputVat(ctx, a.CompanyID, m.Start(), m.End())
		if err != nil {
			return dto.Pp30{}, err
		}
		purchases, err := uc.vat.InputVat(ctx, a.CompanyID, m.Start(), m.End())
		if err != nil {
			return dto.Pp30{}, err
		}
		// ภาษีชำระเกินยกมาเป็นส่วนหนึ่งของแบบ — อ่านเสมอ (ไม่ขึ้นกับสิทธิ์ดูทะเบียนการยื่น)
		prev, err := uc.filings.Active(ctx, a.CompanyID, vo.FormPp30, m.Prev())
		if err != nil {
			return dto.Pp30{}, err
		}
		out := pp30(m, sales, purchases, prev)
		if out.CanSeeFiles, err = uc.canSeeFilings(ctx, a); err != nil {
			return dto.Pp30{}, err
		}
		out.Filing, err = uc.activeFiling(ctx, a, out.CanSeeFiles, vo.FormPp30, m)
		return out, err
	})
}

func pp30(m vo.TaxMonth, sales, purchases ports.VatTotals, prev *entities.TaxFiling) dto.Pp30 {
	carryIn := decimal.Zero
	if prev != nil {
		carryIn = prev.State().Data.CarryForward
	}
	net := sales.Tax.Sub(purchases.Tax).Sub(carryIn)
	out := dto.Pp30{Month: m, Sales: dto.VatAmounts{Base: sales.Base, Tax: sales.Tax, Documents: sales.Documents},
		Purchases: dto.VatAmounts{Base: purchases.Base, Tax: purchases.Tax, Documents: purchases.Documents},
		ZeroRated: decimal.Zero, Exempt: decimal.Zero, CarryIn: carryIn, Net: net, Payable: decimal.Zero, Overpaid: decimal.Zero, PrevFiled: prev != nil}
	if net.IsPositive() {
		out.Payable = net
	} else {
		out.Overpaid = net.Neg()
	}
	return out
}

// VatReport: รายงานภาษีขาย (output) / ภาษีซื้อ (input) ของเดือนตามแบบกรมสรรพากร — ลำดับที่ + ยอดรวม
func (uc *TaxUseCases) VatReport(ctx context.Context, a dto.Actor, kind string, m vo.TaxMonth) (dto.VatReport, error) {
	if kind != "output" && kind != "input" {
		return dto.VatReport{}, domain.ErrInvalidKind
	}
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.VatReport, error) {
		var (
			src []ports.VatRow
			err error
		)
		if kind == "output" {
			src, err = uc.vat.OutputRows(ctx, a.CompanyID, m.Start(), m.End())
		} else {
			src, err = uc.vat.InputRows(ctx, a.CompanyID, m.Start(), m.End())
		}
		if err != nil {
			return dto.VatReport{}, err
		}
		out := dto.VatReport{Kind: kind, Month: m, Rows: make([]dto.VatReportRow, len(src)), Base: decimal.Zero, Tax: decimal.Zero}
		for i, r := range src {
			out.Rows[i] = dto.VatReportRow{Seq: i + 1, Date: r.Date, DocumentNo: r.DocumentNo, PartyName: r.PartyName, TaxID: r.TaxID,
				BranchNo: r.BranchNo, Base: r.Base, Tax: r.Tax}
			out.Base, out.Tax = out.Base.Add(r.Base), out.Tax.Add(r.Tax)
		}
		return out, nil
	})
}

// Overview: ตารางรายเดือนของปี — ยอดภาษีของแต่ละแบบ + บันทึกการยื่น
//
// query: ภาษีหัก ณ ที่จ่ายทั้งปี 1 (GROUP BY เดือน), ทะเบียนการยื่นของปี 1 + ธ.ค. ปีก่อน 1,
// ภ.พ.30 = ยอดภาษีขาย / ซื้อต่อเดือน (เดือนละ 3 query ของ sales / purchasing / เงินสดย่อย) — แยก query ต่อเดือนเพราะยอดอยู่คนละ module
// (ไม่อ่านตารางข้าม module) และคำนวณเฉพาะเดือนที่ถึงแล้ว (ไม่เกิน 12 เดือน)
func (uc *TaxUseCases) Overview(ctx context.Context, a dto.Actor, year int) (dto.TaxOverview, error) {
	if year < 2000 || year > 2200 {
		return dto.TaxOverview{}, domain.ErrInvalidMonth
	}
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.TaxOverview, error) {
		jan := vo.TaxMonthOf(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC))
		last := 12
		now := uc.now()
		switch {
		case year > now.Year():
			last = 0
		case year == now.Year():
			last = int(now.Month())
		}
		out := dto.TaxOverview{Year: year, Months: make([]dto.TaxOverviewMonth, 12)}
		var err error
		if out.CanSeeFiles, err = uc.canSeeFilings(ctx, a); err != nil {
			return dto.TaxOverview{}, err
		}
		// บันทึกการยื่นของปี (ใช้ภาษีชำระเกินยกมาของ ภ.พ.30 ด้วย — อ่านเสมอ) + ธ.ค. ปีก่อน
		filings, err := uc.filings.List(ctx, a.CompanyID, dto.TaxFilingFilter{Year: year, Status: ptr(vo.FilingFiled)})
		if err != nil {
			return dto.TaxOverview{}, err
		}
		byKey := map[string]*entities.TaxFiling{}
		for _, f := range filings {
			s := f.State()
			byKey[string(s.Data.Form)+s.Data.Month.String()] = f
		}
		decPrev, err := uc.filings.Active(ctx, a.CompanyID, vo.FormPp30, jan.Prev())
		if err != nil {
			return dto.TaxOverview{}, err
		}
		wht := map[string]dto.WhtMonthTotal{}
		if last > 0 {
			end := time.Date(year, time.Month(last), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)
			rows, err := uc.reports.WhtMonthly(ctx, a.CompanyID, jan.Start(), end)
			if err != nil {
				return dto.TaxOverview{}, err
			}
			for _, r := range rows {
				wht[string(r.Form)+r.Month.String()] = r
			}
		}
		filingOf := func(form vo.TaxForm, m vo.TaxMonth) *dto.TaxFiling {
			f := byKey[string(form)+m.String()]
			if f == nil || !out.CanSeeFiles {
				return nil
			}
			d := filingDTO(f)
			return &d
		}
		prev := decPrev
		for i := range out.Months {
			m := vo.TaxMonthOf(time.Date(year, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC))
			row := dto.TaxOverviewMonth{Month: m}
			for _, form := range []vo.TaxForm{vo.FormPnd3, vo.FormPnd53} {
				t := wht[string(form)+m.String()]
				cell := dto.TaxOverviewCell{Amount: decimal.Zero, Computed: i < last, Filing: filingOf(form, m)}
				if t.Wht.IsPositive() {
					cell.Amount, cell.Payees = t.Wht, t.Payees
				}
				if form == vo.FormPnd3 {
					row.Pnd3 = cell
				} else {
					row.Pnd53 = cell
				}
			}
			row.Pp30 = dto.TaxOverviewCell{Amount: decimal.Zero, Computed: i < last, Filing: filingOf(vo.FormPp30, m)}
			if i < last {
				sales, err := uc.vat.OutputVat(ctx, a.CompanyID, m.Start(), m.End())
				if err != nil {
					return dto.TaxOverview{}, err
				}
				purchases, err := uc.vat.InputVat(ctx, a.CompanyID, m.Start(), m.End())
				if err != nil {
					return dto.TaxOverview{}, err
				}
				row.Pp30.Amount = pp30(m, sales, purchases, prev).Net
			}
			prev = byKey[string(vo.FormPp30)+m.String()]
			out.Months[i] = row
		}
		return out, nil
	})
}

func ptr[T any](v T) *T { return &v }

// ---------- ทะเบียนการยื่น ----------

func (uc *TaxUseCases) ListFilings(ctx context.Context, a dto.Actor, f dto.TaxFilingFilter) ([]dto.TaxFiling, error) {
	return read(ctx, uc.base, a, vo.PermTaxFilingRead, func() ([]dto.TaxFiling, error) {
		rows, err := uc.filings.List(ctx, a.CompanyID, f)
		out := make([]dto.TaxFiling, len(rows))
		for i, r := range rows {
			out[i] = filingDTO(r)
		}
		return out, err
	})
}

// CreateFiling: บันทึกการยื่น (ไม่ลงบัญชี / ไม่จ่ายเงิน) — 1 แบบ / 1 เดือนยังไม่ยกเลิกได้ 1 รายการ
// (ตรวจก่อนเพื่อข้อความที่ชัด + unique index เป็นตัวกันสุดท้าย); payment_id ต้องเป็นใบจ่ายของบริษัทนี้
func (uc *TaxUseCases) CreateFiling(ctx context.Context, a dto.Actor, d entities.TaxFilingData) (dto.TaxFiling, error) {
	var out dto.TaxFiling
	err := uc.writeWith(ctx, a, vo.PermTaxFilingManage, func(ctx context.Context, s ports.CompanySettings) error {
		f, err := entities.NewTaxFiling(a.CompanyID, d, a.UserID, uc.now(), s.AmountDecimals)
		if err != nil {
			return err
		}
		existing, err := uc.filings.Active(ctx, a.CompanyID, d.Form, d.Month)
		if err != nil {
			return err
		}
		if existing != nil {
			return domain.ErrTaxFilingExists
		}
		if d.PaymentID != nil {
			p, err := uc.payments.Get(ctx, a.CompanyID, *d.PaymentID)
			if err != nil {
				return err
			}
			if p.Status != string(vo.DocPosted) {
				return domain.ErrAlreadyCancelled
			}
		}
		if err := uc.filings.Create(ctx, f); err != nil {
			return err
		}
		out = filingDTO(f)
		return nil
	})
	return out, err
}

func (uc *TaxUseCases) CancelFiling(ctx context.Context, in dto.TaxFilingByID) (dto.TaxFiling, error) {
	var out dto.TaxFiling
	err := uc.write(ctx, in.Actor, vo.PermTaxFilingManage, func(ctx context.Context) error {
		f, err := uc.filings.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := f.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		if err := uc.filings.UpdateStatus(ctx, f); err != nil {
			return err
		}
		out = filingDTO(f)
		return nil
	})
	return out, err
}

func filingDTO(f *entities.TaxFiling) dto.TaxFiling {
	s := f.State()
	out := dto.TaxFiling{ID: s.ID.String(), Form: s.Data.Form, Month: s.Data.Month, FiledDate: s.Data.FiledDate, Amount: s.Data.Amount,
		Surcharge: s.Data.Surcharge, Penalty: s.Data.Penalty, CarryForward: s.Data.CarryForward, Total: f.Total(), Reference: s.Data.Reference,
		Note: s.Data.Note, Status: s.Status, CreatedBy: uuid.UUID(s.CreatedBy).String(), CreatedAt: s.CreatedAt, CancelledAt: s.CancelledAt}
	if s.Data.PaymentID != nil {
		id := s.Data.PaymentID.String()
		out.PaymentID = &id
	}
	return out
}
