package usecases

import (
	"context"

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
func (uc *ReportUseCases) Aging(ctx context.Context, in dto.List[dto.AgingFilter]) (dto.Aging, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Aging, error) {
		rows, totals, err := uc.reports.Aging(ctx, in.CompanyID, in.Filter)
		return dto.Aging{AsOf: in.Filter.AsOf, Rows: rows, Totals: totals}, err
	})
}

// Withholding: ภาษีหัก ณ ที่จ่ายตามวันที่จ่าย แยก ภ.ง.ด.3 / 53 (กรอง form)
func (uc *ReportUseCases) Withholding(ctx context.Context, in dto.List[dto.WithholdingFilter]) (dto.Withholding, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Withholding, error) {
		rows, totals, err := uc.reports.Withholding(ctx, in.CompanyID, in.Filter)
		return dto.Withholding{Start: in.Filter.Start, End: in.Filter.End, Rows: rows, Totals: totals}, err
	})
}
