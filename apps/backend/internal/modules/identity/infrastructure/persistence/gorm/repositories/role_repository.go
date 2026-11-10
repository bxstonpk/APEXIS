package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	domainrepo "account/backend/internal/modules/identity/domain/repositories"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/mapper"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
	"account/backend/internal/platform/database"
)

var _ domainrepo.RoleRepository = (*RoleRepository)(nil)

type RoleRepository struct{ db *gorm.DB }

func NewRoleRepository(db *gorm.DB) *RoleRepository { return &RoleRepository{db: db} }

func (r *RoleRepository) Create(ctx context.Context, role *entities.Role) error {
	m := mapper.RoleToModel(role)
	return database.Transact(ctx, r.db, func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
			if database.IsUniqueViolation(err) {
				return domain.ErrRoleCodeExists
			}
			return err
		}
		return insertRolePermissions(tx, m.Permissions)
	})
}

// Update แก้ชื่อ/คำอธิบาย และแทนที่ permission ทั้งชุด
func (r *RoleRepository) Update(ctx context.Context, role *entities.Role) error {
	m := mapper.RoleToModel(role)
	return database.Transact(ctx, r.db, func(tx *gorm.DB) error {
		res := tx.Model(&m).
			Where("company_id = ?", m.CompanyID).
			Select("name", "description", "updated_at").
			Omit(clause.Associations).
			Updates(&m)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrRoleNotFound
		}
		if err := tx.Where("role_id = ?", m.ID).Delete(&models.RolePermissionModel{}).Error; err != nil {
			return err
		}
		return insertRolePermissions(tx, m.Permissions)
	})
}

func (r *RoleRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) error {
	res := database.Conn(ctx, r.db).
		Where("company_id = ? AND id = ?", uuid.UUID(companyID), uuid.UUID(id)).
		Delete(&models.RoleModel{})
	if res.Error != nil {
		if database.IsForeignKeyViolation(res.Error) {
			return domain.ErrRoleInUse
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

func (r *RoleRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) (*entities.Role, error) {
	var m models.RoleModel
	err := database.Conn(ctx, r.db).Preload("Permissions").
		Where("company_id = ? AND id = ?", uuid.UUID(companyID), uuid.UUID(id)).
		Take(&m).Error
	if err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, err
	}
	return mapper.RoleToEntity(m), nil
}

func (r *RoleRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.RoleID) ([]*entities.Role, error) {
	if len(ids) == 0 {
		return []*entities.Role{}, nil
	}
	var ms []models.RoleModel
	err := database.Conn(ctx, r.db).Preload("Permissions").
		Where("company_id = ? AND id IN ?", uuid.UUID(companyID), mapper.ToUUIDs(ids)).
		Order("code").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.RolesToEntities(ms), nil
}

func (r *RoleRepository) ListByCompany(ctx context.Context, companyID vo.CompanyID) ([]*entities.Role, error) {
	var ms []models.RoleModel
	err := database.Conn(ctx, r.db).Preload("Permissions").
		Where("company_id = ?", uuid.UUID(companyID)).
		Order("is_system DESC, code").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.RolesToEntities(ms), nil
}

func insertRolePermissions(tx *gorm.DB, perms []models.RolePermissionModel) error {
	if len(perms) == 0 {
		return nil
	}
	return tx.Create(&perms).Error
}
