package mapper

import (
	"github.com/google/uuid"

	"account/backend/internal/modules/identity/domain/entities"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
)

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
