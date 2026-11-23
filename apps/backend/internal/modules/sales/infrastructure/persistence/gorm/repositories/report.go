package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/sales/application/dto"
	ports "apexis/backend/internal/modules/sales/application/ports/repositories"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.ReportRepository = (*ReportRepository)(nil)

type ReportRepository struct{ db *gorm.DB }

func NewReportRepository(db *gorm.DB) *ReportRepository { return &ReportRepository{db: db} }

type outstandingRow struct {
	OrderID      uuid.UUID
	OrderNo      string
	DocDate      time.Time
	DeliveryDate *time.Time
	CustomerID   uuid.UUID
	CustomerCode string
	CustomerName string
	LineNo       int
	ItemID       uuid.UUID
	Quantity     decimal.Decimal
	DeliveredQty decimal.Decimal
	InvoicedQty  decimal.Decimal
	ToDeliver    decimal.Decimal
	ToInvoice    decimal.Decimal
	Total        int64
}

func (r outstandingRow) dto() dto.OutstandingRow {
	return dto.OutstandingRow{
		Order: dto.Ref{ID: r.OrderID.String(), Code: r.OrderNo}, Date: dateUTC(r.DocDate), DeliveryDate: dateOpt(r.DeliveryDate),
		Customer: dto.Ref{ID: r.CustomerID.String(), Code: r.CustomerCode, Name: r.CustomerName}, LineNo: r.LineNo,
		Item: dto.Ref{ID: r.ItemID.String()}, Quantity: r.Quantity, Delivered: r.DeliveredQty, Invoiced: r.InvoicedQty,
		ToDeliver: r.ToDeliver, ToInvoice: r.ToInvoice,
	}
}

// Outstanding — 1 query: อนุมัติแล้วยังส่งหรือออกบิลไม่ครบ; ปิดแล้วเหลือเฉพาะส่งแล้วยังไม่ออกบิล
func (r *ReportRepository) Outstanding(ctx context.Context, companyID vo.CompanyID, f dto.OutstandingFilter) (listing.Result[dto.OutstandingRow], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{
		"date": "o.doc_date", "order_no": "o.order_no", "customer_code": "c.code", "delivery_date": "o.delivery_date",
	}, "o.id ASC, l.line_no")
	if err != nil {
		return listing.Result[dto.OutstandingRow]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("sales_order_lines AS l").
			Joins("JOIN sales_orders AS o ON o.id = l.order_id").
			Joins("JOIN sales_customers AS c ON c.id = o.customer_id").
			Where("o.company_id = ?", uuidOf(companyID)).
			Where(`(o.status = 'approved' AND (l.delivered_qty < l.quantity OR l.invoiced_qty < l.delivered_qty))
				OR (o.status = 'closed' AND l.invoiced_qty < l.delivered_qty)`)
		if f.CustomerID != nil {
			q = q.Where("o.customer_id = ?", uuidOf(*f.CustomerID))
		}
		if f.ItemID != nil {
			q = q.Where("l.item_id = ?", uuidOf(*f.ItemID))
		}
		return search(q, f.Search, "o.order_no", "c.code", "c.name")
	}
	rows, total, err := database.FetchPage(base, `o.id AS order_id, o.order_no, o.doc_date, o.delivery_date, o.customer_id,
		c.code AS customer_code, c.name AS customer_name, l.line_no, l.item_id, l.quantity, l.delivered_qty, l.invoiced_qty,
		CASE WHEN o.status = 'approved' THEN l.quantity - l.delivered_qty ELSE 0 END AS to_deliver,
		l.delivered_qty - l.invoiced_qty AS to_invoice`, order, f.Page, func(r outstandingRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, outstandingRow.dto), err
}

type agingRow struct {
	CustomerID   uuid.UUID
	CustomerCode string
	CustomerName string
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

// Aging — 1 query: ใบแจ้งหนี้ (ลงบัญชีถึงวันที่, ยังไม่ยกเลิก) หักยอดรับถึงวันที่ GROUP BY ลูกค้า แยกช่วงอายุจากวันครบกำหนด
func (r *ReportRepository) Aging(ctx context.Context, companyID vo.CompanyID, f dto.AgingFilter) (listing.Result[dto.AgingRow], dto.AgingRow, error) {
	where := []string{"i.company_id = ?", "i.status = 'posted'", "i.doc_date <= ?", "i.open_amount <> 0"}
	args := []any{uuidOf(companyID), f.AsOf, uuidOf(companyID), f.AsOf}
	if f.CustomerID != nil {
		where = append(where, "i.customer_id = ?")
		args = append(args, uuidOf(*f.CustomerID))
	}
	if f.Search != "" {
		p := database.LikeContains(f.Search)
		where = append(where, "(LOWER(c.code) LIKE ? OR LOWER(c.name) LIKE ?)")
		args = append(args, p, p)
	}
	from := ` FROM (
		SELECT inv.company_id, inv.customer_id, inv.status, inv.doc_date, inv.due_date,
		       inv.total_gross - COALESCE(rc.amount, 0) AS open_amount
		  FROM sales_invoices AS inv
		  LEFT JOIN (SELECT st.invoice_id, SUM(st.amount) AS amount
		               FROM sales_invoice_settlements AS st
		              WHERE st.company_id = ? AND st.status = 'posted' AND st.settled_on <= ?
		              GROUP BY st.invoice_id) AS rc ON rc.invoice_id = inv.id) AS i
		JOIN sales_customers AS c ON c.id = i.customer_id WHERE ` + strings.Join(where, " AND ")
	bucket := func(cond string) string { return "SUM(CASE WHEN " + cond + " THEN i.open_amount ELSE 0 END)" }
	overdue := "(CAST(? AS date) - i.due_date)"
	cols := []struct{ expr, name, total string }{
		{bucket(overdue + " <= 0"), "not_due", "t_not_due"},
		{bucket(overdue + " BETWEEN 1 AND 30"), "days_1_30", "t_1_30"},
		{bucket(overdue + " BETWEEN 31 AND 60"), "days_31_60", "t_31_60"},
		{bucket(overdue + " BETWEEN 61 AND 90"), "days_61_90", "t_61_90"},
		{bucket(overdue + " > 90"), "over_90", "t_over_90"},
	}
	selects := []string{"c.id AS customer_id", "c.code AS customer_code", "c.name AS customer_name"}
	var selArgs []any
	for _, c := range cols {
		selects = append(selects, c.expr+" AS "+c.name)
		selArgs = append(selArgs, f.AsOf)
	}
	selects = append(selects, "SUM(i.open_amount) AS amount", "COUNT(*) OVER () AS total")
	for _, c := range cols {
		selects = append(selects, "SUM("+c.expr+") OVER () AS "+c.total)
		selArgs = append(selArgs, f.AsOf)
	}
	selects = append(selects, "SUM(SUM(i.open_amount)) OVER () AS t_amount")
	query := "SELECT " + strings.Join(selects, ", ") + from + " GROUP BY c.id, c.code, c.name ORDER BY c.code, c.id LIMIT ? OFFSET ?"
	all := append(append(append([]any{}, selArgs...), args...), f.Page.Limit(), f.Page.Offset())
	var rows []agingRow
	if err := database.Conn(ctx, r.db).Raw(query, all...).Scan(&rows).Error; err != nil {
		return listing.Result[dto.AgingRow]{}, dto.AgingRow{}, err
	}
	zero := decimal.Zero
	res := listing.Result[dto.AgingRow]{Items: make([]dto.AgingRow, len(rows)), Page: f.Page}
	totals := dto.AgingRow{NotDue: zero, Days1To30: zero, Days31To60: zero, Days61To90: zero, Over90: zero, Total: zero}
	for i, x := range rows {
		res.Items[i] = dto.AgingRow{
			Customer: dto.Ref{ID: x.CustomerID.String(), Code: x.CustomerCode, Name: x.CustomerName},
			NotDue:   x.NotDue, Days1To30: x.Days1To30, Days31To60: x.Days31To60, Days61To90: x.Days61To90, Over90: x.Over90, Total: x.Amount,
		}
	}
	if len(rows) > 0 {
		x := rows[0]
		res.Total = x.Total
		totals = dto.AgingRow{NotDue: x.TNotDue, Days1To30: x.T1To30, Days31To60: x.T31To60, Days61To90: x.T61To90, Over90: x.TOver90, Total: x.TAmount}
	} else if f.Page.Offset() > 0 {
		if err := database.Conn(ctx, r.db).Raw("SELECT COUNT(DISTINCT c.id)"+from, args...).Scan(&res.Total).Error; err != nil {
			return listing.Result[dto.AgingRow]{}, dto.AgingRow{}, err
		}
	}
	return res, totals, nil
}

type taxRow struct {
	DocDate      time.Time
	DocumentID   uuid.UUID
	DocumentNo   string
	CustomerID   uuid.UUID
	CustomerCode string
	CustomerName string
	TaxID        string
	BranchNo     string
	TaxType      string
	Base         decimal.Decimal
	Tax          decimal.Decimal
	Total        int64
	TotalBase    decimal.Decimal
	TotalTax     decimal.Decimal
}

// taxReport — 1 query: แถวตามเอกสาร + ยอดรวมทุกแถวด้วย window function (+1 COUNT เมื่อขอหน้าที่เกินข้อมูล)
func (r *ReportRepository) taxReport(base func() *gorm.DB, columns string, f dto.TaxFilter) (listing.Result[dto.TaxRow], dto.TaxRow, error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "d.doc_date", "document_no": "d.document_no", "customer_code": "c.code"}, "d.id")
	if err != nil {
		return listing.Result[dto.TaxRow]{}, dto.TaxRow{}, err
	}
	var rows []taxRow
	err = base().Select(columns).Order(order).Limit(f.Page.Limit()).Offset(f.Page.Offset()).Scan(&rows).Error
	if err != nil {
		return listing.Result[dto.TaxRow]{}, dto.TaxRow{}, err
	}
	res := listing.Result[dto.TaxRow]{Items: make([]dto.TaxRow, len(rows)), Page: f.Page}
	totals := dto.TaxRow{Base: decimal.Zero, Tax: decimal.Zero}
	for i, x := range rows {
		res.Items[i] = dto.TaxRow{
			Date: dateUTC(x.DocDate), DocumentID: x.DocumentID.String(), DocumentNo: x.DocumentNo,
			Customer: dto.Ref{ID: x.CustomerID.String(), Code: x.CustomerCode, Name: x.CustomerName},
			TaxID:    x.TaxID, BranchNo: x.BranchNo, TaxType: x.TaxType, Base: x.Base, Tax: x.Tax,
		}
	}
	if len(rows) > 0 {
		res.Total, totals.Base, totals.Tax = rows[0].Total, rows[0].TotalBase, rows[0].TotalTax
	} else if f.Page.Offset() > 0 {
		if err := base().Count(&res.Total).Error; err != nil {
			return listing.Result[dto.TaxRow]{}, dto.TaxRow{}, err
		}
	}
	return res, totals, nil
}

// taxCustomer: ชื่อ / เลขผู้เสียภาษี / สาขาตามใบกำกับภาษี ณ วันออก (bill_to_*) — ไม่ใช่ทะเบียนลูกค้าปัจจุบัน
const taxCustomer = `c.id AS customer_id, c.code AS customer_code, d.bill_to_name AS customer_name, d.bill_to_tax_id AS tax_id,
	d.bill_to_branch_no AS branch_no, c.tax_type`

// OutputVat: ใบแจ้งหนี้ / ใบกำกับภาษี (ยังไม่ยกเลิก) ในช่วงวันที่ — ฐาน = ยอดก่อน VAT
func (r *ReportRepository) OutputVat(ctx context.Context, companyID vo.CompanyID, f dto.TaxFilter) (listing.Result[dto.TaxRow], dto.TaxRow, error) {
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("sales_invoices AS d").Joins("JOIN sales_customers AS c ON c.id = d.customer_id").
			Where("d.company_id = ? AND d.status = 'posted' AND d.doc_date BETWEEN ? AND ?", uuidOf(companyID), f.Start, f.End)
		if f.CustomerID != nil {
			q = q.Where("d.customer_id = ?", uuidOf(*f.CustomerID))
		}
		return q
	}
	return r.taxReport(base, `d.doc_date, d.id AS document_id, d.document_no, `+taxCustomer+`, d.total_net AS base, d.total_vat AS tax,
		COUNT(*) OVER () AS total, SUM(d.total_net) OVER () AS total_base, SUM(d.total_vat) OVER () AS total_tax`, f)
}
