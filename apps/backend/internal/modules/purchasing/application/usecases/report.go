package usecases

import (
	"apexis/backend/internal/platform/dashperiod"
	"context"
	"time"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ReportUseCases: รายงานจัดซื้อ (ใช้ร่วมกับ GET /reports?book=)
type ReportUseCases struct {
	base
	reports repositories.ReportRepository
}

// Outstanding: บรรทัดใบสั่งซื้อที่ยังรับหรือแจ้งหนี้ไม่ครบ
func (uc *ReportUseCases) Outstanding(ctx context.Context, in dto.List[dto.OutstandingFilter]) (listing.Result[dto.OutstandingRow], error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (listing.Result[dto.OutstandingRow], error) {
		res, err := uc.reports.Outstanding(ctx, in.CompanyID, in.Filter)
		if err != nil {
			return res, err
		}
		refs := make([]*dto.Ref, len(res.Items))
		for i := range res.Items {
			refs[i] = &res.Items[i].Item
		}
		return res, uc.itemRefs(ctx, in.CompanyID, refs)
	})
}

// Aging: อายุเจ้าหนี้ ณ วันที่
// Dashboard: ภาพรวมแอปจัดซื้อ — สิทธิ์เดียวกับรายงาน (purchasing.report:read), อ่านอย่างเดียว ไม่ใช้ transaction
func (uc *ReportUseCases) Dashboard(ctx context.Context, in dto.List[dashperiod.Period]) (dto.Dashboard, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Dashboard, error) {
		return uc.reports.Dashboard(ctx, in.CompanyID, in.Filter)
	})
}

// InputVat: รายงานภาษีซื้อ (ใบแจ้งหนี้ผู้ขาย + ใบลดหนี้ติดลบ) ตามวันที่ลงบัญชี
func (uc *ReportUseCases) InputVat(ctx context.Context, in dto.List[dto.InputVatFilter]) (dto.InputVat, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.InputVat, error) {
		rows, t, err := uc.reports.InputVat(ctx, in.CompanyID, in.Filter)
		return dto.InputVat{Start: in.Filter.Start, End: in.Filter.End, Rows: rows, TotalBase: t.Base, TotalTax: t.Tax}, err
	})
}

// VatTotals: ยอดรวมภาษีซื้อของช่วงวันที่ — ไม่ตรวจสิทธิ์ (finance เรียกหลังตรวจ finance.report:read แล้ว)
func (uc *ReportUseCases) VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error) {
	return uc.reports.VatTotals(ctx, companyID, start, end)
}

func (uc *ReportUseCases) Aging(ctx context.Context, in dto.List[dto.AgingFilter]) (dto.Aging, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Aging, error) {
		rows, totals, err := uc.reports.Aging(ctx, in.CompanyID, in.Filter)
		return dto.Aging{AsOf: in.Filter.AsOf, Rows: rows, Totals: totals}, err
	})
}
