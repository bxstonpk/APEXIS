package usecases

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/dto"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

const (
	dashboardMonths    = 12 // กราฟรายเดือนสิ้นสุดที่เดือนของ End
	recentDocumentSize = 6
)

// monthStarts: วันที่ 1 ของ 12 เดือนที่สิ้นสุดที่เดือนของ end (เก่าสุดก่อน)
func monthStarts(end time.Time) []time.Time {
	last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]time.Time, dashboardMonths)
	for i := range out {
		out[i] = last.AddDate(0, i-(dashboardMonths-1), 0)
	}
	return out
}

// Dashboard ภาพรวมแอปการเงิน — GET ไม่ใช้ transaction
//
// query: ทะเบียนเงิน 1 + ยอดคงเหลือจาก ledger 1 (ทุกบัญชีในครั้งเดียว — นับยอดยกมาที่ลงด้วยรายการบัญชีด้วย)
// + ยอดรวมของช่วง 1 + รายเดือน 1 + เอกสารล่าสุด 1 — แยก query เพราะแต่ละชุดจัดกลุ่มต่างกัน
func (uc *ReportUseCases) Dashboard(ctx context.Context, a dto.Actor, start, end time.Time) (dto.FinanceDashboard, error) {
	if err := uc.authorize(ctx, a, vo.PermReportRead); err != nil {
		return dto.FinanceDashboard{}, err
	}
	out := dto.FinanceDashboard{Start: start, End: end, CashTotal: decimal.Zero}
	var err error
	if out.CashAccounts, err = uc.reports.ActiveCashAccounts(ctx, a.CompanyID); err != nil {
		return out, err
	}
	if len(out.CashAccounts) > 0 {
		ids := make([]vo.GLAccountID, len(out.CashAccounts))
		for i, c := range out.CashAccounts {
			ids[i] = c.GLAccountID
		}
		balances, err := uc.ledger.Balances(ctx, a.CompanyID, ids, end)
		if err != nil {
			return out, err
		}
		for i := range out.CashAccounts {
			if b, ok := balances[out.CashAccounts[i].GLAccountID]; ok {
				out.CashAccounts[i].Balance = b
			}
			out.CashTotal = out.CashTotal.Add(out.CashAccounts[i].Balance)
		}
	}
	if out.Flows, err = uc.reports.FlowTotals(ctx, a.CompanyID, start, end); err != nil {
		return out, err
	}
	months := monthStarts(end)
	rows, err := uc.reports.MonthlyFlows(ctx, a.CompanyID, months[0], months[len(months)-1].AddDate(0, 1, -1))
	if err != nil {
		return out, err
	}
	byMonth := make(map[time.Time]dto.MonthlyFlow, len(rows))
	for _, r := range rows {
		byMonth[r.Month] = r
	}
	out.Monthly = make([]dto.MonthlyFlow, len(months))
	for i, m := range months {
		f, ok := byMonth[m]
		if !ok {
			f = dto.MonthlyFlow{Month: m, Received: decimal.Zero, Paid: decimal.Zero}
		}
		out.Monthly[i] = f
	}
	out.Recent, err = uc.reports.RecentDocuments(ctx, a.CompanyID, recentDocumentSize)
	return out, err
}
