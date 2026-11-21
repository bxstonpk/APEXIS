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

// ---------- Branch ----------

var _ ports.BranchRepository = (*BranchRepository)(nil)

type BranchRepository struct{ db *gorm.DB }

func NewBranchRepository(db *gorm.DB) *BranchRepository { return &BranchRepository{db: db} }

func (r *BranchRepository) Create(ctx context.Context, b *entities.Branch) error {
	m := gp.BranchToModel(b)
	return create(ctx, r.db, &m, domain.ErrBranchExists)
}

func (r *BranchRepository) Update(ctx context.Context, b *entities.Branch) error {
	m := gp.BranchToModel(b)
	return update(ctx, r.db, &m, m.CompanyID, []string{
		"code", "name", "tax_branch_no", "phone", "status",
		"address_line", "address_sub_district", "address_district", "address_province", "address_postal_code", "address_country",
	}, domain.ErrBranchExists, domain.ErrBranchNotFound)
}

func (r *BranchRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) error {
	return softDelete[models.BranchModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrBranchNotFound)
}

func (r *BranchRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) (*entities.Branch, error) {
	m, err := findByID[models.BranchModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrBranchNotFound)
	if err != nil {
		return nil, err
	}
	return gp.BranchToEntity(m), nil
}

func (r *BranchRepository) Exists(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) (bool, error) {
	return exists(ctx, r.db, "organization_branches", uuidOf(companyID), uuidOf(id))
}

func (r *BranchRepository) HasDependents(ctx context.Context, _ vo.CompanyID, id vo.BranchID) (bool, error) {
	return referenced(ctx, r.db, uuidOf(id),
		reference{"organization_departments", "branch_id"},
		reference{"organization_project_sites", "branch_id"})
}

type branchRow struct {
	ID          uuid.UUID
	Code        string
	Name        string
	TaxBranchNo string
	Address     models.AddressColumns `gorm:"embedded;embeddedPrefix:address_"`
	Phone       string
	Status      string
	Total       int64
}

const branchColumns = `b.id, b.code, b.name, b.tax_branch_no, b.phone, b.status,
	b.address_line, b.address_sub_district, b.address_district, b.address_province, b.address_postal_code, b.address_country`

var branchSortColumns = map[string]string{"code": "b.code", "name": "b.name", "tax_branch_no": "b.tax_branch_no"}

func (r *BranchRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("organization_branches AS b").
		Where("b.company_id = ? AND b.deleted_at IS NULL", uuidOf(companyID))
}

func (r *BranchRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) (dto.Branch, error) {
	row, err := first[branchRow](r.base(ctx, companyID).Select(branchColumns).Where("b.id = ?", uuidOf(id)), domain.ErrBranchNotFound)
	return branchDTO(row), err
}

// List — 1 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล)
func (r *BranchRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.BranchFilter) (listing.Result[dto.Branch], error) {
	order, err := database.OrderBy(f.Sort, branchSortColumns, "b.id")
	if err != nil {
		return listing.Result[dto.Branch]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.base(ctx, companyID), f.Search, "b.code", "b.name")
		if len(f.Statuses) > 0 {
			q = q.Where("b.status IN ?", f.Statuses)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, branchColumns, order, f.Page, func(r branchRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, branchDTO), err
}

func branchDTO(r branchRow) dto.Branch {
	return dto.Branch{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, TaxBranchNo: r.TaxBranchNo,
		IsHeadOffice: vo.TaxBranchNo(r.TaxBranchNo).IsHeadOffice(),
		Address:      gp.AddressToDomain(r.Address), Phone: r.Phone, Status: r.Status,
	}
}

func toResult[R any, D any](rows []R, total int64, page listing.Page, fn func(R) D) listing.Result[D] {
	items := make([]D, len(rows))
	for i, r := range rows {
		items[i] = fn(r)
	}
	return listing.Result[D]{Items: items, Total: total, Page: page}
}
