// Package requests คือรูปแบบ JSON ขาเข้า / URL query ของ inventory และการแปลงเป็น input ของ use case
//
// presentation validation: field ที่ต้องมี, รูปแบบ UUID / วันที่ / ตัวเลข / enum
// กฎธุรกิจ (ทศนิยมตามบริษัท, FIFO, สต็อกห้ามติดลบ ฯลฯ) ตรวจใน domain / use case
package requests

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/domain/entities"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ErrValidation ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client (HTTP 400)
var ErrValidation = errors.New("validation failed")

// Invalid: validation error ที่ handler สร้างเอง (400)
func Invalid(msg string) error { return invalid("%s", msg) }

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

func required(fields map[string]string) error {
	var missing []string
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return invalid("%s is required", strings.Join(missing, ", "))
}

func id[T ~[16]byte](field, raw string) (T, error) {
	v, err := vo.ParseID[T](raw)
	if err != nil {
		return T{}, invalid("%s must be a UUID", field)
	}
	return v, nil
}

func optionalID[T ~[16]byte](field string, raw *string) (*T, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	v, err := id[T](field, *raw)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func status(raw string, isCreate bool) (vo.Status, error) {
	if raw == "" {
		if isCreate {
			return vo.StatusActive, nil
		}
		return "", invalid("status is required")
	}
	s, err := vo.ParseStatus(raw)
	if err != nil {
		return "", invalid("status has an invalid value %q", raw)
	}
	return s, nil
}

// Decimal รับตัวเลขเป็น string ("12.5") หรือ number (12.5) — เก็บข้อความดิบแล้วแปลงแบบไม่ผ่าน float
type Decimal string

func (d *Decimal) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		*d = Decimal(bytes.Trim(b, `"`))
	} else {
		*d = Decimal(b)
	}
	return nil
}

func (d *Decimal) parse(field string) (*decimal.Decimal, error) {
	if d == nil || *d == "" || *d == "null" {
		return nil, nil
	}
	v, err := vo.ParseDecimal(field, string(*d))
	if err != nil {
		return nil, invalid("%s must be a decimal number (up to 18 integer digits and 6 decimal places)", field)
	}
	return &v, nil
}

// ---------- master ----------

// Master: POST / PUT /uoms, /item-categories
type Master struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (r Master) ToData(isCreate bool) (entities.MasterData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return entities.MasterData{}, err
	}
	st, err := status(r.Status, isCreate)
	if err != nil {
		return entities.MasterData{}, err
	}
	return entities.MasterData{Code: r.Code, Name: r.Name, Status: st}, nil
}

// Accounts: บัญชีแยกประเภท (id ของ ledger) — ไม่ส่ง / null = ไม่ตั้ง (ใช้ของหมวด / ค่าเริ่มต้น)
type Accounts struct {
	PurchaseAccountID  *string `json:"purchase_account_id"`
	InventoryAccountID *string `json:"inventory_account_id"`
	RevenueAccountID   *string `json:"revenue_account_id"`
}

func (r Accounts) toGL(prefix string) (vo.GLAccounts, error) {
	purchase, err := optionalID[vo.AccountID](prefix+"purchase_account_id", r.PurchaseAccountID)
	if err != nil {
		return vo.GLAccounts{}, err
	}
	inventory, err := optionalID[vo.AccountID](prefix+"inventory_account_id", r.InventoryAccountID)
	if err != nil {
		return vo.GLAccounts{}, err
	}
	revenue, err := optionalID[vo.AccountID](prefix+"revenue_account_id", r.RevenueAccountID)
	if err != nil {
		return vo.GLAccounts{}, err
	}
	return vo.GLAccounts{Purchase: purchase, Inventory: inventory, Revenue: revenue}, nil
}

// Category: POST / PUT /item-categories — บัญชีของสินค้าในหมวด
type Category struct {
	Master
	Accounts
}

func (r Category) ToData(isCreate bool) (entities.MasterData, error) {
	d, err := r.Master.ToData(isCreate)
	if err != nil {
		return entities.MasterData{}, err
	}
	d.Accounts, err = r.Accounts.toGL("")
	return d, err
}

// Warehouse: POST / PUT /warehouses
type Warehouse struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	BranchID *string `json:"branch_id"`
	Status   string  `json:"status"`
}

func (r Warehouse) ToData(isCreate bool) (entities.WarehouseData, error) {
	m, err := Master{Code: r.Code, Name: r.Name, Status: r.Status}.ToData(isCreate)
	if err != nil {
		return entities.WarehouseData{}, err
	}
	branch, err := optionalID[vo.BranchID]("branch_id", r.BranchID)
	if err != nil {
		return entities.WarehouseData{}, err
	}
	return entities.WarehouseData{Code: m.Code, Name: m.Name, BranchID: branch, Status: m.Status}, nil
}

// Item: POST / PUT /items — type ค่าเริ่มต้น stock ตอน POST, บังคับส่งตอน PUT
type Item struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	NameEN       string   `json:"name_en"`
	Type         string   `json:"type"`
	UomID        string   `json:"uom_id"`
	CategoryID   *string  `json:"category_id"`
	ReorderPoint *Decimal `json:"reorder_point"`
	Status       string   `json:"status"`
	Accounts
}

func (r Item) ToData(isCreate bool) (entities.ItemData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name, "uom_id": r.UomID}); err != nil {
		return entities.ItemData{}, err
	}
	typ := vo.ItemStock
	if r.Type != "" {
		t, err := vo.ParseItemType(r.Type)
		if err != nil {
			return entities.ItemData{}, invalid("type must be one of stock, non_stock, service")
		}
		typ = t
	} else if !isCreate {
		return entities.ItemData{}, invalid("type is required")
	}
	uom, err := id[vo.UomID]("uom_id", r.UomID)
	if err != nil {
		return entities.ItemData{}, err
	}
	category, err := optionalID[vo.CategoryID]("category_id", r.CategoryID)
	if err != nil {
		return entities.ItemData{}, err
	}
	reorder, err := r.ReorderPoint.parse("reorder_point")
	if err != nil {
		return entities.ItemData{}, err
	}
	st, err := status(r.Status, isCreate)
	if err != nil {
		return entities.ItemData{}, err
	}
	accounts, err := r.Accounts.toGL("")
	if err != nil {
		return entities.ItemData{}, err
	}
	return entities.ItemData{
		Code: r.Code, Name: r.Name, NameEN: r.NameEN, Type: typ, UomID: uom, CategoryID: category, ReorderPoint: reorder,
		Accounts: accounts, Status: st,
	}, nil
}

// ---------- stock document ----------

type DocumentLine struct {
	ItemID      string   `json:"item_id"`
	Quantity    *Decimal `json:"quantity"`
	UnitCost    *Decimal `json:"unit_cost"`
	Description string   `json:"description"`
}

// Document: POST / PUT /stock-documents
// status (เฉพาะ POST): "draft" (ค่าเริ่มต้น) หรือ "posted" = สร้างแล้วผ่านเอกสารทันที
type Document struct {
	Type          string         `json:"type"`
	Date          string         `json:"date"`
	WarehouseID   string         `json:"warehouse_id"`
	ToWarehouseID *string        `json:"to_warehouse_id"`
	Reference     string         `json:"reference"`
	Description   string         `json:"description"`
	Status        string         `json:"status"`
	Lines         []DocumentLine `json:"lines"`
}

func (r Document) PostOnCreate() (bool, error) {
	switch r.Status {
	case "", string(vo.DocDraft):
		return false, nil
	case string(vo.DocPosted):
		return true, nil
	}
	return false, invalid(`status must be "draft" or "posted"`)
}

func (r Document) ToData() (entities.DocumentData, error) {
	if err := required(map[string]string{"type": r.Type, "date": r.Date, "warehouse_id": r.WarehouseID}); err != nil {
		return entities.DocumentData{}, err
	}
	typ, err := vo.ParseDocumentType(r.Type)
	if err != nil {
		return entities.DocumentData{}, invalid("type must be one of receipt, issue, transfer, adjustment")
	}
	date, err := listing.ParseDate(r.Date)
	if err != nil {
		return entities.DocumentData{}, invalid("date must be a date in YYYY-MM-DD format")
	}
	wh, err := id[vo.WarehouseID]("warehouse_id", r.WarehouseID)
	if err != nil {
		return entities.DocumentData{}, err
	}
	to, err := optionalID[vo.WarehouseID]("to_warehouse_id", r.ToWarehouseID)
	if err != nil {
		return entities.DocumentData{}, err
	}
	if typ == vo.DocTransfer && to == nil {
		return entities.DocumentData{}, invalid("to_warehouse_id is required for transfer")
	}
	if typ != vo.DocTransfer && to != nil {
		return entities.DocumentData{}, invalid("to_warehouse_id is only allowed for transfer")
	}
	if len(r.Lines) == 0 {
		return entities.DocumentData{}, invalid("lines is required")
	}
	lines := make([]entities.LineData, len(r.Lines))
	for i, l := range r.Lines {
		if lines[i], err = l.toData(i + 1); err != nil {
			return entities.DocumentData{}, err
		}
	}
	return entities.DocumentData{
		Type: typ, Date: time.Time(date), WarehouseID: wh, ToWarehouseID: to,
		Reference: r.Reference, Description: r.Description, Lines: lines,
	}, nil
}

func (l DocumentLine) toData(n int) (entities.LineData, error) {
	field := func(name string) string { return fmt.Sprintf("lines[%d].%s", n, name) }
	if l.ItemID == "" {
		return entities.LineData{}, invalid("%s is required", field("item_id"))
	}
	item, err := id[vo.ItemID](field("item_id"), l.ItemID)
	if err != nil {
		return entities.LineData{}, err
	}
	qty, err := l.Quantity.parse(field("quantity"))
	if err != nil {
		return entities.LineData{}, err
	}
	if qty == nil {
		return entities.LineData{}, invalid("%s is required", field("quantity"))
	}
	cost, err := l.UnitCost.parse(field("unit_cost"))
	if err != nil {
		return entities.LineData{}, err
	}
	return entities.LineData{ItemID: item, Quantity: *qty, UnitCost: cost, Description: l.Description}, nil
}

// UpdateDocumentStatus: PATCH /stock-documents/{documentID} {"status": "posted" | "cancelled"}
type UpdateDocumentStatus struct {
	Status string `json:"status"`
}

func (r UpdateDocumentStatus) Parse() (vo.DocumentStatus, error) {
	switch vo.DocumentStatus(r.Status) {
	case vo.DocPosted, vo.DocCancelled:
		return vo.DocumentStatus(r.Status), nil
	}
	return "", invalid(`status must be "posted" or "cancelled"`)
}

// ---------- ค่าตั้ง / ปิดงวด ----------

// Settings: PUT /inventory-settings — แทนที่ทั้งก้อน (field ที่ไม่ส่ง = ไม่ตั้ง)
type Settings struct {
	DefaultPurchaseAccountID  *string `json:"default_purchase_account_id"`
	DefaultInventoryAccountID *string `json:"default_inventory_account_id"`
	CogsAccountID             *string `json:"cogs_account_id"`
	JournalID                 *string `json:"journal_id"`
}

func (r Settings) ToSettings() (entities.Settings, error) {
	defaults, err := Accounts{PurchaseAccountID: r.DefaultPurchaseAccountID, InventoryAccountID: r.DefaultInventoryAccountID}.toGL("default_")
	if err != nil {
		return entities.Settings{}, err
	}
	cogs, err := optionalID[vo.AccountID]("cogs_account_id", r.CogsAccountID)
	if err != nil {
		return entities.Settings{}, err
	}
	journal, err := optionalID[vo.JournalID]("journal_id", r.JournalID)
	if err != nil {
		return entities.Settings{}, err
	}
	return entities.Settings{Defaults: defaults, CogsAccountID: cogs, JournalID: journal}, nil
}

// AccountingSystem: PUT /inventory-settings/accounting-systems/{fiscalYear}
type AccountingSystem struct {
	System string `json:"system"`
}

func (r AccountingSystem) Parse() (vo.AccountingSystem, error) {
	s, err := vo.ParseAccountingSystem(r.System)
	if err != nil {
		return "", invalid("system must be one of perpetual, periodic")
	}
	return s, nil
}

// Closing: POST /inventory-closings — date คือวันใดก็ได้ในงวดที่จะปิด (YYYY-MM-DD)
type Closing struct {
	Date string `json:"date"`
}

func (r Closing) ParseDate() (time.Time, error) {
	if err := required(map[string]string{"date": r.Date}); err != nil {
		return time.Time{}, err
	}
	d, err := listing.ParseDate(r.Date)
	if err != nil {
		return time.Time{}, invalid("date must be YYYY-MM-DD")
	}
	return d, nil
}

// ClosingStatus: PATCH /inventory-closings/{closingID} — {"status":"cancelled"}
type ClosingStatus struct {
	Status string `json:"status"`
}
