// Package repositories คือสัญญา (port) ที่ application ของ organization ต้องการจาก persistence
// implementation อยู่ใน infrastructure/persistence/gorm/repositories
//
// กติกา:
//   - ทุก method รับ CompanyID (tenant isolation) และไม่เห็น record ที่ถูก soft delete
//   - FindByID คืน entity สำหรับฝั่งเขียน, Get / List คืน read DTO (JOIN ชื่อของ record ที่อ้างถึงใน query เดียว)
//   - Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ
//   - Create / Update คืน domain.ErrCodeExists (หรือ error เฉพาะ) เมื่อชน unique
//   - ห้ามเปิด transaction เอง — ใช้ transaction ที่ use case เปิดไว้ใน ctx
package repositories

import (
	"context"
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/domain/entities"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// CompanyRepository: บริษัทคือ tenant — ไม่มี company_id และไม่มี soft delete
type CompanyRepository interface {
	// Create คืน domain.ErrCodeExists ถ้ารหัสบริษัทซ้ำ
	Create(ctx context.Context, c *entities.Company) error
	Update(ctx context.Context, c *entities.Company) error
	FindByID(ctx context.Context, id vo.CompanyID) (*entities.Company, error)
	// Settings คืนสถานะ + จำนวนทศนิยมของบริษัท — domain.ErrCompanyNotFound ถ้าไม่พบ
	Settings(ctx context.Context, id vo.CompanyID) (dto.CompanySettings, error)
	// ListByIDs กรอง / เรียง / แบ่งหน้าเฉพาะบริษัทใน ids (id ได้จาก identity — JOIN ข้าม module ไม่ได้)
	ListByIDs(ctx context.Context, ids []vo.CompanyID, f dto.CompanyFilter) (listing.Result[dto.CompanySummary], error)
}

// MasterRepository คือ method ร่วมของ master data ที่มีรหัส + soft delete
type MasterRepository[ID any, E any, R any, F any] interface {
	Create(ctx context.Context, e E) error
	Update(ctx context.Context, e E) error
	// Delete คือ soft delete
	Delete(ctx context.Context, companyID vo.CompanyID, id ID) error
	FindByID(ctx context.Context, companyID vo.CompanyID, id ID) (E, error)
	Exists(ctx context.Context, companyID vo.CompanyID, id ID) (bool, error)
	Get(ctx context.Context, companyID vo.CompanyID, id ID) (R, error)
	List(ctx context.Context, companyID vo.CompanyID, f F) (listing.Result[R], error)
}

type BranchRepository interface {
	MasterRepository[vo.BranchID, *entities.Branch, dto.Branch, dto.BranchFilter]
	// HasDependents: มีแผนก / โครงการที่ยังอ้างถึงสาขานี้
	HasDependents(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) (bool, error)
}

type BusinessUnitRepository interface {
	MasterRepository[vo.BusinessUnitID, *entities.BusinessUnit, dto.BusinessUnit, dto.BusinessUnitFilter]
	// HasDependents: มีหน่วยธุรกิจลูก / แผนกที่ยังอ้างถึง
	HasDependents(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) (bool, error)
	// Ancestry คืน [id, parent, grandparent, ...] ด้วย recursive query เดียว — ว่าง = ไม่พบ id
	Ancestry(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) ([]vo.BusinessUnitID, error)
}

type DepartmentRepository interface {
	MasterRepository[vo.DepartmentID, *entities.Department, dto.Department, dto.DepartmentFilter]
	// HasDependents: มีแผนกลูก / ศูนย์ต้นทุนที่ยังอ้างถึง
	HasDependents(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) (bool, error)
	Ancestry(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) ([]vo.DepartmentID, error)
}

type CostCenterRepository interface {
	MasterRepository[vo.CostCenterID, *entities.CostCenter, dto.CostCenter, dto.CostCenterFilter]
	// HasDependents: มีโครงการที่ยังอ้างถึง
	HasDependents(ctx context.Context, companyID vo.CompanyID, id vo.CostCenterID) (bool, error)
}

type ProjectSiteRepository interface {
	MasterRepository[vo.ProjectSiteID, *entities.ProjectSite, dto.ProjectSite, dto.ProjectSiteFilter]
}

type FiscalRepository interface {
	// CreateYear เขียนปีบัญชี + 12 งวด — คืน domain.ErrFiscalYearExists ถ้าปีซ้ำ
	CreateYear(ctx context.Context, fy *entities.FiscalYear) error
	ExistsOverlapping(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (bool, error)
	// FindYearByPeriodForUpdate โหลดปีบัญชีทั้งปีของงวดนั้นพร้อม lock แถวปีบัญชี (SELECT ... FOR UPDATE)
	// กันการปิด / เปิดงวดพร้อมกันจนลำดับงวดผิด
	FindYearByPeriodForUpdate(ctx context.Context, companyID vo.CompanyID, periodID vo.FiscalPeriodID) (*entities.FiscalYear, error)
	UpdatePeriodStatus(ctx context.Context, p *entities.FiscalPeriod) error
	// PeriodAt คืนงวดที่ครอบคลุมวันที่ พร้อม lock แถวงวดแบบ FOR SHARE (ถ้าอยู่ใน transaction)
	// เพื่อให้การปิดงวด (UPDATE แถวงวด) รอจนรายการที่กำลังลงในงวดนั้น commit — คืน ErrFiscalPeriodNotFound ถ้าไม่มี
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (dto.PeriodAt, error)
	GetYear(ctx context.Context, companyID vo.CompanyID, id vo.FiscalYearID) (dto.FiscalYear, error)
	ListYears(ctx context.Context, companyID vo.CompanyID, f dto.FiscalYearFilter) (listing.Result[dto.FiscalYear], error)
	GetPeriod(ctx context.Context, companyID vo.CompanyID, id vo.FiscalPeriodID) (dto.FiscalPeriod, error)
	ListPeriods(ctx context.Context, companyID vo.CompanyID, f dto.FiscalPeriodFilter) (listing.Result[dto.FiscalPeriod], error)
}

// DimensionRepository ตรวจว่า id ของมิติวิเคราะห์มีอยู่ (ยังไม่ถูกลบ) ในบริษัท — query เดียว
type DimensionRepository interface {
	// Missing คืนชื่อชนิดมิติที่มี id ไม่พบ เช่น ["branch"] — ว่าง = ครบ
	Missing(ctx context.Context, companyID vo.CompanyID, d dto.Dimensions) ([]string, error)
}
