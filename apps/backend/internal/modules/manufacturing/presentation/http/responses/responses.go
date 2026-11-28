// Package responses คือรูปแบบ JSON ขาออกของ manufacturing (Response DTO — ไม่เปิดเผย gorm model)
// ตัวเลขเป็น string (ไม่เสียความแม่นยำ), วันที่ YYYY-MM-DD, เวลาเป็น RFC 3339
package responses

import (
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/platform/httpx"
)

func str(d decimal.Decimal) string { return d.String() }

func date(t time.Time) string { return t.Format(time.DateOnly) }

func optDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := date(*t)
	return &s
}

func optStr(d *decimal.Decimal) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type ItemRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	Uom  string `json:"uom"`
}

func ref(r dto.Ref) Ref         { return Ref{ID: r.ID, Code: r.Code, Name: r.Name} }
func itemRef(r dto.Ref) ItemRef { return ItemRef{ID: r.ID, Code: r.Code, Name: r.Name, Uom: r.Uom} }

func optRef(r *dto.Ref) *Ref {
	if r == nil {
		return nil
	}
	out := ref(*r)
	return &out
}

// ---------- ค่าตั้ง ----------

type Settings struct {
	JournalID           *string `json:"journal_id"`
	WipAccountID        *string `json:"wip_account_id"`
	ConversionAccountID *string `json:"conversion_account_id"`
	DefaultWarehouseID  *string `json:"default_warehouse_id"`
	Journal             *Ref    `json:"journal"`
	WipAccount          *Ref    `json:"wip_account"`
	ConversionAccount   *Ref    `json:"conversion_account"`
	DefaultWarehouse    *Ref    `json:"default_warehouse"`
}

func refID(r *dto.Ref) *string {
	if r == nil {
		return nil
	}
	return &r.ID
}

func FromSettings(s dto.Settings) Settings {
	return Settings{JournalID: refID(s.Journal), WipAccountID: refID(s.Wip), ConversionAccountID: refID(s.Conversion),
		DefaultWarehouseID: refID(s.Warehouse), Journal: optRef(s.Journal), WipAccount: optRef(s.Wip),
		ConversionAccount: optRef(s.Conversion), DefaultWarehouse: optRef(s.Warehouse)}
}

// ---------- สูตรการผลิต ----------

type BomRow struct {
	ID             string    `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Item           ItemRef   `json:"item"`
	OutputQuantity string    `json:"output_quantity"`
	Status         string    `json:"status"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BomComponent struct {
	LineNo       int     `json:"line_no"`
	Item         ItemRef `json:"item"`
	Quantity     string  `json:"quantity"`
	ScrapPercent string  `json:"scrap_percent"`
}

type BomConversion struct {
	LineNo      int    `json:"line_no"`
	Description string `json:"description"`
	Account     *Ref   `json:"account"`
	Amount      string `json:"amount"`
}

type Bom struct {
	BomRow
	Components      []BomComponent  `json:"components"`
	ConversionCosts []BomConversion `json:"conversion_costs"`
}

func FromBomRow(b dto.BomRow) BomRow {
	return BomRow{ID: b.ID, Code: b.Code, Name: b.Name, Item: itemRef(b.Item), OutputQuantity: str(b.OutputQuantity),
		Status: string(b.Status), IsDefault: b.IsDefault, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt}
}

func FromBom(b dto.Bom) Bom {
	out := Bom{BomRow: FromBomRow(b.BomRow), Components: make([]BomComponent, len(b.Components)), ConversionCosts: make([]BomConversion, len(b.Conversions))}
	for i, c := range b.Components {
		out.Components[i] = BomComponent{LineNo: c.LineNo, Item: itemRef(c.Item), Quantity: str(c.Quantity), ScrapPercent: str(c.ScrapPercent)}
	}
	for i, c := range b.Conversions {
		out.ConversionCosts[i] = BomConversion{LineNo: c.LineNo, Description: c.Description, Account: optRef(c.Account), Amount: str(c.Amount)}
	}
	return out
}

// ---------- ใบสั่งผลิต ----------

type OrderRow struct {
	ID                string    `json:"id"`
	DocumentNo        string    `json:"document_no"`
	Date              string    `json:"date"`
	Item              ItemRef   `json:"item"`
	Bom               Ref       `json:"bom"`
	Quantity          string    `json:"quantity"`
	Warehouse         Ref       `json:"warehouse"`
	PlannedStart      *string   `json:"planned_start"`
	PlannedEnd        *string   `json:"planned_end"`
	Status            string    `json:"status"`
	CompletedQuantity string    `json:"completed_quantity"`
	CompletedAt       *string   `json:"completed_at"`
	TotalCost         string    `json:"total_cost"`
	CreatedAt         time.Time `json:"created_at"`
}

type OrderMaterial struct {
	LineNo           int     `json:"line_no"`
	Item             ItemRef `json:"item"`
	RequiredQuantity string  `json:"required_quantity"`
	IssuedQuantity   string  `json:"issued_quantity"`
	ReturnedQuantity string  `json:"returned_quantity"`
	NetQuantity      string  `json:"net_quantity"`
	Cost             string  `json:"cost"`
}

type OrderMovement struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Date       string `json:"date"`
	DocumentID string `json:"document_id"`
	DocumentNo string `json:"document_no"`
	Value      string `json:"value"`
	EntryNo    string `json:"entry_no"`
	Cancelled  bool   `json:"cancelled"`
}

type OrderCosts struct {
	Materials  string  `json:"materials"`
	Conversion string  `json:"conversion"`
	Total      string  `json:"total"`
	UnitCost   *string `json:"unit_cost"`
}

type Order struct {
	OrderRow
	Description    string          `json:"description"`
	Materials      []OrderMaterial `json:"materials"`
	ConversionCost string          `json:"conversion_cost"`
	Movements      []OrderMovement `json:"movements"`
	Costs          OrderCosts      `json:"costs"`
	CancelledAt    *time.Time      `json:"cancelled_at"`
	CancelReason   string          `json:"cancel_reason"`
	CreatedBy      string          `json:"created_by"`
}

func FromOrderRow(o dto.OrderRow) OrderRow {
	return OrderRow{ID: o.ID, DocumentNo: o.DocumentNo, Date: date(o.Date), Item: itemRef(o.Item), Bom: ref(o.Bom), Quantity: str(o.Quantity),
		Warehouse: ref(o.Warehouse), PlannedStart: optDate(o.PlannedStart), PlannedEnd: optDate(o.PlannedEnd), Status: string(o.Status),
		CompletedQuantity: str(o.CompletedQuantity), CompletedAt: optDate(o.CompletedAt), TotalCost: str(o.TotalCost), CreatedAt: o.CreatedAt}
}

func FromOrder(o dto.Order) Order {
	out := Order{OrderRow: FromOrderRow(o.OrderRow), Description: o.Description, ConversionCost: str(o.ConversionCost),
		Materials: make([]OrderMaterial, len(o.Materials)), Movements: make([]OrderMovement, len(o.Movements)),
		Costs: OrderCosts{Materials: str(o.Costs.Materials), Conversion: str(o.Costs.Conversion), Total: str(o.Costs.Total),
			UnitCost: optStr(o.Costs.UnitCost)},
		CancelledAt: o.CancelledAt, CancelReason: o.CancelReason, CreatedBy: o.CreatedBy}
	for i, m := range o.Materials {
		out.Materials[i] = OrderMaterial{LineNo: m.LineNo, Item: itemRef(m.Item), RequiredQuantity: str(m.Required), IssuedQuantity: str(m.Issued),
			ReturnedQuantity: str(m.Returned), NetQuantity: str(m.Net), Cost: str(m.Cost)}
	}
	for i, m := range o.Movements {
		out.Movements[i] = OrderMovement{ID: m.ID, Kind: string(m.Kind), Date: date(m.Date), DocumentID: m.DocumentID, DocumentNo: m.DocumentNo,
			Value: str(m.Value), EntryNo: m.EntryNo, Cancelled: m.Cancelled}
	}
	return out
}

// ---------- รายงาน ----------

type OrderRef struct {
	ID         string `json:"id"`
	DocumentNo string `json:"document_no"`
}

func orderRef(r dto.Ref) OrderRef { return OrderRef{ID: r.ID, DocumentNo: r.Code} }

type CostRow struct {
	Order      OrderRef `json:"order"`
	Date       string   `json:"date"`
	Item       ItemRef  `json:"item"`
	Quantity   string   `json:"quantity"`
	Materials  string   `json:"materials"`
	Conversion string   `json:"conversion"`
	Total      string   `json:"total"`
	UnitCost   string   `json:"unit_cost"`
}

type CostTotals struct {
	Materials  string `json:"materials"`
	Conversion string `json:"conversion"`
	Total      string `json:"total"`
}

type CostReport struct {
	Book   string     `json:"book"`
	Rows   []CostRow  `json:"rows"`
	Totals CostTotals `json:"totals"`
}

type CostBody struct {
	Data CostReport `json:"data"`
	Meta httpx.Meta `json:"meta"`
}

func FromCost(book string, r dto.CostReport) CostBody {
	list := httpx.ToList(r.Rows, func(x dto.CostRow) CostRow {
		return CostRow{Order: orderRef(x.Order), Date: date(x.Date), Item: itemRef(x.Item), Quantity: str(x.Quantity), Materials: str(x.Materials),
			Conversion: str(x.Conversion), Total: str(x.Total), UnitCost: str(x.UnitCost)}
	})
	return CostBody{Data: CostReport{Book: book, Rows: list.Data, Totals: CostTotals{Materials: str(r.Totals.Materials),
		Conversion: str(r.Totals.Conversion), Total: str(r.Totals.Total)}}, Meta: list.Meta}
}

type UsageRow struct {
	Order            OrderRef `json:"order"`
	Date             string   `json:"date"`
	Item             ItemRef  `json:"item"`
	Material         ItemRef  `json:"material"`
	StandardQuantity string   `json:"standard_quantity"`
	ActualQuantity   string   `json:"actual_quantity"`
	VarianceQuantity string   `json:"variance_quantity"`
	ActualCost       string   `json:"actual_cost"`
	VarianceValue    string   `json:"variance_value"`
}

type UsageReport struct {
	Book string     `json:"book"`
	Rows []UsageRow `json:"rows"`
}

type UsageBody struct {
	Data UsageReport `json:"data"`
	Meta httpx.Meta  `json:"meta"`
}

func FromUsage(book string, r httpx.List[UsageRow]) UsageBody {
	return UsageBody{Data: UsageReport{Book: book, Rows: r.Data}, Meta: r.Meta}
}

func FromUsageRow(x dto.UsageRow) UsageRow {
	return UsageRow{Order: orderRef(x.Order), Date: date(x.Date), Item: itemRef(x.Item), Material: itemRef(x.Material),
		StandardQuantity: str(x.Standard), ActualQuantity: str(x.Actual), VarianceQuantity: str(x.Variance), ActualCost: str(x.ActualCost),
		VarianceValue: str(x.VarianceValue)}
}

type WipRow struct {
	Order     OrderRef `json:"order"`
	Date      string   `json:"date"`
	Item      ItemRef  `json:"item"`
	Quantity  string   `json:"quantity"`
	Materials string   `json:"materials"`
}

type WipReport struct {
	Book  string   `json:"book"`
	AsOf  string   `json:"end_date"`
	Rows  []WipRow `json:"rows"`
	Total string   `json:"total"`
}

type WipBody struct {
	Data WipReport  `json:"data"`
	Meta httpx.Meta `json:"meta"`
}

func FromWip(book string, asOf time.Time, r dto.WipReport) WipBody {
	list := httpx.ToList(r.Rows, func(x dto.WipRow) WipRow {
		return WipRow{Order: orderRef(x.Order), Date: date(x.Date), Item: itemRef(x.Item), Quantity: str(x.Quantity), Materials: str(x.Materials)}
	})
	return WipBody{Data: WipReport{Book: book, AsOf: date(asOf), Rows: list.Data, Total: str(r.Total)}, Meta: list.Meta}
}
