// Package mapper แปลงระหว่าง domain entity <-> gorm model
// เป็นที่เดียวที่รู้จักทั้งสองฝั่ง
package mapper

import (
	"github.com/google/uuid"

	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/enums"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
)

func UserToModel(u *entities.User) models.UserModel {
	return models.UserModel{
		ID:           uuid.UUID(u.ID()),
		Email:        u.Email().String(),
		Name:         u.Name(),
		PasswordHash: u.PasswordHash(),
		Status:       string(u.Status()),
	}
}

func UserToEntity(m models.UserModel) *entities.User {
	return entities.RehydrateUser(
		vo.UserID(m.ID), vo.Email(m.Email), m.Name, m.PasswordHash,
		enums.UserStatus(m.Status),
	)
}

func UsersToEntities(ms []models.UserModel) []*entities.User { return mapAll(ms, UserToEntity) }

// ToUUIDs แปลง typed ID (UserID, CompanyID, ...) เป็น []uuid.UUID สำหรับ query IN
func ToUUIDs[T ~[16]byte](ids []T) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out
}

func mapAll[M any, E any](ms []M, fn func(M) E) []E {
	out := make([]E, len(ms))
	for i, m := range ms {
		out[i] = fn(m)
	}
	return out
}
