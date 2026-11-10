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

var _ domainrepo.MembershipRepository = (*MembershipRepository)(nil)

type MembershipRepository struct{ db *gorm.DB }

func NewMembershipRepository(db *gorm.DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

func (r *MembershipRepository) Create(ctx context.Context, ms *entities.Membership) error {
	m := mapper.MembershipToModel(ms)
	return database.Transact(ctx, r.db, func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
			if database.IsUniqueViolation(err) {
				return domain.ErrMembershipExists
			}
			return err
		}
		return insertMembershipRoles(tx, m.Roles)
	})
}

// Update แก้สถานะ และแทนที่ role ทั้งชุด
func (r *MembershipRepository) Update(ctx context.Context, ms *entities.Membership) error {
	m := mapper.MembershipToModel(ms)
	return database.Transact(ctx, r.db, func(tx *gorm.DB) error {
		res := tx.Model(&m).
			Where("company_id = ?", m.CompanyID).
			Select("status", "updated_at").
			Omit(clause.Associations).
			Updates(&m)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrMembershipNotFound
		}
		if err := tx.Where("membership_id = ?", m.ID).Delete(&models.MembershipRoleModel{}).Error; err != nil {
			return err
		}
		return insertMembershipRoles(tx, m.Roles)
	})
}

func (r *MembershipRepository) Find(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) (*entities.Membership, error) {
	var m models.MembershipModel
	err := database.Conn(ctx, r.db).Preload("Roles").
		Where("company_id = ? AND user_id = ?", uuid.UUID(companyID), uuid.UUID(userID)).
		Take(&m).Error
	if err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrMembershipNotFound
		}
		return nil, err
	}
	return mapper.MembershipToEntity(m), nil
}

func (r *MembershipRepository) ListByUser(ctx context.Context, userID vo.UserID) ([]*entities.Membership, error) {
	var ms []models.MembershipModel
	err := database.Conn(ctx, r.db).Preload("Roles").
		Where("user_id = ?", uuid.UUID(userID)).
		Order("created_at").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.MembershipsToEntities(ms), nil
}

func (r *MembershipRepository) ListByCompany(ctx context.Context, companyID vo.CompanyID) ([]*entities.Membership, error) {
	var ms []models.MembershipModel
	err := database.Conn(ctx, r.db).Preload("Roles").
		Where("company_id = ?", uuid.UUID(companyID)).
		Order("created_at").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.MembershipsToEntities(ms), nil
}

func insertMembershipRoles(tx *gorm.DB, roles []models.MembershipRoleModel) error {
	if len(roles) == 0 {
		return nil
	}
	return tx.Create(&roles).Error
}
