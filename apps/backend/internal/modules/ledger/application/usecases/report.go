package usecases

import (
	"context"
	"slices"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// ReportUseCases คือรายงานทางบัญชี — คำนวณจากรายการที่มีผล (posted / reversed) ทุกครั้ง
//
// ยังไม่มีการปิดบัญชีสิ้นปี: บัญชีรายได้ / ค่าใช้จ่ายเริ่มนับใหม่ที่วันเริ่มปีบัญชี
// ส่วนกำไร (ขาดทุน) ของปีก่อนๆ แสดงเป็นยอด "กำไรสะสมที่ยังไม่ปิดบัญชี" แยกต่างหาก
//
// แต่ละรายงานใช้ query รวมยอดใน DB 1 ครั้ง (ทุกช่วงวันที่ใน query เดียว) + ผังบัญชี 1 ครั้ง
// + งวดบัญชีจาก organization เท่าที่ต้องใช้ (หาวันเริ่มปีบัญชี)
type ReportUseCases struct {
	base
	accounts repositories.AccountRepository
	reports  repositories.ReportRepository
}

func dayBefore(t time.Time) time.Time { return t.AddDate(0, 0, -1) }

// TrialBalance งบทดลองช่วง [Start, End] — ต้องอยู่ในปีบัญชีเดียวกัน
//
//	ยอดยกมา: บัญชีงบแสดงฐานะ = ตั้งแต่ต้นถึงก่อน Start, รายได้ / ค่าใช้จ่าย = ตั้งแต่วันเริ่มปีบัญชีถึงก่อน Start
//	PriorYearsEarnings = ผลรวม (เดบิต - เครดิต) ของรายได้ / ค่าใช้จ่ายก่อนปีบัญชีนี้ (ยังไม่ได้ปิดเข้าส่วนของเจ้าของ)
//	ผลรวม Closing ทุกบรรทัด + PriorYearsEarnings = 0 เสมอ
func (uc *ReportUseCases) TrialBalance(ctx context.Context, in dto.DateRange) (dto.TrialBalance, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.TrialBalance{}, err
	}
	yearStart, err := uc.sameYearStart(ctx, in.CompanyID, in.Start, in.End)
	if err != nil {
		return dto.TrialBalance{}, err
	}
	before := dayBefore(in.Start)
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, []repositories.Bucket{
		{To: before},                   // 0: ตั้งแต่ต้นถึงก่อน Start
		{From: &yearStart, To: before}, // 1: ต้นปีบัญชีถึงก่อน Start
		{From: &in.Start, To: in.End},  // 2: ช่วงรายงาน
	})
	if err != nil {
		return dto.TrialBalance{}, err
	}
	chart, err := uc.accounts.Chart(ctx, in.CompanyID)
	if err != nil {
		return dto.TrialBalance{}, err
	}
	slices.SortFunc(chart, func(a, b services.AccountNode) int { return compare(a.Code, b.Code) })

	out := dto.TrialBalance{Start: in.Start, End: in.End, Rows: []dto.TrialBalanceRow{}}
	for _, a := range chart {
		s, ok := sums[a.ID]
		if !ok {
			continue
		}
		opening := s[0].Net()
		if !a.Type.IsBalanceSheet() {
			opening = s[1].Net()
			out.PriorYearsEarnings = out.PriorYearsEarnings.Add(s[0].Net().Sub(s[1].Net()))
		}
		row := dto.TrialBalanceRow{
			Account: info(a), Opening: opening, Debit: s[2].Debit, Credit: s[2].Credit,
			Closing: opening.Add(s[2].Debit).Sub(s[2].Credit),
		}
		if row.Opening.IsZero() && row.Debit.IsZero() && row.Credit.IsZero() {
			continue
		}
		out.Rows = append(out.Rows, row)
		out.TotalDebit = out.TotalDebit.Add(row.Debit)
		out.TotalCredit = out.TotalCredit.Add(row.Credit)
	}
	return out, nil
}

// AccountLedger บัญชีแยกประเภทของบัญชีเดียว: ยอดยกมา + รายการเคลื่อนไหวพร้อมยอดสะสม (แบ่งหน้าใน DB)
func (uc *ReportUseCases) AccountLedger(ctx context.Context, in dto.AccountLedgerInput) (dto.AccountLedger, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.AccountLedger{}, err
	}
	acc, err := uc.accounts.FindByID(ctx, in.CompanyID, in.AccountID)
	if err != nil {
		return dto.AccountLedger{}, err
	}
	d := acc.Data()
	bucket := repositories.Bucket{To: dayBefore(in.Start)}
	if !d.Type.IsBalanceSheet() {
		p, err := uc.org.PeriodAt(ctx, in.CompanyID, in.Start)
		if err != nil {
			return dto.AccountLedger{}, err
		}
		bucket.From = &p.YearStart
	}
	id := in.AccountID
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, &id, []repositories.Bucket{bucket})
	if err != nil {
		return dto.AccountLedger{}, err
	}
	var opening vo.Money
	if s, ok := sums[id]; ok {
		opening = s[0].Net()
	}
	lines, total, err := uc.reports.AccountLines(ctx, in.CompanyID, id, in.Start, in.End, in.Page)
	if err != nil {
		return dto.AccountLedger{}, err
	}
	for i := range lines {
		lines[i].Balance = lines[i].Balance.Add(opening)
	}
	out := dto.AccountLedger{
		Account: dto.AccountInfo{ID: id.String(), Code: d.Code, Name: d.Name, Type: string(d.Type)},
		Start:   in.Start, End: in.End, Opening: opening,
	}
	out.Lines.Items, out.Lines.Total, out.Lines.Page = lines, total, in.Page
	return out, nil
}

// BalanceSheet งบแสดงฐานะการเงิน ณ วันที่ — สินทรัพย์ + บัญชีพัก = หนี้สิน + ส่วนของเจ้าของ (+ กำไรสะสมที่ยังไม่ปิดบัญชี)
func (uc *ReportUseCases) BalanceSheet(ctx context.Context, in dto.AsOf) (dto.BalanceSheet, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.BalanceSheet{}, err
	}
	p, err := uc.org.PeriodAt(ctx, in.CompanyID, in.Date)
	if err != nil {
		return dto.BalanceSheet{}, err
	}
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, []repositories.Bucket{
		{To: in.Date},                     // 0: สะสมถึงวันที่
		{From: &p.YearStart, To: in.Date}, // 1: ปีบัญชีปัจจุบันถึงวันที่
	})
	if err != nil {
		return dto.BalanceSheet{}, err
	}
	chart, err := uc.accounts.Chart(ctx, in.CompanyID)
	if err != nil {
		return dto.BalanceSheet{}, err
	}
	net := map[vo.AccountID]vo.Money{}
	types := map[vo.AccountID]vo.AccountType{}
	for _, a := range chart {
		types[a.ID] = a.Type
	}
	out := dto.BalanceSheet{AsOf: in.Date}
	for id, s := range sums {
		if types[id].IsBalanceSheet() {
			net[id] = s[0].Net()
			continue
		}
		// รายได้ / ค่าใช้จ่าย -> กำไร (ขาดทุน) ด้านเครดิตเป็นบวก
		out.CurrentYearEarnings = out.CurrentYearEarnings.Sub(s[1].Net())
		out.RetainedEarningsPrior = out.RetainedEarningsPrior.Sub(s[0].Net().Sub(s[1].Net()))
	}
	out.Assets, out.TotalAssets = services.BuildSection(chart, net, vo.AccountAsset)
	out.Liabilities, out.TotalLiabilities = services.BuildSection(chart, net, vo.AccountLiability)
	var equity vo.Money
	out.Equity, equity = services.BuildSection(chart, net, vo.AccountEquity)
	out.TotalEquity = equity.Add(out.RetainedEarningsPrior).Add(out.CurrentYearEarnings)
	out.TotalLiabilitiesAndEquity = out.TotalLiabilities.Add(out.TotalEquity)
	out.Suspense, out.TotalSuspense = services.BuildSection(chart, net, vo.AccountSuspense)
	return out, nil
}

// IncomeStatement งบกำไรขาดทุนช่วง [Start, End]
func (uc *ReportUseCases) IncomeStatement(ctx context.Context, in dto.DateRange) (dto.IncomeStatement, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.IncomeStatement{}, err
	}
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, []repositories.Bucket{{From: &in.Start, To: in.End}})
	if err != nil {
		return dto.IncomeStatement{}, err
	}
	chart, err := uc.accounts.Chart(ctx, in.CompanyID)
	if err != nil {
		return dto.IncomeStatement{}, err
	}
	net := make(map[vo.AccountID]vo.Money, len(sums))
	for id, s := range sums {
		net[id] = s[0].Net()
	}
	out := dto.IncomeStatement{Start: in.Start, End: in.End}
	out.Revenue, out.TotalRevenue = services.BuildSection(chart, net, vo.AccountRevenue)
	out.Expenses, out.TotalExpenses = services.BuildSection(chart, net, vo.AccountExpense)
	out.NetIncome = out.TotalRevenue.Sub(out.TotalExpenses)
	out.Suspense, out.TotalSuspense = services.BuildSection(chart, net, vo.AccountSuspense)
	return out, nil
}

// sameYearStart คืนวันเริ่มปีบัญชีของ start และตรวจว่า end อยู่ในปีบัญชีเดียวกัน
func (uc *ReportUseCases) sameYearStart(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (time.Time, error) {
	ps, err := uc.org.PeriodAt(ctx, companyID, start)
	if err != nil {
		return time.Time{}, err
	}
	pe, err := uc.org.PeriodAt(ctx, companyID, end)
	if err != nil {
		return time.Time{}, err
	}
	if ps.FiscalYear != pe.FiscalYear {
		return time.Time{}, domain.ErrRangeCrossesYear
	}
	return ps.YearStart, nil
}

func info(a services.AccountNode) dto.AccountInfo {
	return dto.AccountInfo{ID: a.ID.String(), Code: a.Code, Name: a.Name, Type: string(a.Type)}
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
