// Package dto คือข้อมูลขาเข้า (Input / Filter) และขาออก (read model) ของ use case organization
package dto

import (
	"time"

	"apexis/backend/internal/modules/organization/domain/entities"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// DocumentNumbering คือรูปแบบเลขที่ของชนิดเอกสาร — Custom = บริษัทตั้งเอง (ไม่ใช่ค่าเริ่มต้น)
type DocumentNumbering struct {
	DocType       string
	Name          string
	DefaultPrefix string
	Prefix        string
	Template      string
	Example       string // ตัวอย่างเลขที่ของวันนี้ (ลำดับ 1)
	Custom        bool
}

// ---------- Input ----------

// Actor คือผู้เรียกและบริษัทที่กำลังทำงาน — ทุก use case ของ organization อยู่ภายใต้บริษัทเดียว
type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

// Create / Update / ByID / List เป็น input รูปแบบกลางของ master data ทุกชนิด
// D = entities.*Data, ID = vo.*ID, F = *Filter
type Create[D any] struct {
	Actor
	Data D
}

type Update[ID any, D any] struct {
	Actor
	ID   ID
	Data D
}

type ByID[ID any] struct {
	Actor
	ID ID
}

type List[F any] struct {
	Actor
	Filter F
}

// CreateCompanyInput: ผู้สร้างยังไม่ได้อยู่ในบริษัทใด จึงไม่ใช้ Actor
type CreateCompanyInput struct {
	UserID vo.UserID
	Data   entities.CompanyData
}

// UpdateCompanyInput: PUT /companies/{companyID} — แทนที่ทั้งก้อน (ยกเว้นรหัส)
type UpdateCompanyInput struct {
	Actor
	Data entities.CompanyData
}

// ListMyCompaniesInput: GET /me/companies
type ListMyCompaniesInput struct {
	UserID vo.UserID
	Filter CompanyFilter
}

type CreateFiscalYearInput struct {
	Actor
	Year      int
	StartDate time.Time
}

type ChangePeriodStatusInput struct {
	Actor
	PeriodID vo.FiscalPeriodID
	Status   vo.PeriodStatus
}

// ---------- Filter (จาก URL query) ----------
//
// *SortKeys คือคีย์สาธารณะที่ client ส่งใน ?sort= ได้ (presentation ตรวจ, repository map เป็น column ผ่าน whitelist)

var (
	CompanySortKeys    = []string{"code", "name"}
	DefaultCompanySort = []listing.Sort{{Key: "name"}}
)

// CompanyFilter: GET /me/companies?q=&status=&sort=&page=&page_size=
type CompanyFilter struct {
	Search   string // ค้นจาก code / ชื่อ / ชื่อตามกฎหมาย
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

var (
	BranchSortKeys    = []string{"code", "name", "tax_branch_no"}
	DefaultBranchSort = []listing.Sort{{Key: "code"}}
)

// BranchFilter: GET /branches?q=&status=&sort=&page=&page_size=
type BranchFilter struct {
	Search   string // code / name
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

var (
	UnitSortKeys    = []string{"code", "name"}
	DefaultUnitSort = []listing.Sort{{Key: "code"}}
)

// BusinessUnitFilter: GET /business-units?q=&status=&parent_id=&sort=&page=&page_size=
type BusinessUnitFilter struct {
	Search   string
	Statuses []vo.Status
	ParentID *vo.BusinessUnitID
	Sort     []listing.Sort
	Page     listing.Page
}

// DepartmentFilter: GET /departments?q=&status=&parent_id=&business_unit_id=&branch_id=&sort=&page=&page_size=
type DepartmentFilter struct {
	Search         string
	Statuses       []vo.Status
	ParentID       *vo.DepartmentID
	BusinessUnitID *vo.BusinessUnitID
	BranchID       *vo.BranchID
	Sort           []listing.Sort
	Page           listing.Page
}

var (
	CostCenterSortKeys    = []string{"code", "name", "valid_from"}
	DefaultCostCenterSort = []listing.Sort{{Key: "code"}}
)

// CostCenterFilter: GET /cost-centers?q=&status=&department_id=&active_on=YYYY-MM-DD&sort=&page=&page_size=
type CostCenterFilter struct {
	Search       string
	Statuses     []vo.Status
	DepartmentID *vo.DepartmentID
	ActiveOn     *time.Time // ใช้งานได้ ณ วันที่นี้ (valid_from <= d <= valid_to)
	Sort         []listing.Sort
	Page         listing.Page
}

var (
	ProjectSiteSortKeys    = []string{"code", "name", "start_date"}
	DefaultProjectSiteSort = []listing.Sort{{Key: "code"}}
)

// ProjectSiteFilter: GET /project-sites?q=&status=&branch_id=&cost_center_id=&start_date=&end_date=&sort=&page=&page_size=
// start_date / end_date = โครงการที่ช่วงเวลาซ้อนทับกับช่วงนี้
type ProjectSiteFilter struct {
	Search       string
	Statuses     []vo.ProjectStatus
	BranchID     *vo.BranchID
	CostCenterID *vo.CostCenterID
	StartDate    *time.Time
	EndDate      *time.Time
	Sort         []listing.Sort
	Page         listing.Page
}

var (
	FiscalYearSortKeys    = []string{"year"}
	DefaultFiscalYearSort = []listing.Sort{{Key: "year", Desc: true}}
)

// FiscalYearFilter: GET /fiscal-years?year=&sort=&page=&page_size=
type FiscalYearFilter struct {
	Year *int
	Sort []listing.Sort
	Page listing.Page
}

var (
	FiscalPeriodSortKeys    = []string{"start_date"}
	DefaultFiscalPeriodSort = []listing.Sort{{Key: "start_date"}}
)

// FiscalPeriodFilter: GET /fiscal-periods?fiscal_year=&status=&date=YYYY-MM-DD&sort=&page=&page_size=
type FiscalPeriodFilter struct {
	FiscalYear *int
	Statuses   []vo.PeriodStatus
	Date       *time.Time // งวดที่ครอบคลุมวันที่นี้
	Sort       []listing.Sort
	Page       listing.Page
}

// ---------- Output (read model) ----------

// Ref คือข้อมูลอ้างอิงสั้นๆ ของ record ที่เกี่ยวข้อง (ได้จาก JOIN ใน query เดียว)
type Ref struct {
	ID   string
	Code string
	Name string
}

type Company struct {
	ID                   string
	Code                 string
	Name                 string
	TaxID                string
	LegalNameTH          string
	LegalNameEN          string
	Address              vo.Address
	Phone                string
	Email                string
	Website              string
	BaseCurrency         string
	FiscalYearStartMonth int
	AmountDecimals       int
	QuantityDecimals     int
	BusinessType         string
	Status               string
}

// CompanySettings คือค่าตั้งของบริษัทที่ module อื่นต้องใช้ตอนเขียนข้อมูล (อ่านใน query เดียว)
type CompanySettings struct {
	Active           bool
	BusinessType     string
	AmountDecimals   int
	QuantityDecimals int
}

// PeriodAt คืองวดบัญชีที่ครอบคลุมวันที่หนึ่ง (module อื่นใช้ตรวจก่อนลงรายการ / ทำรายงาน)
type PeriodAt struct {
	PeriodID   vo.FiscalPeriodID
	FiscalYear int
	YearStart  time.Time
	Start      time.Time // วันแรกของงวด
	End        time.Time // วันสุดท้ายของงวด
	Status     vo.PeriodStatus
}

// Dimensions คือ id ของมิติวิเคราะห์ที่ module อื่นอ้างถึง (เช่นบรรทัดรายการบัญชี)
type Dimensions struct {
	Branches     []vo.BranchID
	Departments  []vo.DepartmentID
	CostCenters  []vo.CostCenterID
	ProjectSites []vo.ProjectSiteID
}

// CompanySummary ใช้ในรายการ "บริษัทของฉัน" (เลือกเฉพาะ column ที่แสดง)
type CompanySummary struct {
	ID           string
	Code         string
	Name         string
	TaxID        string
	BusinessType string
	Status       string
}

type Branch struct {
	ID           string
	Code         string
	Name         string
	TaxBranchNo  string
	IsHeadOffice bool
	Address      vo.Address
	Phone        string
	Status       string
}

type BusinessUnit struct {
	ID     string
	Code   string
	Name   string
	Parent *Ref
	Status string
}

type Department struct {
	ID           string
	Code         string
	Name         string
	Parent       *Ref
	BusinessUnit *Ref
	Branch       *Ref
	Status       string
}

type CostCenter struct {
	ID         string
	Code       string
	Name       string
	Department *Ref
	ValidFrom  time.Time
	ValidTo    *time.Time
	Status     string
}

type ProjectSite struct {
	ID         string
	Code       string
	Name       string
	Branch     *Ref
	CostCenter *Ref
	Address    vo.Address
	StartDate  time.Time
	EndDate    *time.Time
	Status     string
}

// FiscalYearStatus: ปีบัญชีพร้อมจำนวนงวดทั้งหมด / งวดที่ยังเปิด (ใช้กับการปิดบัญชีสิ้นปีของ ledger)
type FiscalYearStatus struct {
	Year        int
	StartDate   time.Time
	EndDate     time.Time
	Periods     int
	OpenPeriods int
}

type FiscalYear struct {
	ID        string
	Year      int
	StartDate time.Time
	EndDate   time.Time
	Periods   []FiscalPeriod // มีเฉพาะตอน GET รายตัว / สร้าง
}

type FiscalPeriod struct {
	ID           string
	FiscalYearID string
	FiscalYear   int
	PeriodNo     int
	StartDate    time.Time
	EndDate      time.Time
	Status       string
}
