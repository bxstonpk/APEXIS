// Package requests คือรูปแบบ JSON ขาเข้า / URL query ของ manufacturing และการแปลงเป็น input ของ use case
//
// presentation validation: field ที่ต้องมี, รูปแบบ UUID / วันที่ / ตัวเลข / enum
// กฎธุรกิจ (สูตร, ปริมาณ, คืนไม่เกินที่เบิก, ทศนิยมตามบริษัท ฯลฯ) ตรวจใน domain / use case
package requests

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/manufacturing/domain/entities"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

// ErrValidation ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client (HTTP 400)
var ErrValidation = errors.New("validation failed")

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

func date(field, raw string) (time.Time, error) {
	d, err := listing.ParseDate(raw)
	if err != nil {
		return time.Time{}, invalid("%s must be YYYY-MM-DD", field)
	}
	return d, nil
}

func optionalDate(field string, raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	d, err := date(field, *raw)
	return &d, err
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
	v, ok := numeric.Parse(strings.TrimSpace(string(*d)))
	if !ok {
		return nil, invalid("%s must be a decimal number (up to 18 integer digits and 6 decimal places)", field)
	}
	return &v, nil
}

func (d *Decimal) required(field string) (decimal.Decimal, error) {
	v, err := d.parse(field)
	if err != nil {
		return decimal.Zero, err
	}
	if v == nil {
		return decimal.Zero, invalid("%s is required", field)
	}
	return *v, nil
}

func (d *Decimal) optional(field string) (decimal.Decimal, error) {
	v, err := d.parse(field)
	if err != nil || v == nil {
		return decimal.Zero, err
	}
	return *v, nil
}

// ---------- ค่าตั้ง ----------

// Settings: PUT /manufacturing-settings — แทนที่ทั้งก้อน (ที่ไม่ส่ง = ไม่ตั้ง)
type Settings struct {
	JournalID           *string `json:"journal_id"`
	WipAccountID        *string `json:"wip_account_id"`
	ConversionAccountID *string `json:"conversion_account_id"`
	DefaultWarehouseID  *string `json:"default_warehouse_id"`
}

func (r Settings) ToSettings() (entities.Settings, error) {
	var out entities.Settings
	var err error
	if out.JournalID, err = optionalID[vo.JournalID]("journal_id", r.JournalID); err != nil {
		return out, err
	}
	if out.WipAccountID, err = optionalID[vo.AccountID]("wip_account_id", r.WipAccountID); err != nil {
		return out, err
	}
	if out.ConversionAccountID, err = optionalID[vo.AccountID]("conversion_account_id", r.ConversionAccountID); err != nil {
		return out, err
	}
	out.DefaultWarehouseID, err = optionalID[vo.WarehouseID]("default_warehouse_id", r.DefaultWarehouseID)
	return out, err
}

// ---------- สูตรการผลิต ----------

type Component struct {
	ItemID       string   `json:"item_id"`
	Quantity     *Decimal `json:"quantity"`
	ScrapPercent *Decimal `json:"scrap_percent"`
}

type ConversionCost struct {
	Description string   `json:"description"`
	AccountID   *string  `json:"account_id"`
	Amount      *Decimal `json:"amount"`
}

// Bom: POST /boms, PUT /boms/{id} — แทนที่ทั้งก้อน
type Bom struct {
	Code            string           `json:"code"`
	Name            string           `json:"name"`
	ItemID          string           `json:"item_id"`
	OutputQuantity  *Decimal         `json:"output_quantity"`
	Status          string           `json:"status"`
	IsDefault       bool             `json:"is_default"`
	Components      []Component      `json:"components"`
	ConversionCosts []ConversionCost `json:"conversion_costs"`
}

func (r Bom) ToData(isCreate bool) (entities.BomData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name, "item_id": r.ItemID}); err != nil {
		return entities.BomData{}, err
	}
	out := entities.BomData{Code: r.Code, Name: r.Name, IsDefault: r.IsDefault}
	var err error
	if out.ItemID, err = id[vo.ItemID]("item_id", r.ItemID); err != nil {
		return out, err
	}
	if out.OutputQuantity, err = r.OutputQuantity.required("output_quantity"); err != nil {
		return out, err
	}
	switch {
	case r.Status == "" && isCreate:
		out.Status = vo.StatusActive
	case r.Status == "":
		return out, invalid("status is required")
	default:
		if out.Status, err = vo.ParseStatus(r.Status); err != nil {
			return out, invalid("status has an invalid value %q", r.Status)
		}
	}
	if len(r.Components) == 0 {
		return out, invalid("components must have at least 1 line")
	}
	for i, c := range r.Components {
		var d entities.ComponentData
		if d.ItemID, err = id[vo.ItemID](fmt.Sprintf("components[%d].item_id", i), c.ItemID); err != nil {
			return out, err
		}
		if d.Quantity, err = c.Quantity.required(fmt.Sprintf("components[%d].quantity", i)); err != nil {
			return out, err
		}
		if d.ScrapPercent, err = c.ScrapPercent.optional(fmt.Sprintf("components[%d].scrap_percent", i)); err != nil {
			return out, err
		}
		out.Components = append(out.Components, d)
	}
	for i, c := range r.ConversionCosts {
		d := entities.ConversionData{Description: c.Description}
		if d.AccountID, err = optionalID[vo.AccountID](fmt.Sprintf("conversion_costs[%d].account_id", i), c.AccountID); err != nil {
			return out, err
		}
		if d.Amount, err = c.Amount.required(fmt.Sprintf("conversion_costs[%d].amount", i)); err != nil {
			return out, err
		}
		out.Conversions = append(out.Conversions, d)
	}
	return out, nil
}

// ---------- ใบสั่งผลิต ----------

type Material struct {
	ItemID           string   `json:"item_id"`
	RequiredQuantity *Decimal `json:"required_quantity"`
}

// Order: POST /production-orders, PUT /production-orders/{id} (draft; materials ไม่ส่ง = คำนวณจากสูตรใหม่)
type Order struct {
	Date         string     `json:"date"`
	ItemID       string     `json:"item_id"`
	BomID        string     `json:"bom_id"`
	Quantity     *Decimal   `json:"quantity"`
	WarehouseID  string     `json:"warehouse_id"`
	PlannedStart *string    `json:"planned_start"`
	PlannedEnd   *string    `json:"planned_end"`
	Description  string     `json:"description"`
	Materials    []Material `json:"materials"`
}

func (r Order) ToData() (entities.OrderData, []entities.MaterialInput, error) {
	if err := required(map[string]string{"date": r.Date, "item_id": r.ItemID, "bom_id": r.BomID, "warehouse_id": r.WarehouseID}); err != nil {
		return entities.OrderData{}, nil, err
	}
	out := entities.OrderData{Description: r.Description}
	var err error
	if out.Date, err = date("date", r.Date); err != nil {
		return out, nil, err
	}
	if out.ItemID, err = id[vo.ItemID]("item_id", r.ItemID); err != nil {
		return out, nil, err
	}
	if out.BomID, err = id[vo.BomID]("bom_id", r.BomID); err != nil {
		return out, nil, err
	}
	if out.WarehouseID, err = id[vo.WarehouseID]("warehouse_id", r.WarehouseID); err != nil {
		return out, nil, err
	}
	if out.Quantity, err = r.Quantity.required("quantity"); err != nil {
		return out, nil, err
	}
	if out.PlannedStart, err = optionalDate("planned_start", r.PlannedStart); err != nil {
		return out, nil, err
	}
	if out.PlannedEnd, err = optionalDate("planned_end", r.PlannedEnd); err != nil {
		return out, nil, err
	}
	if r.Materials == nil {
		return out, nil, nil
	}
	materials := make([]entities.MaterialInput, len(r.Materials))
	for i, m := range r.Materials {
		if materials[i].ItemID, err = id[vo.ItemID](fmt.Sprintf("materials[%d].item_id", i), m.ItemID); err != nil {
			return out, nil, err
		}
		if materials[i].Required, err = m.RequiredQuantity.required(fmt.Sprintf("materials[%d].required_quantity", i)); err != nil {
			return out, nil, err
		}
	}
	return out, materials, nil
}

type MovementLine struct {
	ItemID   string   `json:"item_id"`
	Quantity *Decimal `json:"quantity"`
}

// Movement: POST /production-orders/{id}/issue | /return
type Movement struct {
	Date  string         `json:"date"`
	Lines []MovementLine `json:"lines"`
}

func (r Movement) ToInput() (time.Time, []entities.MovementLine, error) {
	if err := required(map[string]string{"date": r.Date}); err != nil {
		return time.Time{}, nil, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return d, nil, err
	}
	if len(r.Lines) == 0 {
		return d, nil, invalid("lines must have at least 1 line")
	}
	lines := make([]entities.MovementLine, len(r.Lines))
	for i, l := range r.Lines {
		if lines[i].ItemID, err = id[vo.ItemID](fmt.Sprintf("lines[%d].item_id", i), l.ItemID); err != nil {
			return d, nil, err
		}
		if lines[i].Quantity, err = l.Quantity.required(fmt.Sprintf("lines[%d].quantity", i)); err != nil {
			return d, nil, err
		}
	}
	return d, lines, nil
}

// Complete: POST /production-orders/{id}/complete
type Complete struct {
	Date              string   `json:"date"`
	CompletedQuantity *Decimal `json:"completed_quantity"`
}

func (r Complete) ToInput() (time.Time, decimal.Decimal, error) {
	if err := required(map[string]string{"date": r.Date}); err != nil {
		return time.Time{}, decimal.Zero, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return d, decimal.Zero, err
	}
	q, err := r.CompletedQuantity.required("completed_quantity")
	return d, q, err
}

// Cancel: POST /production-orders/{id}/cancel
type Cancel struct {
	Reason string `json:"reason"`
}
