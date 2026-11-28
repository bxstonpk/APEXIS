package gormpersistence

import (
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
)

// ---------- ค่าตั้ง ----------

func SettingsToModel(companyID vo.CompanyID, s entities.Settings, now time.Time) models.SettingsModel {
	return models.SettingsModel{
		CompanyID:                 uuid.UUID(companyID),
		RetainedEarningsAccountID: optUUID(s.RetainedEarningsAccountID),
		IncomeSummaryAccountID:    optUUID(s.IncomeSummaryAccountID),
		ClosingJournalID:          optUUID(s.ClosingJournalID),
		CreatedAt:                 now, UpdatedAt: now,
	}
}

func SettingsToEntity(m models.SettingsModel) entities.Settings {
	return entities.Settings{
		RetainedEarningsAccountID: optID[vo.AccountID](m.RetainedEarningsAccountID),
		IncomeSummaryAccountID:    optID[vo.AccountID](m.IncomeSummaryAccountID),
		ClosingJournalID:          optID[vo.JournalID](m.ClosingJournalID),
	}
}

// ---------- ปิดบัญชีสิ้นปี ----------

func YearEndToModel(y *entities.YearEnd, now time.Time) models.YearEndClosingModel {
	s := y.State()
	return models.YearEndClosingModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), FiscalYear: s.FiscalYear,
		StartDate: s.StartDate, EndDate: s.EndDate, Status: string(s.Status), NetIncome: s.NetIncome,
		EntryID: optUUID(s.EntryID), ReversalID: optUUID(s.ReversalID),
		ClosedBy: uuid.UUID(s.ClosedBy), ClosedAt: s.ClosedAt, ReopenedBy: optUUID(s.ReopenedBy), ReopenedAt: s.ReopenedAt,
		CreatedAt: now, UpdatedAt: now,
	}
}

func YearEndToEntity(m models.YearEndClosingModel) *entities.YearEnd {
	return entities.RehydrateYearEnd(entities.YearEndState{
		ID: vo.YearEndClosingID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), FiscalYear: m.FiscalYear,
		StartDate: DateUTC(m.StartDate), EndDate: DateUTC(m.EndDate), Status: entities.YearEndStatus(m.Status),
		NetIncome: m.NetIncome, EntryID: optID[vo.EntryID](m.EntryID), ReversalID: optID[vo.EntryID](m.ReversalID),
		ClosedBy: vo.UserID(m.ClosedBy), ClosedAt: m.ClosedAt,
		ReopenedBy: optID[vo.UserID](m.ReopenedBy), ReopenedAt: m.ReopenedAt,
	})
}

// OptEntryID แปลง *uuid -> *vo.EntryID (ใช้ใน read model ของ repository)
func OptEntryID(u *uuid.UUID) *vo.EntryID { return optID[vo.EntryID](u) }
