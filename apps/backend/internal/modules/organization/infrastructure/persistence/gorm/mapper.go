// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ organization
//   - mapper.go       แปลง domain entity <-> gorm model (ที่เดียวที่รู้จักทั้งสองฝั่ง)
//   - transaction.go  implement ports.TransactionManager
//   - models/         รูปร่างตาราง
//   - repositories/   implement application/ports/repositories
package gormpersistence

import (
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/organization/domain/entities"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/models"
)

// ---------- Address ----------

func AddressToModel(a vo.Address) models.AddressColumns {
	return models.AddressColumns{
		Line: a.Line, SubDistrict: a.SubDistrict, District: a.District,
		Province: a.Province, PostalCode: a.PostalCode, Country: a.Country,
	}
}

func AddressToDomain(a models.AddressColumns) vo.Address {
	return vo.Address{
		Line: a.Line, SubDistrict: a.SubDistrict, District: a.District,
		Province: a.Province, PostalCode: a.PostalCode, Country: a.Country,
	}
}

// ---------- Company ----------

func CompanyToModel(c *entities.Company) models.CompanyModel {
	d := c.Data()
	return models.CompanyModel{
		ID: uuid.UUID(c.ID()), Code: d.Code, Name: d.Name, TaxID: d.TaxID,
		LegalNameTH: d.LegalNameTH, LegalNameEN: d.LegalNameEN,
		Address: AddressToModel(d.Address), Phone: d.Phone, Email: d.Email, Website: d.Website,
		BaseCurrency: d.BaseCurrency, FiscalYearStartMonth: d.FiscalYearStartMonth, Status: string(d.Status),
	}
}

func CompanyToEntity(m models.CompanyModel) *entities.Company {
	return entities.RehydrateCompany(vo.CompanyID(m.ID), entities.CompanyData{
		Code: m.Code, Name: m.Name, TaxID: m.TaxID,
		LegalNameTH: m.LegalNameTH, LegalNameEN: m.LegalNameEN, Address: AddressToDomain(m.Address),
		Phone: m.Phone, Email: m.Email, Website: m.Website,
		BaseCurrency: m.BaseCurrency, FiscalYearStartMonth: m.FiscalYearStartMonth, Status: vo.Status(m.Status),
	})
}

// ---------- Branch ----------

func BranchToModel(b *entities.Branch) models.BranchModel {
	d := b.Data()
	return models.BranchModel{
		ID: uuid.UUID(b.ID()), CompanyID: uuid.UUID(b.CompanyID()), Code: d.Code, Name: d.Name,
		TaxBranchNo: d.TaxBranchNo, Address: AddressToModel(d.Address), Phone: d.Phone, Status: string(d.Status),
	}
}

func BranchToEntity(m models.BranchModel) *entities.Branch {
	return entities.RehydrateBranch(vo.BranchID(m.ID), vo.CompanyID(m.CompanyID), entities.BranchData{
		Code: m.Code, Name: m.Name, TaxBranchNo: m.TaxBranchNo, Address: AddressToDomain(m.Address),
		Phone: m.Phone, Status: vo.Status(m.Status),
	})
}

// ---------- Business unit ----------

func BusinessUnitToModel(u *entities.BusinessUnit) models.BusinessUnitModel {
	d := u.Data()
	return models.BusinessUnitModel{
		ID: uuid.UUID(u.ID()), CompanyID: uuid.UUID(u.CompanyID()), Code: d.Code, Name: d.Name,
		ParentID: toUUID(d.ParentID), Status: string(d.Status),
	}
}

func BusinessUnitToEntity(m models.BusinessUnitModel) *entities.BusinessUnit {
	return entities.RehydrateBusinessUnit(vo.BusinessUnitID(m.ID), vo.CompanyID(m.CompanyID), entities.BusinessUnitData{
		Code: m.Code, Name: m.Name, ParentID: fromUUID[vo.BusinessUnitID](m.ParentID), Status: vo.Status(m.Status),
	})
}

// ---------- Department ----------

func DepartmentToModel(dep *entities.Department) models.DepartmentModel {
	d := dep.Data()
	return models.DepartmentModel{
		ID: uuid.UUID(dep.ID()), CompanyID: uuid.UUID(dep.CompanyID()), Code: d.Code, Name: d.Name,
		ParentID: toUUID(d.ParentID), BusinessUnitID: toUUID(d.BusinessUnitID), BranchID: toUUID(d.BranchID),
		Status: string(d.Status),
	}
}

func DepartmentToEntity(m models.DepartmentModel) *entities.Department {
	return entities.RehydrateDepartment(vo.DepartmentID(m.ID), vo.CompanyID(m.CompanyID), entities.DepartmentData{
		Code: m.Code, Name: m.Name,
		ParentID:       fromUUID[vo.DepartmentID](m.ParentID),
		BusinessUnitID: fromUUID[vo.BusinessUnitID](m.BusinessUnitID),
		BranchID:       fromUUID[vo.BranchID](m.BranchID),
		Status:         vo.Status(m.Status),
	})
}

// ---------- Cost center ----------

func CostCenterToModel(c *entities.CostCenter) models.CostCenterModel {
	d := c.Data()
	return models.CostCenterModel{
		ID: uuid.UUID(c.ID()), CompanyID: uuid.UUID(c.CompanyID()), Code: d.Code, Name: d.Name,
		DepartmentID: toUUID(d.DepartmentID), ValidFrom: d.ValidFrom, ValidTo: d.ValidTo, Status: string(d.Status),
	}
}

func CostCenterToEntity(m models.CostCenterModel) *entities.CostCenter {
	return entities.RehydrateCostCenter(vo.CostCenterID(m.ID), vo.CompanyID(m.CompanyID), entities.CostCenterData{
		Code: m.Code, Name: m.Name, DepartmentID: fromUUID[vo.DepartmentID](m.DepartmentID),
		ValidFrom: DateUTC(m.ValidFrom), ValidTo: DatePtrUTC(m.ValidTo), Status: vo.Status(m.Status),
	})
}

// ---------- Project / site ----------

func ProjectSiteToModel(p *entities.ProjectSite) models.ProjectSiteModel {
	d := p.Data()
	return models.ProjectSiteModel{
		ID: uuid.UUID(p.ID()), CompanyID: uuid.UUID(p.CompanyID()), Code: d.Code, Name: d.Name,
		BranchID: toUUID(d.BranchID), CostCenterID: toUUID(d.CostCenterID), Address: AddressToModel(d.Address),
		StartDate: d.StartDate, EndDate: d.EndDate, Status: string(d.Status),
	}
}

func ProjectSiteToEntity(m models.ProjectSiteModel) *entities.ProjectSite {
	return entities.RehydrateProjectSite(vo.ProjectSiteID(m.ID), vo.CompanyID(m.CompanyID), entities.ProjectSiteData{
		Code: m.Code, Name: m.Name,
		BranchID:     fromUUID[vo.BranchID](m.BranchID),
		CostCenterID: fromUUID[vo.CostCenterID](m.CostCenterID),
		Address:      AddressToDomain(m.Address),
		StartDate:    DateUTC(m.StartDate), EndDate: DatePtrUTC(m.EndDate),
		Status: vo.ProjectStatus(m.Status),
	})
}

// ---------- Fiscal ----------

func FiscalYearToModel(fy *entities.FiscalYear) models.FiscalYearModel {
	periods := fy.Periods()
	ps := make([]models.FiscalPeriodModel, len(periods))
	for i, p := range periods {
		ps[i] = FiscalPeriodToModel(p)
	}
	return models.FiscalYearModel{
		ID: uuid.UUID(fy.ID()), CompanyID: uuid.UUID(fy.CompanyID()), Year: fy.Year(),
		StartDate: fy.StartDate(), EndDate: fy.EndDate(), Periods: ps,
	}
}

func FiscalPeriodToModel(p *entities.FiscalPeriod) models.FiscalPeriodModel {
	return models.FiscalPeriodModel{
		ID: uuid.UUID(p.ID()), FiscalYearID: uuid.UUID(p.FiscalYearID()), PeriodNo: p.PeriodNo(),
		StartDate: p.StartDate(), EndDate: p.EndDate(), Status: string(p.Status()),
	}
}

func FiscalYearToEntity(m models.FiscalYearModel) *entities.FiscalYear {
	ps := make([]*entities.FiscalPeriod, len(m.Periods))
	for i, p := range m.Periods {
		ps[i] = entities.RehydrateFiscalPeriod(vo.FiscalPeriodID(p.ID), vo.FiscalYearID(p.FiscalYearID), p.PeriodNo,
			DateUTC(p.StartDate), DateUTC(p.EndDate), vo.PeriodStatus(p.Status))
	}
	return entities.RehydrateFiscalYear(vo.FiscalYearID(m.ID), vo.CompanyID(m.CompanyID), m.Year,
		DateUTC(m.StartDate), DateUTC(m.EndDate), ps)
}

// ---------- helpers ----------

func toUUID[T ~[16]byte](id *T) *uuid.UUID {
	if id == nil {
		return nil
	}
	u := uuid.UUID(*id)
	return &u
}

func fromUUID[T ~[16]byte](u *uuid.UUID) *T {
	if u == nil {
		return nil
	}
	id := T(*u)
	return &id
}

// DateUTC: column type date อ่านกลับมาเป็นเที่ยงคืน — บังคับเป็น UTC ให้ตรงกับ domain
func DateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func DatePtrUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := DateUTC(*t)
	return &d
}
