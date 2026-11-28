package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SettingsModel: ค่าตั้งของ ledger ต่อบริษัท (1 แถว) — บัญชีกำไรสะสม / สรุปผลกำไรขาดทุน / สมุดรายการปิดบัญชี
// ไม่ใส่ FK ไปผังบัญชี / สมุด (ตรวจใน use case ตอนบันทึกและตอนใช้) — ตารางค่าตั้งไม่ขวางการลบ / สร้างตารางหลักใหม่
type SettingsModel struct {
	CompanyID                 uuid.UUID  `gorm:"type:uuid;primaryKey"`
	RetainedEarningsAccountID *uuid.UUID `gorm:"type:uuid"`
	IncomeSummaryAccountID    *uuid.UUID `gorm:"type:uuid"`
	ClosingJournalID          *uuid.UUID `gorm:"type:uuid"`
	CreatedAt                 time.Time  `gorm:"not null"`
	UpdatedAt                 time.Time  `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "ledger_settings" }

// YearEndClosingModel: การปิดบัญชีสิ้นปี — 1 แถวต่อบริษัทต่อปีบัญชี (ปิดใหม่หลังเปิดปีใช้แถวเดิม)
// entry_id / reversal_id อ้างรายการบัญชีที่ระบบสร้าง (posted ลบไม่ได้) จึงไม่ใส่ FK
type YearEndClosingModel struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID  uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_ledger_year_end,priority:1"`
	FiscalYear int             `gorm:"not null;uniqueIndex:uq_ledger_year_end,priority:2"`
	StartDate  time.Time       `gorm:"type:date;not null"`
	EndDate    time.Time       `gorm:"type:date;not null"`
	Status     string          `gorm:"size:20;not null;check:chk_ledger_year_end_status,status IN ('closed','reopened')"`
	NetIncome  decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	EntryID    *uuid.UUID      `gorm:"type:uuid"`
	ReversalID *uuid.UUID      `gorm:"type:uuid"`
	ClosedBy   uuid.UUID       `gorm:"type:uuid;not null"`
	ClosedAt   time.Time       `gorm:"not null"`
	ReopenedBy *uuid.UUID      `gorm:"type:uuid"`
	ReopenedAt *time.Time
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (YearEndClosingModel) TableName() string { return "ledger_year_end_closings" }
