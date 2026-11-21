package repositories

import (
	"context"
	"time"

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

// ---------- Cost center ----------

var _ ports.CostCenterRepository = (*CostCenterRepository)(nil)

type CostCenterRepository struct{ db *gorm.DB }

func NewCostCenterRepository(db *gorm.DB) *CostCenterRepository { return &CostCenterRepository{db: db} }

func (r *CostCenterRepository) Create(ctx context.Context, c *entities.CostCenter) error {
	m := gp.CostCenterToModel(c)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *CostCenterRepository) Update(ctx context.Context, c *entities.CostCenter) error {
	m := gp.CostCenterToModel(c)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "department_id", "valid_from", "valid_to", "status"},
		domain.ErrCodeExists, domain.ErrCostCenterNotFound)
}

func (r *CostCenterRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.CostCenterID) error {
	return softDelete[models.CostCenterModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrCostCenterNotFound)
}

func (r *CostCenterRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CostCenterID) (*entities.CostCenter, error) {
	m, err := findByID[models.CostCenterModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrCostCenterNotFound)
	if err != nil {
		return nil, err
	}
	return gp.CostCenterToEntity(m), nil
}

func (r *CostCenterRepository) Exists(ctx context.Context, companyID vo.CompanyID, id vo.CostCenterID) (bool, error) {
	return exists(ctx, r.db, "organization_cost_centers", uuidOf(companyID), uuidOf(id))
}

func (r *CostCenterRepository) HasDependents(ctx context.Context, _ vo.CompanyID, id vo.CostCenterID) (bool, error) {
	return referenced(ctx, r.db, uuidOf(id), reference{"organization_project_sites", "cost_center_id"})
}

type costCenterRow struct {
	ID         uuid.UUID
	Code       string
	Name       string
	ValidFrom  time.Time
	ValidTo    *time.Time
	Status     string
	Department refCols `gorm:"embedded;embeddedPrefix:dept_"`
	Total      int64
}

const costCenterColumns = `c.id, c.code, c.name, c.valid_from, c.valid_to, c.status,
	d.id AS dept_id, d.code AS dept_code, d.name AS dept_name`

var costCenterSortColumns = map[string]string{"code": "c.code", "name": "c.name", "valid_from": "c.valid_from"}

// base: cost_centers LEFT JOIN departments
func (r *CostCenterRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("organization_cost_centers AS c").
		Joins("LEFT JOIN organization_departments AS d ON d.id = c.department_id AND d.deleted_at IS NULL").
		Where("c.company_id = ? AND c.deleted_at IS NULL", uuidOf(companyID))
}

func (r *CostCenterRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.CostCenterID) (dto.CostCenter, error) {
	row, err := first[costCenterRow](r.base(ctx, companyID).Select(costCenterColumns).Where("c.id = ?", uuidOf(id)), domain.ErrCostCenterNotFound)
	return costCenterDTO(row), err
}

func (r *CostCenterRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.CostCenterFilter) (listing.Result[dto.CostCenter], error) {
	order, err := database.OrderBy(f.Sort, costCenterSortColumns, "c.id")
	if err != nil {
		return listing.Result[dto.CostCenter]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.base(ctx, companyID), f.Search, "c.code", "c.name")
		if len(f.Statuses) > 0 {
			q = q.Where("c.status IN ?", f.Statuses)
		}
		if f.DepartmentID != nil {
			q = q.Where("c.department_id = ?", uuidOf(*f.DepartmentID))
		}
		if f.ActiveOn != nil {
			q = q.Where("c.valid_from <= ? AND (c.valid_to IS NULL OR c.valid_to >= ?)", *f.ActiveOn, *f.ActiveOn)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, costCenterColumns, order, f.Page, func(r costCenterRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, costCenterDTO), err
}

func costCenterDTO(r costCenterRow) dto.CostCenter {
	return dto.CostCenter{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, Department: r.Department.ref(),
		ValidFrom: dateUTC(r.ValidFrom), ValidTo: datePtr(r.ValidTo), Status: r.Status,
	}
}

// ---------- Project / site ----------

var _ ports.ProjectSiteRepository = (*ProjectSiteRepository)(nil)

type ProjectSiteRepository struct{ db *gorm.DB }

func NewProjectSiteRepository(db *gorm.DB) *ProjectSiteRepository {
	return &ProjectSiteRepository{db: db}
}

func (r *ProjectSiteRepository) Create(ctx context.Context, p *entities.ProjectSite) error {
	m := gp.ProjectSiteToModel(p)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *ProjectSiteRepository) Update(ctx context.Context, p *entities.ProjectSite) error {
	m := gp.ProjectSiteToModel(p)
	return update(ctx, r.db, &m, m.CompanyID, []string{
		"code", "name", "branch_id", "cost_center_id", "start_date", "end_date", "status",
		"address_line", "address_sub_district", "address_district", "address_province", "address_postal_code", "address_country",
	}, domain.ErrCodeExists, domain.ErrProjectSiteNotFound)
}

func (r *ProjectSiteRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.ProjectSiteID) error {
	return softDelete[models.ProjectSiteModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrProjectSiteNotFound)
}

func (r *ProjectSiteRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.ProjectSiteID) (*entities.ProjectSite, error) {
	m, err := findByID[models.ProjectSiteModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrProjectSiteNotFound)
	if err != nil {
		return nil, err
	}
	return gp.ProjectSiteToEntity(m), nil
}

func (r *ProjectSiteRepository) Exists(ctx context.Context, companyID vo.CompanyID, id vo.ProjectSiteID) (bool, error) {
	return exists(ctx, r.db, "organization_project_sites", uuidOf(companyID), uuidOf(id))
}

type projectSiteRow struct {
	ID         uuid.UUID
	Code       string
	Name       string
	StartDate  time.Time
	EndDate    *time.Time
	Status     string
	Address    models.AddressColumns `gorm:"embedded;embeddedPrefix:address_"`
	Branch     refCols               `gorm:"embedded;embeddedPrefix:branch_"`
	CostCenter refCols               `gorm:"embedded;embeddedPrefix:cc_"`
	Total      int64
}

const projectSiteColumns = `s.id, s.code, s.name, s.start_date, s.end_date, s.status,
	s.address_line, s.address_sub_district, s.address_district, s.address_province, s.address_postal_code, s.address_country,
	br.id AS branch_id, br.code AS branch_code, br.name AS branch_name,
	cc.id AS cc_id, cc.code AS cc_code, cc.name AS cc_name`

var projectSiteSortColumns = map[string]string{"code": "s.code", "name": "s.name", "start_date": "s.start_date"}

// base: project_sites LEFT JOIN branches / cost_centers
func (r *ProjectSiteRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("organization_project_sites AS s").
		Joins("LEFT JOIN organization_branches AS br ON br.id = s.branch_id AND br.deleted_at IS NULL").
		Joins("LEFT JOIN organization_cost_centers AS cc ON cc.id = s.cost_center_id AND cc.deleted_at IS NULL").
		Where("s.company_id = ? AND s.deleted_at IS NULL", uuidOf(companyID))
}

func (r *ProjectSiteRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.ProjectSiteID) (dto.ProjectSite, error) {
	row, err := first[projectSiteRow](r.base(ctx, companyID).Select(projectSiteColumns).Where("s.id = ?", uuidOf(id)), domain.ErrProjectSiteNotFound)
	return projectSiteDTO(row), err
}

// List: start_date / end_date = โครงการที่ช่วงเวลาซ้อนทับกับช่วงที่ค้นหา (โครงการที่ยังไม่มีวันสิ้นสุดถือว่ายังดำเนินอยู่)
func (r *ProjectSiteRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.ProjectSiteFilter) (listing.Result[dto.ProjectSite], error) {
	order, err := database.OrderBy(f.Sort, projectSiteSortColumns, "s.id")
	if err != nil {
		return listing.Result[dto.ProjectSite]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.base(ctx, companyID), f.Search, "s.code", "s.name")
		if len(f.Statuses) > 0 {
			q = q.Where("s.status IN ?", f.Statuses)
		}
		if f.BranchID != nil {
			q = q.Where("s.branch_id = ?", uuidOf(*f.BranchID))
		}
		if f.CostCenterID != nil {
			q = q.Where("s.cost_center_id = ?", uuidOf(*f.CostCenterID))
		}
		if f.StartDate != nil {
			q = q.Where("(s.end_date IS NULL OR s.end_date >= ?)", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("s.start_date <= ?", *f.EndDate)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, projectSiteColumns, order, f.Page, func(r projectSiteRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, projectSiteDTO), err
}

func projectSiteDTO(r projectSiteRow) dto.ProjectSite {
	return dto.ProjectSite{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, Branch: r.Branch.ref(), CostCenter: r.CostCenter.ref(),
		Address: gp.AddressToDomain(r.Address), StartDate: dateUTC(r.StartDate), EndDate: datePtr(r.EndDate), Status: r.Status,
	}
}
