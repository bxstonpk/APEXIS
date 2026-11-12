package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

var _ ports.CompanyRepository = (*CompanyRepository)(nil)

type CompanyRepository struct{ db *gorm.DB }

func NewCompanyRepository(db *gorm.DB) *CompanyRepository { return &CompanyRepository{db: db} }

func (r *CompanyRepository) Create(ctx context.Context, c *entities.Company) error {
	m := gp.CompanyToModel(c)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

// Update แก้ทุก column ยกเว้น code (แก้ไม่ได้) และ created_at
func (r *CompanyRepository) Update(ctx context.Context, c *entities.Company) error {
	m := gp.CompanyToModel(c)
	res := database.Conn(ctx, r.db).Model(&m).
		Select(
			"name", "tax_id", "legal_name_th", "legal_name_en",
			"address_line", "address_sub_district", "address_district", "address_province", "address_postal_code", "address_country",
			"phone", "email", "website", "base_currency", "fiscal_year_start_month",
			"amount_decimals", "quantity_decimals", "status", "updated_at",
		).
		Omit(clause.Associations).
		Updates(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrCompanyNotFound
	}
	return nil
}

func (r *CompanyRepository) FindByID(ctx context.Context, id vo.CompanyID) (*entities.Company, error) {
	var m models.CompanyModel
	err := database.Conn(ctx, r.db).Where("id = ?", uuid.UUID(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrCompanyNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.CompanyToEntity(m), nil
}

// Settings — 1 query ผ่าน primary key: status + จำนวนทศนิยม (ถูกเรียกทุกครั้งที่เขียนข้อมูลของบริษัท)
func (r *CompanyRepository) Settings(ctx context.Context, id vo.CompanyID) (dto.CompanySettings, error) {
	var rows []struct {
		Status           string
		AmountDecimals   int
		QuantityDecimals int
	}
	err := database.Conn(ctx, r.db).Model(&models.CompanyModel{}).
		Select("status, amount_decimals, quantity_decimals").
		Where("id = ?", uuid.UUID(id)).Limit(1).Scan(&rows).Error
	if err != nil {
		return dto.CompanySettings{}, err
	}
	if len(rows) == 0 {
		return dto.CompanySettings{}, domain.ErrCompanyNotFound
	}
	s := rows[0]
	return dto.CompanySettings{
		Active: s.Status == string(vo.StatusActive), AmountDecimals: s.AmountDecimals, QuantityDecimals: s.QuantityDecimals,
	}, nil
}

var companySortColumns = map[string]string{
	"code": "c.code",
	"name": "c.name",
}

// ListByIDs — 1 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล), ไม่มี id = ไม่ query
func (r *CompanyRepository) ListByIDs(ctx context.Context, ids []vo.CompanyID, f dto.CompanyFilter) (listing.Result[dto.CompanySummary], error) {
	if len(ids) == 0 {
		return listing.Result[dto.CompanySummary]{Items: []dto.CompanySummary{}, Page: f.Page}, nil
	}
	order, err := database.OrderBy(f.Sort, companySortColumns, "c.id")
	if err != nil {
		return listing.Result[dto.CompanySummary]{}, err
	}
	keys := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		keys[i] = uuid.UUID(id)
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("organization_companies AS c").Where("c.id IN ?", keys)
		if len(f.Statuses) > 0 {
			q = q.Where("c.status IN ?", f.Statuses)
		}
		return search(q, f.Search, "c.code", "c.name", "c.legal_name_th")
	}
	type row struct {
		ID     uuid.UUID
		Code   string
		Name   string
		TaxID  string
		Status string
		Total  int64
	}
	rows, total, err := database.FetchPage(base, "c.id, c.code, c.name, c.tax_id, c.status", order, f.Page,
		func(r row) int64 { return r.Total })
	if err != nil {
		return listing.Result[dto.CompanySummary]{}, err
	}
	items := make([]dto.CompanySummary, len(rows))
	for i, row := range rows {
		items[i] = dto.CompanySummary{ID: row.ID.String(), Code: row.Code, Name: row.Name, TaxID: row.TaxID, Status: row.Status}
	}
	return listing.Result[dto.CompanySummary]{Items: items, Total: total, Page: f.Page}, nil
}
