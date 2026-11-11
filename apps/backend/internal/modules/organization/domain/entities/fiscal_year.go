package entities

import (
	"time"

	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// PeriodsPerYear: ปีบัญชีแบ่งเป็น 12 งวดรายเดือน
const PeriodsPerYear = 12

// FiscalYear คือปีบัญชี (aggregate root) ที่มีงวดบัญชี 12 งวด
//
// กฎ:
//   - เริ่มวันที่ 1 ของเดือนใดก็ได้ (เช่น 1 ม.ค. หรือ 1 ต.ค.) ยาว 12 เดือนพอดี
//   - ปีบัญชีของบริษัทเดียวกันห้ามซ้อนกัน (ตรวจใน use case + unique (company_id, year))
//   - ปิดงวดได้เมื่องวดก่อนหน้าปิดหมดแล้ว / เปิดงวดคืนได้เมื่องวดหลังจากนั้นยังเปิดอยู่ทั้งหมด
type FiscalYear struct {
	id        vo.FiscalYearID
	companyID vo.CompanyID
	year      int
	startDate time.Time
	endDate   time.Time
	periods   []*FiscalPeriod // เรียงตาม periodNo
}

type FiscalPeriod struct {
	id           vo.FiscalPeriodID
	fiscalYearID vo.FiscalYearID
	periodNo     int
	startDate    time.Time
	endDate      time.Time
	status       vo.PeriodStatus
}

// NewFiscalYear สร้างปีบัญชีพร้อมงวดรายเดือน 12 งวด (สถานะเปิดทั้งหมด)
// year คือปีบัญชีที่ใช้อ้างอิง (ค.ศ.) เช่น 2026
func NewFiscalYear(companyID vo.CompanyID, year int, start time.Time) (*FiscalYear, error) {
	if year < 1900 || year > 9999 {
		return nil, domain.ErrInvalidFiscalYear
	}
	if !vo.IsDateOnly(start) || start.Day() != 1 {
		return nil, domain.ErrInvalidFiscalStart
	}
	fy := &FiscalYear{
		id:        vo.NewID[vo.FiscalYearID](),
		companyID: companyID,
		year:      year,
		startDate: start,
		endDate:   start.AddDate(1, 0, -1),
	}
	for i := range PeriodsPerYear {
		fy.periods = append(fy.periods, &FiscalPeriod{
			id:           vo.NewID[vo.FiscalPeriodID](),
			fiscalYearID: fy.id,
			periodNo:     i + 1,
			startDate:    start.AddDate(0, i, 0),
			endDate:      start.AddDate(0, i+1, -1),
			status:       vo.PeriodOpen,
		})
	}
	return fy, nil
}

func RehydrateFiscalYear(id vo.FiscalYearID, companyID vo.CompanyID, year int, start, end time.Time, periods []*FiscalPeriod) *FiscalYear {
	return &FiscalYear{id: id, companyID: companyID, year: year, startDate: start, endDate: end, periods: periods}
}

func RehydrateFiscalPeriod(id vo.FiscalPeriodID, fiscalYearID vo.FiscalYearID, no int, start, end time.Time, status vo.PeriodStatus) *FiscalPeriod {
	return &FiscalPeriod{id: id, fiscalYearID: fiscalYearID, periodNo: no, startDate: start, endDate: end, status: status}
}

func (y *FiscalYear) ID() vo.FiscalYearID     { return y.id }
func (y *FiscalYear) CompanyID() vo.CompanyID { return y.companyID }
func (y *FiscalYear) Year() int               { return y.year }
func (y *FiscalYear) StartDate() time.Time    { return y.startDate }
func (y *FiscalYear) EndDate() time.Time      { return y.endDate }
func (y *FiscalYear) Periods() []*FiscalPeriod {
	out := make([]*FiscalPeriod, len(y.periods))
	copy(out, y.periods)
	return out
}

func (p *FiscalPeriod) ID() vo.FiscalPeriodID         { return p.id }
func (p *FiscalPeriod) FiscalYearID() vo.FiscalYearID { return p.fiscalYearID }
func (p *FiscalPeriod) PeriodNo() int                 { return p.periodNo }
func (p *FiscalPeriod) StartDate() time.Time          { return p.startDate }
func (p *FiscalPeriod) EndDate() time.Time            { return p.endDate }
func (p *FiscalPeriod) Status() vo.PeriodStatus       { return p.status }
func (p *FiscalPeriod) IsOpen() bool                  { return p.status == vo.PeriodOpen }

// ChangePeriodStatus ปิด / เปิดงวดตามลำดับ คืนงวดที่ถูกเปลี่ยน
func (y *FiscalYear) ChangePeriodStatus(periodID vo.FiscalPeriodID, status vo.PeriodStatus) (*FiscalPeriod, error) {
	idx := -1
	for i, p := range y.periods {
		if p.id == periodID {
			idx = i
		}
	}
	if idx < 0 {
		return nil, domain.ErrFiscalPeriodNotFound
	}
	switch status {
	case vo.PeriodClosed:
		for _, p := range y.periods[:idx] {
			if p.IsOpen() {
				return nil, domain.ErrPeriodCloseOrder
			}
		}
	case vo.PeriodOpen:
		for _, p := range y.periods[idx+1:] {
			if !p.IsOpen() {
				return nil, domain.ErrPeriodReopenOrder
			}
		}
	default:
		return nil, domain.ErrInvalidStatus
	}
	y.periods[idx].status = status
	return y.periods[idx], nil
}
