package usecases

import (
	"context"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/application/ports/repositories"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ReportUseCases คือรายงานของการผลิต (GET /reports?book=...) — 1 query ต่อหน้า + ชื่อสินค้า 1 query
type ReportUseCases struct {
	base
	reports repositories.ReportRepository
}

// ProductionCost: ต้นทุนการผลิตของใบสั่งผลิตที่ปิดงานในช่วงวันที่
func (uc *ReportUseCases) ProductionCost(ctx context.Context, a dto.Actor, f dto.CostFilter) (dto.CostReport, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.CostReport, error) {
		res, err := uc.reports.ProductionCost(ctx, a.CompanyID, f)
		if err != nil {
			return res, err
		}
		refs := make([]*dto.Ref, len(res.Rows.Items))
		for i := range res.Rows.Items {
			refs[i] = &res.Rows.Items[i].Item
		}
		return res, uc.itemRefs(ctx, a.CompanyID, refs)
	})
}

// MaterialUsage: วัตถุดิบตามแผน (ปรับตามจำนวนที่ผลิตได้) เทียบกับที่เบิกสุทธิ ของใบสั่งผลิตที่ปิดงานในช่วงวันที่
func (uc *ReportUseCases) MaterialUsage(ctx context.Context, a dto.Actor, f dto.CostFilter) (listing.Result[dto.UsageRow], error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (listing.Result[dto.UsageRow], error) {
		s, err := uc.org.CompanySettings(ctx, a.CompanyID)
		if err != nil {
			return listing.Result[dto.UsageRow]{}, err
		}
		f.QuantityDecimals, f.AmountDecimals = s.QuantityDecimals, s.AmountDecimals
		res, err := uc.reports.MaterialUsage(ctx, a.CompanyID, f)
		if err != nil {
			return res, err
		}
		refs := make([]*dto.Ref, 0, 2*len(res.Items))
		for i := range res.Items {
			refs = append(refs, &res.Items[i].Item, &res.Items[i].Material)
		}
		return res, uc.itemRefs(ctx, a.CompanyID, refs)
	})
}

// Wip: งานระหว่างผลิต (ใบสั่งผลิตที่ยังไม่ปิดงาน ณ วันที่) = วัตถุดิบที่เบิกสุทธิถึงวันที่
func (uc *ReportUseCases) Wip(ctx context.Context, a dto.Actor, f dto.WipFilter) (dto.WipReport, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.WipReport, error) {
		res, err := uc.reports.Wip(ctx, a.CompanyID, f)
		if err != nil {
			return res, err
		}
		refs := make([]*dto.Ref, len(res.Rows.Items))
		for i := range res.Rows.Items {
			refs[i] = &res.Rows.Items[i].Item
		}
		return res, uc.itemRefs(ctx, a.CompanyID, refs)
	})
}
