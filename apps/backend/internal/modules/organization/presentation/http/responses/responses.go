// Package responses คือรูปแบบ JSON ขาออกของ organization (แปลงจาก application/dto)
// วันที่ส่งออกเป็น "YYYY-MM-DD" (ค.ศ.) — envelope (data / meta / error) อยู่ที่ platform/httpx
package responses

import (
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func ref(r *dto.Ref) *Ref {
	if r == nil {
		return nil
	}
	return &Ref{ID: r.ID, Code: r.Code, Name: r.Name}
}

type Address struct {
	Line        string `json:"line"`
	SubDistrict string `json:"sub_district"`
	District    string `json:"district"`
	Province    string `json:"province"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"`
}

func address(a vo.Address) Address {
	return Address{
		Line: a.Line, SubDistrict: a.SubDistrict, District: a.District,
		Province: a.Province, PostalCode: a.PostalCode, Country: a.Country,
	}
}

func date(t time.Time) string { return t.Format(time.DateOnly) }

func datePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := date(*t)
	return &s
}

type Company struct {
	ID                   string  `json:"id"`
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
	AmountDecimals       int     `json:"amount_decimals"`
	QuantityDecimals     int     `json:"quantity_decimals"`
	Status               string  `json:"status"`
}

func FromCompany(c dto.Company) Company {
	return Company{
		ID: c.ID, Code: c.Code, Name: c.Name, TaxID: c.TaxID,
		LegalNameTH: c.LegalNameTH, LegalNameEN: c.LegalNameEN, Address: address(c.Address),
		Phone: c.Phone, Email: c.Email, Website: c.Website,
		BaseCurrency: c.BaseCurrency, FiscalYearStartMonth: c.FiscalYearStartMonth,
		AmountDecimals: c.AmountDecimals, QuantityDecimals: c.QuantityDecimals, Status: c.Status,
	}
}

type CompanySummary struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	TaxID  string `json:"tax_id"`
	Status string `json:"status"`
}

// Letterhead คือข้อมูลหัวเอกสารที่พิมพ์บนใบเสนอราคา / ใบกำกับภาษี
type Letterhead struct {
	Name        string  `json:"name"`
	LegalNameTH string  `json:"legal_name_th"`
	LegalNameEN string  `json:"legal_name_en"`
	TaxID       string  `json:"tax_id"`
	Address     Address `json:"address"`
	Phone       string  `json:"phone"`
	Email       string  `json:"email"`
	Website     string  `json:"website"`
}

func FromLetterhead(c dto.Company) Letterhead {
	return Letterhead{Name: c.Name, LegalNameTH: c.LegalNameTH, LegalNameEN: c.LegalNameEN, TaxID: c.TaxID, Address: address(c.Address),
		Phone: c.Phone, Email: c.Email, Website: c.Website}
}

func FromCompanySummary(c dto.CompanySummary) CompanySummary {
	return CompanySummary{ID: c.ID, Code: c.Code, Name: c.Name, TaxID: c.TaxID, Status: c.Status}
}

type Branch struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	TaxBranchNo  string  `json:"tax_branch_no"`
	IsHeadOffice bool    `json:"is_head_office"`
	Address      Address `json:"address"`
	Phone        string  `json:"phone"`
	Status       string  `json:"status"`
}

func FromBranch(b dto.Branch) Branch {
	return Branch{
		ID: b.ID, Code: b.Code, Name: b.Name, TaxBranchNo: b.TaxBranchNo, IsHeadOffice: b.IsHeadOffice,
		Address: address(b.Address), Phone: b.Phone, Status: b.Status,
	}
}

type BusinessUnit struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Parent *Ref   `json:"parent"`
	Status string `json:"status"`
}

func FromBusinessUnit(u dto.BusinessUnit) BusinessUnit {
	return BusinessUnit{ID: u.ID, Code: u.Code, Name: u.Name, Parent: ref(u.Parent), Status: u.Status}
}

type Department struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Parent       *Ref   `json:"parent"`
	BusinessUnit *Ref   `json:"business_unit"`
	Branch       *Ref   `json:"branch"`
	Status       string `json:"status"`
}

func FromDepartment(d dto.Department) Department {
	return Department{
		ID: d.ID, Code: d.Code, Name: d.Name, Parent: ref(d.Parent),
		BusinessUnit: ref(d.BusinessUnit), Branch: ref(d.Branch), Status: d.Status,
	}
}

type CostCenter struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Department *Ref    `json:"department"`
	ValidFrom  string  `json:"valid_from"`
	ValidTo    *string `json:"valid_to"`
	Status     string  `json:"status"`
}

func FromCostCenter(c dto.CostCenter) CostCenter {
	return CostCenter{
		ID: c.ID, Code: c.Code, Name: c.Name, Department: ref(c.Department),
		ValidFrom: date(c.ValidFrom), ValidTo: datePtr(c.ValidTo), Status: c.Status,
	}
}

type ProjectSite struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Branch     *Ref    `json:"branch"`
	CostCenter *Ref    `json:"cost_center"`
	Address    Address `json:"address"`
	StartDate  string  `json:"start_date"`
	EndDate    *string `json:"end_date"`
	Status     string  `json:"status"`
}

func FromProjectSite(p dto.ProjectSite) ProjectSite {
	return ProjectSite{
		ID: p.ID, Code: p.Code, Name: p.Name, Branch: ref(p.Branch), CostCenter: ref(p.CostCenter),
		Address: address(p.Address), StartDate: date(p.StartDate), EndDate: datePtr(p.EndDate), Status: p.Status,
	}
}

type FiscalYear struct {
	ID        string         `json:"id"`
	Year      int            `json:"year"`
	StartDate string         `json:"start_date"`
	EndDate   string         `json:"end_date"`
	Periods   []FiscalPeriod `json:"periods,omitempty"`
}

func FromFiscalYear(y dto.FiscalYear) FiscalYear {
	out := FiscalYear{ID: y.ID, Year: y.Year, StartDate: date(y.StartDate), EndDate: date(y.EndDate)}
	for _, p := range y.Periods {
		out.Periods = append(out.Periods, FromFiscalPeriod(p))
	}
	return out
}

type FiscalPeriod struct {
	ID           string `json:"id"`
	FiscalYearID string `json:"fiscal_year_id"`
	FiscalYear   int    `json:"fiscal_year"`
	PeriodNo     int    `json:"period_no"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Status       string `json:"status"`
}

func FromFiscalPeriod(p dto.FiscalPeriod) FiscalPeriod {
	return FiscalPeriod{
		ID: p.ID, FiscalYearID: p.FiscalYearID, FiscalYear: p.FiscalYear, PeriodNo: p.PeriodNo,
		StartDate: date(p.StartDate), EndDate: date(p.EndDate), Status: p.Status,
	}
}

type DocumentNumbering struct {
	DocType       string `json:"doc_type"`
	Name          string `json:"name"`
	DefaultPrefix string `json:"default_prefix"`
	Prefix        string `json:"prefix"`
	Template      string `json:"template"`
	Example       string `json:"example"`
	Custom        bool   `json:"custom"`
}

func FromDocumentNumbering(n dto.DocumentNumbering) DocumentNumbering {
	return DocumentNumbering{
		DocType: n.DocType, Name: n.Name, DefaultPrefix: n.DefaultPrefix, Prefix: n.Prefix, Template: n.Template,
		Example: n.Example, Custom: n.Custom,
	}
}
