package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/application/dto"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/dashperiod"
	"apexis/backend/internal/platform/database"
)

// Dashboard — 5 query แบบ aggregate (ไม่มี N+1, ไม่ใช้ transaction):
//  1. ยอดซื้อช่วงนี้ / ช่วงก่อน / จำนวนใบ  2. ยอดซื้อรายเดือน 12 เดือน  3. ใบสั่งซื้อค้าง (sub-select ใน query เดียว)
//  4. เจ้าหนี้ค้างตามอายุ ณ end_date  5. ผู้ขาย 5 อันดับ — แยกเพราะอ่านตาราง / ช่วงวันที่ต่างกัน
func (r *ReportRepository) Dashboard(ctx context.Context, companyID vo.CompanyID, p dashperiod.Period) (dto.Dashboard, error) {
	db := database.Conn(ctx, r.db)
	prev := p.Previous()
	args := map[string]any{
		"company": uuidOf(companyID), "start": p.Start, "end": p.End, "prev_start": prev.Start, "prev_end": prev.End,
		"trend_start": p.TrendStart(), "trend_end": p.TrendEnd(), "week": p.End.AddDate(0, 0, 7),
	}
	out := dto.Dashboard{Period: p}

	var buy struct {
		Current      decimal.Decimal
		Previous     decimal.Decimal
		InvoiceCount int64
	}
	if err := db.Raw(`SELECT
			COALESCE(SUM(total_net) FILTER (WHERE posting_date BETWEEN @start AND @end), 0) AS current,
			COALESCE(SUM(total_net) FILTER (WHERE posting_date BETWEEN @prev_start AND @prev_end), 0) AS previous,
			COUNT(*) FILTER (WHERE posting_date BETWEEN @start AND @end) AS invoice_count
		FROM purchasing_invoices
		WHERE company_id = @company AND status = 'posted' AND posting_date BETWEEN @prev_start AND @end`, args).Scan(&buy).Error; err != nil {
		return out, err
	}
	out.PurchasesCurrent, out.PurchasesPrevious, out.InvoiceCount = buy.Current, buy.Previous, buy.InvoiceCount

	var months []struct {
		Month string
		Net   decimal.Decimal
	}
	if err := db.Raw(`SELECT to_char(posting_date, 'YYYY-MM') AS month, SUM(total_net) AS net
		FROM purchasing_invoices
		WHERE company_id = @company AND status = 'posted' AND posting_date BETWEEN @trend_start AND @trend_end
		GROUP BY 1`, args).Scan(&months).Error; err != nil {
		return out, err
	}
	byMonth := make(map[string]decimal.Decimal, len(months))
	for _, m := range months {
		byMonth[m.Month] = m.Net
	}
	for _, key := range p.Months() {
		out.Monthly = append(out.Monthly, dto.MonthNet{Month: key, Net: byMonth[key].Add(decimal.Zero)})
	}

	var pipe struct {
		OrderDraftCount     int64
		OrderToReceiveCount int64
		OrderToInvoiceCount int64
	}
	if err := db.Raw(`SELECT
			(SELECT COUNT(*) FROM purchasing_orders WHERE company_id = @company AND status = 'draft') AS order_draft_count,
			(SELECT COUNT(*) FROM purchasing_orders AS o WHERE o.company_id = @company AND o.status = 'approved'
				AND EXISTS (SELECT 1 FROM purchasing_order_lines AS l WHERE l.order_id = o.id AND l.received_qty < l.quantity)) AS order_to_receive_count,
			(SELECT COUNT(*) FROM purchasing_orders AS o WHERE o.company_id = @company AND o.status IN ('approved', 'closed')
				AND EXISTS (SELECT 1 FROM purchasing_order_lines AS l WHERE l.order_id = o.id AND l.invoiced_qty < l.received_qty)) AS order_to_invoice_count`,
		args).Scan(&pipe).Error; err != nil {
		return out, err
	}
	out.OrderDraftCount, out.OrderToReceiveCount, out.OrderToInvoiceCount = pipe.OrderDraftCount, pipe.OrderToReceiveCount, pipe.OrderToInvoiceCount

	// เจ้าหนี้ ณ end_date — สูตรเดียวกับรายงานอายุเจ้าหนี้ (ยอดรวม VAT - ยอดจ่ายที่ลงวันที่ไม่เกิน end_date)
	var ap struct {
		Outstanding decimal.Decimal
		Overdue     decimal.Decimal
		Due7Days    decimal.Decimal `gorm:"column:due_7_days"`
		NotDue      decimal.Decimal
		Days1To30   decimal.Decimal `gorm:"column:days_1_30"`
		Days31To60  decimal.Decimal `gorm:"column:days_31_60"`
		Days61To90  decimal.Decimal `gorm:"column:days_61_90"`
		Over90      decimal.Decimal `gorm:"column:over_90"`
	}
	if err := db.Raw(`SELECT
			COALESCE(SUM(i.open_amount), 0) AS outstanding,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age > 0), 0) AS overdue,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age <= 0 AND i.due_date <= @week), 0) AS due_7_days,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age <= 0), 0) AS not_due,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age BETWEEN 1 AND 30), 0) AS days_1_30,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age BETWEEN 31 AND 60), 0) AS days_31_60,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age BETWEEN 61 AND 90), 0) AS days_61_90,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age > 90), 0) AS over_90
		FROM (SELECT inv.due_date, CAST(@end AS date) - inv.due_date AS age, inv.total_gross - COALESCE(pd.amount, 0) AS open_amount
		        FROM purchasing_invoices AS inv
		        LEFT JOIN (SELECT st.invoice_id, SUM(st.amount) AS amount
		                     FROM purchasing_invoice_settlements AS st
		                    WHERE st.company_id = @company AND st.status = 'posted' AND st.settled_on <= @end
		                    GROUP BY st.invoice_id) AS pd ON pd.invoice_id = inv.id
		       WHERE inv.company_id = @company AND inv.status = 'posted' AND inv.posting_date <= @end) AS i
		WHERE i.open_amount <> 0`, args).Scan(&ap).Error; err != nil {
		return out, err
	}
	out.Payables = dto.PayableSummary{Outstanding: ap.Outstanding, Overdue: ap.Overdue, Due7Days: ap.Due7Days, NotDue: ap.NotDue,
		Days1To30: ap.Days1To30, Days31To60: ap.Days31To60, Days61To90: ap.Days61To90, Over90: ap.Over90}

	var top []struct {
		ID   uuid.UUID
		Code string
		Name string
		Net  decimal.Decimal
	}
	if err := db.Raw(`SELECT s.id, s.code, s.name, SUM(d.total_net) AS net
		FROM purchasing_invoices AS d JOIN purchasing_suppliers AS s ON s.id = d.supplier_id
		WHERE d.company_id = @company AND d.status = 'posted' AND d.posting_date BETWEEN @start AND @end
		GROUP BY s.id, s.code, s.name
		ORDER BY net DESC, s.code
		LIMIT 5`, args).Scan(&top).Error; err != nil {
		return out, err
	}
	out.TopSuppliers = make([]dto.SupplierAmount, len(top))
	for i, t := range top {
		out.TopSuppliers[i] = dto.SupplierAmount{Supplier: dto.Ref{ID: t.ID.String(), Code: t.Code, Name: t.Name}, Net: t.Net}
	}
	return out, nil
}
