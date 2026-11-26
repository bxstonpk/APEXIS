// Package models คือ "รูปร่างตาราง" ของ manufacturing สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - ทุกตารางมี company_id — FK เฉพาะภายใน module (สูตร / ใบสั่งผลิต); สินค้า / คลัง / บัญชี / เอกสารสต็อก ตรวจผ่าน module เจ้าของ
//   - สูตรใช้ soft delete + partial unique index (รหัส, สูตรหลักที่ใช้งานอยู่ 1 สูตรต่อสินค้า)
//   - ใบสั่งผลิตไม่มี soft delete (ยกเลิกด้วยการยกเลิกเอกสารสต็อก / กลับรายการ)
//   - จำนวน / มูลค่าเป็น NUMERIC(24,6) — ทศนิยมที่ยอมรับกำหนดต่อบริษัท
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SettingsModel struct {
	CompanyID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	JournalID           *uuid.UUID `gorm:"type:uuid"`
	WipAccountID        *uuid.UUID `gorm:"type:uuid"`
	ConversionAccountID *uuid.UUID `gorm:"type:uuid"`
	DefaultWarehouseID  *uuid.UUID `gorm:"type:uuid"`
	UpdatedAt           time.Time  `gorm:"not null"`
}

func (SettingsModel) TableName() string { return "manufacturing_settings" }

type BomModel struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_mfg_boms_code,priority:1,where:deleted_at IS NULL;uniqueIndex:uq_mfg_boms_default,priority:1,where:is_default AND status = 'active' AND deleted_at IS NULL"`
	Code           string          `gorm:"size:30;not null;uniqueIndex:uq_mfg_boms_code,priority:2,where:deleted_at IS NULL"`
	Name           string          `gorm:"size:200;not null"`
	ItemID         uuid.UUID       `gorm:"type:uuid;not null;index;uniqueIndex:uq_mfg_boms_default,priority:2,where:is_default AND status = 'active' AND deleted_at IS NULL"`
	OutputQuantity decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_boms_output,output_quantity > 0"`
	Status         string          `gorm:"size:20;not null;check:chk_mfg_boms_status,status IN ('active','inactive')"`
	IsDefault      bool            `gorm:"not null;default:false"`
	CreatedAt      time.Time       `gorm:"not null"`
	UpdatedAt      time.Time       `gorm:"not null"`
	DeletedAt      gorm.DeletedAt  `gorm:"index"`
}

func (BomModel) TableName() string { return "manufacturing_boms" }

type BomComponentModel struct {
	BomID        uuid.UUID       `gorm:"type:uuid;primaryKey;uniqueIndex:uq_mfg_bom_components_item,priority:1"`
	LineNo       int             `gorm:"primaryKey;check:chk_mfg_bom_components_line,line_no > 0"`
	Bom          *BomModel       `gorm:"foreignKey:BomID;constraint:OnDelete:CASCADE"`
	ItemID       uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_mfg_bom_components_item,priority:2"`
	Quantity     decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_bom_components_qty,quantity > 0"`
	ScrapPercent decimal.Decimal `gorm:"type:numeric(5,2);not null;default:0;check:chk_mfg_bom_components_scrap,scrap_percent >= 0 AND scrap_percent < 100"`
}

func (BomComponentModel) TableName() string { return "manufacturing_bom_components" }

type BomConversionModel struct {
	BomID       uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo      int             `gorm:"primaryKey;check:chk_mfg_bom_conversions_line,line_no > 0"`
	Bom         *BomModel       `gorm:"foreignKey:BomID;constraint:OnDelete:CASCADE"`
	Description string          `gorm:"size:200;not null"`
	AccountID   *uuid.UUID      `gorm:"type:uuid"`
	Amount      decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_bom_conversions_amount,amount >= 0"`
}

func (BomConversionModel) TableName() string { return "manufacturing_bom_conversions" }

type OrderModel struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey"`
	CompanyID         uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uq_mfg_orders_no,priority:1;index:idx_mfg_orders_date,priority:1;index:idx_mfg_orders_status,priority:1"`
	DocumentNo        string          `gorm:"size:30;not null;uniqueIndex:uq_mfg_orders_no,priority:2"`
	OrderDate         time.Time       `gorm:"type:date;not null;index:idx_mfg_orders_date,priority:2"`
	ItemID            uuid.UUID       `gorm:"type:uuid;not null;index"`
	BomID             uuid.UUID       `gorm:"type:uuid;not null;index"`
	Bom               *BomModel       `gorm:"foreignKey:BomID;constraint:OnDelete:RESTRICT"`
	Quantity          decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_orders_qty,quantity > 0"`
	OutputQuantity    decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_orders_output,output_quantity > 0"`
	WarehouseID       uuid.UUID       `gorm:"type:uuid;not null"`
	PlannedStart      *time.Time      `gorm:"type:date"`
	PlannedEnd        *time.Time      `gorm:"type:date;check:chk_mfg_orders_planned,planned_start IS NULL OR planned_end IS NULL OR planned_end >= planned_start"`
	Status            string          `gorm:"size:20;not null;index:idx_mfg_orders_status,priority:2;check:chk_mfg_orders_status,status IN ('draft','released','completed','cancelled')"`
	Description       string          `gorm:"size:500;not null;default:''"`
	CompletedQuantity decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_mfg_orders_completed,status <> 'completed' OR (completed_quantity > 0 AND completed_at IS NOT NULL)"`
	CompletedAt       *time.Time      `gorm:"type:date"`
	ConversionApplied decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_mfg_orders_applied,conversion_applied >= 0"`
	CancelledAt       *time.Time      `gorm:"check:chk_mfg_orders_cancelled,status <> 'cancelled' OR cancelled_at IS NOT NULL"`
	CancelReason      string          `gorm:"size:500;not null;default:''"`
	CreatedBy         uuid.UUID       `gorm:"type:uuid;not null"`
	CreatedAt         time.Time       `gorm:"not null"`
	UpdatedAt         time.Time       `gorm:"not null"`
}

func (OrderModel) TableName() string { return "manufacturing_orders" }

type OrderMaterialModel struct {
	OrderID      uuid.UUID       `gorm:"type:uuid;primaryKey;uniqueIndex:uq_mfg_order_materials_item,priority:1"`
	LineNo       int             `gorm:"primaryKey;check:chk_mfg_order_materials_line,line_no > 0"`
	Order        *OrderModel     `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	ItemID       uuid.UUID       `gorm:"type:uuid;not null;index;uniqueIndex:uq_mfg_order_materials_item,priority:2"`
	Required     decimal.Decimal `gorm:"column:required_quantity;type:numeric(24,6);not null;check:chk_mfg_order_materials_required,required_quantity >= 0"`
	Issued       decimal.Decimal `gorm:"column:issued_quantity;type:numeric(24,6);not null;default:0;check:chk_mfg_order_materials_returned,returned_quantity <= issued_quantity"`
	Returned     decimal.Decimal `gorm:"column:returned_quantity;type:numeric(24,6);not null;default:0;check:chk_mfg_order_materials_ret_pos,returned_quantity >= 0"`
	IssuedCost   decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0;check:chk_mfg_order_materials_cost,returned_cost <= issued_cost"`
	ReturnedCost decimal.Decimal `gorm:"type:numeric(24,6);not null;default:0"`
}

func (OrderMaterialModel) TableName() string { return "manufacturing_order_materials" }

type OrderConversionModel struct {
	OrderID     uuid.UUID       `gorm:"type:uuid;primaryKey"`
	LineNo      int             `gorm:"primaryKey"`
	Order       *OrderModel     `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Description string          `gorm:"size:200;not null"`
	AccountID   *uuid.UUID      `gorm:"type:uuid"`
	BatchAmount decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_order_conversions_amount,batch_amount >= 0"`
}

func (OrderConversionModel) TableName() string { return "manufacturing_order_conversions" }

// MovementModel คือการเคลื่อนไหวสต็อกของใบสั่งผลิต (อ้างเอกสารสต็อกของ inventory + รายการบัญชีของ ledger)
type MovementModel struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey"`
	OrderID         uuid.UUID       `gorm:"type:uuid;not null;index:idx_mfg_movements_order,priority:1"`
	Order           *OrderModel     `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Position        int             `gorm:"not null;index:idx_mfg_movements_order,priority:2"`
	Kind            string          `gorm:"size:20;not null;check:chk_mfg_movements_kind,kind IN ('issue','return','receipt')"`
	MovementDate    time.Time       `gorm:"type:date;not null"`
	StockDocumentID uuid.UUID       `gorm:"type:uuid;not null"`
	StockDocumentNo string          `gorm:"size:30;not null"`
	Value           decimal.Decimal `gorm:"type:numeric(24,6);not null;check:chk_mfg_movements_value,value >= 0"`
	EntryID         *uuid.UUID      `gorm:"type:uuid"`
	EntryNo         string          `gorm:"size:30;not null;default:''"`
	Cancelled       bool            `gorm:"not null;default:false"`
}

func (MovementModel) TableName() string { return "manufacturing_movements" }

// SequenceModel คือลำดับเลขที่เอกสาร (ต่อบริษัท / ชนิด / ช่วงตามรูปแบบเลขที่)
type SequenceModel struct {
	CompanyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	DocType   string    `gorm:"size:50;primaryKey"`
	PeriodKey string    `gorm:"size:20;primaryKey"`
	LastNo    int64     `gorm:"not null"`
}

func (SequenceModel) TableName() string { return "manufacturing_number_sequences" }

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&SettingsModel{}, &BomModel{}, &BomComponentModel{}, &BomConversionModel{},
		&OrderModel{}, &OrderMaterialModel{}, &OrderConversionModel{}, &MovementModel{}, &SequenceModel{},
	)
}
