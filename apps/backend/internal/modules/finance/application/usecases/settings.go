package usecases

import (
	"context"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/application/ports"
	"apexis/backend/internal/modules/finance/application/ports/repositories"
	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

func optString[T ~[16]byte](p *T) *string {
	if p == nil {
		return nil
	}
	s := vo.IDString(*p)
	return &s
}

// ---------- ค่าตั้งการเงิน ----------

type SettingsUseCases struct{ base }

func (uc *SettingsUseCases) Get(ctx context.Context, a dto.Actor) (dto.Settings, error) {
	return read(ctx, uc.base, a, vo.PermSettingsRead, func() (dto.Settings, error) {
		s, err := uc.settings.Get(ctx, a.CompanyID)
		return settingsDTO(s), err
	})
}

// Update: สมุดรายวันต้องใช้งานได้ และบัญชีต้องลงรายการได้ (ตรวจกับ ledger ใน transaction เดียวกัน)
func (uc *SettingsUseCases) Update(ctx context.Context, a dto.Actor, s entities.Settings) (dto.Settings, error) {
	err := uc.write(ctx, a, vo.PermSettingsManage, func(ctx context.Context) error {
		for _, j := range s.JournalIDs() {
			ok, err := uc.ledger.JournalUsable(ctx, a.CompanyID, j)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrJournalInvalid
			}
		}
		if err := uc.checkAccounts(ctx, a.CompanyID, s.AccountIDs()); err != nil {
			return err
		}
		return uc.settings.Save(ctx, a.CompanyID, s)
	})
	if err != nil {
		return dto.Settings{}, err
	}
	return uc.Get(ctx, a)
}

func settingsDTO(s entities.Settings) dto.Settings {
	return dto.Settings{
		ReceiptJournalID: optString(s.ReceiptJournalID), PaymentJournalID: optString(s.PaymentJournalID),
		TransferJournalID: optString(s.TransferJournalID), WhtReceivedAccountID: optString(s.WhtReceivedAccountID),
		WhtPnd3AccountID: optString(s.WhtPnd3AccountID), WhtPnd53AccountID: optString(s.WhtPnd53AccountID),
	}
}

// ---------- ประเภทภาษีหัก ณ ที่จ่าย (สิทธิ์ค่าตั้งการเงิน) ----------

type WhtTypeUseCases struct {
	base
	repo repositories.WhtTypeRepository
}

func (uc *WhtTypeUseCases) List(ctx context.Context, a dto.Actor, f dto.WhtTypeFilter) (listing.Result[dto.WhtType], error) {
	return read(ctx, uc.base, a, vo.PermSettingsRead, func() (listing.Result[dto.WhtType], error) { return uc.repo.List(ctx, a.CompanyID, f) })
}

// ListForPayment: ประเภทที่ใช้งานอยู่สำหรับหน้าจ่ายเงิน (ผู้มีสิทธิ์จ่ายเงินเลือกได้ โดยไม่ต้องมีสิทธิ์ค่าตั้ง)
func (uc *WhtTypeUseCases) ListForPayment(ctx context.Context, a dto.Actor, f dto.WhtTypeFilter) (listing.Result[dto.WhtType], error) {
	f.Statuses = []vo.Status{vo.StatusActive}
	return read(ctx, uc.base, a, vo.PermPaymentRead, func() (listing.Result[dto.WhtType], error) { return uc.repo.List(ctx, a.CompanyID, f) })
}

func (uc *WhtTypeUseCases) Get(ctx context.Context, in dto.ByDoc[vo.WhtTypeID]) (dto.WhtType, error) {
	return read(ctx, uc.base, in.Actor, vo.PermSettingsRead, func() (dto.WhtType, error) {
		w, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return dto.WhtType{}, err
		}
		return whtDTO(w), nil
	})
}

func (uc *WhtTypeUseCases) Create(ctx context.Context, a dto.Actor, d entities.WhtTypeData) (dto.WhtType, error) {
	w, err := entities.NewWhtType(a.CompanyID, d)
	if err != nil {
		return dto.WhtType{}, err
	}
	if err := uc.write(ctx, a, vo.PermSettingsManage, func(ctx context.Context) error { return uc.repo.Create(ctx, w) }); err != nil {
		return dto.WhtType{}, err
	}
	return whtDTO(w), nil
}

// Update: อัตราใหม่มีผลกับใบจ่ายหลังจากนี้ (ใบจ่ายเดิมเก็บอัตรา ณ วันจ่ายไว้แล้ว)
func (uc *WhtTypeUseCases) Update(ctx context.Context, in dto.ByDoc[vo.WhtTypeID], d entities.WhtTypeData) (dto.WhtType, error) {
	var out dto.WhtType
	err := uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error {
		w, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if d.IncomeType == "" { // ไม่ส่ง = คงประเภทเงินได้เดิม
			d.IncomeType = w.Data().IncomeType
		}
		if err := w.Update(d); err != nil {
			return err
		}
		out = whtDTO(w)
		return uc.repo.Update(ctx, w)
	})
	return out, err
}

// Delete: soft delete — ใบจ่ายเดิมยังอ้างถึงได้
func (uc *WhtTypeUseCases) Delete(ctx context.Context, in dto.ByDoc[vo.WhtTypeID]) error {
	return uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error { return uc.repo.Delete(ctx, in.CompanyID, in.ID) })
}

func whtDTO(w *entities.WhtType) dto.WhtType {
	d := w.Data()
	return dto.WhtType{ID: w.ID().String(), Code: d.Code, Name: d.Name, Rate: d.Rate, IncomeType: string(d.IncomeType), Status: string(d.Status)}
}

// ---------- รายงาน ----------

type ReportUseCases struct {
	base
	reports repositories.ReportRepository
	vat     ports.Vat
}

// WithholdingPaid: ภาษีหัก ณ ที่จ่ายตามวันที่จ่าย แยก ภ.ง.ด.3 / 53 (กรอง form)
func (uc *ReportUseCases) WithholdingPaid(ctx context.Context, a dto.Actor, f dto.WithholdingFilter) (dto.Withholding, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.Withholding, error) {
		rows, totals, err := uc.reports.WithholdingPaid(ctx, a.CompanyID, f)
		return dto.Withholding{Start: f.Start, End: f.End, Rows: rows, Totals: totals}, err
	})
}

// WithholdingReceived: ภาษีที่ลูกค้าหักไว้ตามวันที่รับเงิน
func (uc *ReportUseCases) WithholdingReceived(ctx context.Context, a dto.Actor, f dto.WithholdingFilter) (dto.Withholding, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.Withholding, error) {
		rows, totals, err := uc.reports.WithholdingReceived(ctx, a.CompanyID, f)
		return dto.Withholding{Start: f.Start, End: f.End, Rows: rows, Totals: totals}, err
	})
}

// VatSummary: สรุปภาษีมูลค่าเพิ่มสำหรับ ภ.พ.30 — ภาษีขาย (sales) - ภาษีซื้อ (purchasing) ของช่วงวันที่ (ปกติ = 1 เดือนภาษี)
// อ่านอย่างเดียว ไม่ใช้ transaction (2 คำขอที่ไม่ขึ้นต่อกัน)
func (uc *ReportUseCases) VatSummary(ctx context.Context, a dto.Actor, f dto.VatSummaryFilter) (dto.VatSummary, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.VatSummary, error) {
		output, err := uc.vat.OutputVat(ctx, a.CompanyID, f.Start, f.End)
		if err != nil {
			return dto.VatSummary{}, err
		}
		input, err := uc.vat.InputVat(ctx, a.CompanyID, f.Start, f.End)
		if err != nil {
			return dto.VatSummary{}, err
		}
		return vatSummary(f, output, input), nil
	})
}

func vatSummary(f dto.VatSummaryFilter, output, input ports.VatTotals) dto.VatSummary {
	net := output.Tax.Sub(input.Tax)
	out := dto.VatSummary{Start: f.Start, End: f.End, NetTax: net, Payable: decimal.Zero, Refundable: decimal.Zero,
		Output: dto.VatTotals{Base: output.Base, Tax: output.Tax, Documents: output.Documents},
		Input:  dto.VatTotals{Base: input.Base, Tax: input.Tax, Documents: input.Documents}}
	if net.IsPositive() {
		out.Payable = net
	} else {
		out.Refundable = net.Neg()
	}
	return out
}
