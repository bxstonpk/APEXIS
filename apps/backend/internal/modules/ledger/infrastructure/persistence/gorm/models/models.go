// Package models คือ "รูปร่างตาราง" ของ ledger สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - ทุกตารางมี company_id — ไม่ใส่ FK ข้าม module (organization_companies / organization_fiscal_periods /
//     มิติวิเคราะห์) เพื่อให้แต่ละ module แยก schema ได้อิสระ; ความถูกต้องตรวจผ่าน organization ตอนบันทึก
//   - ผังบัญชี / สมุดรายวันใช้ soft delete และ partial unique index (รหัสใช้ซ้ำได้หลังลบ)
//   - รายการบัญชีไม่มี soft delete: draft ลบจริง, posted ลบไม่ได้ (กลับรายการแทน)
//   - จำนวนเงินเป็น NUMERIC(24,6) (ทศนิยมที่ยอมรับกำหนดต่อบริษัท); CHECK บังคับเดบิต/เครดิตของบรรทัด และความสมดุลของรายการที่ผ่านแล้ว
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type AccountModel struct {
	ID        uuid.UUID     `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex:uq_ledger_accounts_code,priority:1,where:deleted_at IS NULL"`
	Code      string        `gorm:"size:20;not null;uniqueIndex:uq_ledger_accounts_code,priority:2,where:deleted_at IS NULL"`
	Name      string        `gorm:"size:200;not null"`
	NameEN    string        `gorm:"column:name_en;size:200;not null;default:''"`
	Type      string        `gorm:"size:20;not null;check:chk_ledger_accounts_type,type IN ('asset','liability','equity','revenue','expense','suspense')"`
	ParentID  *uuid.UUID    `gorm:"type:uuid;index"`
	Parent    *AccountModel `gorm:"foreignKey:ParentID;constraint:OnDelete:RESTRICT"`
	Postable  bool          `gorm:"not null"`
	Status    string        `gorm:"size:20;not null;check:chk_ledger_accounts_status,status IN ('active','inactive')"`
	// Description คำอธิบายการใช้บัญชี
	Description string         `gorm:"size:500;not null;default:''"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (AccountModel) TableName() string { return "ledger_accounts" }

type JournalModel struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_ledger_journals_code,priority:1,where:deleted_at IS NULL"`
	Code           string         `gorm:"size:10;not null;uniqueIndex:uq_ledger_journals_code,priority:2,where:deleted_at IS NULL"`
	Name           string         `gorm:"size:200;not null"`
	NumberTemplate string         `gorm:"size:60;not null;default:'{prefix}{fy}-{seq:6}'"`
	Status         string         `gorm:"size:20;not null;check:chk_ledger_journals_status,status IN ('active','inactive')"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (JournalModel) TableName() string { return "ledger_journals" }

// EntryModel: หัวรายการบัญชี — total_debit / total_credit เก็บไว้เพื่อแสดงในรายการโดยไม่ต้องรวมบรรทัด
type EntryModel struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID       `gorm:"type:uuid;not null;index:idx_ledger_entries_date,priority:1;uniqueIndex:uq_ledger_entries_no,priority:1,where:entry_no IS NOT NULL"`
	JournalID      uuid.UUID       `gorm:"type:uuid;not null;index"`
	Journal        *JournalModel   `gorm:"foreignKey:JournalID;constraint:OnDelete:RESTRICT"`
	EntryNo        *string         `gorm:"size:30;uniqueIndex:uq_ledger_entries_no,priority:2,where:entry_no IS NOT NULL;check:chk_ledger_entries_posted,status = 'draft' OR (entry_no IS NOT NULL AND fiscal_period_id IS NOT NULL AND posted_at IS NOT NULL AND posted_by IS NOT NULL)"`
	EntryDate      time.Time       `gorm:"type:date;not null;index:idx_ledger_entries_date,priority:2"`
	FiscalPeriodID *uuid.UUID      `gorm:"type:uuid"`
	Description    string          `gorm:"size:500;not null;default:''"`
	Reference      string          `gorm:"size:100;not null;default:''"`
	Status         string          `gorm:"size:20;not null;check:chk_ledger_entries_status,status IN ('draft','posted','reversed')"`
	TotalDebit     decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_ledger_entries_balanced,status = 'draft' OR (total_debit = total_credit AND total_debit > 0)"`
	TotalCredit    decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	ReversalOfID   *uuid.UUID      `gorm:"type:uuid;index"`
	ReversalOf     *EntryModel     `gorm:"foreignKey:ReversalOfID;constraint:OnDelete:RESTRICT"`
	ReversedByID   *uuid.UUID      `gorm:"type:uuid"`
	ReversedBy     *EntryModel     `gorm:"foreignKey:ReversedByID;constraint:OnDelete:RESTRICT"`
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
	EntryID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo        int             `gorm:"primaryKey;check:chk_ledger_lines_no,line_no > 0"`
	AccountID     uuid.UUID       `gorm:"type:uuid;not null;index"`
	Account       *AccountModel   `gorm:"foreignKey:AccountID;constraint:OnDelete:RESTRICT"`
	Description   string          `gorm:"size:500;not null;default:''"`
	Debit         decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_ledger_lines_amount,debit >= 0 AND credit >= 0 AND (debit > 0) <> (credit > 0)"`
	Credit        decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	BranchID      *uuid.UUID      `gorm:"type:uuid"`
	DepartmentID  *uuid.UUID      `gorm:"type:uuid"`
	CostCenterID  *uuid.UUID      `gorm:"type:uuid"`
	ProjectSiteID *uuid.UUID      `gorm:"type:uuid"`
}

func (LineModel) TableName() string { return "ledger_journal_lines" }

// หมายเหตุ: gorm อ่าน tag check ได้ field ละ 1 ข้อ — CHECK หลายข้อของตารางจึงกระจายไว้หลาย field

// SequenceModel: เลขลำดับล่าสุดของสมุด + คีย์ช่วงนับของรูปแบบเลขที่ (เช่น F2026, Y2026M02)
// (เพิ่มด้วย INSERT ... ON CONFLICT DO UPDATE ... RETURNING)
type SequenceModel struct {
	JournalID uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Journal   *JournalModel `gorm:"foreignKey:JournalID;constraint:OnDelete:RESTRICT"`
	PeriodKey string        `gorm:"size:20;primaryKey"`
	CompanyID uuid.UUID     `gorm:"type:uuid;not null"`
	LastNo    int64         `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "ledger_number_sequences" }

// AutoMigrate สร้าง/อัปเดตตารางของ ledger (เรียงตาม dependency ของ FK)
// สำหรับ production แนะนำเปลี่ยนเป็น versioned migration (เช่น goose / golang-migrate)
func AutoMigrate(db *gorm.DB) error {
	// CHECK ของประเภทบัญชีเพิ่ม 'suspense' — AutoMigrate ไม่แก้ constraint ที่มีชื่อเดิมอยู่แล้ว จึงลบให้สร้างใหม่
	if m := db.Migrator(); m.HasTable(&AccountModel{}) && m.HasConstraint(&AccountModel{}, "chk_ledger_accounts_type") {
		if err := m.DropConstraint(&AccountModel{}, "chk_ledger_accounts_type"); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&AccountModel{}, &JournalModel{}, &EntryModel{}, &LineModel{}, &SequenceModel{}); err != nil {
		return err
	}
	// ลำดับเลขแบบเดิม (ต่อปีบัญชี) -> คีย์ช่วงนับของรูปแบบเริ่มต้น "F<ปีบัญชี>"
	if db.Migrator().HasTable("ledger_journal_sequences") {
		if err := db.Exec(`INSERT INTO ledger_number_sequences (journal_id, period_key, company_id, last_no)
			SELECT journal_id, 'F' || fiscal_year, company_id, last_no FROM ledger_journal_sequences
			ON CONFLICT DO NOTHING`).Error; err != nil {
			return err
		}
		return db.Migrator().DropTable("ledger_journal_sequences")
	}
	return nil
}
