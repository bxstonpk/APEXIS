package requests

import (
	"net/url"
	"strconv"
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// การค้นหาทุกหน้าใช้ GET + URL query เท่านั้น — frontend เก็บ query เดียวกันไว้ใน URL ของหน้า
// แล้วส่งต่อให้ API ตรงๆ เพื่อให้กด Back / refresh / bookmark แล้วได้หน้าการค้นหาเดิม
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

// GET /me/companies?q=&status=active&sort=name&page=1&page_size=20
func ParseCompanyFilter(v url.Values) (dto.CompanyFilter, error) {
	q := &query{v: v}
	f := dto.CompanyFilter{
		Search: q.search(), Statuses: q.statuses(),
		Sort: q.sort(dto.CompanySortKeys, dto.DefaultCompanySort), Page: q.page(),
	}
	return f, q.err
}

// GET /branches?q=&status=active,inactive&sort=code&page=1&page_size=20
func ParseBranchFilter(v url.Values) (dto.BranchFilter, error) {
	q := &query{v: v}
	f := dto.BranchFilter{
		Search: q.search(), Statuses: q.statuses(),
		Sort: q.sort(dto.BranchSortKeys, dto.DefaultBranchSort), Page: q.page(),
	}
	return f, q.err
}

// GET /business-units?q=&status=&parent_id=&sort=&page=&page_size=
func ParseBusinessUnitFilter(v url.Values) (dto.BusinessUnitFilter, error) {
	q := &query{v: v}
	f := dto.BusinessUnitFilter{
		Search: q.search(), Statuses: q.statuses(), ParentID: optionalIDParam[vo.BusinessUnitID](q, "parent_id"),
		Sort: q.sort(dto.UnitSortKeys, dto.DefaultUnitSort), Page: q.page(),
	}
	return f, q.err
}

// GET /departments?q=&status=&parent_id=&business_unit_id=&branch_id=&sort=&page=&page_size=
func ParseDepartmentFilter(v url.Values) (dto.DepartmentFilter, error) {
	q := &query{v: v}
	f := dto.DepartmentFilter{
		Search: q.search(), Statuses: q.statuses(),
		ParentID:       optionalIDParam[vo.DepartmentID](q, "parent_id"),
		BusinessUnitID: optionalIDParam[vo.BusinessUnitID](q, "business_unit_id"),
		BranchID:       optionalIDParam[vo.BranchID](q, "branch_id"),
		Sort:           q.sort(dto.UnitSortKeys, dto.DefaultUnitSort), Page: q.page(),
	}
	return f, q.err
}

// GET /cost-centers?q=&status=&department_id=&active_on=2026-12-19&sort=&page=&page_size=
func ParseCostCenterFilter(v url.Values) (dto.CostCenterFilter, error) {
	q := &query{v: v}
	f := dto.CostCenterFilter{
		Search: q.search(), Statuses: q.statuses(),
		DepartmentID: optionalIDParam[vo.DepartmentID](q, "department_id"),
		ActiveOn:     parse(q, func() (*time.Time, error) { return listing.ParseOptional(v, "active_on", listing.ParseDate) }),
		Sort:         q.sort(dto.CostCenterSortKeys, dto.DefaultCostCenterSort), Page: q.page(),
	}
	return f, q.err
}

// GET /project-sites?q=&status=planned,active&branch_id=&cost_center_id=&start_date=&end_date=&sort=&page=&page_size=
func ParseProjectSiteFilter(v url.Values) (dto.ProjectSiteFilter, error) {
	q := &query{v: v}
	f := dto.ProjectSiteFilter{
		Search:       q.search(),
		Statuses:     parse(q, func() ([]vo.ProjectStatus, error) { return listing.ParseList(v, "status", vo.ParseProjectStatus) }),
		BranchID:     optionalIDParam[vo.BranchID](q, "branch_id"),
		CostCenterID: optionalIDParam[vo.CostCenterID](q, "cost_center_id"),
		Sort:         q.sort(dto.ProjectSiteSortKeys, dto.DefaultProjectSiteSort), Page: q.page(),
	}
	if q.err == nil {
		f.StartDate, f.EndDate, q.err = listing.ParseDateRange(v, "start_date", "end_date")
	}
	return f, q.err
}

// GET /fiscal-years?year=2026&sort=-year&page=&page_size=
func ParseFiscalYearFilter(v url.Values) (dto.FiscalYearFilter, error) {
	q := &query{v: v}
	f := dto.FiscalYearFilter{
		Year: parse(q, func() (*int, error) { return listing.ParseOptional(v, "year", strconv.Atoi) }),
		Sort: q.sort(dto.FiscalYearSortKeys, dto.DefaultFiscalYearSort), Page: q.page(),
	}
	return f, q.err
}

// GET /fiscal-periods?fiscal_year=2026&status=open&date=2026-12-19&sort=start_date&page=&page_size=
func ParseFiscalPeriodFilter(v url.Values) (dto.FiscalPeriodFilter, error) {
	q := &query{v: v}
	f := dto.FiscalPeriodFilter{
		FiscalYear: parse(q, func() (*int, error) { return listing.ParseOptional(v, "fiscal_year", strconv.Atoi) }),
		Statuses:   parse(q, func() ([]vo.PeriodStatus, error) { return listing.ParseList(v, "status", vo.ParsePeriodStatus) }),
		Date:       parse(q, func() (*time.Time, error) { return listing.ParseOptional(v, "date", listing.ParseDate) }),
		Sort:       q.sort(dto.FiscalPeriodSortKeys, dto.DefaultFiscalPeriodSort), Page: q.page(),
	}
	return f, q.err
}
