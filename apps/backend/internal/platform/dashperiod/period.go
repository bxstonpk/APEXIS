// Package dashperiod คือช่วงเวลาของหน้า dashboard: ?start_date=&end_date= (บังคับทั้งคู่)
//
//   - Previous: ช่วงก่อนหน้าที่ยาวเท่ากัน ต่อจาก start_date ถอยหลัง (ใช้เทียบ "เดือนนี้ vs เดือนก่อน")
//   - Months: 12 เดือนปฏิทินที่จบที่เดือนของ end_date (เก่า -> ใหม่) สำหรับกราฟแนวโน้ม
//
// value object ล้วน (ไม่มี I/O) ใช้ได้จาก application ของทุก module
package dashperiod

import (
	"fmt"
	"net/url"
	"time"

	"apexis/backend/internal/platform/listing"
)

// TrendMonths คือจำนวนเดือนของกราฟแนวโน้ม
const TrendMonths = 12

type Period struct {
	Start time.Time
	End   time.Time
	// TrendFrom: เดือนแรกของกราฟ (เช่น เดือนเริ่มปีบัญชี) — ว่าง = 12 เดือนถึงเดือนของ end_date
	TrendFrom time.Time
	// FiscalYear: ปีบัญชีของกราฟ (0 = ไม่มีปีบัญชีครอบคลุม end_date, ใช้ 12 เดือนล่าสุด) — มี = เทียบปีบัญชีก่อนหน้าได้
	FiscalYear int
}

// InFiscalYear: กราฟ = 12 เดือนของปีบัญชีที่เริ่ม yearStart (เดือนเริ่มงวดถึงสิ้นงวด) และเทียบกับปีก่อนหน้า
func (p Period) InFiscalYear(year int, yearStart time.Time) Period {
	p.FiscalYear, p.TrendFrom = year, time.Date(yearStart.Year(), yearStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	return p
}

// Parse อ่าน start_date / end_date (YYYY-MM-DD) — ขาด / ผิดรูปแบบ / start > end = listing.ErrInvalidQuery (400)
func Parse(v url.Values) (Period, error) {
	var p Period
	for _, f := range []struct {
		key string
		dst *time.Time
	}{{"start_date", &p.Start}, {"end_date", &p.End}} {
		raw := v.Get(f.key)
		t, err := listing.ParseDate(raw)
		if raw == "" || err != nil {
			return Period{}, fmt.Errorf("%w: %s is required (YYYY-MM-DD)", listing.ErrInvalidQuery, f.key)
		}
		*f.dst = t
	}
	if p.Start.After(p.End) {
		return Period{}, fmt.Errorf("%w: start_date must not be after end_date", listing.ErrInvalidQuery)
	}
	return p, nil
}

// Days: จำนวนวันในช่วง (รวมวันแรกและวันสุดท้าย)
func (p Period) Days() int { return int(p.End.Sub(p.Start).Hours()/24) + 1 }

// Previous: ช่วงยาวเท่ากันก่อน start_date
func (p Period) Previous() Period {
	end := p.Start.AddDate(0, 0, -1)
	return Period{Start: end.AddDate(0, 0, -(p.Days() - 1)), End: end}
}

// TrendStart: วันแรกของเดือนแรกในกราฟ 12 เดือน (ต้นปีบัญชี หรือ 11 เดือนก่อนเดือนของ end_date)
func (p Period) TrendStart() time.Time {
	if !p.TrendFrom.IsZero() {
		return p.TrendFrom
	}
	first := time.Date(p.End.Year(), p.End.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, -(TrendMonths - 1), 0)
}

// TrendEnd: วันสุดท้ายของเดือนที่ 12 ของกราฟ
func (p Period) TrendEnd() time.Time {
	return p.TrendStart().AddDate(0, TrendMonths, -1)
}

// PreviousTrendStart: เดือนแรกของปีบัญชีก่อนหน้า (ใช้เมื่อ FiscalYear > 0)
func (p Period) PreviousTrendStart() time.Time { return p.TrendStart().AddDate(-1, 0, 0) }

// PreviousMonths: 12 เดือนของปีบัญชีก่อนหน้า เก่า -> ใหม่ (ตรงกับ Months() ทีละช่อง)
func (p Period) PreviousMonths() []string {
	out := make([]string, TrendMonths)
	start := p.PreviousTrendStart()
	for i := range out {
		out[i] = MonthKey(start.AddDate(0, i, 0))
	}
	return out
}

// Months: คีย์ "YYYY-MM" 12 เดือน เก่า -> ใหม่
func (p Period) Months() []string {
	out := make([]string, TrendMonths)
	start := p.TrendStart()
	for i := range out {
		out[i] = MonthKey(start.AddDate(0, i, 0))
	}
	return out
}

func MonthKey(t time.Time) string { return t.Format("2006-01") }

// Earliest: วันแรกสุดที่ dashboard ต้องอ่าน (ช่วงก่อนหน้า หรือเดือนแรกของกราฟ)
func (p Period) Earliest() time.Time {
	prev, trend := p.Previous().Start, p.TrendStart()
	if prev.Before(trend) {
		return prev
	}
	return trend
}

// Latest: วันสุดท้ายที่ dashboard ต้องอ่าน (ปลายเดือนของ end_date)
func (p Period) Latest() time.Time { return p.TrendEnd() }
