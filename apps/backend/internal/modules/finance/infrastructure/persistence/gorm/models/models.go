// Package models คือรูปร่างตารางของ finance (gorm เท่านั้น)
//
// id ของ module อื่น (บัญชี / สมุดรายวันของ ledger, ลูกค้า / ผู้ขาย / ใบแจ้งหนี้ของ sales / purchasing)
// เก็บเป็นค่าอ้างอิงโดยไม่มี FK ข้าม module — ความถูกต้องตรวจผ่าน public API ของ module นั้นใน transaction เดียวกัน
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CashAccountModel: ทะเบียนเงินสด / บัญชีธนาคาร — gl_account_id เป็นบัญชีของ ledger
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

func (CashAccountModel) TableName() string { return "finance_cash_accounts" }

type SettingsModel struct {
	CompanyID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ReceiptJournalID     *uuid.UUID `gorm:"type:uuid"`
	PaymentJournalID     *uuid.UUID `gorm:"type:uuid"`
	TransferJournalID    *uuid.UUID `gorm:"type:uuid"`
	WhtReceivedAccountID *uuid.UUID `gorm:"type:uuid"`
	WhtPnd3AccountID     *uuid.UUID `gorm:"column:wht_pnd3_account_id;type:uuid"`
	WhtPnd53AccountID    *uuid.UUID `gorm:"column:wht_pnd53_account_id;type:uuid"`
	UpdatedAt            time.Time  `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "finance_settings" }

// WhtTypeModel: ประเภทเงินได้ที่หักภาษี ณ ที่จ่าย + อัตรา
type WhtTypeModel struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_fin_wht_types_code,priority:1,where:deleted_at IS NULL"`
	Code      string          `gorm:"size:30;not null;uniqueIndex:uq_fin_wht_types_code,priority:2,where:deleted_at IS NULL"`
	Name      string          `gorm:"size:200;not null"`
	Rate      decimal.Decimal `gorm:"type:numeric(5,2);not null;check:chk_fin_wht_types_rate,rate > 0 AND rate <= 100"`
	// IncomeType: ประเภทเงินได้ตาม 50 ทวิ — แถวเดิมก่อนมีคอลัมน์นี้ได้ค่าเริ่มต้น 3_tres (ค่าบริการ / ค่าเช่า / ค่าจ้างทำของ ฯลฯ)
	IncomeType string         `gorm:"size:10;not null;default:'3_tres';check:chk_fin_wht_types_income_type,income_type IN ('40_1','40_2','40_3','40_4a','40_4b','3_tres','other')"`
	Status     string         `gorm:"size:20;not null;check:chk_fin_wht_types_status,status IN ('active','inactive')"`
	CreatedAt  time.Time      `gorm:"not null"`
	UpdatedAt  time.Time      `gorm:"not null"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (WhtTypeModel) TableName() string { return "finance_wht_types" }

// DocumentModel คือส่วนร่วมของหัวเอกสารรับ / จ่าย / โอน (embedded)
type DocumentModel struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Description    string     `gorm:"size:500;not null;default:''"`
	Status         string     `gorm:"size:20;not null;check:status IN ('posted','cancelled')"`
	FiscalPeriodID uuid.UUID  `gorm:"type:uuid;not null"`
	EntryID        *uuid.UUID `gorm:"type:uuid"`
	EntryNo        string     `gorm:"size:30;not null;default:''"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;not null"`
	CancelledBy    *uuid.UUID `gorm:"type:uuid"`
	CancelledAt    *time.Time
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

// PartyModel คือคู่ค้า ณ วันเอกสาร (embedded, column prefix party_)
type PartyModel struct {
	PartyID       *uuid.UUID `gorm:"type:uuid;index"`
	PartyCode     string     `gorm:"size:30;not null;default:''"`
	PartyName     string     `gorm:"size:200;not null;default:''"`
	PartyTaxID    string     `gorm:"size:20;not null;default:''"`
	PartyBranchNo string     `gorm:"size:5;not null;default:''"`
	PartyAddress  string     `gorm:"size:500;not null;default:''"`
	PartyTaxType  string     `gorm:"size:20;not null;default:''"`
}

// MethodModel คือช่องทางรับ / จ่าย (gl_account_id = บัญชีที่ลงจริง ณ วันเอกสาร)
type MethodModel struct {
	LineNo        int               `gorm:"primaryKey"`
	CashAccountID uuid.UUID         `gorm:"type:uuid;not null;index"`
	CashAccount   *CashAccountModel `gorm:"foreignKey:CashAccountID;constraint:OnDelete:RESTRICT"`
	GLAccountID   uuid.UUID         `gorm:"column:gl_account_id;type:uuid;not null"`
	Amount        decimal.Decimal   `gorm:"type:numeric(24,6);not null;check:amount > 0"`
	Reference     string            `gorm:"size:50;not null;default:''"`
}

// ReceiptModel: ใบรับเงิน — total_received = total_amount - total_wht (CHECK)
type ReceiptModel struct {
	CompanyID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_fin_receipts_no,priority:1;index:idx_fin_receipts_date,priority:1"`
	DocumentNo string    `gorm:"size:30;not null;uniqueIndex:uq_fin_receipts_no,priority:2"`
	DocDate    time.Time `gorm:"type:date;not null;index:idx_fin_receipts_date,priority:2"`
	DocumentModel
	Kind string `gorm:"size:20;not null;check:chk_fin_receipts_kind,kind IN ('customer','other')"`
	PartyModel
	TotalAmount   decimal.Decimal      `gorm:"type:numeric(24,6);not null;check:chk_fin_receipts_amount,total_amount > 0"`
	TotalWht      decimal.Decimal      `gorm:"type:numeric(24,6);not null;check:chk_fin_receipts_wht,total_wht >= 0 AND total_wht <= total_amount"`
	TotalReceived decimal.Decimal      `gorm:"type:numeric(24,6);not null;check:chk_fin_receipts_received,total_received = total_amount - total_wht"`
	Lines         []ReceiptLineModel   `gorm:"foreignKey:ReceiptID;constraint:OnDelete:CASCADE"`
	Methods       []ReceiptMethodModel `gorm:"foreignKey:ReceiptID;constraint:OnDelete:CASCADE"`
}

func (ReceiptModel) TableName() string { return "finance_receipts" }

// ReceiptLineModel: invoice_id (ใบแจ้งหนี้ขาย) หรือ account_id (รับเงินอื่น) อย่างใดอย่างหนึ่ง
type ReceiptLineModel struct {
	ReceiptID   uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo      int             `gorm:"primaryKey"`
	InvoiceID   *uuid.UUID      `gorm:"type:uuid;index;check:chk_fin_receipt_lines_target,(invoice_id IS NULL) <> (account_id IS NULL)"`
	InvoiceNo   string          `gorm:"size:30;not null;default:''"`
	AccountID   *uuid.UUID      `gorm:"type:uuid"`
	Description string          `gorm:"size:500;not null;default:''"`
	Amount      decimal.Decimal `gorm:"type:numeric(24,6);not null;check:amount > 0"`
	Wht         decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:wht >= 0 AND wht <= amount"`
}

func (ReceiptLineModel) TableName() string { return "finance_receipt_lines" }

type ReceiptMethodModel struct {
	ReceiptID uuid.UUID `gorm:"type:uuid;primaryKey"`
	MethodModel
}

func (ReceiptMethodModel) TableName() string { return "finance_receipt_methods" }

// PaymentModel: ใบสำคัญจ่าย — total_paid = total_amount - total_wht (CHECK)
type PaymentModel struct {
	CompanyID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_fin_payments_no,priority:1;index:idx_fin_payments_date,priority:1;uniqueIndex:uq_fin_payments_wht_cert,priority:1"`
	DocumentNo string    `gorm:"size:30;not null;uniqueIndex:uq_fin_payments_no,priority:2"`
	DocDate    time.Time `gorm:"type:date;not null;index:idx_fin_payments_date,priority:2"`
	DocumentModel
	Kind string `gorm:"size:20;not null;check:chk_fin_payments_kind,kind IN ('supplier','other')"`
	PartyModel
	TotalAmount decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_fin_payments_amount,total_amount > 0"`
	TotalWht    decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_fin_payments_wht,total_wht >= 0 AND total_wht <= total_amount"`
	TotalPaid   decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_fin_payments_paid,total_paid = total_amount - total_wht"`
	// WhtCertificateNo: เลขที่หนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ) — NULL = ไม่มีภาษีหัก (unique ต่อบริษัท, NULL ซ้ำได้)
	WhtCertificateNo *string              `gorm:"size:30;uniqueIndex:uq_fin_payments_wht_cert,priority:2"`
	Lines            []PaymentLineModel   `gorm:"foreignKey:PaymentID;constraint:OnDelete:CASCADE"`
	Methods          []PaymentMethodModel `gorm:"foreignKey:PaymentID;constraint:OnDelete:CASCADE"`
}

func (PaymentModel) TableName() string { return "finance_payments" }

type PaymentLineModel struct {
	PaymentID         uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo            int             `gorm:"primaryKey"`
	InvoiceID         *uuid.UUID      `gorm:"type:uuid;index;check:chk_fin_payment_lines_target,(invoice_id IS NULL) <> (account_id IS NULL)"`
	InvoiceNo         string          `gorm:"size:30;not null;default:''"`
	SupplierInvoiceNo string          `gorm:"size:50;not null;default:''"`
	AccountID         *uuid.UUID      `gorm:"type:uuid"`
	Description       string          `gorm:"size:500;not null;default:''"`
	Amount            decimal.Decimal `gorm:"type:numeric(24,6);not null;check:amount > 0"`
	WhtTypeID         *uuid.UUID      `gorm:"type:uuid"`
	WhtType           *WhtTypeModel   `gorm:"foreignKey:WhtTypeID;constraint:OnDelete:RESTRICT"`
	WhtRate           decimal.Decimal `gorm:"type:numeric(5,2);not null;default:0"`
	WhtBase           decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	Wht               decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_fin_payment_lines_wht,wht >= 0 AND wht <= amount"`
}

func (PaymentLineModel) TableName() string { return "finance_payment_lines" }

type PaymentMethodModel struct {
	PaymentID uuid.UUID `gorm:"type:uuid;primaryKey"`
	MethodModel
}

func (PaymentMethodModel) TableName() string { return "finance_payment_methods" }

// TransferModel: โอนเงินระหว่างทะเบียน — ต้นทาง <> ปลายทาง (CHECK)
type TransferModel struct {
	CompanyID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_fin_transfers_no,priority:1;index:idx_fin_transfers_date,priority:1"`
	DocumentNo string    `gorm:"size:30;not null;uniqueIndex:uq_fin_transfers_no,priority:2"`
	DocDate    time.Time `gorm:"type:date;not null;index:idx_fin_transfers_date,priority:2"`
	DocumentModel
	Reference       string            `gorm:"size:50;not null;default:''"`
	FromAccountID   uuid.UUID         `gorm:"type:uuid;not null;index;check:chk_fin_transfers_accounts,from_account_id <> to_account_id"`
	FromAccount     *CashAccountModel `gorm:"foreignKey:FromAccountID;constraint:OnDelete:RESTRICT"`
	FromGLAccountID uuid.UUID         `gorm:"column:from_gl_account_id;type:uuid;not null"`
	ToAccountID     uuid.UUID         `gorm:"type:uuid;not null;index"`
	ToAccount       *CashAccountModel `gorm:"foreignKey:ToAccountID;constraint:OnDelete:RESTRICT"`
	ToGLAccountID   uuid.UUID         `gorm:"column:to_gl_account_id;type:uuid;not null"`
	Amount          decimal.Decimal   `gorm:"type:numeric(24,6);not null;check:chk_fin_transfers_amount,amount > 0"`
}

func (TransferModel) TableName() string { return "finance_transfers" }

type SequenceModel struct {
	CompanyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	DocType   string    `gorm:"size:30;primaryKey"`
	PeriodKey string    `gorm:"size:20;primaryKey"`
	LastNo    int64     `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "finance_number_sequences" }

// AutoMigrate สร้าง / อัปเดตตาราง — ทะเบียนเงินจากตารางเดิมของ cashbank เปลี่ยนชื่อเป็นของ finance (คงข้อมูล)
// สำหรับ production แนะนำเปลี่ยนเป็น versioned migration
func AutoMigrate(db *gorm.DB) error {
	if m := db.Migrator(); m.HasTable("cashbank_accounts") && !m.HasTable("finance_cash_accounts") {
		if err := m.RenameTable("cashbank_accounts", "finance_cash_accounts"); err != nil {
			return err
		}
	}
	return db.AutoMigrate(
		&CashAccountModel{}, &SettingsModel{}, &WhtTypeModel{},
		&ReceiptModel{}, &ReceiptLineModel{}, &ReceiptMethodModel{},
		&PaymentModel{}, &PaymentLineModel{}, &PaymentMethodModel{},
		&TransferModel{}, &SequenceModel{},
	)
}
