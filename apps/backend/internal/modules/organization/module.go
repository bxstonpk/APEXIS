// Package organization คือบริษัท (tenant ของทั้งระบบ) และโครงสร้างองค์กร:
// บริษัท / ข้อมูลนิติบุคคล, สาขา, หน่วยธุรกิจ, แผนก, ศูนย์ต้นทุน, โครงการ / ไซต์งาน, ปีบัญชี / งวดบัญชี
//
// พึ่งพา identity ผ่าน interface Identity เท่านั้น
// (ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์, ตั้ง owner ของบริษัทใหม่, บริษัทที่ user เป็นสมาชิก)
package organization

import (
	"context"
	"errors"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/usecases"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	gp "apexis/backend/internal/modules/organization/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/organization/infrastructure/services"
	orghttp "apexis/backend/internal/modules/organization/presentation/http"
	"apexis/backend/internal/modules/organization/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
)

// Identity คือสิ่งที่ organization ต้องการจาก identity module (*identity.Module implement)
type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
	ProvisionCompanyOwner(ctx context.Context, companyID, ownerID uuid.UUID) (bool, error)
	MemberCompanyIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

type Module struct {
	UseCases      *usecases.UseCases
	identity      Identity
	documentTypes *usecases.DocumentTypes
	provisioning  *services.Provisioning
	handler       *handlers.Handler
}

func New(db *gorm.DB, identity Identity) *Module {
	idn := services.NewIdentity(identity)
	types := usecases.NewDocumentTypes()
	provisioning := services.NewProvisioning()
	uc := usecases.New(usecases.Deps{
		Tx:            gp.NewTransactionManager(db),
		Auth:          idn,
		Members:       idn,
		Provisioner:   provisioning,
		FiscalHooks:   provisioning,
		Companies:     gormrepo.NewCompanyRepository(db),
		Branches:      gormrepo.NewBranchRepository(db),
		BusinessUnits: gormrepo.NewBusinessUnitRepository(db),
		Departments:   gormrepo.NewDepartmentRepository(db),
		CostCenters:   gormrepo.NewCostCenterRepository(db),
		ProjectSites:  gormrepo.NewProjectSiteRepository(db),
		Fiscal:        gormrepo.NewFiscalRepository(db),
		Dimensions:    gormrepo.NewDimensionRepository(db),
		Numbering:     gormrepo.NewNumberingRepository(db),
		Templates:     gormrepo.NewDocumentTemplateRepository(db),
		Assets:        gormrepo.NewDocumentAssetRepository(db),
		DocumentTypes: types,
	})
	return &Module{UseCases: uc, identity: identity, documentTypes: types, provisioning: provisioning, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	orghttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// ---------- public API สำหรับ module อื่น (เช่น ledger) ----------
//
// ใช้ uuid / primitive type เท่านั้น — ไม่ตรวจสิทธิ์ (module ที่เรียกตรวจสิทธิ์ของตัวเองแล้ว)
// เข้าร่วม transaction ใน ctx ของผู้เรียกได้

// CompanySettings คืนสถานะและจำนวนทศนิยมของบริษัทใน query เดียว
//   - active = false เมื่อบริษัท inactive หรือไม่พบ (บริษัท inactive เป็นอ่านอย่างเดียว)
//   - amountDecimals / quantityDecimals คือทศนิยมของจำนวนเงิน / จำนวนสินค้าที่บริษัทกำหนด
func (m *Module) CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error) {
	s, err := m.UseCases.Lookups.CompanySettings(ctx, vo.CompanyID(companyID))
	if errors.Is(err, domain.ErrCompanyNotFound) {
		return false, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, err
	}
	return s.Active, s.AmountDecimals, s.QuantityDecimals, nil
}

// CompanyBusinessType คืนประเภทธุรกิจของบริษัท ("" เมื่อไม่พบบริษัท) — ใช้เลือกผังบัญชีมาตรฐาน
func (m *Module) CompanyBusinessType(ctx context.Context, companyID uuid.UUID) (integration.BusinessType, error) {
	s, err := m.UseCases.Lookups.CompanySettings(ctx, vo.CompanyID(companyID))
	if errors.Is(err, domain.ErrCompanyNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return integration.BusinessType(s.BusinessType), nil
}

// OnCompanyCreated ลงทะเบียน hook ที่ตั้งค่าเริ่มต้นให้บริษัทใหม่ (เรียกตอนเริ่มโปรแกรม)
// hook ทำงานตามลำดับที่ลงทะเบียน ใน transaction เดียวกับการสร้างบริษัท — error = rollback ทั้งหมด
func (m *Module) OnCompanyCreated(h integration.CompanyCreatedHook) {
	m.provisioning.Add(h)
}

// OnFiscalYearCreated ลงทะเบียน hook ที่เรียกหลังสร้างปีบัญชี (ตามลำดับที่ลงทะเบียน ใน transaction เดียวกัน)
func (m *Module) OnFiscalYearCreated(h integration.FiscalYearCreatedHook) {
	m.provisioning.AddFiscal(h)
}

// FiscalYears คืนปีบัญชีทั้งหมดของบริษัท (เรียงตามปี) พร้อมจำนวนงวด / งวดที่ยังเปิด — ledger ใช้ตรวจเงื่อนไขปิดบัญชีสิ้นปี
func (m *Module) FiscalYears(ctx context.Context, companyID uuid.UUID) ([]integration.FiscalYearStatus, error) {
	ys, err := m.UseCases.Lookups.YearStatuses(ctx, vo.CompanyID(companyID))
	out := make([]integration.FiscalYearStatus, len(ys))
	for i, y := range ys {
		out[i] = integration.FiscalYearStatus{Year: y.Year, Start: y.StartDate, End: y.EndDate, Periods: y.Periods, OpenPeriods: y.OpenPeriods}
	}
	return out, err
}

// FiscalPeriodAt คืนงวดบัญชีที่ครอบคลุม date: id งวด, ปีบัญชี, วันเริ่มปีบัญชี, งวดเปิดอยู่หรือไม่
// periodID = uuid.Nil เมื่อไม่มีงวดครอบคลุมวันนั้น
// ภายใน transaction จะ lock แถวงวด (FOR SHARE) จน commit — การปิดงวดพร้อมกันต้องรอ
func (m *Module) FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error) {
	p, err := m.UseCases.Lookups.PeriodAt(ctx, vo.CompanyID(companyID), date)
	if errors.Is(err, domain.ErrFiscalPeriodNotFound) {
		return uuid.Nil, 0, time.Time{}, false, nil
	}
	if err != nil {
		return uuid.Nil, 0, time.Time{}, false, err
	}
	return uuid.UUID(p.PeriodID), p.FiscalYear, p.YearStart, p.Status == vo.PeriodOpen, nil
}

// FiscalPeriodRange คืนงวดที่ครอบคลุม date พร้อมวันแรก / วันสุดท้ายของงวด (periodID = uuid.Nil เมื่อไม่มีงวด)
// ใช้กับงานสิ้นงวด เช่น ปรับปรุงสินค้าคงเหลือ — lock แถวงวดแบบเดียวกับ FiscalPeriodAt
func (m *Module) FiscalPeriodRange(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, start, end time.Time, open bool, err error) {
	p, err := m.UseCases.Lookups.PeriodAt(ctx, vo.CompanyID(companyID), date)
	if errors.Is(err, domain.ErrFiscalPeriodNotFound) {
		return uuid.Nil, 0, time.Time{}, time.Time{}, false, nil
	}
	if err != nil {
		return uuid.Nil, 0, time.Time{}, time.Time{}, false, err
	}
	return uuid.UUID(p.PeriodID), p.FiscalYear, p.Start, p.End, p.Status == vo.PeriodOpen, nil
}

// MissingDimensions คืนชนิดมิติที่มี id ไม่พบในบริษัท ("branch", "department", "cost_center", "project_site")
func (m *Module) MissingDimensions(ctx context.Context, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error) {
	return m.UseCases.Lookups.MissingDimensions(ctx, vo.CompanyID(companyID), dto.Dimensions{
		Branches: ids[vo.BranchID](branches), Departments: ids[vo.DepartmentID](departments),
		CostCenters: ids[vo.CostCenterID](costCenters), ProjectSites: ids[vo.ProjectSiteID](projectSites),
	})
}

func ids[T ~[16]byte](in []uuid.UUID) []T {
	out := make([]T, len(in))
	for i, id := range in {
		out[i] = T(id)
	}
	return out
}

// Migrate สร้างตารางของ organization
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }

// RegisterDocumentTypes ลงทะเบียนชนิดเอกสารของ module อื่น (เรียกตอนเริ่มโปรแกรม) — key ซ้ำ = panic
func (m *Module) RegisterDocumentTypes(types ...integration.DocumentType) {
	m.documentTypes.Register(types...)
}

// DocumentFormat คืนรูปแบบเลขที่ของชนิดเอกสาร (ไม่ได้ตั้ง = prefix เริ่มต้น + numbering.DefaultTemplate)
// module เจ้าของเอกสารออกเลขเอง: format.PeriodKey -> ลำดับถัดไปของตัวเอง -> format.Render
func (m *Module) DocumentFormat(ctx context.Context, companyID uuid.UUID, docType string) (numbering.Format, error) {
	return m.UseCases.Numbering.Format(ctx, vo.CompanyID(companyID), docType)
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}
