package gormpersistence

import (
	"github.com/google/uuid"

	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/models"
)

func TaxFilingToModel(f *entities.TaxFiling) models.TaxFilingModel {
	s := f.State()
	d := s.Data
	return models.TaxFilingModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), Form: string(d.Form), TaxMonth: d.Month.Start(), FiledDate: d.FiledDate,
		Amount: d.Amount, Surcharge: d.Surcharge, Penalty: d.Penalty, CarryForward: d.CarryForward, Reference: d.Reference, Note: d.Note,
		PaymentID: OptUUID(d.PaymentID), Status: string(s.Status), CreatedBy: uuid.UUID(s.CreatedBy), CreatedAt: s.CreatedAt,
		CancelledBy: OptUUID(s.CancelledBy), CancelledAt: s.CancelledAt,
	}
}

func TaxFilingToEntity(m models.TaxFilingModel) *entities.TaxFiling {
	s := entities.TaxFilingState{
		ID: vo.TaxFilingID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Status: vo.TaxFilingStatus(m.Status),
		CreatedBy: vo.UserID(m.CreatedBy), CreatedAt: m.CreatedAt, CancelledAt: m.CancelledAt,
		Data: entities.TaxFilingData{
			Form: vo.TaxForm(m.Form), Month: vo.TaxMonthOf(m.TaxMonth), FiledDate: DateUTC(m.FiledDate), Amount: m.Amount, Surcharge: m.Surcharge,
			Penalty: m.Penalty, CarryForward: m.CarryForward, Reference: m.Reference, Note: m.Note,
		},
	}
	if m.PaymentID != nil {
		id := vo.PaymentID(*m.PaymentID)
		s.Data.PaymentID = &id
	}
	if m.CancelledBy != nil {
		by := vo.UserID(*m.CancelledBy)
		s.CancelledBy = &by
	}
	return entities.RehydrateTaxFiling(s)
}
