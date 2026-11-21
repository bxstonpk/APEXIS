// Package repositories คือ implementation ของ application/ports/repositories ด้วย gorm
//
// กติกา:
//   - ทุก method ใช้ database.Conn(ctx, db) เพื่อเข้าร่วม transaction ที่ use case เปิดไว้ใน ctx
//   - การเขียนหลายตาราง (aggregate + children) ใช้ database.TxConn(ctx) ซึ่งบังคับว่าต้องมี transaction
//   - repository ไม่เปิด transaction เอง
package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	ports "apexis/backend/internal/modules/identity/application/ports/repositories"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	gp "apexis/backend/internal/modules/identity/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
)

var _ ports.UserRepository = (*UserRepository)(nil)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *entities.User) error {
	m := gp.UserToModel(u)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *entities.User) error {
	m := gp.UserToModel(u)
	res := database.Conn(ctx, r.db).Model(&m).
		Select("email", "name", "password_hash", "status", "must_change_password", "updated_at").
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

func (r *UserRepository) findOne(ctx context.Context, query string, args ...any) (*entities.User, error) {
	var m models.UserModel
	err := database.Conn(ctx, r.db).
		Select("id", "email", "name", "password_hash", "status", "must_change_password").
		Where(query, args...).
		Take(&m).Error
	if err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return gp.UserToEntity(m), nil
}
