package entities

import (
	"slices"
	"time"

	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// ---------- Business Unit ----------

type BusinessUnitData struct {
	Code     string
	Name     string
	ParentID *vo.BusinessUnitID // หน่วยธุรกิจแม่ (เป็นลำดับชั้นได้)
	Status   vo.Status
}

// BusinessUnit คือหน่วยธุรกิจ / สายธุรกิจ (เช่น ค้าปลีก, ก่อสร้าง)
// การตรวจ parent วนเป็นวง ใช้ services.EnsureNoCycle ก่อน Update
type BusinessUnit struct {
	id        vo.BusinessUnitID
	companyID vo.CompanyID
	data      BusinessUnitData
}

func NewBusinessUnit(companyID vo.CompanyID, d BusinessUnitData) (*BusinessUnit, error) {
	u := &BusinessUnit{id: vo.NewID[vo.BusinessUnitID](), companyID: companyID}
	if err := u.Update(d); err != nil {
		return nil, err
	}
	return u, nil
}

func RehydrateBusinessUnit(id vo.BusinessUnitID, companyID vo.CompanyID, d BusinessUnitData) *BusinessUnit {
	return &BusinessUnit{id: id, companyID: companyID, data: d}
}

func (u *BusinessUnit) ID() vo.BusinessUnitID   { return u.id }
func (u *BusinessUnit) CompanyID() vo.CompanyID { return u.companyID }
func (u *BusinessUnit) Data() BusinessUnitData  { return u.data }

func (u *BusinessUnit) Update(d BusinessUnitData) error {
	code, name, err := codeAndName(d.Code, d.Name)
	if err != nil {
		return err
	}
	if err := ensureStatus(d.Status); err != nil {
		return err
	}
	if d.ParentID != nil && *d.ParentID == u.id {
		return domain.ErrHierarchyCycle
	}
	u.data = BusinessUnitData{Code: code, Name: name, ParentID: d.ParentID, Status: d.Status}
	return nil
}

// ---------- Department ----------

type DepartmentData struct {
	Code           string
	Name           string
	ParentID       *vo.DepartmentID   // แผนกแม่ (เป็นลำดับชั้นได้)
	BusinessUnitID *vo.BusinessUnitID // สังกัดหน่วยธุรกิจ (ถ้ามี)
	BranchID       *vo.BranchID       // ประจำสาขา (ถ้ามี)
	Status         vo.Status
}

type Department struct {
	id        vo.DepartmentID
	companyID vo.CompanyID
	data      DepartmentData
}

func NewDepartment(companyID vo.CompanyID, d DepartmentData) (*Department, error) {
	dep := &Department{id: vo.NewID[vo.DepartmentID](), companyID: companyID}
	if err := dep.Update(d); err != nil {
		return nil, err
	}
	return dep, nil
}

func RehydrateDepartment(id vo.DepartmentID, companyID vo.CompanyID, d DepartmentData) *Department {
	return &Department{id: id, companyID: companyID, data: d}
}

func (d *Department) ID() vo.DepartmentID     { return d.id }
func (d *Department) CompanyID() vo.CompanyID { return d.companyID }
func (d *Department) Data() DepartmentData    { return d.data }

func (d *Department) Update(in DepartmentData) error {
	code, name, err := codeAndName(in.Code, in.Name)
	if err != nil {
		return err
	}
	if err := ensureStatus(in.Status); err != nil {
		return err
	}
	if in.ParentID != nil && *in.ParentID == d.id {
		return domain.ErrHierarchyCycle
	}
	in.Code, in.Name = code, name
	d.data = in
	return nil
}

// ---------- Cost Center ----------

type CostCenterData struct {
	Code         string
	Name         string
	DepartmentID *vo.DepartmentID // แผนกเจ้าของศูนย์ต้นทุน (ถ้ามี)
	ValidFrom    time.Time
	ValidTo      *time.Time // nil = ไม่มีวันสิ้นสุด
	Status       vo.Status
}

// CostCenter คือศูนย์ต้นทุน ใช้ปันส่วนค่าใช้จ่ายในบัญชีบริหาร มีช่วงวันที่ใช้งาน
type CostCenter struct {
	id        vo.CostCenterID
	companyID vo.CompanyID
	data      CostCenterData
}

func NewCostCenter(companyID vo.CompanyID, d CostCenterData) (*CostCenter, error) {
	c := &CostCenter{id: vo.NewID[vo.CostCenterID](), companyID: companyID}
	if err := c.Update(d); err != nil {
		return nil, err
	}
	return c, nil
}

func RehydrateCostCenter(id vo.CostCenterID, companyID vo.CompanyID, d CostCenterData) *CostCenter {
	return &CostCenter{id: id, companyID: companyID, data: d}
}

func (c *CostCenter) ID() vo.CostCenterID     { return c.id }
func (c *CostCenter) CompanyID() vo.CompanyID { return c.companyID }
func (c *CostCenter) Data() CostCenterData    { return c.data }

// IsValidOn บอกว่าศูนย์ต้นทุนใช้ได้ในวันที่ระบุหรือไม่ (ใช้ตอนบันทึกรายการในอนาคต)
func (c *CostCenter) IsValidOn(day time.Time) bool {
	return c.data.Status == vo.StatusActive && !day.Before(c.data.ValidFrom) &&
		(c.data.ValidTo == nil || !day.After(*c.data.ValidTo))
}

func (c *CostCenter) Update(d CostCenterData) error {
	code, name, err := codeAndName(d.Code, d.Name)
	if err != nil {
		return err
	}
	if err := ensureStatus(d.Status); err != nil {
		return err
	}
	if err := vo.EnsureDateRange(d.ValidFrom, d.ValidTo); err != nil {
		return err
	}
	d.Code, d.Name = code, name
	c.data = d
	return nil
}

// ---------- Project / Site ----------

type ProjectSiteData struct {
	Code         string
	Name         string
	BranchID     *vo.BranchID     // สาขาที่ดูแล (ถ้ามี)
	CostCenterID *vo.CostCenterID // ศูนย์ต้นทุนที่รับรู้ค่าใช้จ่าย (ถ้ามี)
	Address      vo.Address
	StartDate    time.Time
	EndDate      *time.Time
	Status       vo.ProjectStatus
}

// ProjectSite คือโครงการ / ไซต์งาน (เช่น ไซต์ก่อสร้าง, คลังสินค้าชั่วคราว)
// กฎ: ปิดโครงการ (closed) ต้องระบุวันสิ้นสุด
type ProjectSite struct {
	id        vo.ProjectSiteID
	companyID vo.CompanyID
	data      ProjectSiteData
}

func NewProjectSite(companyID vo.CompanyID, d ProjectSiteData) (*ProjectSite, error) {
	p := &ProjectSite{id: vo.NewID[vo.ProjectSiteID](), companyID: companyID}
	if err := p.Update(d); err != nil {
		return nil, err
	}
	return p, nil
}

func RehydrateProjectSite(id vo.ProjectSiteID, companyID vo.CompanyID, d ProjectSiteData) *ProjectSite {
	return &ProjectSite{id: id, companyID: companyID, data: d}
}

func (p *ProjectSite) ID() vo.ProjectSiteID    { return p.id }
func (p *ProjectSite) CompanyID() vo.CompanyID { return p.companyID }
func (p *ProjectSite) Data() ProjectSiteData   { return p.data }

func (p *ProjectSite) Update(d ProjectSiteData) error {
	code, name, err := codeAndName(d.Code, d.Name)
	if err != nil {
		return err
	}
	if !slices.Contains(vo.ProjectStatuses, d.Status) {
		return domain.ErrInvalidStatus
	}
	if err := vo.EnsureDateRange(d.StartDate, d.EndDate); err != nil {
		return err
	}
	if d.Status == vo.ProjectClosed && d.EndDate == nil {
		return domain.ErrProjectEndDateRequired
	}
	if d.Address, err = vo.NewAddress(d.Address); err != nil {
		return err
	}
	d.Code, d.Name = code, name
	p.data = d
	return nil
}

func codeAndName(rawCode, rawName string) (string, string, error) {
	code, err := vo.NewCode(rawCode)
	if err != nil {
		return "", "", err
	}
	name, err := vo.NewName(rawName, 200)
	if err != nil {
		return "", "", err
	}
	return code.String(), name, nil
}
