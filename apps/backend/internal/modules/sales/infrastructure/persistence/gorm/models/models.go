// Package models คือรูปร่างตารางของ sales (gorm เท่านั้น — ไม่มี business rule)
//
// ข้อตกลง:
//   - FK เฉพาะภายใน module (ลูกค้า / ใบเสนอราคา / ใบสั่งขาย / ใบแจ้งหนี้); สินค้า / คลัง / บัญชี / งวด / ทะเบียนเงินเป็น id ของ module เจ้าของ
//   - ลูกค้าใช้ soft delete; เอกสารไม่มี soft delete (draft ลบจริง, ลงบัญชีแล้วยกเลิกด้วยการกลับรายการ)
//   - CHECK บังคับ: ส่งไม่เกินที่สั่ง, ออกบิลไม่เกินที่ส่ง, รับชำระไม่เกินยอดบิล, บริษัทไม่จด VAT = อัตรา 0 + ราคาไม่รวม VAT
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SettingsModel struct {
	CompanyID          uuid.UUID       `gorm:"type:uuid;primaryKey"`
	VatRegistered      bool            `gorm:"not null"`
	VatRate            decimal.Decimal `gorm:"type:numeric(5,2);not null;check:chk_sal_settings_vat,vat_rate BETWEEN 0 AND 100"`
	JournalID          *uuid.UUID      `gorm:"type:uuid"`
	ARAccountID        *uuid.UUID      `gorm:"column:ar_account_id;type:uuid"`
	OutputVatAccountID *uuid.UUID      `gorm:"type:uuid"`
	RevenueAccountID   *uuid.UUID      `gorm:"type:uuid"`
	CogsAccountID      *uuid.UUID      `gorm:"type:uuid"`
	UpdatedAt          time.Time       `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "sales_settings" }

type CustomerModel struct {
	ID              uuid.UUID        `gorm:"type:uuid;primaryKey"`
	CompanyID       uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:uq_sal_customers_code,priority:1,where:deleted_at IS NULL"`
	Code            string           `gorm:"size:30;not null;uniqueIndex:uq_sal_customers_code,priority:2,where:deleted_at IS NULL"`
	Name            string           `gorm:"size:200;not null"`
	TaxID           string           `gorm:"size:13;not null;default:''"`
	BranchNo        string           `gorm:"size:5;not null;default:'00000'"`
	Address         string           `gorm:"size:500;not null;default:''"`
	PaymentTermDays int              `gorm:"not null;default:0;check:chk_sal_customers_term,payment_term_days BETWEEN 0 AND 365"`
	TaxType         string           `gorm:"size:20;not null;check:chk_sal_customers_tax_type,tax_type IN ('individual','juristic')"`
	CreditLimit     *decimal.Decimal `gorm:"type:numeric(24,6);check:chk_sal_customers_credit,credit_limit IS NULL OR credit_limit >= 0"`
	Status          string           `gorm:"size:20;not null;check:chk_sal_customers_status,status IN ('active','inactive')"`
	CreatedAt       time.Time        `gorm:"not null"`
	UpdatedAt       time.Time        `gorm:"not null"`
	DeletedAt       gorm.DeletedAt   `gorm:"index"`
}

func (CustomerModel) TableName() string { return "sales_customers" }

// LineModel คือบรรทัดราคาของใบเสนอราคา / ใบสั่งขาย (ตารางแยก — ฝังคอลัมน์ร่วม)
type LineModel struct {
	LineNo      int             `gorm:"primaryKey;check:line_no > 0"`
	ItemID      uuid.UUID       `gorm:"type:uuid;not null;index"`
	Description string          `gorm:"size:500;not null;default:''"`
	Quantity    decimal.Decimal `gorm:"type:numeric(24,6);not null;check:quantity > 0"`
	UnitPrice   decimal.Decimal `gorm:"type:numeric(24,6);not null;check:unit_price >= 0"`
	Net         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Vat         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Gross       decimal.Decimal `gorm:"type:numeric(24,6);not null"`
}

// SalesDocModel คือคอลัมน์ร่วมของหัวใบเสนอราคา / ใบสั่งขาย
type SalesDocModel struct {
	CustomerID       uuid.UUID       `gorm:"type:uuid;not null;index"`
	Reference        string          `gorm:"size:100;not null;default:''"`
	Description      string          `gorm:"size:500;not null;default:''"`
	VatRegistered    bool            `gorm:"not null"`
	VatRate          decimal.Decimal `gorm:"type:numeric(5,2);not null;check:vat_registered OR (vat_rate = 0 AND NOT price_includes_vat)"`
	PriceIncludesVat bool            `gorm:"not null"`
	PaymentTermDays  int             `gorm:"not null"`
	TotalNet         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	TotalVat         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	TotalGross       decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	CreatedBy        uuid.UUID       `gorm:"type:uuid;not null"`
	CreatedAt        time.Time       `gorm:"not null"`
	UpdatedAt        time.Time       `gorm:"not null"`
}

type QuotationModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	CompanyID   uuid.UUID `gorm:"type:uuid;not null;index:idx_sal_quotations_date,priority:1;uniqueIndex:uq_sal_quotations_no,priority:1,where:quotation_no IS NOT NULL"`
	QuotationNo *string   `gorm:"size:30;uniqueIndex:uq_sal_quotations_no,priority:2,where:quotation_no IS NOT NULL;check:chk_sal_quotations_sent,status = 'draft' OR quotation_no IS NOT NULL"`
	FiscalYear  *int
	DocDate     time.Time `gorm:"type:date;not null;index:idx_sal_quotations_date,priority:2"`
	SalesDocModel
	ValidUntil time.Time            `gorm:"type:date;not null;check:chk_sal_quotations_valid,valid_until >= doc_date"`
	Status     string               `gorm:"size:20;not null;check:chk_sal_quotations_status,status IN ('draft','sent','accepted','rejected')"`
	OrderID    *uuid.UUID           `gorm:"type:uuid"`
	Lines      []QuotationLineModel `gorm:"foreignKey:QuotationID;constraint:OnDelete:CASCADE"`
}

func (QuotationModel) TableName() string { return "sales_quotations" }

type QuotationLineModel struct {
	QuotationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	LineModel
}

func (QuotationLineModel) TableName() string { return "sales_quotation_lines" }

type OrderModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	CompanyID   uuid.UUID `gorm:"type:uuid;not null;index:idx_sal_orders_date,priority:1;uniqueIndex:uq_sal_orders_no,priority:1,where:order_no IS NOT NULL"`
	OrderNo     *string   `gorm:"size:30;uniqueIndex:uq_sal_orders_no,priority:2,where:order_no IS NOT NULL;check:chk_sal_orders_approved,status IN ('draft','cancelled') OR (order_no IS NOT NULL AND approved_at IS NOT NULL)"`
	FiscalYear  *int
	QuotationID *uuid.UUID      `gorm:"type:uuid;uniqueIndex:uq_sal_orders_quotation,where:quotation_id IS NOT NULL"`
	Quotation   *QuotationModel `gorm:"foreignKey:QuotationID;constraint:OnDelete:RESTRICT"`
	DocDate     time.Time       `gorm:"type:date;not null;index:idx_sal_orders_date,priority:2"`
	SalesDocModel
	DeliveryDate *time.Time `gorm:"type:date"`
	Status       string     `gorm:"size:20;not null;check:chk_sal_orders_status,status IN ('draft','approved','closed','cancelled')"`
	ApprovedBy   *uuid.UUID `gorm:"type:uuid"`
	ApprovedAt   *time.Time
	ClosedBy     *uuid.UUID `gorm:"type:uuid"`
	ClosedAt     *time.Time
	CancelledBy  *uuid.UUID       `gorm:"type:uuid"`
	CancelledAt  *time.Time       `gorm:"check:chk_sal_orders_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL)"`
	Lines        []OrderLineModel `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
}

func (OrderModel) TableName() string { return "sales_orders" }

type OrderLineModel struct {
	OrderID uuid.UUID `gorm:"type:uuid;primaryKey"`
	LineModel
	DeliveredQty decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_sal_order_lines_delivered,delivered_qty >= 0 AND delivered_qty <= quantity"`
	InvoicedQty  decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_sal_order_lines_invoiced,invoiced_qty >= 0 AND invoiced_qty <= delivered_qty"`
}

func (OrderLineModel) TableName() string { return "sales_order_lines" }

// PostedModel คือคอลัมน์ร่วมของใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ
type PostedModel struct {
	FiscalYear     int        `gorm:"not null"`
	FiscalPeriodID uuid.UUID  `gorm:"type:uuid;not null"`
	CustomerID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	Description    string     `gorm:"size:500;not null;default:''"`
	EntryID        *uuid.UUID `gorm:"type:uuid"`
	EntryNo        string     `gorm:"size:30;not null;default:''"`
	Status         string     `gorm:"size:20;not null;check:status IN ('posted','cancelled')"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;not null"`
	PostedAt       time.Time  `gorm:"not null"`
	CancelledBy    *uuid.UUID `gorm:"type:uuid"`
	CancelledAt    *time.Time `gorm:"check:(status = 'cancelled') = (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)"`
	CreatedAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time  `gorm:"not null"`
}

// StockModel คือคอลัมน์ตัดสต็อกของใบส่งของ / ใบส่งของ-ใบกำกับภาษี
type StockModel struct {
	WarehouseID     *uuid.UUID `gorm:"type:uuid"`
	System          string     `gorm:"size:20;not null;default:'perpetual';check:system IN ('perpetual','periodic')"`
	StockDocumentID *uuid.UUID `gorm:"type:uuid"`
	StockDocumentNo string     `gorm:"size:30;not null;default:''"`
}

type DeliveryModel struct {
	ID         uuid.UUID   `gorm:"type:uuid;primaryKey"`
	CompanyID  uuid.UUID   `gorm:"type:uuid;not null;index:idx_sal_deliveries_date,priority:1;uniqueIndex:uq_sal_deliveries_no,priority:1"`
	DocumentNo string      `gorm:"size:30;not null;uniqueIndex:uq_sal_deliveries_no,priority:2"`
	DocDate    time.Time   `gorm:"type:date;not null;index:idx_sal_deliveries_date,priority:2"`
	OrderID    uuid.UUID   `gorm:"type:uuid;not null;index"`
	Order      *OrderModel `gorm:"foreignKey:OrderID;constraint:OnDelete:RESTRICT"`
	PostedModel
	StockModel
	Reference string              `gorm:"size:100;not null;default:''"`
	TotalCost decimal.Decimal     `gorm:"type:numeric(24,6);not null"`
	Lines     []DeliveryLineModel `gorm:"foreignKey:DeliveryID;constraint:OnDelete:CASCADE"`
}

func (DeliveryModel) TableName() string { return "sales_deliveries" }

type DeliveryLineModel struct {
	DeliveryID         uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo             int             `gorm:"primaryKey"`
	OrderLineNo        int             `gorm:"not null"`
	ItemID             uuid.UUID       `gorm:"type:uuid;not null"`
	Quantity           decimal.Decimal `gorm:"type:numeric(24,6);not null;check:quantity > 0"`
	Stock              bool            `gorm:"not null"`
	Cost               decimal.Decimal `gorm:"type:numeric(24,6);not null;check:cost >= 0"`
	InventoryAccountID *uuid.UUID      `gorm:"type:uuid"`
}

func (DeliveryLineModel) TableName() string { return "sales_delivery_lines" }

type InvoiceModel struct {
	ID         uuid.UUID   `gorm:"type:uuid;primaryKey"`
	CompanyID  uuid.UUID   `gorm:"type:uuid;not null;index:idx_sal_invoices_date,priority:1;uniqueIndex:uq_sal_invoices_no,priority:1"`
	DocumentNo string      `gorm:"size:30;not null;uniqueIndex:uq_sal_invoices_no,priority:2"`
	DocDate    time.Time   `gorm:"type:date;not null;index:idx_sal_invoices_date,priority:2"`
	OrderID    uuid.UUID   `gorm:"type:uuid;not null;index"`
	Order      *OrderModel `gorm:"foreignKey:OrderID;constraint:OnDelete:RESTRICT"`
	PostedModel
	StockModel
	WithDelivery     bool               `gorm:"not null"`
	DueDate          time.Time          `gorm:"type:date;not null"`
	Reference        string             `gorm:"size:100;not null;default:''"`
	VatRate          decimal.Decimal    `gorm:"type:numeric(5,2);not null"`
	PriceIncludesVat bool               `gorm:"not null"`
	TotalNet         decimal.Decimal    `gorm:"type:numeric(24,6);not null"`
	TotalVat         decimal.Decimal    `gorm:"type:numeric(24,6);not null"`
	TotalGross       decimal.Decimal    `gorm:"type:numeric(24,6);not null;check:chk_sal_invoices_total,total_gross = total_net + total_vat"`
	ReceivedAmount   decimal.Decimal    `gorm:"type:numeric(24,6);not null;default:0;check:chk_sal_invoices_received,received_amount >= 0 AND received_amount <= total_gross"`
	Lines            []InvoiceLineModel `gorm:"foreignKey:InvoiceID;constraint:OnDelete:CASCADE"`
}

func (InvoiceModel) TableName() string { return "sales_invoices" }

type InvoiceLineModel struct {
	InvoiceID          uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo             int             `gorm:"primaryKey"`
	OrderLineNo        int             `gorm:"not null"`
	ItemID             uuid.UUID       `gorm:"type:uuid;not null"`
	Quantity           decimal.Decimal `gorm:"type:numeric(24,6);not null;check:quantity > 0"`
	UnitPrice          decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Net                decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Vat                decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Gross              decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	RevenueAccountID   uuid.UUID       `gorm:"type:uuid;not null"`
	Stock              bool            `gorm:"not null;default:false"`
	Cost               decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	InventoryAccountID *uuid.UUID      `gorm:"type:uuid"`
}

func (InvoiceLineModel) TableName() string { return "sales_invoice_lines" }

// SettlementModel: ประวัติการตัดยอดใบแจ้งหนี้จากเอกสารรับเงินของ finance
// (document_id ไม่มี FK เพราะเป็นตารางของ module อื่น) — อายุลูกหนี้รวมยอดตาม settled_on
type SettlementModel struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID  uuid.UUID       `gorm:"type:uuid;not null;index:idx_sal_settlements_doc,priority:1"`
	DocumentID uuid.UUID       `gorm:"type:uuid;not null;index:idx_sal_settlements_doc,priority:2"`
	DocumentNo string          `gorm:"size:30;not null"`
	SettledOn  time.Time       `gorm:"type:date;not null"`
	InvoiceID  uuid.UUID       `gorm:"type:uuid;not null;index"`
	Invoice    *InvoiceModel   `gorm:"foreignKey:InvoiceID;constraint:OnDelete:RESTRICT"`
	Amount     decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_sal_settlements_amount,amount > 0"`
	Status     string          `gorm:"size:20;not null;check:chk_sal_settlements_status,status IN ('posted','cancelled')"`
	CreatedAt  time.Time       `gorm:"not null"`
}

func (SettlementModel) TableName() string { return "sales_invoice_settlements" }

type SequenceModel struct {
	CompanyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	DocType   string    `gorm:"size:30;primaryKey"`
	PeriodKey string    `gorm:"size:20;primaryKey"`
	LastNo    int64     `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "sales_number_sequences" }

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&SettingsModel{}, &CustomerModel{}, &QuotationModel{}, &QuotationLineModel{}, &OrderModel{}, &OrderLineModel{},
		&DeliveryModel{}, &DeliveryLineModel{}, &InvoiceModel{}, &InvoiceLineModel{},
		&SettlementModel{}, &SequenceModel{},
	); err != nil {
		return err
	}
	// ใบเสร็จรับเงินย้ายไป finance (ตัดสินใจ 2026-11-23: ข้อมูลทดสอบเดิมไม่ย้าย) + ค่าตั้งภาษีถูกหักย้ายไปค่าตั้งการเงิน
	for _, t := range []string{"sales_receipt_methods", "sales_receipt_lines", "sales_receipts"} {
		if err := db.Migrator().DropTable(t); err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn(&SettingsModel{}, "wht_account_id") {
		return db.Migrator().DropColumn(&SettingsModel{}, "wht_account_id")
	}
	return nil
}
