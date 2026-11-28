// Package dto คือ input / output ของ use case ใน manufacturing (ไม่ผูกกับ HTTP หรือ database)
package dto

import (
	"time"

	"apexis/backend/internal/modules/manufacturing/domain/entities"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// Actor คือผู้ใช้ที่เรียก use case ในบริษัทหนึ่ง
type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

// Ref คือข้อมูลอ้างอิงสำหรับแสดงผล (สินค้า / คลัง / บัญชี / สมุดรายวัน / สูตร) — Uom ใช้กับสินค้าเท่านั้น
type Ref struct {
	ID   string
	Code string
	Name string
	Uom  string
}

// ---------- สูตรการผลิต ----------

var (
	BomSortKeys    = []string{"code", "name", "updated_at"}
	DefaultBomSort = []listing.Sort{{Key: "code"}}
)

// BomFilter: GET /boms?q=&item_id=&status=&sort=&page=&page_size=
type BomFilter struct {
	Search string
	ItemID *vo.ItemID
	Status []vo.Status
	Sort   []listing.Sort
	Page   listing.Page
}

type BomRow struct {
	ID             string
	Code           string
	Name           string
	Item           Ref
	OutputQuantity vo.Quantity
	Status         vo.Status
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type BomComponent struct {
	LineNo       int
	Item         Ref
	Quantity     vo.Quantity
	ScrapPercent vo.Quantity
}

type BomConversion struct {
	LineNo      int
	Description string
	Account     *Ref
	Amount      vo.Money
}

type Bom struct {
	BomRow
	Components  []BomComponent
	Conversions []BomConversion
}

type BomInput struct {
	Actor
	ID   vo.BomID // ใช้ตอนแก้ไข
	Data entities.BomData
}

// ---------- ใบสั่งผลิต ----------

var (
	OrderSortKeys    = []string{"date", "document_no", "created_at"}
	DefaultOrderSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "document_no", Desc: true}}
)

// OrderFilter: GET /production-orders?q=&status=&item_id=&from=&to=&sort=&page=&page_size=
type OrderFilter struct {
	Search string
	Status []vo.OrderStatus
	ItemID *vo.ItemID
	From   *time.Time
	To     *time.Time
	Sort   []listing.Sort
	Page   listing.Page
}

type OrderRow struct {
	ID                string
	DocumentNo        string
	Date              time.Time
	Item              Ref
	Bom               Ref
	Quantity          vo.Quantity
	Warehouse         Ref
	PlannedStart      *time.Time
	PlannedEnd        *time.Time
	Status            vo.OrderStatus
	CompletedQuantity vo.Quantity
	CompletedAt       *time.Time
	TotalCost         vo.Money
	CreatedAt         time.Time
}

type OrderMaterial struct {
	LineNo   int
	Item     Ref
	Required vo.Quantity
	Issued   vo.Quantity
	Returned vo.Quantity
	Net      vo.Quantity
	Cost     vo.Money
}

type OrderMovement struct {
	ID         string
	Kind       vo.MovementKind
	Date       time.Time
	DocumentID string
	DocumentNo string
	Value      vo.Money
	EntryNo    string
	Cancelled  bool
}

type OrderCosts struct {
	Materials  vo.Money
	Conversion vo.Money
	Total      vo.Money
	UnitCost   *vo.Money
}

type Order struct {
	OrderRow
	Description    string
	Materials      []OrderMaterial
	ConversionCost vo.Money // ค่าแรง / ค่าใช้จ่ายมาตรฐานตามจำนวนสั่งผลิต
	Movements      []OrderMovement
	Costs          OrderCosts
	CancelledAt    *time.Time
	CancelReason   string
	CreatedBy      string
}

type CreateOrderInput struct {
	Actor
	Data entities.OrderData
}

type UpdateOrderInput struct {
	Actor
	ID        vo.OrderID
	Data      entities.OrderData
	Materials []entities.MaterialInput // nil = คำนวณจากสูตรใหม่
}

type OrderAction struct {
	Actor
	ID vo.OrderID
}

type MovementInput struct {
	Actor
	ID    vo.OrderID
	Date  time.Time
	Lines []entities.MovementLine
}

type CompleteInput struct {
	Actor
	ID       vo.OrderID
	Date     time.Time
	Quantity vo.Quantity
}

type CancelInput struct {
	Actor
	ID     vo.OrderID
	Reason string
}

// ---------- ค่าตั้ง ----------

type Settings struct {
	Journal    *Ref
	Wip        *Ref
	Conversion *Ref
	Warehouse  *Ref
}

type SettingsInput struct {
	Actor
	Settings entities.Settings
}

// ---------- รายงาน ----------

// CostFilter: book=production-cost / material-usage — ใบสั่งผลิตที่ปิดงานในช่วงวันที่
type CostFilter struct {
	Start  time.Time
	End    time.Time
	ItemID *vo.ItemID
	Page   listing.Page
	// ทศนิยมของบริษัท (use case เติม) — ปัดปริมาณตามแผน / มูลค่าผลต่างของ material-usage
	QuantityDecimals int
	AmountDecimals   int
}

type CostRow struct {
	Order      Ref // ID + Code = เลขที่ใบสั่งผลิต
	Date       time.Time
	Item       Ref
	Quantity   vo.Quantity
	Materials  vo.Money
	Conversion vo.Money
	Total      vo.Money
	UnitCost   vo.Money
}

type CostTotals struct {
	Materials  vo.Money
	Conversion vo.Money
	Total      vo.Money
}

type CostReport struct {
	Rows   listing.Result[CostRow]
	Totals CostTotals
}

type UsageRow struct {
	Order         Ref
	Date          time.Time
	Item          Ref // สินค้าที่ผลิต
	Material      Ref
	Standard      vo.Quantity // ปริมาณตามแผน × (จำนวนที่ผลิตได้ / จำนวนสั่งผลิต)
	Actual        vo.Quantity // เบิกสุทธิ
	Variance      vo.Quantity // actual − standard (บวก = ใช้เกิน)
	ActualCost    vo.Money
	VarianceValue vo.Money // ผลต่างจำนวน × ต้นทุนเฉลี่ยที่เบิกจริง
}

// WipFilter: book=wip — ใบสั่งผลิตที่ยังเปิดอยู่ (released) ณ วันที่
type WipFilter struct {
	AsOf time.Time
	Page listing.Page
}

type WipRow struct {
	Order     Ref
	Date      time.Time
	Item      Ref
	Quantity  vo.Quantity
	Materials vo.Money
}

type WipReport struct {
	Rows  listing.Result[WipRow]
	Total vo.Money
}
