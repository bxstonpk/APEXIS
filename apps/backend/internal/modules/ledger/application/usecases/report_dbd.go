package usecases

import (
	"context"
	"errors"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// งบการเงินตามแบบกรมพัฒนาธุรกิจการค้า (dbd-balance-sheet / dbd-income-statement)
// 2 คอลัมน์: ณ วันที่ / ปีบัญชีปัจจุบัน และวันเดียวกันของปีก่อน — 1 query รวมยอดทุกคอลัมน์ + ผังบัญชี 1 query
// ปีก่อนไม่มีงวดบัญชี = คอลัมน์เป็นศูนย์

type dbdCol struct {
	end       time.Time
	yearStart *time.Time // nil = ไม่มีปีบัญชีครอบคลุม
	year      int
}

func (uc *ReportUseCases) dbdColumns(ctx context.Context, companyID vo.CompanyID, end time.Time) ([]dbdCol, error) {
	p, err := uc.org.PeriodAt(ctx, companyID, end)
	if err != nil {
		return nil, err
	}
	cols := []dbdCol{{end: end, yearStart: &p.YearStart, year: p.FiscalYear}}
	prevEnd := end.AddDate(-1, 0, 0)
	prev := dbdCol{end: prevEnd, year: p.FiscalYear - 1}
	if pp, err := uc.org.PeriodAt(ctx, companyID, prevEnd); err == nil {
		prev.yearStart, prev.year = &pp.YearStart, pp.FiscalYear
	} else if !errors.Is(err, domain.ErrNoFiscalPeriod) {
		return nil, err
	}
	return append(cols, prev), nil
}

// DBDBalanceSheet งบแสดงฐานะการเงิน ณ end (เทียบปีก่อน)
func (uc *ReportUseCases) DBDBalanceSheet(ctx context.Context, in dto.AsOf) (dto.DBDStatement, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.DBDStatement{}, err
	}
	cols, err := uc.dbdColumns(ctx, in.CompanyID, in.Date)
	if err != nil {
		return dto.DBDStatement{}, err
	}
	var buckets []repositories.Bucket
	for _, c := range cols {
		ys := c.end.AddDate(0, 0, 1) // ไม่มีปีบัญชี: ไม่มีกำไรปีปัจจุบัน
		if c.yearStart != nil {
			ys = *c.yearStart
		}
		buckets = append(buckets, repositories.Bucket{To: c.end}, repositories.Bucket{From: &ys, To: c.end})
	}
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, buckets)
	if err != nil {
		return dto.DBDStatement{}, err
	}
	chart, err := uc.accounts.Chart(ctx, in.CompanyID)
	if err != nil {
		return dto.DBDStatement{}, err
	}
	types := make(map[vo.AccountID]vo.AccountType, len(chart))
	for _, a := range chart {
		types[a.ID] = a.Type
	}
	bs := make([]services.BSColumn, len(cols))
	for i := range cols {
		bs[i] = services.BSColumn{Net: map[vo.AccountID]vo.Money{}}
		for id, s := range sums {
			all, year := s[2*i].Net(), s[2*i+1].Net()
			if types[id].IsBalanceSheet() {
				bs[i].Net[id] = all
				continue
			}
			// รายได้ / ค่าใช้จ่าย (เครดิตเป็นบวก): ปีปัจจุบัน และปีก่อนๆ ที่ยังไม่ปิดบัญชี
			bs[i].CurrentYear = bs[i].CurrentYear.Sub(year)
			bs[i].PriorUnclosed = bs[i].PriorUnclosed.Sub(all.Sub(year))
		}
	}
	out := dto.DBDStatement{Rows: services.DBDBalanceSheet(chart, bs)}
	for _, c := range cols {
		out.Columns = append(out.Columns, dto.DBDColumn{End: c.end, FiscalYear: c.year})
	}
	return out, nil
}

// DBDIncomeStatement งบกำไรขาดทุนตั้งแต่ต้นปีบัญชีถึง end (เทียบช่วงเดียวกันของปีก่อน) — ไม่นับรายการปิดบัญชี
func (uc *ReportUseCases) DBDIncomeStatement(ctx context.Context, in dto.AsOf) (dto.DBDStatement, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.DBDStatement{}, err
	}
	cols, err := uc.dbdColumns(ctx, in.CompanyID, in.Date)
	if err != nil {
		return dto.DBDStatement{}, err
	}
	var buckets []repositories.Bucket
	out := dto.DBDStatement{}
	for _, c := range cols {
		start := c.end.AddDate(0, 0, 1) // ไม่มีปีบัญชี = ช่วงว่าง
		if c.yearStart != nil {
			start = *c.yearStart
		}
		s := start
		buckets = append(buckets, repositories.Bucket{From: &s, To: c.end, ExcludeClosing: true})
		out.Columns = append(out.Columns, dto.DBDColumn{Start: &s, End: c.end, FiscalYear: c.year})
	}
	sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, buckets)
	if err != nil {
		return dto.DBDStatement{}, err
	}
	chart, err := uc.accounts.Chart(ctx, in.CompanyID)
	if err != nil {
		return dto.DBDStatement{}, err
	}
	is := make([]services.ISColumn, len(cols))
	for i := range cols {
		is[i] = services.ISColumn{}
		for id, s := range sums {
			is[i][id] = s[i].Net()
		}
	}
	out.Rows = services.DBDIncomeStatement(chart, is)
	return out, nil
}
