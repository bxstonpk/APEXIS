// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ identity
//   - mapper.go       แปลง domain entity <-> gorm model (ที่เดียวที่รู้จักทั้งสองฝั่ง)
//   - transaction.go  implement ports.TransactionManager
//   - models/         รูปร่างตาราง
//   - repositories/   implement application/ports/repositories
package gormpersistence

import (
	"github.com/google/uuid"

	"apexis/backend/internal/modules/identity/domain/entities"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
)

// ---------- User ----------

func UserToModel(u *entities.User) models.UserModel {
	return models.UserModel{
		ID:                 uuid.UUID(u.ID()),
		Email:              u.Email().String(),
		Name:               u.Name(),
		PasswordHash:       u.PasswordHash(),
		Status:             string(u.Status()),
		MustChangePassword: u.MustChangePassword(),
	}
}

func UserToEntity(m models.UserModel) *entities.User {
	return entities.RehydrateUser(
		vo.UserID(m.ID), vo.Email(m.Email), m.Name, m.PasswordHash,
		vo.UserStatus(m.Status), m.MustChangePassword,
	)
}

// ---------- Role ----------

func RoleToModel(r *entities.Role) models.RoleModel {
	return models.RoleModel{
		ID:          uuid.UUID(r.ID()),
		CompanyID:   uuid.UUID(r.CompanyID()),
		Code:        r.Code(),
		Name:        r.Name(),
		Description: r.Description(),
		IsSystem:    r.IsSystem(),
		Permissions: RolePermissionsToModel(r),
	}
}

func RolePermissionsToModel(r *entities.Role) []models.RolePermissionModel {
	perms := r.Permissions().Sorted()
	out := make([]models.RolePermissionModel, len(perms))
	for i, p := range perms {
		out[i] = models.RolePermissionModel{RoleID: uuid.UUID(r.ID()), Permission: p.String()}
	}
	return out
}

func RoleToEntity(m models.RoleModel) *entities.Role {
	perms := vo.NewPermissionSet()
	for _, p := range m.Permissions {
		perms.Add(vo.Permission(p.Permission))
	}
	return entities.RehydrateRole(
		vo.RoleID(m.ID), vo.CompanyID(m.CompanyID), m.Code, m.Name, m.Description,
		perms, m.IsSystem,
	)
}

func RolesToEntities(ms []models.RoleModel) []*entities.Role { return mapAll(ms, RoleToEntity) }

// ---------- Membership ----------

func MembershipToModel(m *entities.Membership) models.MembershipModel {
	return models.MembershipModel{
		ID:        uuid.UUID(m.ID()),
		CompanyID: uuid.UUID(m.CompanyID()),
		UserID:    uuid.UUID(m.UserID()),
		Status:    string(m.Status()),
		Roles:     MembershipRolesToModel(m),
	}
}

func MembershipRolesToModel(m *entities.Membership) []models.MembershipRoleModel {
	ids := m.RoleIDs()
	out := make([]models.MembershipRoleModel, len(ids))
	for i, id := range ids {
		out[i] = models.MembershipRoleModel{MembershipID: uuid.UUID(m.ID()), RoleID: uuid.UUID(id)}
	}
	return out
}

func MembershipToEntity(m models.MembershipModel) *entities.Membership {
	roleIDs := make([]vo.RoleID, len(m.Roles))
	for i, r := range m.Roles {
		roleIDs[i] = vo.RoleID(r.RoleID)
	}
	return entities.RehydrateMembership(
		vo.MembershipID(m.ID), vo.UserID(m.UserID), vo.CompanyID(m.CompanyID),
		roleIDs, vo.MembershipStatus(m.Status),
	)
}

// ---------- helpers ----------

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
