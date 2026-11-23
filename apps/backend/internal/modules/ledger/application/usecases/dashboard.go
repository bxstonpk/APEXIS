package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// dashboardMonths คือจำนวนเดือนของกราฟรายเดือน (สิ้นสุดที่เดือนของ End)
const dashboardMonths = 12

// recentEntryLimit คือจำนวนรายการบัญชีล่าสุดที่แสดง
const recentEntryLimit = 5

// previousRange: ช่วงก่อนหน้าความยาวเท่ากัน ต่อท้ายก่อน start พอดี
func previousRange(start, end time.Time) (time.Time, time.Time) {
	days := int(end.Sub(start).Hours()/24) + 1
	return start.AddDate(0, 0, -days), start.AddDate(0, 0, -1)
}

// monthStarts: วันที่ 1 ของ 12 เดือนที่สิ้นสุดที่เดือนของ end (เก่าสุดก่อน)
func monthStarts(end time.Time) []time.Time {
	last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]time.Time, dashboardMonths)
	for i := range out {
		out[i] = last.AddDate(0, i-(dashboardMonths-1), 0)
	}
	return out
}

// Dashboard ภาพรวมแอปบัญชี — GET ไม่ใช้ transaction
//
// query: ยอดทุกช่วง (ช่วงที่เลือก / ช่วงก่อน / 12 เดือน / บัญชีพักสะสม) รวมใน SumByAccount ครั้งเดียว
// + ผังบัญชี 1 + จำนวนรายการ 1 + รายการล่าสุด 1 + งวดจาก organization 1
func (uc *ReportUseCases) Dashboard(ctx context.Context, in dto.DateRange) (dto.AccountingDashboard, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.AccountingDashboard{}, err
	}
	prevStart, prevEnd := previousRange(in.Start, in.End)
	months := monthStarts(in.End)
	buckets := []repositories.Bucket{
		{From: &in.Start, To: in.End},   // 0: ช่วงที่เลือก
		{From: &prevStart, To: prevEnd}, // 1: ช่วงก่อนหน้า
		{To: in.End},                    // 2: สะสมถึง End (บัญชีพัก)
	}
	for i := range months {
		from := months[i]
		buckets = append(buckets, repositories.Bucket{From: &from, To: from.AddDate(0, 1, -1)}) // 3..14: รายเดือน
	}
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, buckets)
	if err != nil {
		return dto.AccountingDashboard{}, err
	}
	chart, err := uc.accounts.Chart(ctx, in.CompanyID)
	if err != nil {
		return dto.AccountingDashboard{}, err
	}
	types := make(map[vo.AccountID]vo.AccountType, len(chart))
	for _, a := range chart {
		types[a.ID] = a.Type
	}

	out := dto.AccountingDashboard{Start: in.Start, End: in.End, Monthly: make([]dto.MonthlyIncome, len(months))}
	for i, m := range months {
		out.Monthly[i].Month = m
	}
	// รายได้ = เครดิต - เดบิต, ค่าใช้จ่าย = เดบิต - เครดิต (เครื่องหมายเดียวกับงบกำไรขาดทุน); บัญชีพัก = เดบิต - เครดิต
	add := func(s *dto.IncomeSummary, t vo.AccountType, net vo.Money) {
		switch t {
		case vo.AccountRevenue:
			s.Revenue = s.Revenue.Sub(net)
		case vo.AccountExpense:
			s.Expense = s.Expense.Add(net)
		}
	}
	for id, s := range sums {
		t := types[id]
		add(&out.Income, t, s[0].Net())
		add(&out.Previous, t, s[1].Net())
		if t == vo.AccountSuspense {
			out.SuspenseBalance = out.SuspenseBalance.Add(s[2].Net())
		}
		for i := range months {
			var m dto.IncomeSummary
			add(&m, t, s[3+i].Net())
			out.Monthly[i].Revenue = out.Monthly[i].Revenue.Add(m.Revenue)
			out.Monthly[i].Expense = out.Monthly[i].Expense.Add(m.Expense)
		}
	}
	out.Income.NetIncome = out.Income.Revenue.Sub(out.Income.Expense)
	out.Previous.NetIncome = out.Previous.Revenue.Sub(out.Previous.Expense)

	if out.DraftCount, out.PostedCount, err = uc.reports.EntryActivity(ctx, in.CompanyID, in.Start, in.End); err != nil {
		return dto.AccountingDashboard{}, err
	}
	if out.RecentEntries, err = uc.reports.RecentEntries(ctx, in.CompanyID, recentEntryLimit); err != nil {
		return dto.AccountingDashboard{}, err
	}
	p, err := uc.org.PeriodRange(ctx, in.CompanyID, in.End)
	if err != nil {
		return dto.AccountingDashboard{}, err
	}
	if p != nil {
		out.Period = &dto.DashboardPeriod{FiscalYear: p.FiscalYear, Start: p.Start, End: p.End, Open: p.Open}
	}
	return out, nil
}
