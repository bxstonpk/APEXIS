// Package models คือ "รูปร่างตาราง" ของ pettycash สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - ทุกตารางมี company_id — FK เฉพาะภายใน module (กองทุน / ใบสำคัญจ่าย / คำขอ); บัญชี / ผู้ใช้ ตรวจผ่าน module เจ้าของ
//   - ไม่มี soft delete (ยกเลิกด้วยสถานะ — เลขที่เอกสารไม่หาย)
//   - จำนวนเงินเป็น NUMERIC(24,6) — ทศนิยมที่ยอมรับกำหนดต่อบริษัท
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SettingsModel struct {
	CompanyID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	InputVatAccountID *uuid.UUID `gorm:"type:uuid"`
	UpdatedAt         time.Time  `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "petty_cash_settings" }

type FundModel struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID     uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_pc_funds_code,priority:1"`
	Code          string          `gorm:"size:20;not null;uniqueIndex:uq_pc_funds_code,priority:2"`
	Name          string          `gorm:"size:200;not null"`
	CustodianID   uuid.UUID       `gorm:"type:uuid;not null"`
	GLAccountID   uuid.UUID       `gorm:"type:uuid;not null"`
	ImprestAmount decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pc_funds_imprest,imprest_amount > 0"`
	Status        string          `gorm:"size:20;not null;check:chk_pc_funds_status,status IN ('active','inactive')"`
	CreatedAt     time.Time       `gorm:"not null"`
	UpdatedAt     time.Time       `gorm:"not null"`
}

func (FundModel) TableName() string { return "petty_cash_funds" }

// ReplenishmentModel: คำขอตั้ง / เพิ่มวงเงิน / เบิกชดเชย — payment_* = ใบสำคัญจ่ายของการเงินที่จ่ายคำขอ (status = paid)
type ReplenishmentModel struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID         uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_pc_requests_no,priority:1;index:idx_pc_requests_fund,priority:1;index:idx_pc_requests_status,priority:1"`
	DocumentNo        string          `gorm:"size:30;not null;uniqueIndex:uq_pc_requests_no,priority:2"`
	FundID            uuid.UUID       `gorm:"type:uuid;not null;index:idx_pc_requests_fund,priority:2"`
	Fund              *FundModel      `gorm:"foreignKey:FundID;constraint:OnDelete:RESTRICT"`
	Kind              string          `gorm:"size:20;not null;check:chk_pc_requests_kind,kind IN ('establish','increase','replenish')"`
	DocDate           time.Time       `gorm:"type:date;not null"`
	Description       string          `gorm:"size:500;not null;default:''"`
	Amount            decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pc_requests_amount,amount > 0"`
	Status            string          `gorm:"size:20;not null;index:idx_pc_requests_status,priority:2;check:chk_pc_requests_status,status IN ('requested','paid','cancelled')"`
	PaymentDocumentID *uuid.UUID      `gorm:"type:uuid;index:idx_pc_requests_payment"`
	PaymentDocumentNo string          `gorm:"size:30;not null;default:''"`
	PaidDate          *time.Time      `gorm:"type:date;check:chk_pc_requests_paid,(status = 'paid') = (payment_document_id IS NOT NULL AND paid_date IS NOT NULL)"`
	CreatedBy         uuid.UUID       `gorm:"type:uuid;not null"`
	CreatedAt         time.Time       `gorm:"not null"`
	UpdatedAt         time.Time       `gorm:"not null"`
	CancelledAt       *time.Time
}

func (ReplenishmentModel) TableName() string { return "petty_cash_replenishments" }

// VoucherModel: ใบสำคัญจ่ายเงินสดย่อย — total_gross = total_net + total_vat (CHECK); มี VAT ต้องมีใบกำกับภาษี (CHECK)
type VoucherModel struct {
	ID               uuid.UUID           `gorm:"type:uuid;primaryKey"`
	CompanyID        uuid.UUID           `gorm:"type:uuid;not null;uniqueIndex:uq_pc_vouchers_no,priority:1;index:idx_pc_vouchers_fund,priority:1;index:idx_pc_vouchers_date,priority:1"`
	DocumentNo       string              `gorm:"size:30;not null;uniqueIndex:uq_pc_vouchers_no,priority:2"`
	FundID           uuid.UUID           `gorm:"type:uuid;not null;index:idx_pc_vouchers_fund,priority:2"`
	Fund             *FundModel          `gorm:"foreignKey:FundID;constraint:OnDelete:RESTRICT"`
	DocDate          time.Time           `gorm:"type:date;not null;index:idx_pc_vouchers_date,priority:2"`
	PayeeName        string              `gorm:"size:200;not null"`
	PayeeTaxID       string              `gorm:"size:20;not null;default:''"`
	PayeeBranchNo    string              `gorm:"size:5;not null;default:''"`
	Description      string              `gorm:"size:500;not null;default:''"`
	VatRate          decimal.Decimal     `gorm:"type:numeric(5,2);not null;default:0;check:chk_pc_vouchers_rate,vat_rate >= 0 AND vat_rate <= 100"`
	PriceIncludesVat bool                `gorm:"not null;default:false"`
	TaxInvoiceNo     string              `gorm:"size:50;not null;default:''"`
	TaxInvoiceDate   *time.Time          `gorm:"type:date"`
	TotalNet         decimal.Decimal     `gorm:"type:numeric(24,6);not null"`
	TotalVat         decimal.Decimal     `gorm:"type:numeric(24,6);not null;check:chk_pc_vouchers_vat,total_vat = 0 OR (tax_invoice_no <> '' AND tax_invoice_date IS NOT NULL)"`
	TotalGross       decimal.Decimal     `gorm:"type:numeric(24,6);not null;check:chk_pc_vouchers_gross,total_gross > 0 AND total_gross = total_net + total_vat"`
	Status           string              `gorm:"size:20;not null;index:idx_pc_vouchers_fund,priority:3;check:chk_pc_vouchers_status,status IN ('draft','approved','replenished','cancelled')"`
	ReplenishmentID  *uuid.UUID          `gorm:"type:uuid;index"`
	Replenishment    *ReplenishmentModel `gorm:"foreignKey:ReplenishmentID;constraint:OnDelete:RESTRICT"`
	CreatedBy        uuid.UUID           `gorm:"type:uuid;not null"`
	ApprovedBy       *uuid.UUID          `gorm:"type:uuid"`
	ApprovedAt       *time.Time
	CancelledAt      *time.Time
	CreatedAt        time.Time          `gorm:"not null"`
	UpdatedAt        time.Time          `gorm:"not null"`
	Lines            []VoucherLineModel `gorm:"foreignKey:VoucherID;constraint:OnDelete:CASCADE"`
}

func (VoucherModel) TableName() string { return "petty_cash_vouchers" }

type VoucherLineModel struct {
	VoucherID   uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo      int             `gorm:"primaryKey;check:chk_pc_voucher_lines_no,line_no > 0"`
	AccountID   uuid.UUID       `gorm:"type:uuid;not null"`
	Description string          `gorm:"size:300;not null;default:''"`
	Amount      decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pc_voucher_lines_amount,amount > 0"`
	Net         decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pc_voucher_lines_net,net >= 0"`
}

func (VoucherLineModel) TableName() string { return "petty_cash_voucher_lines" }

type SequenceModel struct {
	CompanyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	DocType   string    `gorm:"size:50;primaryKey"`
	PeriodKey string    `gorm:"size:20;primaryKey"`
	LastNo    int64     `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "petty_cash_number_sequences" }

// AutoMigrate สร้าง / อัปเดตตารางของ pettycash (เรียงตาม FK)
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&SettingsModel{}, &FundModel{}, &ReplenishmentModel{}, &VoucherModel{}, &VoucherLineModel{}, &SequenceModel{})
}
