package requests

import (
	"fmt"
	"net/url"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// การค้นหา / รายงานทุกหน้าใช้ GET + URL query เท่านั้น — frontend เก็บ query เดียวกันไว้ใน URL ของหน้า
// แล้วส่งต่อให้ API ตรงๆ เพื่อให้กด Back / refresh / bookmark แล้วได้หน้าเดิม
//
// วันที่ใน URL ใช้ YYYY-MM-DD (ค.ศ.) เช่น ?start_date=2026-12-19&end_date=2026-12-31
// parameter ที่ไม่รู้จักถูกเพิกเฉย, ค่าที่ไม่ถูกต้องของ parameter ที่รู้จัก -> 400

// query สะสม error ตัวแรก เพื่อให้ parser แต่ละตัวอ่านง่าย
type query struct {
	v   url.Values
	err error
}

func parse[T any](q *query, fn func() (T, error)) T {
	var zero T
	if q.err != nil {
		return zero
	}
	v, err := fn()
	if err != nil {
		q.err = err
		return zero
	}
	return v
}

func (q *query) search() string {
	return parse(q, func() (string, error) { return listing.ParseSearch(q.v) })
}

func (q *query) sort(keys []string, def []listing.Sort) []listing.Sort {
	return parse(q, func() ([]listing.Sort, error) { return listing.ParseSort(q.v, keys, def) })
}

func (q *query) page() listing.Page {
	return parse(q, func() (listing.Page, error) { return listing.ParsePage(q.v) })
}

func (q *query) statuses() []vo.Status {
	return parse(q, func() ([]vo.Status, error) { return listing.ParseList(q.v, "status", vo.ParseStatus) })
}

func optionalIDParam[T ~[16]byte](q *query, key string) *T {
	return parse(q, func() (*T, error) { return listing.ParseOptional(q.v, key, vo.ParseID[T]) })
}

// requiredDate: parameter วันที่ที่รายงานต้องมี
func (q *query) requiredDate(key string) time.Time {
	return parse(q, func() (time.Time, error) {
		raw := q.v.Get(key)
		if raw == "" {
			return time.Time{}, fmt.Errorf("%w: %s is required (YYYY-MM-DD)", listing.ErrInvalidQuery, key)
		}
		t, err := listing.ParseDate(raw)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %s has an invalid value %q", listing.ErrInvalidQuery, key, raw)
		}
		return t, nil
	})
}

func (q *query) dateRange() (time.Time, time.Time) {
	start, end := q.requiredDate("start_date"), q.requiredDate("end_date")
	if q.err == nil && start.After(end) {
		q.err = fmt.Errorf("%w: start_date must not be after end_date", listing.ErrInvalidQuery)
	}
	return start, end
}

// GET /accounts?q=&type=asset,liability&status=&postable=true&parent_id=&sort=code&page=&page_size=
func ParseAccountFilter(v url.Values) (dto.AccountFilter, error) {
	q := &query{v: v}
	f := dto.AccountFilter{
		Search:   q.search(),
		Types:    parse(q, func() ([]vo.AccountType, error) { return listing.ParseList(v, "type", vo.ParseAccountType) }),
		Statuses: q.statuses(),
		Postable: parse(q, func() (*bool, error) { return listing.ParseOptional(v, "postable", listing.ParseBool) }),
		ParentID: optionalIDParam[vo.AccountID](q, "parent_id"),
		Sort:     q.sort(dto.AccountSortKeys, dto.DefaultAccountSort), Page: q.page(),
	}
	return f, q.err
}

// GET /journals?q=&status=&sort=&page=&page_size=
func ParseJournalFilter(v url.Values) (dto.JournalFilter, error) {
	q := &query{v: v}
	f := dto.JournalFilter{
		Search: q.search(), Statuses: q.statuses(),
		Sort: q.sort(dto.JournalSortKeys, dto.DefaultJournalSort), Page: q.page(),
	}
	return f, q.err
}

// GET /journal-entries?q=&status=draft,posted&journal_id=&account_id=&start_date=&end_date=&sort=-entry_date&page=&page_size=
func ParseEntryFilter(v url.Values) (dto.EntryFilter, error) {
	q := &query{v: v}
	f := dto.EntryFilter{
		Search:    q.search(),
		Statuses:  parse(q, func() ([]vo.EntryStatus, error) { return listing.ParseList(v, "status", vo.ParseEntryStatus) }),
		JournalID: optionalIDParam[vo.JournalID](q, "journal_id"),
		AccountID: optionalIDParam[vo.AccountID](q, "account_id"),
		Sort:      q.sort(dto.EntrySortKeys, dto.DefaultEntrySort), Page: q.page(),
	}
	if q.err == nil {
		f.StartDate, f.EndDate, q.err = listing.ParseDateRange(v, "start_date", "end_date")
	}
	return f, q.err
}

// GET /adjusting-entries?template=accrued_revenue,...&status=&start_date=&end_date=&q=&sort=&page= — รายการปรับปรุงสิ้นงวดเท่านั้น
func ParseAdjustingFilter(v url.Values) (dto.EntryFilter, error) {
	f, err := ParseEntryFilter(v)
	if err != nil {
		return f, err
	}
	f.Adjustments = true
	ts, err := listing.ParseList(v, "template", func(s string) (string, error) {
		t, err := entities.ParseTemplate(s)
		return string(t), err
	})
	if err != nil {
		return f, fmt.Errorf("%w: template must be one of accrued_revenue, accrued_expense, deferred_revenue, prepaid_expense, other", listing.ErrInvalidQuery)
	}
	f.Templates = ts
	return f, nil
}

// Book คือชนิดรายงาน — หน้ารายงานของ frontend เป็นหน้าเดียว เลือกรายงานด้วย ?book=
type Book string

const (
	BookTrialBalance    Book = "trial-balance"    // งบทดลอง: start_date, end_date
	BookBalanceSheet    Book = "balance-sheet"    // งบแสดงฐานะการเงิน ณ end_date (start_date ไม่ใช้)
	BookIncomeStatement Book = "income-statement" // งบกำไรขาดทุน: start_date, end_date
	BookGeneralLedger   Book = "general-ledger"   // บัญชีแยกประเภท: account_id, start_date, end_date, page, page_size
	// งบการเงินตามแบบกรมพัฒนาธุรกิจการค้า ณ end_date เทียบปีก่อน (งบกำไรขาดทุน = ต้นปีบัญชีถึง end_date)
	BookDBDBalanceSheet    Book = "dbd-balance-sheet"
	BookDBDIncomeStatement Book = "dbd-income-statement"
)

var Books = []Book{BookTrialBalance, BookBalanceSheet, BookIncomeStatement, BookGeneralLedger, BookDBDBalanceSheet, BookDBDIncomeStatement}

// ReportQuery คือ URL query ของ GET /reports — ทุกรายงานใช้ชื่อ parameter ชุดเดียวกัน
// parameter ที่รายงานนั้นไม่ใช้ถูกเพิกเฉย จึงเปลี่ยน book โดยคงค่าอื่นใน URL ไว้ได้
type ReportQuery struct {
	Book      Book
	Start     time.Time // ไม่ใช้ใน balance-sheet
	End       time.Time
	AccountID vo.AccountID // เฉพาะ general-ledger
	Page      listing.Page // เฉพาะ general-ledger
}

// ParseReport อ่าน
//
//	GET /reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31
//	GET /reports?book=balance-sheet&end_date=2026-12-31
//	GET /reports?book=income-statement&start_date=2026-01-01&end_date=2026-12-31
//	GET /reports?book=general-ledger&account_id=...&start_date=2026-01-01&end_date=2026-12-31&page=1&page_size=50
func ParseReport(v url.Values) (ReportQuery, error) {
	q := &query{v: v}
	out := ReportQuery{Book: Book(v.Get("book"))}
	switch out.Book {
	case BookTrialBalance, BookIncomeStatement:
		out.Start, out.End = q.dateRange()
	case BookBalanceSheet, BookDBDBalanceSheet, BookDBDIncomeStatement:
		out.End = q.requiredDate("end_date")
	case BookGeneralLedger:
		id := optionalIDParam[vo.AccountID](q, "account_id")
		if q.err == nil && id == nil {
			q.err = fmt.Errorf("%w: account_id is required for book=%s", listing.ErrInvalidQuery, out.Book)
		}
		if id != nil {
			out.AccountID = *id
		}
		out.Start, out.End = q.dateRange()
		out.Page = q.page()
	case "":
		return out, fmt.Errorf("%w: book is required (%s)", listing.ErrInvalidQuery, bookList())
	default:
		return out, fmt.Errorf("%w: book must be one of %s", listing.ErrInvalidQuery, bookList())
	}
	return out, q.err
}

func bookList() string {
	s := ""
	for i, b := range Books {
		if i > 0 {
			s += ", "
		}
		s += string(b)
	}
	return s
}

// ParseDateRange: GET /dashboards/accounting?start_date=&end_date= — ต้องมีทั้งคู่ (YYYY-MM-DD) และ start <= end
func ParseDateRange(v url.Values) (time.Time, time.Time, error) {
	q := &query{v: v}
	start, end := q.dateRange()
	return start, end, q.err
}
