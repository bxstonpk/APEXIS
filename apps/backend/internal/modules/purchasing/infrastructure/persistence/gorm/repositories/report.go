package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/purchasing/application/dto"
	ports "apexis/backend/internal/modules/purchasing/application/ports/repositories"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.ReportRepository = (*ReportRepository)(nil)

type ReportRepository struct{ db *gorm.DB }

func NewReportRepository(db *gorm.DB) *ReportRepository { return &ReportRepository{db: db} }

type outstandingRow struct {
	OrderID      uuid.UUID
	OrderNo      string
	OrderDate    time.Time
	DeliveryDate *time.Time
	SupplierID   uuid.UUID
	SupplierCode string
	SupplierName string
	LineNo       int
	ItemID       uuid.UUID
	Quantity     decimal.Decimal
	ReceivedQty  decimal.Decimal
	InvoicedQty  decimal.Decimal
	ToReceive    decimal.Decimal
	ToInvoice    decimal.Decimal
	Total        int64
}

func (r outstandingRow) dto() dto.OutstandingRow {
	return dto.OutstandingRow{
		Order: dto.Ref{ID: r.OrderID.String(), Code: r.OrderNo}, Date: gp.DateUTC(r.OrderDate), DeliveryDate: dateOpt(r.DeliveryDate),
		Supplier: dto.Ref{ID: r.SupplierID.String(), Code: r.SupplierCode, Name: r.SupplierName}, LineNo: r.LineNo,
		Item: dto.Ref{ID: r.ItemID.String()}, Quantity: r.Quantity, Received: r.ReceivedQty, Invoiced: r.InvoicedQty,
		ToReceive: r.ToReceive, ToInvoice: r.ToInvoice,
	}
}

// Outstanding — 1 query: บรรทัดใบสั่งซื้อ JOIN หัว + ผู้ขาย
// อนุมัติแล้ว: ยังรับไม่ครบหรือแจ้งหนี้ไม่ครบ; ปิดแล้ว: เหลือเฉพาะส่วนที่รับแล้วยังไม่แจ้งหนี้ (ไม่รับเพิ่ม)
func (r *ReportRepository) Outstanding(ctx context.Context, companyID vo.CompanyID, f dto.OutstandingFilter) (listing.Result[dto.OutstandingRow], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{
		"date": "o.order_date", "order_no": "o.order_no", "supplier_code": "s.code", "delivery_date": "o.delivery_date",
	}, "o.id ASC, l.line_no")
	if err != nil {
		return listing.Result[dto.OutstandingRow]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("purchasing_order_lines AS l").
			Joins("JOIN purchasing_orders AS o ON o.id = l.order_id").
			Joins("JOIN purchasing_suppliers AS s ON s.id = o.supplier_id").
			Where("o.company_id = ?", uuidOf(companyID)).
			Where(`(o.status = 'approved' AND (l.received_qty < l.quantity OR l.invoiced_qty < l.received_qty))
				OR (o.status = 'closed' AND l.invoiced_qty < l.received_qty)`).
			Where("o.kind = ?", string(vo.KindOrDefault(string(f.Kind))))
		if f.SupplierID != nil {
			q = q.Where("o.supplier_id = ?", uuidOf(*f.SupplierID))
		}
		if f.ItemID != nil {
			q = q.Where("l.item_id = ?", uuidOf(*f.ItemID))
		}
		return search(q, f.Search, "o.order_no", "s.code", "s.name")
	}
	rows, total, err := database.FetchPage(base, `o.id AS order_id, o.order_no, o.order_date, o.delivery_date, o.supplier_id,
		s.code AS supplier_code, s.name AS supplier_name, l.line_no, l.item_id, l.quantity, l.received_qty, l.invoiced_qty,
		CASE WHEN o.status = 'approved' THEN l.quantity - l.received_qty ELSE 0 END AS to_receive,
		l.received_qty - l.invoiced_qty AS to_invoice`, order, f.Page, func(r outstandingRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, outstandingRow.dto), err
}

type agingRow struct {
	SupplierID   uuid.UUID
	SupplierCode string
	SupplierName string
	NotDue       decimal.Decimal
	Days1To30    decimal.Decimal `gorm:"column:days_1_30"`
	Days31To60   decimal.Decimal `gorm:"column:days_31_60"`
	Days61To90   decimal.Decimal `gorm:"column:days_61_90"`
	Over90       decimal.Decimal `gorm:"column:over_90"`
	Amount       decimal.Decimal
	Total        int64
	TNotDue      decimal.Decimal `gorm:"column:t_not_due"`
	T1To30       decimal.Decimal `gorm:"column:t_1_30"`
	T31To60      decimal.Decimal `gorm:"column:t_31_60"`
	T61To90      decimal.Decimal `gorm:"column:t_61_90"`
	TOver90      decimal.Decimal `gorm:"column:t_over_90"`
	TAmount      decimal.Decimal `gorm:"column:t_amount"`
}

// Aging — 1 query: ใบแจ้งหนี้ที่ลงบัญชีถึงวันที่ (ยังไม่ยกเลิก) GROUP BY ผู้ขาย, แยกช่วงอายุด้วย CASE,
// ยอดรวมทุกผู้ขายด้วย window function, แบ่งหน้าใน DB (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล)
// ยอดค้าง = ยอดรวม VAT ของใบแจ้งหนี้ - ยอดจ่ายถึงวันที่รายงาน (ใบที่จ่ายครบแล้วไม่แสดง)
func (r *ReportRepository) Aging(ctx context.Context, companyID vo.CompanyID, f dto.AgingFilter) (listing.Result[dto.AgingRow], dto.AgingRow, error) {
	where := []string{"i.company_id = ?", "i.status = 'posted'", "i.posting_date <= ?", "i.open_amount <> 0"}
	// placeholder ของ subquery ยอดจ่าย (บริษัท, ณ วันที่) อยู่ใน FROM ก่อน WHERE
	args := []any{uuidOf(companyID), f.AsOf, uuidOf(companyID), f.AsOf}
	if f.SupplierID != nil {
		where = append(where, "i.supplier_id = ?")
		args = append(args, uuidOf(*f.SupplierID))
	}
	if f.Search != "" {
		p := database.LikeContains(f.Search)
		where = append(where, "(LOWER(s.code) LIKE ? OR LOWER(s.name) LIKE ?)")
		args = append(args, p, p)
	}
	// ยอดค้าง ณ วันที่ = ยอดรวม VAT - ยอดที่จ่ายด้วยใบจ่าย (posted) ที่ลงวันที่ไม่เกินวันที่รายงาน
	from := ` FROM (
		SELECT inv.company_id, inv.supplier_id, inv.status, inv.posting_date, inv.due_date,
		       inv.total_gross - COALESCE(pd.amount, 0) AS open_amount
		  FROM purchasing_invoices AS inv
		  LEFT JOIN (SELECT st.invoice_id, SUM(st.amount) AS amount
		               FROM purchasing_invoice_settlements AS st
		              WHERE st.company_id = ? AND st.status = 'posted' AND st.settled_on <= ?
		              GROUP BY st.invoice_id) AS pd ON pd.invoice_id = inv.id) AS i
		JOIN purchasing_suppliers AS s ON s.id = i.supplier_id WHERE ` + strings.Join(where, " AND ")
	bucket := func(cond string) string { return "SUM(CASE WHEN " + cond + " THEN i.open_amount ELSE 0 END)" }
	// ?::date - due_date = จำนวนวันที่เลยกำหนด
	overdue := "(CAST(? AS date) - i.due_date)"
	cols := []struct{ expr, name string }{
		{bucket(overdue + " <= 0"), "not_due"},
		{bucket(overdue + " BETWEEN 1 AND 30"), "days_1_30"},
		{bucket(overdue + " BETWEEN 31 AND 60"), "days_31_60"},
		{bucket(overdue + " BETWEEN 61 AND 90"), "days_61_90"},
		{bucket(overdue + " > 90"), "over_90"},
	}
	selects := []string{"s.id AS supplier_id", "s.code AS supplier_code", "s.name AS supplier_name"}
	var selArgs []any
	totals := map[string]string{"not_due": "t_not_due", "days_1_30": "t_1_30", "days_31_60": "t_31_60", "days_61_90": "t_61_90", "over_90": "t_over_90"}
	for _, c := range cols {
		selects = append(selects, c.expr+" AS "+c.name)
		selArgs = append(selArgs, f.AsOf)
	}
	selects = append(selects, "SUM(i.open_amount) AS amount", "COUNT(*) OVER () AS total")
	for _, c := range cols {
		selects = append(selects, "SUM("+c.expr+") OVER () AS "+totals[c.name])
		selArgs = append(selArgs, f.AsOf)
	}
	selects = append(selects, "SUM(SUM(i.open_amount)) OVER () AS t_amount")

	query := "SELECT " + strings.Join(selects, ", ") + from +
		" GROUP BY s.id, s.code, s.name ORDER BY s.code, s.id LIMIT ? OFFSET ?"
	var rows []agingRow
	all := append(append(append([]any{}, selArgs...), args...), f.Page.Limit(), f.Page.Offset())
	if err := database.Conn(ctx, r.db).Raw(query, all...).Scan(&rows).Error; err != nil {
		return listing.Result[dto.AgingRow]{}, dto.AgingRow{}, err
	}
	zero := decimal.Zero
	res := listing.Result[dto.AgingRow]{Items: make([]dto.AgingRow, len(rows)), Page: f.Page}
	sum := dto.AgingRow{NotDue: zero, Days1To30: zero, Days31To60: zero, Days61To90: zero, Over90: zero, Total: zero}
	for i, row := range rows {
		res.Items[i] = dto.AgingRow{
			Supplier: dto.Ref{ID: row.SupplierID.String(), Code: row.SupplierCode, Name: row.SupplierName},
			NotDue:   row.NotDue, Days1To30: row.Days1To30, Days31To60: row.Days31To60, Days61To90: row.Days61To90,
			Over90: row.Over90, Total: row.Amount,
		}
	}
	if len(rows) > 0 {
		r0 := rows[0]
		res.Total = r0.Total
		sum = dto.AgingRow{NotDue: r0.TNotDue, Days1To30: r0.T1To30, Days31To60: r0.T31To60, Days61To90: r0.T61To90, Over90: r0.TOver90, Total: r0.TAmount}
	} else if f.Page.Offset() > 0 {
		if err := database.Conn(ctx, r.db).Raw("SELECT COUNT(DISTINCT s.id)"+from, args...).Scan(&res.Total).Error; err != nil {
			return listing.Result[dto.AgingRow]{}, dto.AgingRow{}, err
		}
	}
	return res, sum, nil
}
