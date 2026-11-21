// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ ledger
//   - mapper.go       แปลง domain entity <-> gorm model (ที่เดียวที่รู้จักทั้งสองฝั่ง)
//   - transaction.go  implement ports.TransactionManager
//   - models/         รูปร่างตาราง
//   - repositories/   implement application/ports/repositories
package gormpersistence

import (
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
)

// ---------- id ----------

func optUUID[T ~[16]byte](id *T) *uuid.UUID {
	if id == nil {
		return nil
	}
	u := uuid.UUID(*id)
	return &u
}

func optID[T ~[16]byte](u *uuid.UUID) *T {
	if u == nil {
		return nil
	}
	id := T(*u)
	return &id
}

// DateUTC: คอลัมน์ date อ่านกลับมาอาจมี location ของ driver — แปลงเป็นเที่ยงคืน UTC ตามข้อตกลงของ domain
func DateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ---------- Account ----------

func AccountToModel(a *entities.Account) models.AccountModel {
	d := a.Data()
	return models.AccountModel{
		ID: uuid.UUID(a.ID()), CompanyID: uuid.UUID(a.CompanyID()), Code: d.Code, Name: d.Name, NameEN: d.NameEN,
		Type: string(d.Type), ParentID: optUUID(d.ParentID), Postable: d.Postable, Status: string(d.Status),
	}
}

func AccountToEntity(m models.AccountModel) *entities.Account {
	return entities.RehydrateAccount(vo.AccountID(m.ID), vo.CompanyID(m.CompanyID), entities.AccountData{
		Code: m.Code, Name: m.Name, NameEN: m.NameEN, Type: vo.AccountType(m.Type),
		ParentID: optID[vo.AccountID](m.ParentID), Postable: m.Postable, Status: vo.Status(m.Status),
	})
}

// ---------- Journal ----------

func JournalToModel(j *entities.Journal) models.JournalModel {
	d := j.Data()
	return models.JournalModel{
		ID: uuid.UUID(j.ID()), CompanyID: uuid.UUID(j.CompanyID()), Code: d.Code, Name: d.Name,
		NumberTemplate: d.NumberTemplate, Status: string(d.Status),
	}
}

func JournalToEntity(m models.JournalModel) *entities.Journal {
	return entities.RehydrateJournal(vo.JournalID(m.ID), vo.CompanyID(m.CompanyID), entities.JournalData{
		Code: m.Code, Name: m.Name, NumberTemplate: m.NumberTemplate, Status: vo.Status(m.Status),
	})
}

// ---------- Entry ----------

func EntryToModel(e *entities.Entry) models.EntryModel {
	s := e.State()
	debit, credit := e.Totals()
	m := models.EntryModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), JournalID: uuid.UUID(s.Data.JournalID),
		EntryDate: s.Data.Date, Description: s.Data.Description, Reference: s.Data.Reference, Status: string(s.Status),
		TotalDebit: debit, TotalCredit: credit,
		ReversalOfID: optUUID(s.ReversalOf), ReversedByID: optUUID(s.ReversedBy), CreatedBy: uuid.UUID(s.CreatedBy),
	}
	if p := s.Posting; p != nil {
		no, period, by, at := p.EntryNo, uuid.UUID(p.PeriodID), uuid.UUID(p.By), p.At
		m.EntryNo, m.FiscalPeriodID, m.PostedBy, m.PostedAt = &no, &period, &by, &at
	}
	m.Lines = make([]models.LineModel, len(s.Data.Lines))
	for i, l := range s.Data.Lines {
		m.Lines[i] = models.LineModel{
			EntryID: m.ID, LineNo: i + 1, AccountID: uuid.UUID(l.AccountID), Description: l.Description,
			Debit: l.Debit, Credit: l.Credit,
			BranchID: optUUID(l.BranchID), DepartmentID: optUUID(l.DepartmentID),
			CostCenterID: optUUID(l.CostCenterID), ProjectSiteID: optUUID(l.ProjectSiteID),
		}
	}
	return m
}

func EntryToEntity(m models.EntryModel) *entities.Entry {
	s := entities.EntryState{
		ID: vo.EntryID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Status: vo.EntryStatus(m.Status),
		ReversalOf: optID[vo.EntryID](m.ReversalOfID), ReversedBy: optID[vo.EntryID](m.ReversedByID),
		CreatedBy: vo.UserID(m.CreatedBy),
		Data: entities.EntryData{
			JournalID: vo.JournalID(m.JournalID), Date: DateUTC(m.EntryDate),
			Description: m.Description, Reference: m.Reference, Lines: make([]entities.LineData, len(m.Lines)),
		},
	}
	if m.EntryNo != nil && m.FiscalPeriodID != nil && m.PostedBy != nil && m.PostedAt != nil {
		s.Posting = &entities.Posting{
			EntryNo: *m.EntryNo, PeriodID: vo.FiscalPeriodID(*m.FiscalPeriodID), By: vo.UserID(*m.PostedBy), At: *m.PostedAt,
		}
	}
	for i, l := range m.Lines {
		s.Data.Lines[i] = entities.LineData{
			AccountID: vo.AccountID(l.AccountID), Description: l.Description,
			Debit: l.Debit, Credit: l.Credit,
			BranchID: optID[vo.BranchID](l.BranchID), DepartmentID: optID[vo.DepartmentID](l.DepartmentID),
			CostCenterID: optID[vo.CostCenterID](l.CostCenterID), ProjectSiteID: optID[vo.ProjectSiteID](l.ProjectSiteID),
		}
	}
	return entities.RehydrateEntry(s)
}
