package mapper

import (
	"github.com/google/uuid"

	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/enums"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
)

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
		roleIDs, enums.MembershipStatus(m.Status),
	)
}

func MembershipsToEntities(ms []models.MembershipModel) []*entities.Membership {
	return mapAll(ms, MembershipToEntity)
}
