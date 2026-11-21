// Package responses คือรูปแบบ JSON ขาออกของ inventory
//
//   - วันที่ "YYYY-MM-DD" (ค.ศ.)
//   - จำนวน / มูลค่าเป็น string ทศนิยมตามค่าจริง (ไม่ใช้ number เพื่อไม่ให้ client ปัดเศษผิด)
//     frontend จัดรูปแบบตามทศนิยมที่บริษัทกำหนด (company.amount_decimals / quantity_decimals)
package responses

import (
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/listing"
)

func date(t time.Time) string { return t.Format(time.DateOnly) }

func optDecimal(d *decimal.Decimal) *string {
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

func ref(r dto.Ref) Ref { return Ref{ID: r.ID, Code: r.Code, Name: r.Name} }

func meta[T any](r listing.Result[T]) httpx.Meta {
	return httpx.Meta{Page: r.Page.Number, PageSize: r.Page.Size, Total: r.Total}
}

// ---------- master ----------

type Master struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func FromMaster(m dto.Master) Master {
	return Master{ID: m.ID, Code: m.Code, Name: m.Name, Status: m.Status}
}

type Accounts struct {
	PurchaseAccountID  *string `json:"purchase_account_id"`
	InventoryAccountID *string `json:"inventory_account_id"`
}

func accounts(a dto.Accounts) Accounts {
	return Accounts{PurchaseAccountID: a.PurchaseAccountID, InventoryAccountID: a.InventoryAccountID}
}

type Category struct {
	Master
	Accounts
}

func FromCategory(m dto.Master) Category {
	return Category{Master: FromMaster(m), Accounts: accounts(m.Accounts)}
}

type Warehouse struct {
	Master
	BranchID *string `json:"branch_id"`
}

func FromWarehouse(m dto.Master) Warehouse {
	return Warehouse{Master: FromMaster(m), BranchID: m.BranchID}
}

type Item struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	NameEN       string  `json:"name_en"`
	Type         string  `json:"type"`
	Uom          Ref     `json:"uom"`
	Category     *Ref    `json:"category"`
	ReorderPoint *string `json:"reorder_point"`
	Accounts
	Status string `json:"status"`
}

func FromItem(i dto.Item) Item {
	out := Item{
		ID: i.ID, Code: i.Code, Name: i.Name, NameEN: i.NameEN, Type: i.Type, Uom: ref(i.Uom),
		ReorderPoint: optDecimal(i.ReorderPoint), Accounts: accounts(i.Accounts), Status: i.Status,
	}
	if i.Category != nil {
		c := ref(*i.Category)
		out.Category = &c
	}
	return out
}

// ---------- stock document ----------

type DocumentSummary struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	DocumentNo  *string    `json:"document_no"` // null จนกว่าจะผ่านเอกสาร
	Date        string     `json:"date"`
	Warehouse   Ref        `json:"warehouse"`
	ToWarehouse *Ref       `json:"to_warehouse"`
	Reference   string     `json:"reference"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	TotalValue  string     `json:"total_value"`
	PostedAt    *time.Time `json:"posted_at"`
}

func FromDocumentSummary(d dto.DocumentSummary) DocumentSummary {
	out := DocumentSummary{
		ID: d.ID, Type: d.Type, Date: date(d.Date), Warehouse: ref(d.Warehouse), Reference: d.Reference,
		Description: d.Description, Status: d.Status, TotalValue: d.TotalValue.String(), PostedAt: d.PostedAt,
	}
	if d.DocumentNo != "" {
		no := d.DocumentNo
		out.DocumentNo = &no
	}
	if d.ToWarehouse != nil {
		t := ref(*d.ToWarehouse)
		out.ToWarehouse = &t
	}
	return out
}

type DocumentLine struct {
	LineNo      int     `json:"line_no"`
	Item        Ref     `json:"item"`
	UomCode     string  `json:"uom_code"`
	Quantity    string  `json:"quantity"`
	UnitCost    *string `json:"unit_cost"`
	Value       string  `json:"value"` // ต้นทุนของบรรทัด (ขาออกคิดแบบ FIFO ตอนผ่านเอกสาร)
	Description string  `json:"description"`
}

type Document struct {
	DocumentSummary
	FiscalPeriodID *string        `json:"fiscal_period_id"`
	CreatedBy      string         `json:"created_by"`
	PostedBy       *string        `json:"posted_by"`
	CancelledAt    *time.Time     `json:"cancelled_at"`
	CancelledBy    *string        `json:"cancelled_by"`
	Lines          []DocumentLine `json:"lines"`
}

func FromDocument(d dto.Document) Document {
	out := Document{
		DocumentSummary: FromDocumentSummary(d.DocumentSummary), FiscalPeriodID: d.FiscalPeriodID, CreatedBy: d.CreatedBy,
		PostedBy: d.PostedBy, CancelledAt: d.CancelledAt, CancelledBy: d.CancelledBy, Lines: make([]DocumentLine, len(d.Lines)),
	}
	for i, l := range d.Lines {
		out.Lines[i] = DocumentLine{
			LineNo: l.LineNo, Item: ref(l.Item), UomCode: l.UomCode, Quantity: l.Quantity.String(),
			UnitCost: optDecimal(l.UnitCost), Value: l.Value.String(), Description: l.Description,
		}
	}
	return out
}

// ---------- รายงาน (data + meta สำหรับแบ่งหน้า) ----------

type StockOnHandRow struct {
	Item      Ref    `json:"item"`
	UomCode   string `json:"uom_code"`
	Warehouse Ref    `json:"warehouse"`
	Quantity  string `json:"quantity"`
	Value     string `json:"value"`
}

type StockOnHand struct {
	Book       string           `json:"book"`
	AsOf       string           `json:"as_of"`
	Rows       []StockOnHandRow `json:"rows"`
	TotalValue string           `json:"total_value"`
}

type StockOnHandBody struct {
	Data StockOnHand `json:"data"`
	Meta httpx.Meta  `json:"meta"`
}

func FromStockOnHand(book string, s dto.StockOnHand) StockOnHandBody {
	rows := make([]StockOnHandRow, len(s.Rows.Items))
	for i, r := range s.Rows.Items {
		rows[i] = StockOnHandRow{Item: ref(r.Item), UomCode: r.UomCode, Warehouse: ref(r.Warehouse), Quantity: r.Quantity.String(), Value: r.Value.String()}
	}
	return StockOnHandBody{
		Data: StockOnHand{Book: book, AsOf: date(s.AsOf), Rows: rows, TotalValue: s.TotalValue.String()},
		Meta: meta(s.Rows),
	}
}

type StockCardLine struct {
	Date            string `json:"date"`
	DocumentID      string `json:"document_id"`
	DocumentNo      string `json:"document_no"`
	DocumentType    string `json:"document_type"`
	Warehouse       Ref    `json:"warehouse"`
	Quantity        string `json:"quantity"` // + เข้า / - ออก
	Value           string `json:"value"`
	BalanceQuantity string `json:"balance_quantity"`
	BalanceValue    string `json:"balance_value"`
}

type StockCard struct {
	Book            string          `json:"book"`
	Item            Ref             `json:"item"`
	UomCode         string          `json:"uom_code"`
	Warehouse       *Ref            `json:"warehouse"` // null = ทุกคลัง
	StartDate       string          `json:"start_date"`
	EndDate         string          `json:"end_date"`
	OpeningQuantity string          `json:"opening_quantity"`
	OpeningValue    string          `json:"opening_value"`
	Lines           []StockCardLine `json:"lines"`
}

type StockCardBody struct {
	Data StockCard  `json:"data"`
	Meta httpx.Meta `json:"meta"`
}

func FromStockCard(book string, c dto.StockCard) StockCardBody {
	lines := make([]StockCardLine, len(c.Lines.Items))
	for i, l := range c.Lines.Items {
		lines[i] = StockCardLine{
			Date: date(l.Date), DocumentID: l.DocumentID, DocumentNo: l.DocumentNo, DocumentType: l.DocumentType,
			Warehouse: ref(l.Warehouse), Quantity: l.Quantity.String(), Value: l.Value.String(),
			BalanceQuantity: l.BalanceQuantity.String(), BalanceValue: l.BalanceValue.String(),
		}
	}
	out := StockCard{
		Book: book, Item: ref(c.Item), UomCode: c.UomCode, StartDate: date(c.Start), EndDate: date(c.End),
		OpeningQuantity: c.OpeningQuantity.String(), OpeningValue: c.OpeningValue.String(), Lines: lines,
	}
	if c.Warehouse != nil {
		w := ref(*c.Warehouse)
		out.Warehouse = &w
	}
	return StockCardBody{Data: out, Meta: meta(c.Lines)}
}

// ---------- ค่าตั้ง / ปิดงวด ----------

type SystemSetting struct {
	FromYear int    `json:"from_fiscal_year"`
	System   string `json:"system"`
}

type Settings struct {
	DefaultPurchaseAccountID  *string         `json:"default_purchase_account_id"`
	DefaultInventoryAccountID *string         `json:"default_inventory_account_id"`
	CogsAccountID             *string         `json:"cogs_account_id"`
	JournalID                 *string         `json:"journal_id"`
	AccountingSystems         []SystemSetting `json:"accounting_systems"` // ปีที่ไม่ได้ตั้ง = perpetual
}

func FromSettings(s dto.Settings) Settings {
	out := Settings{
		DefaultPurchaseAccountID: s.Defaults.PurchaseAccountID, DefaultInventoryAccountID: s.Defaults.InventoryAccountID,
		CogsAccountID: s.CogsAccountID, JournalID: s.JournalID, AccountingSystems: make([]SystemSetting, len(s.Systems)),
	}
	for i, x := range s.Systems {
		out.AccountingSystems[i] = SystemSetting{FromYear: x.FromYear, System: x.System}
	}
	return out
}

type Closing struct {
	ID             string     `json:"id"`
	FiscalPeriodID string     `json:"fiscal_period_id"`
	FiscalYear     int        `json:"fiscal_year"`
	PeriodEnd      string     `json:"period_end"`
	EntryID        *string    `json:"entry_id"` // null = ไม่มีส่วนต่างต้องปรับ
	EntryNo        string     `json:"entry_no"`
	Adjustment     string     `json:"adjustment"` // สินค้าคงเหลือเพิ่ม (+) / ลด (-)
	Status         string     `json:"status"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	CancelledBy    *string    `json:"cancelled_by"`
	CancelledAt    *time.Time `json:"cancelled_at"`
}

func FromClosing(c dto.Closing) Closing {
	return Closing{
		ID: c.ID, FiscalPeriodID: c.PeriodID, FiscalYear: c.FiscalYear, PeriodEnd: date(c.PeriodEnd), EntryID: c.EntryID,
		EntryNo: c.EntryNo, Adjustment: c.Adjustment.String(), Status: c.Status, CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt,
		CancelledBy: c.CancelledBy, CancelledAt: c.CancelledAt,
	}
}
