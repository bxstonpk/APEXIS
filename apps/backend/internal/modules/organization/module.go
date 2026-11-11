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
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity) *Module {
	idn := services.NewIdentity(identity)
	uc := usecases.New(usecases.Deps{
		Tx:            gp.NewTransactionManager(db),
		Auth:          idn,
		Members:       idn,
		Companies:     gormrepo.NewCompanyRepository(db),
		Branches:      gormrepo.NewBranchRepository(db),
		BusinessUnits: gormrepo.NewBusinessUnitRepository(db),
		Departments:   gormrepo.NewDepartmentRepository(db),
		CostCenters:   gormrepo.NewCostCenterRepository(db),
		ProjectSites:  gormrepo.NewProjectSiteRepository(db),
		Fiscal:        gormrepo.NewFiscalRepository(db),
		Dimensions:    gormrepo.NewDimensionRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	orghttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// ---------- public API สำหรับ module อื่น (เช่น ledger) ----------
//
// ใช้ uuid / primitive type เท่านั้น — ไม่ตรวจสิทธิ์ (module ที่เรียกตรวจสิทธิ์ของตัวเองแล้ว)
// เข้าร่วม transaction ใน ctx ของผู้เรียกได้

// CompanyActive คืน false เมื่อบริษัท inactive หรือไม่พบ — บริษัท inactive เป็นอ่านอย่างเดียว
func (m *Module) CompanyActive(ctx context.Context, companyID uuid.UUID) (bool, error) {
	ok, err := m.UseCases.Lookups.CompanyActive(ctx, vo.CompanyID(companyID))
	if errors.Is(err, domain.ErrCompanyNotFound) {
		return false, nil
	}
	return ok, err
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
