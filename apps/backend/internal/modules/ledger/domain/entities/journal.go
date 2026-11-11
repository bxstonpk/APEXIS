package entities

import (
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

type JournalData struct {
	Code   string // นำหน้าเลขที่เอกสาร เช่น JV -> JV2026-000001
	Name   string
	Status vo.Status
}

// Journal คือสมุดรายวัน (เช่น ทั่วไป JV, รับ RV, จ่าย PV) — เลขที่เอกสารเรียงแยกตามสมุดและปีบัญชี
type Journal struct {
	id        vo.JournalID
	companyID vo.CompanyID
	data      JournalData
}

func NewJournal(companyID vo.CompanyID, d JournalData) (*Journal, error) {
	n, err := normalizeJournal(d)
	if err != nil {
		return nil, err
	}
	return &Journal{id: vo.NewID[vo.JournalID](), companyID: companyID, data: n}, nil
}

func RehydrateJournal(id vo.JournalID, companyID vo.CompanyID, d JournalData) *Journal {
	return &Journal{id: id, companyID: companyID, data: d}
}

func (j *Journal) ID() vo.JournalID        { return j.id }
func (j *Journal) CompanyID() vo.CompanyID { return j.companyID }
func (j *Journal) Data() JournalData       { return j.data }
func (j *Journal) IsActive() bool          { return j.data.Status == vo.StatusActive }

func (j *Journal) Update(d JournalData) error {
	n, err := normalizeJournal(d)
	if err != nil {
		return err
	}
	j.data = n
	return nil
}

// EnsureDeletable: สมุดที่มีรายการแล้ว (ทุกสถานะ) ลบไม่ได้ — ปิดใช้งานแทน
func (j *Journal) EnsureDeletable(hasEntries bool) error {
	if hasEntries {
		return domain.ErrInUse
	}
	return nil
}

func normalizeJournal(d JournalData) (JournalData, error) {
	var err error
	out := JournalData{}
	if out.Code, err = vo.NewJournalCode(d.Code); err != nil {
		return JournalData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return JournalData{}, err
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return JournalData{}, err
	}
	return out, nil
}
