package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/organization/application/dto"
	ports "apexis/backend/internal/modules/organization/application/ports/repositories"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	gp "apexis/backend/internal/modules/organization/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var unitSortColumns = func(alias string) map[string]string {
	return map[string]string{"code": alias + ".code", "name": alias + ".name"}
}

// ---------- Business unit ----------

var _ ports.BusinessUnitRepository = (*BusinessUnitRepository)(nil)

type BusinessUnitRepository struct{ db *gorm.DB }

func NewBusinessUnitRepository(db *gorm.DB) *BusinessUnitRepository {
	return &BusinessUnitRepository{db: db}
}

func (r *BusinessUnitRepository) Create(ctx context.Context, u *entities.BusinessUnit) error {
	m := gp.BusinessUnitToModel(u)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *BusinessUnitRepository) Update(ctx context.Context, u *entities.BusinessUnit) error {
	m := gp.BusinessUnitToModel(u)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "parent_id", "status"},
		domain.ErrCodeExists, domain.ErrBusinessUnitNotFound)
}

func (r *BusinessUnitRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) error {
	return softDelete[models.BusinessUnitModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrBusinessUnitNotFound)
}

func (r *BusinessUnitRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) (*entities.BusinessUnit, error) {
	m, err := findByID[models.BusinessUnitModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrBusinessUnitNotFound)
	if err != nil {
		return nil, err
	}
	return gp.BusinessUnitToEntity(m), nil
}

func (r *BusinessUnitRepository) Exists(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) (bool, error) {
	return exists(ctx, r.db, "organization_business_units", uuidOf(companyID), uuidOf(id))
}

func (r *BusinessUnitRepository) HasDependents(ctx context.Context, _ vo.CompanyID, id vo.BusinessUnitID) (bool, error) {
	return referenced(ctx, r.db, uuidOf(id),
		reference{"organization_business_units", "parent_id"},
		reference{"organization_departments", "business_unit_id"})
}

func (r *BusinessUnitRepository) Ancestry(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) ([]vo.BusinessUnitID, error) {
	chain, err := ancestry(ctx, r.db, "organization_business_units", uuidOf(companyID), uuidOf(id))
	return castIDs[vo.BusinessUnitID](chain), err
}

type businessUnitRow struct {
	ID     uuid.UUID
	Code   string
	Name   string
	Status string
	Parent refCols `gorm:"embedded;embeddedPrefix:parent_"`
	Total  int64
}

const businessUnitColumns = `u.id, u.code, u.name, u.status,
	p.id AS parent_id, p.code AS parent_code, p.name AS parent_name`

// base: business_units LEFT JOIN parent — ได้ชื่อหน่วยแม่ใน query เดียว
func (r *BusinessUnitRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("organization_business_units AS u").
		Joins("LEFT JOIN organization_business_units AS p ON p.id = u.parent_id AND p.deleted_at IS NULL").
		Where("u.company_id = ? AND u.deleted_at IS NULL", uuidOf(companyID))
}

func (r *BusinessUnitRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.BusinessUnitID) (dto.BusinessUnit, error) {
	row, err := first[businessUnitRow](r.base(ctx, companyID).Select(businessUnitColumns).Where("u.id = ?", uuidOf(id)), domain.ErrBusinessUnitNotFound)
	return businessUnitDTO(row), err
}

func (r *BusinessUnitRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.BusinessUnitFilter) (listing.Result[dto.BusinessUnit], error) {
	order, err := database.OrderBy(f.Sort, unitSortColumns("u"), "u.id")
	if err != nil {
		return listing.Result[dto.BusinessUnit]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.base(ctx, companyID), f.Search, "u.code", "u.name")
		if len(f.Statuses) > 0 {
			q = q.Where("u.status IN ?", f.Statuses)
		}
		if f.ParentID != nil {
			q = q.Where("u.parent_id = ?", uuidOf(*f.ParentID))
		}
		return q
	}
	rows, total, err := database.FetchPage(base, businessUnitColumns, order, f.Page, func(r businessUnitRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, businessUnitDTO), err
}

func businessUnitDTO(r businessUnitRow) dto.BusinessUnit {
	return dto.BusinessUnit{ID: r.ID.String(), Code: r.Code, Name: r.Name, Parent: r.Parent.ref(), Status: r.Status}
}

// ---------- Department ----------

var _ ports.DepartmentRepository = (*DepartmentRepository)(nil)

type DepartmentRepository struct{ db *gorm.DB }

func NewDepartmentRepository(db *gorm.DB) *DepartmentRepository { return &DepartmentRepository{db: db} }

func (r *DepartmentRepository) Create(ctx context.Context, d *entities.Department) error {
	m := gp.DepartmentToModel(d)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *DepartmentRepository) Update(ctx context.Context, d *entities.Department) error {
	m := gp.DepartmentToModel(d)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "parent_id", "business_unit_id", "branch_id", "status"},
		domain.ErrCodeExists, domain.ErrDepartmentNotFound)
}

func (r *DepartmentRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) error {
	return softDelete[models.DepartmentModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrDepartmentNotFound)
}

func (r *DepartmentRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) (*entities.Department, error) {
	m, err := findByID[models.DepartmentModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrDepartmentNotFound)
	if err != nil {
		return nil, err
	}
	return gp.DepartmentToEntity(m), nil
}

func (r *DepartmentRepository) Exists(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) (bool, error) {
	return exists(ctx, r.db, "organization_departments", uuidOf(companyID), uuidOf(id))
}

func (r *DepartmentRepository) HasDependents(ctx context.Context, _ vo.CompanyID, id vo.DepartmentID) (bool, error) {
	return referenced(ctx, r.db, uuidOf(id),
		reference{"organization_departments", "parent_id"},
		reference{"organization_cost_centers", "department_id"})
}

func (r *DepartmentRepository) Ancestry(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) ([]vo.DepartmentID, error) {
	chain, err := ancestry(ctx, r.db, "organization_departments", uuidOf(companyID), uuidOf(id))
	return castIDs[vo.DepartmentID](chain), err
}

type departmentRow struct {
	ID           uuid.UUID
	Code         string
	Name         string
	Status       string
	Parent       refCols `gorm:"embedded;embeddedPrefix:parent_"`
	BusinessUnit refCols `gorm:"embedded;embeddedPrefix:bu_"`
	Branch       refCols `gorm:"embedded;embeddedPrefix:branch_"`
	Total        int64
}

const departmentColumns = `d.id, d.code, d.name, d.status,
	p.id AS parent_id, p.code AS parent_code, p.name AS parent_name,
	bu.id AS bu_id, bu.code AS bu_code, bu.name AS bu_name,
	br.id AS branch_id, br.code AS branch_code, br.name AS branch_name`

// base: departments LEFT JOIN แผนกแม่ / หน่วยธุรกิจ / สาขา — ได้ข้อมูลอ้างอิงครบใน query เดียว (ไม่มี N+1)
func (r *DepartmentRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("organization_departments AS d").
		Joins("LEFT JOIN organization_departments AS p ON p.id = d.parent_id AND p.deleted_at IS NULL").
		Joins("LEFT JOIN organization_business_units AS bu ON bu.id = d.business_unit_id AND bu.deleted_at IS NULL").
		Joins("LEFT JOIN organization_branches AS br ON br.id = d.branch_id AND br.deleted_at IS NULL").
		Where("d.company_id = ? AND d.deleted_at IS NULL", uuidOf(companyID))
}

func (r *DepartmentRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.DepartmentID) (dto.Department, error) {
	row, err := first[departmentRow](r.base(ctx, companyID).Select(departmentColumns).Where("d.id = ?", uuidOf(id)), domain.ErrDepartmentNotFound)
	return departmentDTO(row), err
}

func (r *DepartmentRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DepartmentFilter) (listing.Result[dto.Department], error) {
	order, err := database.OrderBy(f.Sort, unitSortColumns("d"), "d.id")
	if err != nil {
		return listing.Result[dto.Department]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.base(ctx, companyID), f.Search, "d.code", "d.name")
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.ParentID != nil {
			q = q.Where("d.parent_id = ?", uuidOf(*f.ParentID))
		}
		if f.BusinessUnitID != nil {
			q = q.Where("d.business_unit_id = ?", uuidOf(*f.BusinessUnitID))
		}
		if f.BranchID != nil {
			q = q.Where("d.branch_id = ?", uuidOf(*f.BranchID))
		}
		return q
	}
	rows, total, err := database.FetchPage(base, departmentColumns, order, f.Page, func(r departmentRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, departmentDTO), err
}

func departmentDTO(r departmentRow) dto.Department {
	return dto.Department{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, Status: r.Status,
		Parent: r.Parent.ref(), BusinessUnit: r.BusinessUnit.ref(), Branch: r.Branch.ref(),
	}
}

func castIDs[T ~[16]byte](in []uuid.UUID) []T {
	out := make([]T, len(in))
	for i, id := range in {
		out[i] = T(id)
	}
	return out
}
