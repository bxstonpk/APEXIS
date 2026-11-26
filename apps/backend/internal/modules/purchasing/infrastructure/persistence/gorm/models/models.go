// Package models คือ "รูปร่างตาราง" ของ purchasing สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - ทุกตารางมี company_id — FK เฉพาะภายใน module (ผู้ขาย / ใบสั่งซื้อ); สินค้า / คลัง / บัญชี / งวด ตรวจผ่าน module เจ้าของ
//   - ผู้ขายใช้ soft delete + partial unique index; เอกสารไม่มี soft delete (ยกเลิกด้วยการกลับรายการ)
//   - จำนวน / มูลค่าเป็น NUMERIC(24,6) — ทศนิยมที่ยอมรับกำหนดต่อบริษัท
//   - CHECK บังคับ: รับไม่เกินที่สั่ง, แจ้งหนี้ไม่เกินที่รับ, ผู้ขายไม่จด VAT = อัตรา 0 และราคาไม่รวม VAT,
//     ใบที่อนุมัติ / ลงบัญชีแล้วต้องมีเลขที่ (gorm อ่าน tag check ได้ field ละ 1 ข้อ จึงกระจายไว้หลาย field)
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SettingsModel struct {
	CompanyID                uuid.UUID       `gorm:"type:uuid;primaryKey"`
	VatRate                  decimal.Decimal `gorm:"type:numeric(5,2);not null;check:chk_pur_settings_vat,vat_rate BETWEEN 0 AND 100"`
	JournalID                *uuid.UUID      `gorm:"type:uuid"`
	GRIRAccountID            *uuid.UUID      `gorm:"column:grir_account_id;type:uuid"`
	APAccountID              *uuid.UUID      `gorm:"column:ap_account_id;type:uuid"`
	InputVatAccountID        *uuid.UUID      `gorm:"type:uuid"`
	PriceVarianceAccountID   *uuid.UUID      `gorm:"type:uuid"`
	PurchaseReturnsAccountID *uuid.UUID      `gorm:"type:uuid"`
	UpdatedAt                time.Time       `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "purchasing_settings" }

type SupplierModel struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID       uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_pur_suppliers_code,priority:1,where:deleted_at IS NULL"`
	Code            string         `gorm:"size:30;not null;uniqueIndex:uq_pur_suppliers_code,priority:2,where:deleted_at IS NULL"`
	Name            string         `gorm:"size:200;not null"`
	TaxID           string         `gorm:"size:13;not null;default:''"`
	BranchNo        string         `gorm:"size:5;not null;default:'00000'"`
	Address         string         `gorm:"size:500;not null;default:''"`
	AddressDetail   AddressColumns `gorm:"embedded;embeddedPrefix:addr_"`
	PaymentTermDays int            `gorm:"not null;default:0;check:chk_pur_suppliers_term,payment_term_days BETWEEN 0 AND 365"`
	VatRegistered   bool           `gorm:"not null"`
	TaxType         string         `gorm:"size:20;not null;default:'juristic';check:chk_pur_suppliers_tax_type,tax_type IN ('individual','juristic')"`
	Status          string         `gorm:"size:20;not null;check:chk_pur_suppliers_status,status IN ('active','inactive')"`
	CreatedAt       time.Time      `gorm:"not null"`
	UpdatedAt       time.Time      `gorm:"not null"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (SupplierModel) TableName() string { return "purchasing_suppliers" }

// SupplierBranchModel คือสาขาเพิ่มเติมของผู้ขาย (สาขาหลักอยู่ในตารางผู้ขาย) — แทนที่ทั้งชุดตอนบันทึกผู้ขาย
type SupplierBranchModel struct {
	SupplierID    uuid.UUID      `gorm:"type:uuid;primaryKey"`
	BranchNo      string         `gorm:"size:5;primaryKey"`
	Supplier      *SupplierModel `gorm:"foreignKey:SupplierID;constraint:OnDelete:CASCADE"`
	Name          string         `gorm:"size:200;not null;default:''"`
	Address       string         `gorm:"size:500;not null;default:''"`
	AddressDetail AddressColumns `gorm:"embedded;embeddedPrefix:addr_"`
	Position      int            `gorm:"not null"`
}

func (SupplierBranchModel) TableName() string { return "purchasing_supplier_branches" }

type OrderModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	CompanyID        uuid.UUID `gorm:"type:uuid;not null;index:idx_pur_orders_date,priority:1;uniqueIndex:uq_pur_orders_no,priority:1,where:order_no IS NOT NULL"`
	OrderNo          *string   `gorm:"size:30;uniqueIndex:uq_pur_orders_no,priority:2,where:order_no IS NOT NULL;check:chk_pur_orders_approved,status IN ('draft','cancelled') OR (order_no IS NOT NULL AND approved_at IS NOT NULL AND approved_by IS NOT NULL)"`
	FiscalYear       *int
	SupplierID       uuid.UUID       `gorm:"type:uuid;not null;index"`
	Supplier         *SupplierModel  `gorm:"foreignKey:SupplierID;constraint:OnDelete:RESTRICT"`
	SupplierBranchNo string          `gorm:"size:5;not null;default:''"` // ว่าง = สาขาหลักของผู้ขาย
	OrderDate        time.Time       `gorm:"type:date;not null;index:idx_pur_orders_date,priority:2"`
	DeliveryDate     *time.Time      `gorm:"type:date"`
	Reference        string          `gorm:"size:100;not null;default:''"`
	Description      string          `gorm:"size:500;not null;default:''"`
	VatRegistered    bool            `gorm:"not null"`
	VatRate          decimal.Decimal `gorm:"type:numeric(5,2);not null;check:chk_pur_orders_vat,vat_registered OR (vat_rate = 0 AND NOT price_includes_vat)"`
	PriceIncludesVat bool            `gorm:"not null"`
	PaymentTermDays  int             `gorm:"not null"`
	TotalNet         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	TotalVat         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	TotalGross       decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Status           string          `gorm:"size:20;not null;check:chk_pur_orders_status,status IN ('draft','approved','closed','cancelled')"`
	CreatedBy        uuid.UUID       `gorm:"type:uuid;not null"`
	ApprovedBy       *uuid.UUID      `gorm:"type:uuid"`
	ApprovedAt       *time.Time
	ClosedBy         *uuid.UUID `gorm:"type:uuid"`
	ClosedAt         *time.Time
	CancelledBy      *uuid.UUID       `gorm:"type:uuid"`
	CancelledAt      *time.Time       `gorm:"check:chk_pur_orders_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL)"`
	Lines            []OrderLineModel `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	CreatedAt        time.Time        `gorm:"not null"`
	UpdatedAt        time.Time        `gorm:"not null"`
}

func (OrderModel) TableName() string { return "purchasing_orders" }

type OrderLineModel struct {
	OrderID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo        int             `gorm:"primaryKey;check:chk_pur_order_lines_no,line_no > 0"`
	ItemID        uuid.UUID       `gorm:"type:uuid;not null;index"`
	Description   string          `gorm:"size:500;not null;default:''"`
	Quantity      decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_order_lines_qty,quantity > 0"`
	UnitPrice     decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_order_lines_price,unit_price >= 0"`
	Net           decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Vat           decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Gross         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	ReceivedQty   decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_pur_order_lines_received,received_qty >= 0 AND received_qty <= quantity"`
	InvoicedQty   decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_pur_order_lines_invoiced,invoiced_qty >= 0 AND invoiced_qty <= received_qty"`
	ReceivedValue decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	ClearedValue  decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
}

func (OrderLineModel) TableName() string { return "purchasing_order_lines" }

type ReceiptModel struct {
	ID              uuid.UUID          `gorm:"type:uuid;primaryKey"`
	CompanyID       uuid.UUID          `gorm:"type:uuid;not null;index:idx_pur_receipts_date,priority:1;uniqueIndex:uq_pur_receipts_no,priority:1"`
	DocumentNo      string             `gorm:"size:30;not null;uniqueIndex:uq_pur_receipts_no,priority:2"`
	FiscalYear      int                `gorm:"not null"`
	FiscalPeriodID  uuid.UUID          `gorm:"type:uuid;not null"`
	OrderID         uuid.UUID          `gorm:"type:uuid;not null;index"`
	Order           *OrderModel        `gorm:"foreignKey:OrderID;constraint:OnDelete:RESTRICT"`
	SupplierID      uuid.UUID          `gorm:"type:uuid;not null;index"`
	Supplier        *SupplierModel     `gorm:"foreignKey:SupplierID;constraint:OnDelete:RESTRICT"`
	ReceiptDate     time.Time          `gorm:"type:date;not null;index:idx_pur_receipts_date,priority:2"`
	WarehouseID     *uuid.UUID         `gorm:"type:uuid"`
	Reference       string             `gorm:"size:100;not null;default:''"`
	Description     string             `gorm:"size:500;not null;default:''"`
	System          string             `gorm:"size:20;not null;check:chk_pur_receipts_system,system IN ('perpetual','periodic')"`
	StockDocumentID *uuid.UUID         `gorm:"type:uuid"`
	StockDocumentNo string             `gorm:"size:30;not null;default:''"`
	TotalValue      decimal.Decimal    `gorm:"type:numeric(24,6);not null;check:chk_pur_receipts_total,total_value >= 0"`
	EntryID         *uuid.UUID         `gorm:"type:uuid"`
	EntryNo         string             `gorm:"size:30;not null;default:''"`
	Status          string             `gorm:"size:20;not null;check:chk_pur_receipts_status,status IN ('posted','cancelled')"`
	CreatedBy       uuid.UUID          `gorm:"type:uuid;not null"`
	PostedAt        time.Time          `gorm:"not null"`
	CancelledBy     *uuid.UUID         `gorm:"type:uuid"`
	CancelledAt     *time.Time         `gorm:"check:chk_pur_receipts_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)"`
	Lines           []ReceiptLineModel `gorm:"foreignKey:ReceiptID;constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time          `gorm:"not null"`
	UpdatedAt       time.Time          `gorm:"not null"`
}

func (ReceiptModel) TableName() string { return "purchasing_receipts" }

type ReceiptLineModel struct {
	ReceiptID   uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo      int             `gorm:"primaryKey"`
	OrderLineNo int             `gorm:"not null"`
	ItemID      uuid.UUID       `gorm:"type:uuid;not null"`
	Stock       bool            `gorm:"not null"`
	Quantity    decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_receipt_lines_qty,quantity > 0"`
	UnitCost    decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Value       decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_receipt_lines_value,value >= 0"`
	AccountID   uuid.UUID       `gorm:"type:uuid;not null"`
}

func (ReceiptLineModel) TableName() string { return "purchasing_receipt_lines" }

// InvoiceModel: เลขที่ใบแจ้งหนี้ของผู้ขายห้ามซ้ำต่อผู้ขาย (เฉพาะที่ยังไม่ยกเลิก)
type InvoiceModel struct {
	ID                uuid.UUID          `gorm:"type:uuid;primaryKey"`
	CompanyID         uuid.UUID          `gorm:"type:uuid;not null;index:idx_pur_invoices_date,priority:1;uniqueIndex:uq_pur_invoices_no,priority:1;uniqueIndex:uq_pur_invoices_supplier_no,priority:1,where:status = 'posted'"`
	DocumentNo        string             `gorm:"size:30;not null;uniqueIndex:uq_pur_invoices_no,priority:2"`
	FiscalYear        int                `gorm:"not null"`
	FiscalPeriodID    uuid.UUID          `gorm:"type:uuid;not null"`
	OrderID           uuid.UUID          `gorm:"type:uuid;not null;index"`
	Order             *OrderModel        `gorm:"foreignKey:OrderID;constraint:OnDelete:RESTRICT"`
	SupplierID        uuid.UUID          `gorm:"type:uuid;not null;index;uniqueIndex:uq_pur_invoices_supplier_no,priority:2,where:status = 'posted'"`
	Supplier          *SupplierModel     `gorm:"foreignKey:SupplierID;constraint:OnDelete:RESTRICT"`
	SupplierInvoiceNo string             `gorm:"size:50;not null;uniqueIndex:uq_pur_invoices_supplier_no,priority:3,where:status = 'posted'"`
	BillFromName      string             `gorm:"size:200;not null;default:''"`
	BillFromTaxID     string             `gorm:"size:13;not null;default:''"`
	BillFromBranchNo  string             `gorm:"size:5;not null;default:''"`
	BillFromAddress   string             `gorm:"size:500;not null;default:''"`
	InvoiceDate       time.Time          `gorm:"type:date;not null"`
	PostingDate       time.Time          `gorm:"type:date;not null;index:idx_pur_invoices_date,priority:2"`
	DueDate           time.Time          `gorm:"type:date;not null;check:chk_pur_invoices_due,due_date >= posting_date"`
	Description       string             `gorm:"size:500;not null;default:''"`
	VatRate           decimal.Decimal    `gorm:"type:numeric(5,2);not null"`
	PriceIncludesVat  bool               `gorm:"not null"`
	TotalNet          decimal.Decimal    `gorm:"type:numeric(24,6);not null"`
	TotalVat          decimal.Decimal    `gorm:"type:numeric(24,6);not null"`
	TotalGross        decimal.Decimal    `gorm:"type:numeric(24,6);not null;check:chk_pur_invoices_total,total_gross = total_net + total_vat"`
	PaidAmount        decimal.Decimal    `gorm:"type:numeric(24,6);not null;default:0;check:chk_pur_invoices_paid,paid_amount >= 0 AND paid_amount <= total_gross"`
	EntryID           *uuid.UUID         `gorm:"type:uuid"`
	EntryNo           string             `gorm:"size:30;not null;default:''"`
	Status            string             `gorm:"size:20;not null;check:chk_pur_invoices_status,status IN ('posted','cancelled')"`
	CreatedBy         uuid.UUID          `gorm:"type:uuid;not null"`
	PostedAt          time.Time          `gorm:"not null"`
	CancelledBy       *uuid.UUID         `gorm:"type:uuid"`
	CancelledAt       *time.Time         `gorm:"check:chk_pur_invoices_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)"`
	Lines             []InvoiceLineModel `gorm:"foreignKey:InvoiceID;constraint:OnDelete:CASCADE"`
	CreatedAt         time.Time          `gorm:"not null"`
	UpdatedAt         time.Time          `gorm:"not null"`
}

func (InvoiceModel) TableName() string { return "purchasing_invoices" }

type InvoiceLineModel struct {
	InvoiceID         uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo            int             `gorm:"primaryKey"`
	OrderLineNo       int             `gorm:"not null"`
	ItemID            uuid.UUID       `gorm:"type:uuid;not null"`
	Quantity          decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_invoice_lines_qty,quantity > 0"`
	UnitPrice         decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Net               decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Vat               decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Gross             decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Cleared           decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	VarianceAccountID *uuid.UUID      `gorm:"type:uuid"`
}

func (InvoiceLineModel) TableName() string { return "purchasing_invoice_lines" }

// CreditNoteModel: ใบลดหนี้ผู้ขาย — เลขที่ใบลดหนี้ของผู้ขายห้ามซ้ำต่อผู้ขาย (เฉพาะที่ยังไม่ยกเลิก)
// ยอดลดหนี้ตัดยอดค้างของใบแจ้งหนี้เดิมผ่าน purchasing_invoice_settlements (document_id = id ใบลดหนี้)
type CreditNoteModel struct {
	ID                   uuid.UUID             `gorm:"type:uuid;primaryKey"`
	CompanyID            uuid.UUID             `gorm:"type:uuid;not null;index:idx_pur_credit_notes_date,priority:1;uniqueIndex:uq_pur_credit_notes_no,priority:1;uniqueIndex:uq_pur_credit_notes_supplier_no,priority:1,where:status = 'posted'"`
	DocumentNo           string                `gorm:"size:30;not null;uniqueIndex:uq_pur_credit_notes_no,priority:2"`
	FiscalYear           int                   `gorm:"not null"`
	FiscalPeriodID       uuid.UUID             `gorm:"type:uuid;not null"`
	InvoiceID            uuid.UUID             `gorm:"type:uuid;not null;index"`
	Invoice              *InvoiceModel         `gorm:"foreignKey:InvoiceID;constraint:OnDelete:RESTRICT"`
	OrderID              uuid.UUID             `gorm:"type:uuid;not null;index"`
	Order                *OrderModel           `gorm:"foreignKey:OrderID;constraint:OnDelete:RESTRICT"`
	SupplierID           uuid.UUID             `gorm:"type:uuid;not null;index;uniqueIndex:uq_pur_credit_notes_supplier_no,priority:2,where:status = 'posted'"`
	Supplier             *SupplierModel        `gorm:"foreignKey:SupplierID;constraint:OnDelete:RESTRICT"`
	SupplierCreditNoteNo string                `gorm:"size:50;not null;uniqueIndex:uq_pur_credit_notes_supplier_no,priority:3,where:status = 'posted';check:chk_pur_credit_notes_supplier_no,supplier_credit_note_no <> ''"`
	BillFromName         string                `gorm:"size:200;not null;default:''"`
	BillFromTaxID        string                `gorm:"size:13;not null;default:''"`
	BillFromBranchNo     string                `gorm:"size:5;not null;default:''"`
	BillFromAddress      string                `gorm:"size:500;not null;default:''"`
	CreditNoteDate       time.Time             `gorm:"type:date;not null"`
	PostingDate          time.Time             `gorm:"type:date;not null;index:idx_pur_credit_notes_date,priority:2"`
	Kind                 string                `gorm:"size:20;not null;check:chk_pur_credit_notes_kind,kind IN ('return','allowance')"`
	Reason               string                `gorm:"size:500;not null;check:chk_pur_credit_notes_reason,reason <> ''"`
	Description          string                `gorm:"size:500;not null;default:''"`
	VatRate              decimal.Decimal       `gorm:"type:numeric(5,2);not null"`
	PriceIncludesVat     bool                  `gorm:"not null"`
	TotalNet             decimal.Decimal       `gorm:"type:numeric(24,6);not null;check:chk_pur_credit_notes_net,total_net >= 0"`
	TotalVat             decimal.Decimal       `gorm:"type:numeric(24,6);not null"`
	TotalGross           decimal.Decimal       `gorm:"type:numeric(24,6);not null;check:chk_pur_credit_notes_total,total_gross = total_net + total_vat"`
	WarehouseID          *uuid.UUID            `gorm:"type:uuid"`
	System               string                `gorm:"size:20;not null;check:chk_pur_credit_notes_system,system IN ('perpetual','periodic')"`
	StockDocumentID      *uuid.UUID            `gorm:"type:uuid"`
	StockDocumentNo      string                `gorm:"size:30;not null;default:''"`
	EntryID              *uuid.UUID            `gorm:"type:uuid"`
	EntryNo              string                `gorm:"size:30;not null;default:''"`
	Status               string                `gorm:"size:20;not null;check:chk_pur_credit_notes_status,status IN ('posted','cancelled')"`
	CreatedBy            uuid.UUID             `gorm:"type:uuid;not null"`
	PostedAt             time.Time             `gorm:"not null"`
	CancelledBy          *uuid.UUID            `gorm:"type:uuid"`
	CancelledAt          *time.Time            `gorm:"check:chk_pur_credit_notes_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)"`
	Lines                []CreditNoteLineModel `gorm:"foreignKey:CreditNoteID;constraint:OnDelete:CASCADE"`
	CreatedAt            time.Time             `gorm:"not null"`
	UpdatedAt            time.Time             `gorm:"not null"`
}

func (CreditNoteModel) TableName() string { return "purchasing_credit_notes" }

// CreditNoteLineModel: invoice_line_no อ้าง line_no ของใบแจ้งหนี้เดิม; ลดราคา = quantity 0
type CreditNoteLineModel struct {
	CreditNoteID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo             int             `gorm:"primaryKey"`
	InvoiceLineNo      int             `gorm:"not null;check:chk_pur_credit_note_lines_invoice_line,invoice_line_no > 0"`
	ItemID             uuid.UUID       `gorm:"type:uuid;not null"`
	Quantity           decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_credit_note_lines_qty,quantity >= 0"`
	UnitPrice          decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Net                decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_credit_note_lines_net,net > 0"`
	Vat                decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	Gross              decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_credit_note_lines_gross,gross = net + vat"`
	AccountID          uuid.UUID       `gorm:"type:uuid;not null"`
	Stock              bool            `gorm:"not null"`
	Cost               decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_pur_credit_note_lines_cost,cost >= 0"`
	InventoryAccountID *uuid.UUID      `gorm:"type:uuid"`
	Variance           decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
}

func (CreditNoteLineModel) TableName() string { return "purchasing_credit_note_lines" }

// SequenceModel: เลขลำดับล่าสุดต่อชนิดเอกสาร + คีย์ช่วงนับของรูปแบบเลขที่ (เช่น F2026, Y2026M02)
type SequenceModel struct {
	CompanyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	DocType   string    `gorm:"size:30;primaryKey"`
	PeriodKey string    `gorm:"size:20;primaryKey"`
	LastNo    int64     `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "purchasing_number_sequences" }

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&SettingsModel{}, &SupplierModel{}, &SupplierBranchModel{}, &OrderModel{}, &OrderLineModel{},
		&ReceiptModel{}, &ReceiptLineModel{}, &InvoiceModel{}, &InvoiceLineModel{}, &SequenceModel{},
		&SettlementModel{}, &CreditNoteModel{}, &CreditNoteLineModel{},
	); err != nil {
		return err
	}
	// ใบจ่ายชำระ / ประเภทภาษีหัก ณ ที่จ่ายย้ายไป finance (ตัดสินใจ 2026-11-23: ข้อมูลทดสอบเดิมไม่ย้าย)
	for _, t := range []string{"purchasing_payment_methods", "purchasing_payment_lines", "purchasing_payments", "purchasing_wht_types"} {
		if err := db.Migrator().DropTable(t); err != nil {
			return err
		}
	}
	for _, c := range []string{"wht_pnd3_account_id", "wht_pnd53_account_id"} {
		if db.Migrator().HasColumn(&SettingsModel{}, c) {
			if err := db.Migrator().DropColumn(&SettingsModel{}, c); err != nil {
				return err
			}
		}
	}
	// ลำดับเลขแบบเดิม (prefix + ปีบัญชี) -> ชนิดเอกสาร + คีย์ช่วงนับของรูปแบบเริ่มต้น "F<ปีบัญชี>"
	if db.Migrator().HasTable("purchasing_sequences") {
		if err := db.Exec(`INSERT INTO purchasing_number_sequences (company_id, doc_type, period_key, last_no)
			SELECT company_id, CASE prefix WHEN 'PO' THEN 'purchasing.order' WHEN 'RC' THEN 'purchasing.receipt'
				ELSE 'purchasing.invoice' END, 'F' || fiscal_year, last_no
			FROM purchasing_sequences ON CONFLICT DO NOTHING`).Error; err != nil {
			return err
		}
		return db.Migrator().DropTable("purchasing_sequences")
	}
	return nil
}

// SettlementModel: ประวัติการตัดยอดใบแจ้งหนี้จากเอกสารจ่ายเงินของ finance
// (document_id ไม่มี FK เพราะเป็นตารางของ module อื่น) — อายุเจ้าหนี้รวมยอดตาม settled_on
type SettlementModel struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID  uuid.UUID       `gorm:"type:uuid;not null;index:idx_pur_settlements_doc,priority:1"`
	DocumentID uuid.UUID       `gorm:"type:uuid;not null;index:idx_pur_settlements_doc,priority:2"`
	DocumentNo string          `gorm:"size:30;not null"`
	SettledOn  time.Time       `gorm:"type:date;not null"`
	InvoiceID  uuid.UUID       `gorm:"type:uuid;not null;index"`
	Invoice    *InvoiceModel   `gorm:"foreignKey:InvoiceID;constraint:OnDelete:RESTRICT"`
	Amount     decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_pur_settlements_amount,amount > 0"`
	Status     string          `gorm:"size:20;not null;check:chk_pur_settlements_status,status IN ('posted','cancelled')"`
	CreatedAt  time.Time       `gorm:"not null"`
}

func (SettlementModel) TableName() string { return "purchasing_invoice_settlements" }

// AddressColumns คือที่อยู่ตามแบบสรรพากร (คอลัมน์ addr_*) — คอลัมน์ address เก็บข้อความที่ประกอบแล้ว (พิมพ์ / snapshot)
type AddressColumns struct {
	Building    string `gorm:"size:100;not null;default:''"`
	RoomNo      string `gorm:"size:100;not null;default:''"`
	FloorNo     string `gorm:"size:100;not null;default:''"`
	Village     string `gorm:"size:100;not null;default:''"`
	HouseNo     string `gorm:"size:100;not null;default:''"`
	Moo         string `gorm:"size:100;not null;default:''"`
	Soi         string `gorm:"size:100;not null;default:''"`
	Yaek        string `gorm:"size:100;not null;default:''"`
	Street      string `gorm:"size:100;not null;default:''"`
	SubDistrict string `gorm:"size:100;not null;default:''"`
	District    string `gorm:"size:100;not null;default:''"`
	Province    string `gorm:"size:100;not null;default:''"`
	PostalCode  string `gorm:"size:5;not null;default:''"`
}
