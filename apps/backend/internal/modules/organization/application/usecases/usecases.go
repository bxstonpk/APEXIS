// Package usecases คือ business operation ของ organization
//
// กติกา:
//   - CREATE / UPDATE / DELETE ทุกตัวทำใน tx.WithinTransaction และตรวจสิทธิ์ใน transaction เดียวกัน
//   - บริษัทที่ inactive เป็นอ่านอย่างเดียว: การเขียนผ่าน base.write ได้ domain.ErrCompanyInactive
//   - GET ไม่เปิด transaction
//   - หลังเขียนสำเร็จจะอ่าน read model กลับ 1 query (ได้ชื่อของ record ที่อ้างถึงจาก JOIN)
//   - record ที่อ้างถึง (parent, สาขา, แผนก ฯลฯ) ต้องอยู่ในบริษัทเดียวกันและยังไม่ถูกลบ
package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports"
	"apexis/backend/internal/modules/organization/application/ports/repositories"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	"apexis/backend/internal/modules/organization/domain/services"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type Deps struct {
	Tx            ports.TransactionManager
	Auth          ports.Authorizer
	Members       ports.Membership
	Companies     repositories.CompanyRepository
	Branches      repositories.BranchRepository
	BusinessUnits repositories.BusinessUnitRepository
	Departments   repositories.DepartmentRepository
	CostCenters   repositories.CostCenterRepository
	ProjectSites  repositories.ProjectSiteRepository
	Fiscal        repositories.FiscalRepository
	Dimensions    repositories.DimensionRepository
	Numbering     repositories.NumberingRepository
	DocumentTypes *DocumentTypes
	Clock         func() time.Time // optional
}

type UseCases struct {
	Companies     *CompanyUseCases
	Branches      *BranchUseCases
	BusinessUnits *BusinessUnitUseCases
	Departments   *DepartmentUseCases
	CostCenters   *CostCenterUseCases
	ProjectSites  *ProjectSiteUseCases
	Fiscal        *FiscalUseCases
	Lookups       *LookupUseCases
	Numbering     *NumberingUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, companies: d.Companies}
	return &UseCases{
		Companies: &CompanyUseCases{base: b, members: d.Members},
		Branches: &BranchUseCases{
			master: newMaster(b, d.Branches, vo.PermBranchRead, vo.PermBranchManage), repo: d.Branches,
		},
		BusinessUnits: &BusinessUnitUseCases{
			master: newMaster(b, d.BusinessUnits, vo.PermBusinessUnitRead, vo.PermBusinessUnitMng), repo: d.BusinessUnits,
		},
		Departments: &DepartmentUseCases{
			master: newMaster(b, d.Departments, vo.PermDepartmentRead, vo.PermDepartmentManage), repo: d.Departments,
			units: d.BusinessUnits, branches: d.Branches,
		},
		CostCenters: &CostCenterUseCases{
			master: newMaster(b, d.CostCenters, vo.PermCostCenterRead, vo.PermCostCenterManage), repo: d.CostCenters,
			departments: d.Departments,
		},
		ProjectSites: &ProjectSiteUseCases{
			master: newMaster(b, d.ProjectSites, vo.PermProjectSiteRead, vo.PermProjectSiteManage), repo: d.ProjectSites,
			branches: d.Branches, costCenters: d.CostCenters,
		},
		Fiscal:    &FiscalUseCases{base: b, fiscal: d.Fiscal},
		Lookups:   &LookupUseCases{companies: d.Companies, fiscal: d.Fiscal, dimensions: d.Dimensions},
		Numbering: &NumberingUseCases{base: b, repo: d.Numbering, types: d.DocumentTypes, now: d.Clock},
	}
}

// ---------- base ----------

type base struct {
	tx        ports.TransactionManager
	auth      ports.Authorizer
	companies repositories.CompanyRepository
}

func (b base) authorize(ctx context.Context, a dto.Actor, p vo.Permission) error {
	return b.auth.Authorize(ctx, a.UserID, a.CompanyID, p)
}

// write เปิด transaction -> ตรวจสิทธิ์ -> บริษัทต้อง active -> fn  (error ใดๆ = rollback ทั้งหมด)
func (b base) write(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context) error) error {
	return b.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := b.authorize(ctx, a, p); err != nil {
			return err
		}
		s, err := b.companies.Settings(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if !s.Active {
			return domain.ErrCompanyInactive
		}
		return fn(ctx)
	})
}

// ---------- master: Get / List / write / remove ที่ master data ทุกชนิดใช้ร่วมกัน ----------

type master[ID any, E any, R any, F any] struct {
	base
	repo   repositories.MasterRepository[ID, E, R, F]
	read   vo.Permission
	manage vo.Permission
}

func newMaster[ID any, E any, R any, F any](b base, repo repositories.MasterRepository[ID, E, R, F], read, manage vo.Permission) master[ID, E, R, F] {
	return master[ID, E, R, F]{base: b, repo: repo, read: read, manage: manage}
}

func (m master[ID, E, R, F]) Get(ctx context.Context, in dto.ByID[ID]) (R, error) {
	if err := m.authorize(ctx, in.Actor, m.read); err != nil {
		var zero R
		return zero, err
	}
	return m.repo.Get(ctx, in.CompanyID, in.ID)
}

func (m master[ID, E, R, F]) List(ctx context.Context, in dto.List[F]) (listing.Result[R], error) {
	if err := m.authorize(ctx, in.Actor, m.read); err != nil {
		return listing.Result[R]{}, err
	}
	return m.repo.List(ctx, in.CompanyID, in.Filter)
}

// save รัน fn ใน transaction (สิทธิ์ manage) แล้วอ่าน read model ของ id กลับหลัง commit
func (m master[ID, E, R, F]) save(ctx context.Context, a dto.Actor, id ID, fn func(ctx context.Context) error) (R, error) {
	if err := m.write(ctx, a, m.manage, fn); err != nil {
		var zero R
		return zero, err
	}
	return m.repo.Get(ctx, a.CompanyID, id)
}

// edit = โหลด entity ใน transaction -> apply -> Update
func (m master[ID, E, R, F]) edit(ctx context.Context, a dto.Actor, id ID, apply func(ctx context.Context, e E) error) (R, error) {
	return m.save(ctx, a, id, func(ctx context.Context) error {
		e, err := m.repo.FindByID(ctx, a.CompanyID, id)
		if err != nil {
			return err
		}
		if err := apply(ctx, e); err != nil {
			return err
		}
		return m.repo.Update(ctx, e)
	})
}

// remove = โหลด entity -> ตรวจกฎการลบ / record ที่อ้างถึง -> soft delete (ทั้งหมดใน transaction เดียว)
func (m master[ID, E, R, F]) remove(ctx context.Context, in dto.ByID[ID], check func(ctx context.Context, e E) error) error {
	return m.write(ctx, in.Actor, m.manage, func(ctx context.Context) error {
		e, err := m.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if check != nil {
			if err := check(ctx, e); err != nil {
				return err
			}
		}
		return m.repo.Delete(ctx, in.CompanyID, in.ID)
	})
}

// ---------- helper ตรวจ record ที่อ้างถึง ----------

func unused(used bool, err error) error {
	if err != nil {
		return err
	}
	if used {
		return domain.ErrInUse
	}
	return nil
}

// requireRef: ถ้าระบุ id ต้องมีอยู่ในบริษัทเดียวกัน
func requireRef[ID any](ctx context.Context, companyID vo.CompanyID, id *ID, exists func(context.Context, vo.CompanyID, ID) (bool, error), notFound error) error {
	if id == nil {
		return nil
	}
	ok, err := exists(ctx, companyID, *id)
	if err != nil {
		return err
	}
	if !ok {
		return notFound
	}
	return nil
}

// requireParent: parent ต้องมีอยู่ และต้องไม่ใช่ตัวเองหรือลูกหลานของตัวเอง (1 recursive query)
func requireParent[ID comparable](ctx context.Context, companyID vo.CompanyID, self ID, parent *ID, ancestry func(context.Context, vo.CompanyID, ID) ([]ID, error), notFound error) error {
	if parent == nil {
		return nil
	}
	chain, err := ancestry(ctx, companyID, *parent)
	if err != nil {
		return err
	}
	if len(chain) == 0 {
		return notFound
	}
	return services.EnsureNoCycle(self, chain)
}
