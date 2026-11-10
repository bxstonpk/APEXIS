package mapper

import (
	"github.com/google/uuid"

	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/enums"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
)

func CompanyToModel(c *entities.Company) models.CompanyModel {
	return models.CompanyModel{
		ID:     uuid.UUID(c.ID()),
		Code:   c.Code(),
		Name:   c.Name(),
		TaxID:  c.TaxID(),
		Status: string(c.Status()),
	}
}

func CompanyToEntity(m models.CompanyModel) *entities.Company {
	return entities.RehydrateCompany(
		vo.CompanyID(m.ID), m.Code, m.Name, m.TaxID,
		enums.CompanyStatus(m.Status),
	)
}

func CompaniesToEntities(ms []models.CompanyModel) []*entities.Company {
	return mapAll(ms, CompanyToEntity)
}
