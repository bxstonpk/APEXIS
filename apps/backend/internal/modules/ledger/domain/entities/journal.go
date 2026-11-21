package entities

import (
	"fmt"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
)

type JournalData struct {
	Code           string // prefix ของเลขที่รายการ เช่น JV -> JV2026-000001
	Name           string
	NumberTemplate string // รูปแบบเลขที่ (platform/numbering) — ว่าง = numbering.DefaultTemplate
	Status         vo.Status
}

// Journal คือสมุดรายวัน (เช่น ทั่วไป JV, รับ RV, จ่าย PV) — เลขที่รายการเรียงแยกตามสมุด
// ตามรูปแบบที่ตั้งไว้ (prefix = รหัสสมุด; นับใหม่ทุกปีบัญชี / ปี / เดือน ตามตัวแปรในรูปแบบ)
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

// NumberFormat คือรูปแบบเลขที่รายการของสมุดนี้
func (j *Journal) NumberFormat() numbering.Format {
	return numbering.Format{Prefix: j.data.Code, Template: j.data.NumberTemplate}
}

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
	out.NumberTemplate = d.NumberTemplate
	if out.NumberTemplate == "" {
		out.NumberTemplate = numbering.DefaultTemplate
	}
	f, err := numbering.Normalize(numbering.Format{Prefix: out.Code, Template: out.NumberTemplate})
	if err != nil {
		return JournalData{}, fmt.Errorf("%w: %v", domain.ErrInvalidNumbering, err)
	}
	out.NumberTemplate = f.Template
	return out, nil
}
