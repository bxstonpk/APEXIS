// Package models คือรูปร่างตารางของ cashbank (gorm เท่านั้น)
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CashAccountModel: ทะเบียนเงินสด / บัญชีธนาคาร — gl_account_id เป็นบัญชีของ ledger (ไม่มี FK ข้าม module)
type CashAccountModel struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID   uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_cb_accounts_code,priority:1,where:deleted_at IS NULL"`
	Code        string         `gorm:"size:30;not null;uniqueIndex:uq_cb_accounts_code,priority:2,where:deleted_at IS NULL"`
	Name        string         `gorm:"size:200;not null"`
	Type        string         `gorm:"size:10;not null;check:chk_cb_accounts_type,type IN ('cash','bank')"`
	BankName    string         `gorm:"size:100;not null;default:''"`
	BankBranch  string         `gorm:"size:100;not null;default:''"`
	AccountNo   string         `gorm:"size:30;not null;default:'';check:chk_cb_accounts_bank,type = 'cash' OR (bank_name <> '' AND account_no <> '')"`
	GLAccountID uuid.UUID      `gorm:"column:gl_account_id;type:uuid;not null"`
	Status      string         `gorm:"size:20;not null;check:chk_cb_accounts_status,status IN ('active','inactive')"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (CashAccountModel) TableName() string { return "cashbank_accounts" }

func AutoMigrate(db *gorm.DB) error { return db.AutoMigrate(&CashAccountModel{}) }
