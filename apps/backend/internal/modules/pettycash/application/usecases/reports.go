package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/application/ports/repositories"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

type ReportUseCases struct {
	base
	reports repositories.ReportRepository
}

// Register: ทะเบียนใบสำคัญจ่ายของช่วงวันที่ (กรองกองทุน / สถานะ) + ยอดรวม — แบ่งหน้าใน DB
func (uc *ReportUseCases) Register(ctx context.Context, a dto.Actor, f dto.RegisterFilter) (dto.RegisterReport, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.RegisterReport, error) { return uc.reports.Register(ctx, a.CompanyID, f) })
}

// FundBalance: ยอดของทุกกองทุน ณ วันที่ (เงินเข้าตามวันที่การเงินจ่าย, ใบสำคัญจ่ายตามวันที่เอกสาร)
// query: กองทุน 1 + ยอด 1 + ชื่อผู้ถือเงิน 1 + ชื่อบัญชีไม่ใช้
func (uc *ReportUseCases) FundBalance(ctx context.Context, a dto.Actor, asOf time.Time) ([]dto.FundBalanceRow, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() ([]dto.FundBalanceRow, error) {
		rows, err := uc.reports.FundRows(ctx, a.CompanyID)
		if err != nil {
			return nil, err
		}
		ids := make([]vo.FundID, 0, len(rows))
		users := make([]*dto.Ref, len(rows))
		for i := range rows {
			if id, err := vo.ParseID[vo.FundID](rows[i].Fund.ID); err == nil {
				ids = append(ids, id)
			}
			users[i] = &rows[i].Custodian
		}
		bal, err := uc.funds.Balances(ctx, a.CompanyID, ids, &asOf)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			if id, err := vo.ParseID[vo.FundID](rows[i].Fund.ID); err == nil {
				rows[i].Balance = bal[id]
			}
		}
		return rows, uc.userRefs(ctx, a.CompanyID, users)
	})
}

// InputVat: ภาษีซื้อของใบสำคัญจ่ายที่ชดเชยแล้วและมีใบกำกับภาษี (ตามวันที่ใบกำกับภาษี)
func (uc *ReportUseCases) InputVat(ctx context.Context, a dto.Actor, f dto.InputVatFilter) (dto.InputVatReport, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() (dto.InputVatReport, error) { return uc.reports.InputVat(ctx, a.CompanyID, f) })
}
