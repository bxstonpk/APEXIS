// Package repositories คือ implementation ของ domain/repositories ด้วย gorm
// ทุก method ใช้ database.Conn(ctx, db) เพื่อให้เข้าร่วม transaction ที่อยู่ใน ctx อัตโนมัติ
package repositories

import (
	"context"

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

var _ domainrepo.UserRepository = (*UserRepository)(nil)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *entities.User) error {
	m := mapper.UserToModel(u)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *entities.User) error {
	m := mapper.UserToModel(u)
	res := database.Conn(ctx, r.db).Model(&m).
		Select("email", "name", "password_hash", "status", "updated_at").
		Updates(&m)
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrEmailAlreadyExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id vo.UserID) (*entities.User, error) {
	return r.findOne(ctx, "id = ?", uuid.UUID(id))
}

func (r *UserRepository) FindByEmail(ctx context.Context, email vo.Email) (*entities.User, error) {
	return r.findOne(ctx, "email = ?", email.String())
}

func (r *UserRepository) FindByIDs(ctx context.Context, ids []vo.UserID) ([]*entities.User, error) {
	if len(ids) == 0 {
		return []*entities.User{}, nil
	}
	var ms []models.UserModel
	if err := database.Conn(ctx, r.db).Where("id IN ?", mapper.ToUUIDs(ids)).Order("name").Find(&ms).Error; err != nil {
		return nil, err
	}
	return mapper.UsersToEntities(ms), nil
}

func (r *UserRepository) findOne(ctx context.Context, query string, args ...any) (*entities.User, error) {
	var m models.UserModel
	if err := database.Conn(ctx, r.db).Where(query, args...).Take(&m).Error; err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return mapper.UserToEntity(m), nil
}
