// Package models คือ "รูปร่างตาราง" ของ inventory สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - ทุกตารางมี company_id — ไม่มี FK ข้าม module (บริษัท / สาขา / งวดบัญชี ตรวจผ่าน organization)
//   - master data ใช้ soft delete + partial unique index (รหัสใช้ซ้ำได้หลังลบ)
//   - เอกสารไม่มี soft delete: draft ลบจริง, posted ยกเลิกด้วยการกลับผล (cancelled)
//   - จำนวน / มูลค่าเป็น NUMERIC(24,6) — ทศนิยมที่ยอมรับกำหนดต่อบริษัท
//   - CHECK บังคับ: จำนวนในชั้นต้นทุนไม่ติดลบ, คงเหลือไม่เกินจำนวนรับ, เอกสารที่ผ่านแล้วต้องมีเลขที่
//     (gorm อ่าน tag check ได้ field ละ 1 ข้อ จึงกระจาย CHECK ไว้หลาย field)
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type UomModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_inv_uoms_code,priority:1,where:deleted_at IS NULL"`
	Code      string         `gorm:"size:30;not null;uniqueIndex:uq_inv_uoms_code,priority:2,where:deleted_at IS NULL"`
	Name      string         `gorm:"size:200;not null"`
	Status    string         `gorm:"size:20;not null;check:chk_inv_uoms_status,status IN ('active','inactive')"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (UomModel) TableName() string { return "inventory_uoms" }

type CategoryModel struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID          uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_inv_categories_code,priority:1,where:deleted_at IS NULL"`
	Code               string         `gorm:"size:30;not null;uniqueIndex:uq_inv_categories_code,priority:2,where:deleted_at IS NULL"`
	Name               string         `gorm:"size:200;not null"`
	PurchaseAccountID  *uuid.UUID     `gorm:"type:uuid"` // บัญชีของ ledger (ไม่มี FK ข้าม module — ตรวจผ่าน ledger)
	InventoryAccountID *uuid.UUID     `gorm:"type:uuid"`
	Status             string         `gorm:"size:20;not null;check:chk_inv_categories_status,status IN ('active','inactive')"`
	CreatedAt          time.Time      `gorm:"not null"`
	UpdatedAt          time.Time      `gorm:"not null"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (CategoryModel) TableName() string { return "inventory_item_categories" }

type WarehouseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_inv_warehouses_code,priority:1,where:deleted_at IS NULL"`
	Code      string         `gorm:"size:30;not null;uniqueIndex:uq_inv_warehouses_code,priority:2,where:deleted_at IS NULL"`
	Name      string         `gorm:"size:200;not null"`
	BranchID  *uuid.UUID     `gorm:"type:uuid"`
	Status    string         `gorm:"size:20;not null;check:chk_inv_warehouses_status,status IN ('active','inactive')"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (WarehouseModel) TableName() string { return "inventory_warehouses" }

type ItemModel struct {
	ID                 uuid.UUID        `gorm:"type:uuid;primaryKey"`
	CompanyID          uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:uq_inv_items_code,priority:1,where:deleted_at IS NULL"`
	Code               string           `gorm:"size:30;not null;uniqueIndex:uq_inv_items_code,priority:2,where:deleted_at IS NULL"`
	Name               string           `gorm:"size:200;not null"`
	NameEN             string           `gorm:"column:name_en;size:200;not null;default:''"`
	Type               string           `gorm:"size:20;not null;check:chk_inv_items_type,type IN ('stock','non_stock','service')"`
	UomID              uuid.UUID        `gorm:"type:uuid;not null;index"`
	Uom                *UomModel        `gorm:"foreignKey:UomID;constraint:OnDelete:RESTRICT"`
	CategoryID         *uuid.UUID       `gorm:"type:uuid;index"`
	Category           *CategoryModel   `gorm:"foreignKey:CategoryID;constraint:OnDelete:RESTRICT"`
	ReorderPoint       *decimal.Decimal `gorm:"type:numeric(24,6);check:chk_inv_items_reorder,reorder_point IS NULL OR reorder_point >= 0"`
	PurchaseAccountID  *uuid.UUID       `gorm:"type:uuid"`
	InventoryAccountID *uuid.UUID       `gorm:"type:uuid"`
	Status             string           `gorm:"size:20;not null;check:chk_inv_items_status,status IN ('active','inactive')"`
	CreatedAt          time.Time        `gorm:"not null"`
	UpdatedAt          time.Time        `gorm:"not null"`
	DeletedAt          gorm.DeletedAt   `gorm:"index"`
}

func (ItemModel) TableName() string { return "inventory_items" }

type DocumentModel struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_documents_date,priority:1;uniqueIndex:uq_inv_documents_no,priority:1,where:document_no IS NOT NULL"`
	DocType        string          `gorm:"size:20;not null;check:chk_inv_documents_type,doc_type IN ('receipt','issue','transfer','adjustment')"`
	DocumentNo     *string         `gorm:"size:30;uniqueIndex:uq_inv_documents_no,priority:2,where:document_no IS NOT NULL;check:chk_inv_documents_posted,status = 'draft' OR (document_no IS NOT NULL AND fiscal_period_id IS NOT NULL AND posted_at IS NOT NULL AND posted_by IS NOT NULL)"`
	DocDate        time.Time       `gorm:"type:date;not null;index:idx_inv_documents_date,priority:2"`
	WarehouseID    uuid.UUID       `gorm:"type:uuid;not null;index"`
	Warehouse      *WarehouseModel `gorm:"foreignKey:WarehouseID;constraint:OnDelete:RESTRICT"`
	ToWarehouseID  *uuid.UUID      `gorm:"type:uuid;index;check:chk_inv_documents_transfer,(doc_type = 'transfer') = (to_warehouse_id IS NOT NULL) AND (to_warehouse_id IS NULL OR to_warehouse_id <> warehouse_id)"`
	ToWarehouse    *WarehouseModel `gorm:"foreignKey:ToWarehouseID;constraint:OnDelete:RESTRICT"`
	FiscalPeriodID *uuid.UUID      `gorm:"type:uuid"`
	FiscalYear     *int            `gorm:"index:idx_inv_documents_year"` // ปีบัญชีของงวดที่ผ่านเอกสาร (ตรวจการเปลี่ยนระบบบัญชี)
	Reference      string          `gorm:"size:100;not null;default:''"`
	Description    string          `gorm:"size:500;not null;default:''"`
	Status         string          `gorm:"size:20;not null;check:chk_inv_documents_status,status IN ('draft','posted','cancelled')"`
	TotalValue     decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_inv_documents_total,total_value >= 0"`
	PostedAt       *time.Time
	PostedBy       *uuid.UUID `gorm:"type:uuid"`
	CancelledAt    *time.Time
	CancelledBy    *uuid.UUID          `gorm:"type:uuid;check:chk_inv_documents_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)"`
	CreatedBy      uuid.UUID           `gorm:"type:uuid;not null"`
	Lines          []DocumentLineModel `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE"`
	CreatedAt      time.Time           `gorm:"not null"`
	UpdatedAt      time.Time           `gorm:"not null"`
}

func (DocumentModel) TableName() string { return "inventory_documents" }

type DocumentLineModel struct {
	DocumentID  uuid.UUID        `gorm:"type:uuid;primaryKey"`
	LineNo      int              `gorm:"primaryKey;check:chk_inv_lines_no,line_no > 0"`
	ItemID      uuid.UUID        `gorm:"type:uuid;not null;index"`
	Item        *ItemModel       `gorm:"foreignKey:ItemID;constraint:OnDelete:RESTRICT"`
	Quantity    decimal.Decimal  `gorm:"type:numeric(24,6);not null;check:chk_inv_lines_qty,quantity <> 0"`
	UnitCost    *decimal.Decimal `gorm:"type:numeric(24,6);check:chk_inv_lines_cost,unit_cost IS NULL OR unit_cost >= 0"`
	Value       decimal.Decimal  `gorm:"type:numeric(24,6);not null;default:0;check:chk_inv_lines_value,value >= 0"`
	Description string           `gorm:"size:500;not null;default:''"`
}

func (DocumentLineModel) TableName() string { return "inventory_document_lines" }

// LayerModel คือชั้นต้นทุน FIFO — index บางส่วน (เฉพาะชั้นที่ยังเหลือ) ใช้หาชั้นตอนตัดสต็อก
type LayerModel struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey;index:idx_inv_layers_open,priority:5"`
	CompanyID         uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_layers_open,priority:1,where:remaining_quantity > 0"`
	ItemID            uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_layers_open,priority:2"`
	Item              *ItemModel      `gorm:"foreignKey:ItemID;constraint:OnDelete:RESTRICT"`
	WarehouseID       uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_layers_open,priority:3"`
	Warehouse         *WarehouseModel `gorm:"foreignKey:WarehouseID;constraint:OnDelete:RESTRICT"`
	LayerDate         time.Time       `gorm:"type:date;not null;index:idx_inv_layers_open,priority:4"`
	DocumentID        uuid.UUID       `gorm:"type:uuid;not null;index"`
	Document          *DocumentModel  `gorm:"foreignKey:DocumentID;constraint:OnDelete:RESTRICT"`
	LineNo            int             `gorm:"not null"`
	Quantity          decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_layers_qty,quantity > 0"`
	Value             decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_layers_value,value >= 0"`
	RemainingQuantity decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_layers_remaining,remaining_quantity >= 0 AND remaining_quantity <= quantity"`
	RemainingValue    decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_layers_remaining_value,remaining_value >= 0"`
	CreatedAt         time.Time       `gorm:"not null"`
}

func (LayerModel) TableName() string { return "inventory_layers" }

// MovementModel คือการเคลื่อนไหวสต็อก (+ เข้า / - ออก) — ที่มาของยอดคงเหลือและ stock card
type MovementModel struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID    uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_movements_stock,priority:1"`
	ItemID       uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_movements_stock,priority:2"`
	Item         *ItemModel      `gorm:"foreignKey:ItemID;constraint:OnDelete:RESTRICT"`
	WarehouseID  uuid.UUID       `gorm:"type:uuid;not null;index:idx_inv_movements_stock,priority:3"`
	Warehouse    *WarehouseModel `gorm:"foreignKey:WarehouseID;constraint:OnDelete:RESTRICT"`
	MovementDate time.Time       `gorm:"type:date;not null;index:idx_inv_movements_stock,priority:4"`
	DocumentID   uuid.UUID       `gorm:"type:uuid;not null;index"`
	Document     *DocumentModel  `gorm:"foreignKey:DocumentID;constraint:OnDelete:RESTRICT"`
	LineNo       int             `gorm:"not null"`
	Quantity     decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_movements_qty,quantity <> 0"`
	Value        decimal.Decimal `gorm:"type:numeric(24,6);not null"`
	CreatedAt    time.Time       `gorm:"not null"`
}

func (MovementModel) TableName() string { return "inventory_movements" }

// AllocationModel คือส่วนที่การเคลื่อนไหวขาออกตัดจากชั้นต้นทุน (ใช้คืนชั้นเมื่อยกเลิก + ตรวจสอบย้อนหลัง)
type AllocationModel struct {
	MovementID uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Movement   *MovementModel  `gorm:"foreignKey:MovementID;constraint:OnDelete:RESTRICT"`
	LayerID    uuid.UUID       `gorm:"type:uuid;primaryKey;index"`
	Layer      *LayerModel     `gorm:"foreignKey:LayerID;constraint:OnDelete:RESTRICT"`
	Quantity   decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_allocations_qty,quantity > 0"`
	Value      decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_inv_allocations_value,value >= 0"`
}

func (AllocationModel) TableName() string { return "inventory_allocations" }

// SequenceModel: เลขที่ล่าสุดของชนิดเอกสาร + ปีบัญชี (INSERT ... ON CONFLICT DO UPDATE ... RETURNING)
type SequenceModel struct {
	CompanyID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	DocType    string    `gorm:"size:20;primaryKey"`
	FiscalYear int       `gorm:"primaryKey"`
	LastNo     int64     `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "inventory_sequences" }

// SettingsModel คือค่าตั้งบัญชีสินค้าของบริษัท (1 แถวต่อบริษัท) — แถวนี้ใช้ lock การเปลี่ยนระบบบัญชีด้วย
type SettingsModel struct {
	CompanyID                 uuid.UUID  `gorm:"type:uuid;primaryKey"`
	DefaultPurchaseAccountID  *uuid.UUID `gorm:"type:uuid"`
	DefaultInventoryAccountID *uuid.UUID `gorm:"type:uuid"`
	CogsAccountID             *uuid.UUID `gorm:"type:uuid"`
	JournalID                 *uuid.UUID `gorm:"type:uuid"`
	UpdatedAt                 time.Time  `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "inventory_settings" }

// SystemModel: ใช้ระบบบัญชี system ตั้งแต่ปีบัญชี from_year
type SystemModel struct {
	CompanyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	FromYear  int       `gorm:"primaryKey;check:chk_inv_systems_year,from_year BETWEEN 1900 AND 9999"`
	System    string    `gorm:"size:20;not null;check:chk_inv_systems_system,system IN ('perpetual','periodic')"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (SystemModel) TableName() string { return "inventory_accounting_systems" }

// ClosingModel คือการปรับปรุงสินค้าคงเหลือสิ้นงวด — งวดหนึ่งมี posted ได้แถวเดียว (partial unique)
type ClosingModel struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_inv_closings_period,priority:1,where:status = 'posted';index:idx_inv_closings_end,priority:1"`
	FiscalPeriodID uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_inv_closings_period,priority:2,where:status = 'posted'"`
	FiscalYear     int             `gorm:"not null"`
	PeriodEnd      time.Time       `gorm:"type:date;not null;index:idx_inv_closings_end,priority:2"`
	EntryID        *uuid.UUID      `gorm:"type:uuid"`
	EntryNo        string          `gorm:"size:30;not null;default:''"`
	Adjustment     decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
	Status         string          `gorm:"size:20;not null;check:chk_inv_closings_status,status IN ('posted','cancelled')"`
	CreatedBy      uuid.UUID       `gorm:"type:uuid;not null"`
	CancelledBy    *uuid.UUID      `gorm:"type:uuid;check:chk_inv_closings_cancelled,(status = 'cancelled') = (cancelled_at IS NOT NULL AND cancelled_by IS NOT NULL)"`
	CancelledAt    *time.Time
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (ClosingModel) TableName() string { return "inventory_period_closings" }

// AutoMigrate สร้าง/อัปเดตตารางของ inventory (เรียงตาม dependency ของ FK)
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&UomModel{}, &CategoryModel{}, &WarehouseModel{}, &ItemModel{},
		&DocumentModel{}, &DocumentLineModel{}, &LayerModel{}, &MovementModel{}, &AllocationModel{}, &SequenceModel{},
		&SettingsModel{}, &SystemModel{}, &ClosingModel{},
	); err != nil {
		return err
	}
	// เอกสารที่ผ่านก่อนมีคอลัมน์ fiscal_year: ปีอยู่ในเลขที่เอกสาร (เช่น GR2026-000001)
	return db.Exec(`UPDATE inventory_documents SET fiscal_year = substring(document_no from '([0-9]{4})-[0-9]+$')::int
		WHERE fiscal_year IS NULL AND document_no IS NOT NULL`).Error
}
