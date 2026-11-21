// Package requests คือรูปแบบ JSON ขาเข้า / URL query ของ organization และการแปลงเป็น input ของ use case
//
// presentation validation: field ที่ต้องมี, รูปแบบ UUID / วันที่ (YYYY-MM-DD) / enum
// กฎธุรกิจ (รูปแบบรหัส, เลขที่สาขา, ช่วงวันที่, ลำดับชั้น ฯลฯ) ตรวจใน domain
package requests

import (
	"apexis/backend/internal/platform/numbering"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"apexis/backend/internal/modules/organization/domain/entities"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ErrValidation ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client (HTTP 400)
var ErrValidation = errors.New("validation failed")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

func required(fields map[string]string) error {
	var missing []string
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return invalid("%s is required", strings.Join(missing, ", "))
}

// optionalID แปลง "parent_id": "..." / null เป็น *ID
func optionalID[T ~[16]byte](field string, raw *string) (*T, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := vo.ParseID[T](*raw)
	if err != nil {
		return nil, invalid("%s must be a UUID", field)
	}
	return &id, nil
}

func date(field, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, invalid("%s is required", field)
	}
	t, err := listing.ParseDate(raw)
	if err != nil {
		return time.Time{}, invalid("%s must be a date in YYYY-MM-DD format", field)
	}
	return t, nil
}

func optionalDate(field string, raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := date(field, *raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// status: POST ไม่ส่ง = ค่าเริ่มต้น, PUT ต้องส่ง (แทนที่ทั้งก้อน)
func status[T ~string](raw string, def T, isCreate bool, parse func(string) (T, error)) (T, error) {
	if raw == "" {
		if isCreate {
			return def, nil
		}
		return "", invalid("status is required")
	}
	s, err := parse(raw)
	if err != nil {
		return "", invalid("status has an invalid value %q", raw)
	}
	return s, nil
}

// ---------- shared ----------

type Address struct {
	Line        string `json:"line"`
	SubDistrict string `json:"sub_district"`
	District    string `json:"district"`
	Province    string `json:"province"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"`
}

func (a Address) toDomain() vo.Address {
	return vo.Address{
		Line: a.Line, SubDistrict: a.SubDistrict, District: a.District,
		Province: a.Province, PostalCode: a.PostalCode, Country: a.Country,
	}
}

// ---------- company ----------

// POST /companies และ PUT /companies/{companyID}
//
// POST: ต้องมี code, name — ที่เหลือมีค่าเริ่มต้น (base_currency THB, fiscal_year_start_month 1,
// status active, legal_name_th = name)
// PUT: แทนที่ทั้งก้อน ต้องส่ง name, base_currency, fiscal_year_start_month, amount_decimals, quantity_decimals, status
// — code แก้ไม่ได้ (ไม่ต้องส่ง)
type Company struct {
	Code                 string  `json:"code"`
	Name                 string  `json:"name"`
	TaxID                string  `json:"tax_id"`
	LegalNameTH          string  `json:"legal_name_th"`
	LegalNameEN          string  `json:"legal_name_en"`
	Address              Address `json:"address"`
	Phone                string  `json:"phone"`
	Email                string  `json:"email"`
	Website              string  `json:"website"`
	BaseCurrency         string  `json:"base_currency"`
	FiscalYearStartMonth int     `json:"fiscal_year_start_month"`
	AmountDecimals       *int    `json:"amount_decimals"`   // POST ไม่ส่ง = 2, PUT ต้องส่ง
	QuantityDecimals     *int    `json:"quantity_decimals"` // POST ไม่ส่ง = 2, PUT ต้องส่ง
	Status               string  `json:"status"`
}

// decimals: 0 เป็นค่าที่ถูกต้อง จึงรับเป็น pointer เพื่อแยก "ไม่ได้ส่ง"
func decimals(field string, v *int, isCreate bool) (int, error) {
	switch {
	case v != nil:
		return *v, nil
	case isCreate:
		return 2, nil
	}
	return 0, invalid("%s is required", field)
}

func (r Company) ToData(isCreate bool) (entities.CompanyData, error) {
	fields := map[string]string{"name": r.Name}
	if isCreate {
		fields["code"] = r.Code
		if r.BaseCurrency == "" {
			r.BaseCurrency = "THB"
		}
		if r.FiscalYearStartMonth == 0 {
			r.FiscalYearStartMonth = 1
		}
	} else {
		fields["base_currency"] = r.BaseCurrency
	}
	if err := required(fields); err != nil {
		return entities.CompanyData{}, err
	}
	if r.FiscalYearStartMonth == 0 {
		return entities.CompanyData{}, invalid("fiscal_year_start_month is required")
	}
	st, err := status(r.Status, vo.StatusActive, isCreate, vo.ParseStatus)
	if err != nil {
		return entities.CompanyData{}, err
	}
	amountDecimals, err := decimals("amount_decimals", r.AmountDecimals, isCreate)
	if err != nil {
		return entities.CompanyData{}, err
	}
	quantityDecimals, err := decimals("quantity_decimals", r.QuantityDecimals, isCreate)
	if err != nil {
		return entities.CompanyData{}, err
	}
	return entities.CompanyData{
		Code: r.Code, Name: r.Name, TaxID: r.TaxID, LegalNameTH: r.LegalNameTH, LegalNameEN: r.LegalNameEN,
		Address: r.Address.toDomain(), Phone: r.Phone, Email: r.Email, Website: r.Website,
		BaseCurrency: r.BaseCurrency, FiscalYearStartMonth: r.FiscalYearStartMonth,
		AmountDecimals: amountDecimals, QuantityDecimals: quantityDecimals, Status: st,
	}, nil
}

// ---------- branch ----------

type Branch struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	TaxBranchNo string  `json:"tax_branch_no"`
	Address     Address `json:"address"`
	Phone       string  `json:"phone"`
	Status      string  `json:"status"`
}

func (r Branch) ToData(isCreate bool) (entities.BranchData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name, "tax_branch_no": r.TaxBranchNo}); err != nil {
		return entities.BranchData{}, err
	}
	st, err := status(r.Status, vo.StatusActive, isCreate, vo.ParseStatus)
	if err != nil {
		return entities.BranchData{}, err
	}
	return entities.BranchData{
		Code: r.Code, Name: r.Name, TaxBranchNo: r.TaxBranchNo, Address: r.Address.toDomain(),
		Phone: r.Phone, Status: st,
	}, nil
}

// ---------- business unit ----------

type BusinessUnit struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
	Status   string  `json:"status"`
}

func (r BusinessUnit) ToData(isCreate bool) (entities.BusinessUnitData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return entities.BusinessUnitData{}, err
	}
	parent, err := optionalID[vo.BusinessUnitID]("parent_id", r.ParentID)
	if err != nil {
		return entities.BusinessUnitData{}, err
	}
	st, err := status(r.Status, vo.StatusActive, isCreate, vo.ParseStatus)
	if err != nil {
		return entities.BusinessUnitData{}, err
	}
	return entities.BusinessUnitData{Code: r.Code, Name: r.Name, ParentID: parent, Status: st}, nil
}

// ---------- department ----------

type Department struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	ParentID       *string `json:"parent_id"`
	BusinessUnitID *string `json:"business_unit_id"`
	BranchID       *string `json:"branch_id"`
	Status         string  `json:"status"`
}

func (r Department) ToData(isCreate bool) (entities.DepartmentData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return entities.DepartmentData{}, err
	}
	parent, err := optionalID[vo.DepartmentID]("parent_id", r.ParentID)
	if err != nil {
		return entities.DepartmentData{}, err
	}
	unit, err := optionalID[vo.BusinessUnitID]("business_unit_id", r.BusinessUnitID)
	if err != nil {
		return entities.DepartmentData{}, err
	}
	branch, err := optionalID[vo.BranchID]("branch_id", r.BranchID)
	if err != nil {
		return entities.DepartmentData{}, err
	}
	st, err := status(r.Status, vo.StatusActive, isCreate, vo.ParseStatus)
	if err != nil {
		return entities.DepartmentData{}, err
	}
	return entities.DepartmentData{
		Code: r.Code, Name: r.Name, ParentID: parent, BusinessUnitID: unit, BranchID: branch, Status: st,
	}, nil
}

// ---------- cost center ----------

type CostCenter struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	DepartmentID *string `json:"department_id"`
	ValidFrom    string  `json:"valid_from"` // YYYY-MM-DD
	ValidTo      *string `json:"valid_to"`   // YYYY-MM-DD หรือ null
	Status       string  `json:"status"`
}

func (r CostCenter) ToData(isCreate bool) (entities.CostCenterData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return entities.CostCenterData{}, err
	}
	dept, err := optionalID[vo.DepartmentID]("department_id", r.DepartmentID)
	if err != nil {
		return entities.CostCenterData{}, err
	}
	from, err := date("valid_from", r.ValidFrom)
	if err != nil {
		return entities.CostCenterData{}, err
	}
	to, err := optionalDate("valid_to", r.ValidTo)
	if err != nil {
		return entities.CostCenterData{}, err
	}
	st, err := status(r.Status, vo.StatusActive, isCreate, vo.ParseStatus)
	if err != nil {
		return entities.CostCenterData{}, err
	}
	return entities.CostCenterData{
		Code: r.Code, Name: r.Name, DepartmentID: dept, ValidFrom: from, ValidTo: to, Status: st,
	}, nil
}

// ---------- project / site ----------

type ProjectSite struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	BranchID     *string `json:"branch_id"`
	CostCenterID *string `json:"cost_center_id"`
	Address      Address `json:"address"`
	StartDate    string  `json:"start_date"`
	EndDate      *string `json:"end_date"`
	Status       string  `json:"status"`
}

func (r ProjectSite) ToData(isCreate bool) (entities.ProjectSiteData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return entities.ProjectSiteData{}, err
	}
	branch, err := optionalID[vo.BranchID]("branch_id", r.BranchID)
	if err != nil {
		return entities.ProjectSiteData{}, err
	}
	cc, err := optionalID[vo.CostCenterID]("cost_center_id", r.CostCenterID)
	if err != nil {
		return entities.ProjectSiteData{}, err
	}
	start, err := date("start_date", r.StartDate)
	if err != nil {
		return entities.ProjectSiteData{}, err
	}
	end, err := optionalDate("end_date", r.EndDate)
	if err != nil {
		return entities.ProjectSiteData{}, err
	}
	st, err := status(r.Status, vo.ProjectPlanned, isCreate, vo.ParseProjectStatus)
	if err != nil {
		return entities.ProjectSiteData{}, err
	}
	return entities.ProjectSiteData{
		Code: r.Code, Name: r.Name, BranchID: branch, CostCenterID: cc, Address: r.Address.toDomain(),
		StartDate: start, EndDate: end, Status: st,
	}, nil
}

// ---------- fiscal ----------

// POST /companies/{companyID}/fiscal-years
type CreateFiscalYear struct {
	Year      int    `json:"year"`
	StartDate string `json:"start_date"` // ต้องเป็นวันที่ 1 ของเดือน
}

func (r CreateFiscalYear) Parse() (int, time.Time, error) {
	if r.Year == 0 {
		return 0, time.Time{}, invalid("year is required")
	}
	start, err := date("start_date", r.StartDate)
	return r.Year, start, err
}

// PATCH /companies/{companyID}/fiscal-periods/{fiscalPeriodID}
type UpdateFiscalPeriod struct {
	Status string `json:"status"` // open | closed
}

func (r UpdateFiscalPeriod) Parse() (vo.PeriodStatus, error) {
	return status(r.Status, "", false, vo.ParsePeriodStatus)
}

// Numbering: PUT /document-numberings/{docType} — {"prefix": "PV", "template": "{prefix}{yy}{mm}-{seq:5}"}
type Numbering struct {
	Prefix   string `json:"prefix"`
	Template string `json:"template"`
}

func (r Numbering) ToFormat() (numbering.Format, error) {
	if err := required(map[string]string{"template": r.Template}); err != nil {
		return numbering.Format{}, err
	}
	return numbering.Format{Prefix: r.Prefix, Template: r.Template}, nil
}
