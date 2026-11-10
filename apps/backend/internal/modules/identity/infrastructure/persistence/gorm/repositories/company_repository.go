package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	domainrepo "account/backend/internal/modules/identity/domain/repositories"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/mapper"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
	"account/backend/internal/platform/database"
)

var _ domainrepo.CompanyRepository = (*CompanyRepository)(nil)

type CompanyRepository struct{ db *gorm.DB }

func NewCompanyRepository(db *gorm.DB) *CompanyRepository { return &CompanyRepository{db: db} }

func (r *CompanyRepository) Create(ctx context.Context, c *entities.Company) error {
	m := mapper.CompanyToModel(c)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrCompanyCodeExists
		}
		return err
	}
	return nil
}

func (r *CompanyRepository) Update(ctx context.Context, c *entities.Company) error {
	m := mapper.CompanyToModel(c)
	res := database.Conn(ctx, r.db).Model(&m).
		Select("name", "tax_id", "status", "updated_at").
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
	return r.findOne(ctx, "id = ?", uuid.UUID(id))
}

func (r *CompanyRepository) FindByCode(ctx context.Context, code string) (*entities.Company, error) {
	return r.findOne(ctx, "code = ?", strings.ToUpper(strings.TrimSpace(code)))
}

func (r *CompanyRepository) FindByIDs(ctx context.Context, ids []vo.CompanyID) ([]*entities.Company, error) {
	if len(ids) == 0 {
		return []*entities.Company{}, nil
	}
	var ms []models.CompanyModel
	if err := database.Conn(ctx, r.db).Where("id IN ?", mapper.ToUUIDs(ids)).Order("name").Find(&ms).Error; err != nil {
		return nil, err
	}
	return mapper.CompaniesToEntities(ms), nil
}

func (r *CompanyRepository) findOne(ctx context.Context, query string, args ...any) (*entities.Company, error) {
	var m models.CompanyModel
	if err := database.Conn(ctx, r.db).Where(query, args...).Take(&m).Error; err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrCompanyNotFound
		}
		return nil, err
	}
	return mapper.CompanyToEntity(m), nil
}
