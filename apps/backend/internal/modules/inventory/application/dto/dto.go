// Package dto คือข้อมูลขาเข้า (Input / Filter) และขาออก (read model) ของ use case inventory
package dto

import (
	"time"

	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- Input ----------

type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

type Create[D any] struct {
	Actor
	Data D
}

type Update[ID any, D any] struct {
	Actor
	ID   ID
	Data D
}

type ByID[ID any] struct {
	Actor
	ID ID
}

type List[F any] struct {
	Actor
	Filter F
}

// CreateDocument: Post = true คือสร้างแล้วผ่านเอกสารทันทีใน transaction เดียว
type CreateDocument[D any] struct {
	Actor
	Data D
	Post bool
}

// ---------- Filter (จาก URL query) ----------

var (
	MasterSortKeys    = []string{"code", "name"}
	DefaultMasterSort = []listing.Sort{{Key: "code"}}
)

// MasterFilter: GET /uoms, /item-categories, /warehouses ?q=&status=&sort=&page=&page_size=
type MasterFilter struct {
	Search   string
	Statuses []vo.Status
	BranchID *vo.BranchID // เฉพาะ warehouses
	Sort     []listing.Sort
	Page     listing.Page
}

var (
	ItemSortKeys    = []string{"code", "name", "type"}
	DefaultItemSort = []listing.Sort{{Key: "code"}}
)

// ItemFilter: GET /items?q=&type=&status=&category_id=&uom_id=&sort=&page=&page_size=
type ItemFilter struct {
	Search     string
	Types      []vo.ItemType
	Statuses   []vo.Status
	CategoryID *vo.CategoryID
	UomID      *vo.UomID
	Sort       []listing.Sort
	Page       listing.Page
}

var (
	DocumentSortKeys    = []string{"date", "document_no", "created_at"}
	DefaultDocumentSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "document_no", Desc: true}}
)

// DocumentFilter: GET /stock-documents?q=&type=&status=&warehouse_id=&item_id=&start_date=&end_date=&sort=&page=&page_size=
type DocumentFilter struct {
	Search      string // เลขที่เอกสาร / เลขอ้างอิง / คำอธิบาย
	Types       []vo.DocumentType
	Statuses    []vo.DocumentStatus
	WarehouseID *vo.WarehouseID // คลังต้นทางหรือปลายทาง
	ItemID      *vo.ItemID      // มีบรรทัดของสินค้านี้
	StartDate   *time.Time
	EndDate     *time.Time
	Sort        []listing.Sort
	Page        listing.Page
}

var (
	StockOnHandSortKeys    = []string{"item_code", "item_name", "warehouse_code", "quantity", "value"}
	DefaultStockOnHandSort = []listing.Sort{{Key: "item_code"}, {Key: "warehouse_code"}}
)

// StockOnHandFilter: book=stock-on-hand — ยอดคงเหลือ ณ วันที่ (end_date) ต่อสินค้า × คลัง
type StockOnHandFilter struct {
	AsOf        time.Time
	Search      string // รหัส / ชื่อสินค้า
	WarehouseID *vo.WarehouseID
	ItemID      *vo.ItemID
	CategoryID  *vo.CategoryID
	Sort        []listing.Sort
	Page        listing.Page
}

// StockCardFilter: book=stock-card — การเคลื่อนไหวของสินค้าหนึ่ง (ทุกคลังหรือคลังเดียว)
type StockCardFilter struct {
	ItemID      vo.ItemID
	WarehouseID *vo.WarehouseID
	Start, End  time.Time
	Page        listing.Page
}

// ---------- Output (read model) ----------
//
// จำนวน / มูลค่าเป็น decimal ตามค่าจริง — frontend จัดรูปแบบตามทศนิยมที่บริษัทกำหนด

type Ref struct {
	ID   string
	Code string
	Name string
}

type Master struct {
	ID       string
	Code     string
	Name     string
	BranchID *string // เฉพาะ warehouse
	Status   string
}

type Item struct {
	ID           string
	Code         string
	Name         string
	NameEN       string
	Type         string
	Uom          Ref
	Category     *Ref
	ReorderPoint *vo.Quantity
	Status       string
}

type DocumentSummary struct {
	ID          string
	Type        string
	DocumentNo  string // ว่างจนกว่าจะผ่านเอกสาร
	Date        time.Time
	Warehouse   Ref
	ToWarehouse *Ref
	Reference   string
	Description string
	Status      string
	TotalValue  vo.Money
	PostedAt    *time.Time
}

type DocumentLine struct {
	LineNo      int
	Item        Ref
	UomCode     string
	Quantity    vo.Quantity
	UnitCost    *vo.Money
	Value       vo.Money
	Description string
}

type Document struct {
	DocumentSummary
	FiscalPeriodID *string
	CreatedBy      string
	PostedBy       *string
	CancelledAt    *time.Time
	CancelledBy    *string
	Lines          []DocumentLine
}

type StockOnHandRow struct {
	Item      Ref
	UomCode   string
	Warehouse Ref
	Quantity  vo.Quantity
	Value     vo.Money
}

type StockOnHand struct {
	AsOf       time.Time
	Rows       listing.Result[StockOnHandRow]
	TotalValue vo.Money // มูลค่ารวมทุกแถวที่ตรงเงื่อนไข (ไม่ใช่แค่หน้านี้)
}

type StockCardLine struct {
	Date            time.Time
	DocumentID      string
	DocumentNo      string
	DocumentType    string
	Warehouse       Ref
	Quantity        vo.Quantity // + เข้า / - ออก
	Value           vo.Money
	BalanceQuantity vo.Quantity // คงเหลือสะสม (รวมยอดยกมา)
	BalanceValue    vo.Money
}

type StockCard struct {
	Item            Ref
	UomCode         string
	Warehouse       *Ref
	Start, End      time.Time
	OpeningQuantity vo.Quantity
	OpeningValue    vo.Money
	Lines           listing.Result[StockCardLine]
}
