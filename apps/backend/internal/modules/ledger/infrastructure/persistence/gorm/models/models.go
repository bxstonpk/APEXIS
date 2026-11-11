// Package models คือ "รูปร่างตาราง" ของ ledger สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - ทุกตารางมี company_id — ไม่ใส่ FK ข้าม module (organization_companies / organization_fiscal_periods /
//     มิติวิเคราะห์) เพื่อให้แต่ละ module แยก schema ได้อิสระ; ความถูกต้องตรวจผ่าน organization ตอนบันทึก
//   - ผังบัญชี / สมุดรายวันใช้ soft delete และ partial unique index (รหัสใช้ซ้ำได้หลังลบ)
//   - รายการบัญชีไม่มี soft delete: draft ลบจริง, posted ลบไม่ได้ (กลับรายการแทน)
//   - จำนวนเงินเป็น NUMERIC(18,2); CHECK บังคับเดบิต/เครดิตของบรรทัด และความสมดุลของรายการที่ผ่านแล้ว
package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// Amount แปลง NUMERIC(18,2) <-> vo.Money (หน่วยย่อย int64) โดยไม่ผ่าน float
type Amount int64

func (a Amount) Value() (driver.Value, error) { return vo.Money(a).String(), nil }

func (a *Amount) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		*a = 0
		return nil
	case int64:
		*a = Amount(v * 100)
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("models: cannot scan %T into Amount", src)
	}
	// SUM / CASE อาจคืนทศนิยมเกิน 2 ตำแหน่ง (เป็นศูนย์) เช่น "10.5000"
	if whole, frac, ok := strings.Cut(s, "."); ok && len(frac) > 2 {
		if strings.Trim(frac[2:], "0") != "" {
			return fmt.Errorf("models: amount %q has more than 2 decimal places", s)
		}
		s = whole + "." + frac[:2]
	}
	m, err := vo.ParseMoney(s)
	if err != nil {
		return err
	}
	*a = Amount(m)
	return nil
}

type AccountModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_ledger_accounts_code,priority:1,where:deleted_at IS NULL"`
	Code      string         `gorm:"size:20;not null;uniqueIndex:uq_ledger_accounts_code,priority:2,where:deleted_at IS NULL"`
	Name      string         `gorm:"size:200;not null"`
	NameEN    string         `gorm:"column:name_en;size:200;not null;default:''"`
	Type      string         `gorm:"size:20;not null;check:chk_ledger_accounts_type,type IN ('asset','liability','equity','revenue','expense')"`
	ParentID  *uuid.UUID     `gorm:"type:uuid;index"`
	Parent    *AccountModel  `gorm:"foreignKey:ParentID;constraint:OnDelete:RESTRICT"`
	Postable  bool           `gorm:"not null"`
	Status    string         `gorm:"size:20;not null;check:chk_ledger_accounts_status,status IN ('active','inactive')"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (AccountModel) TableName() string { return "ledger_accounts" }

type JournalModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_ledger_journals_code,priority:1,where:deleted_at IS NULL"`
	Code      string         `gorm:"size:10;not null;uniqueIndex:uq_ledger_journals_code,priority:2,where:deleted_at IS NULL"`
	Name      string         `gorm:"size:200;not null"`
	Status    string         `gorm:"size:20;not null;check:chk_ledger_journals_status,status IN ('active','inactive')"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (JournalModel) TableName() string { return "ledger_journals" }

// EntryModel: หัวรายการบัญชี — total_debit / total_credit เก็บไว้เพื่อแสดงในรายการโดยไม่ต้องรวมบรรทัด
type EntryModel struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID     `gorm:"type:uuid;not null;index:idx_ledger_entries_date,priority:1;uniqueIndex:uq_ledger_entries_no,priority:1,where:entry_no IS NOT NULL"`
	JournalID      uuid.UUID     `gorm:"type:uuid;not null;index"`
	Journal        *JournalModel `gorm:"foreignKey:JournalID;constraint:OnDelete:RESTRICT"`
	EntryNo        *string       `gorm:"size:30;uniqueIndex:uq_ledger_entries_no,priority:2,where:entry_no IS NOT NULL;check:chk_ledger_entries_posted,status = 'draft' OR (entry_no IS NOT NULL AND fiscal_period_id IS NOT NULL AND posted_at IS NOT NULL AND posted_by IS NOT NULL)"`
	EntryDate      time.Time     `gorm:"type:date;not null;index:idx_ledger_entries_date,priority:2"`
	FiscalPeriodID *uuid.UUID    `gorm:"type:uuid"`
	Description    string        `gorm:"size:500;not null;default:''"`
	Reference      string        `gorm:"size:100;not null;default:''"`
	Status         string        `gorm:"size:20;not null;check:chk_ledger_entries_status,status IN ('draft','posted','reversed')"`
	TotalDebit     Amount        `gorm:"type:numeric(18,2);not null;default:0;check:chk_ledger_entries_balanced,status = 'draft' OR (total_debit = total_credit AND total_debit > 0)"`
	TotalCredit    Amount        `gorm:"type:numeric(18,2);not null;default:0"`
	ReversalOfID   *uuid.UUID    `gorm:"type:uuid;index"`
	ReversalOf     *EntryModel   `gorm:"foreignKey:ReversalOfID;constraint:OnDelete:RESTRICT"`
	ReversedByID   *uuid.UUID    `gorm:"type:uuid"`
	ReversedBy     *EntryModel   `gorm:"foreignKey:ReversedByID;constraint:OnDelete:RESTRICT"`
	PostedAt       *time.Time
	PostedBy       *uuid.UUID  `gorm:"type:uuid"`
	CreatedBy      uuid.UUID   `gorm:"type:uuid;not null"`
	Lines          []LineModel `gorm:"foreignKey:EntryID;constraint:OnDelete:CASCADE"`
	CreatedAt      time.Time   `gorm:"not null"`
	UpdatedAt      time.Time   `gorm:"not null"`
}

func (EntryModel) TableName() string { return "ledger_journal_entries" }

// LineModel: บรรทัดรายการ — มิติวิเคราะห์เป็น id ของ organization (ไม่มี FK ข้าม module)
type LineModel struct {
	EntryID       uuid.UUID     `gorm:"type:uuid;primaryKey"`
	LineNo        int           `gorm:"primaryKey;check:chk_ledger_lines_no,line_no > 0"`
	AccountID     uuid.UUID     `gorm:"type:uuid;not null;index"`
	Account       *AccountModel `gorm:"foreignKey:AccountID;constraint:OnDelete:RESTRICT"`
	Description   string        `gorm:"size:500;not null;default:''"`
	Debit         Amount        `gorm:"type:numeric(18,2);not null;default:0;check:chk_ledger_lines_amount,debit >= 0 AND credit >= 0 AND (debit > 0) <> (credit > 0)"`
	Credit        Amount        `gorm:"type:numeric(18,2);not null;default:0"`
	BranchID      *uuid.UUID    `gorm:"type:uuid"`
	DepartmentID  *uuid.UUID    `gorm:"type:uuid"`
	CostCenterID  *uuid.UUID    `gorm:"type:uuid"`
	ProjectSiteID *uuid.UUID    `gorm:"type:uuid"`
}

func (LineModel) TableName() string { return "ledger_journal_lines" }

// หมายเหตุ: gorm อ่าน tag check ได้ field ละ 1 ข้อ — CHECK หลายข้อของตารางจึงกระจายไว้หลาย field

// SequenceModel: เลขที่ล่าสุดของสมุด + ปีบัญชี (เพิ่มด้วย INSERT ... ON CONFLICT DO UPDATE ... RETURNING)
type SequenceModel struct {
	JournalID  uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Journal    *JournalModel `gorm:"foreignKey:JournalID;constraint:OnDelete:RESTRICT"`
	FiscalYear int           `gorm:"primaryKey"`
	CompanyID  uuid.UUID     `gorm:"type:uuid;not null"`
	LastNo     int64         `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "ledger_journal_sequences" }

// AutoMigrate สร้าง/อัปเดตตารางของ ledger (เรียงตาม dependency ของ FK)
// สำหรับ production แนะนำเปลี่ยนเป็น versioned migration (เช่น goose / golang-migrate)
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&AccountModel{}, &JournalModel{}, &EntryModel{}, &LineModel{}, &SequenceModel{})
}
