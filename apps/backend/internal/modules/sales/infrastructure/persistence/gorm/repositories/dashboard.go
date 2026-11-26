package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/dashperiod"
	"apexis/backend/internal/platform/database"
)

// Dashboard — 5 query แบบ aggregate (ไม่มี N+1, ไม่ใช้ transaction):
//  1. ยอดขายช่วงนี้ / ช่วงก่อน / จำนวนใบ (FILTER)  2. ยอดขายรายเดือน 12 เดือน (GROUP BY)
//  3. ใบเสนอราคา / ใบสั่งขายค้าง (sub-select ใน query เดียว)  4. ลูกหนี้ค้างตามอายุ ณ end_date  5. ลูกค้า 5 อันดับ
//
// แยก 5 query เพราะแต่ละชุดอ่านตาราง / ช่วงวันที่ต่างกัน — รวมเป็น query เดียวจะซับซ้อนเกินประโยชน์
func (r *ReportRepository) Dashboard(ctx context.Context, companyID vo.CompanyID, p dashperiod.Period) (dto.Dashboard, error) {
	db := database.Conn(ctx, r.db)
	prev := p.Previous()
	args := map[string]any{
		"company": uuidOf(companyID), "start": p.Start, "end": p.End, "prev_start": prev.Start, "prev_end": prev.End,
		"trend_start": p.TrendStart(), "trend_end": p.TrendEnd(), "months_from": p.TrendStart(),
	}
	out := dto.Dashboard{Period: p}
	if p.FiscalYear > 0 {
		args["months_from"] = p.PreviousTrendStart() // ปีบัญชีก่อนหน้า + ปีนี้ใน query เดียว
	}

	var sales struct {
		Current      decimal.Decimal
		Previous     decimal.Decimal
		InvoiceCount int64
	}
	if err := db.Raw(`SELECT
			COALESCE(SUM(total_net) FILTER (WHERE doc_date BETWEEN @start AND @end), 0) AS current,
			COALESCE(SUM(total_net) FILTER (WHERE doc_date BETWEEN @prev_start AND @prev_end), 0) AS previous,
			COUNT(*) FILTER (WHERE invoice AND doc_date BETWEEN @start AND @end) AS invoice_count
		FROM `+netSales+`
		WHERE doc_date BETWEEN @prev_start AND @end`, args).Scan(&sales).Error; err != nil {
		return out, err
	}
	out.SalesCurrent, out.SalesPrevious, out.InvoiceCount = sales.Current, sales.Previous, sales.InvoiceCount

	var months []struct {
		Month string
		Net   decimal.Decimal
	}
	if err := db.Raw(`SELECT to_char(doc_date, 'YYYY-MM') AS month, SUM(total_net) AS net
		FROM `+netSales+`
		WHERE doc_date BETWEEN @months_from AND @trend_end
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
	if p.FiscalYear > 0 {
		hasPrev := false
		prevYear := make([]dto.MonthNet, 0, dashperiod.TrendMonths)
		for _, key := range p.PreviousMonths() {
			v := byMonth[key].Add(decimal.Zero)
			hasPrev = hasPrev || !v.IsZero()
			prevYear = append(prevYear, dto.MonthNet{Month: key, Net: v})
		}
		if hasPrev {
			out.PreviousYear = prevYear
		}
	}

	var pipe struct {
		QuotationOpenCount  int64
		QuotationOpenAmount decimal.Decimal
		OrderDraftCount     int64
		OrderToDeliverCount int64
		OrderToInvoiceCount int64
	}
	if err := db.Raw(`SELECT
			q.open_count AS quotation_open_count, q.open_amount AS quotation_open_amount,
			(SELECT COUNT(*) FROM sales_orders WHERE company_id = @company AND status = 'draft') AS order_draft_count,
			(SELECT COUNT(*) FROM sales_orders AS o WHERE o.company_id = @company AND o.status = 'approved'
				AND EXISTS (SELECT 1 FROM sales_order_lines AS l WHERE l.order_id = o.id AND l.delivered_qty < l.quantity)) AS order_to_deliver_count,
			(SELECT COUNT(*) FROM sales_orders AS o WHERE o.company_id = @company AND o.status = 'approved'
				AND EXISTS (SELECT 1 FROM sales_order_lines AS l WHERE l.order_id = o.id AND l.invoiced_qty < l.quantity)) AS order_to_invoice_count
		FROM (SELECT COUNT(*) AS open_count, COALESCE(SUM(total_gross), 0) AS open_amount
		        FROM sales_quotations WHERE company_id = @company AND status = 'sent') AS q`, args).Scan(&pipe).Error; err != nil {
		return out, err
	}
	out.QuotationOpenCount, out.QuotationOpenAmount = pipe.QuotationOpenCount, pipe.QuotationOpenAmount
	out.OrderDraftCount, out.OrderToDeliverCount, out.OrderToInvoiceCount = pipe.OrderDraftCount, pipe.OrderToDeliverCount, pipe.OrderToInvoiceCount

	// ลูกหนี้ ณ end_date — สูตรเดียวกับรายงานอายุลูกหนี้ (ยอดรวม VAT - ยอดรับที่ลงวันที่ไม่เกิน end_date)
	var ar struct {
		Outstanding decimal.Decimal
		Overdue     decimal.Decimal
		NotDue      decimal.Decimal
		Days1To30   decimal.Decimal `gorm:"column:days_1_30"`
		Days31To60  decimal.Decimal `gorm:"column:days_31_60"`
		Days61To90  decimal.Decimal `gorm:"column:days_61_90"`
		Over90      decimal.Decimal `gorm:"column:over_90"`
	}
	if err := db.Raw(`SELECT
			COALESCE(SUM(i.open_amount), 0) AS outstanding,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age > 0), 0) AS overdue,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age <= 0), 0) AS not_due,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age BETWEEN 1 AND 30), 0) AS days_1_30,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age BETWEEN 31 AND 60), 0) AS days_31_60,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age BETWEEN 61 AND 90), 0) AS days_61_90,
			COALESCE(SUM(i.open_amount) FILTER (WHERE i.age > 90), 0) AS over_90
		FROM (SELECT CAST(@end AS date) - inv.due_date AS age, inv.total_gross - COALESCE(rc.amount, 0) AS open_amount
		        FROM sales_invoices AS inv
		        LEFT JOIN (SELECT st.invoice_id, SUM(st.amount) AS amount
		                     FROM sales_invoice_settlements AS st
		                    WHERE st.company_id = @company AND st.status = 'posted' AND st.settled_on <= @end
		                    GROUP BY st.invoice_id) AS rc ON rc.invoice_id = inv.id
		       WHERE inv.company_id = @company AND inv.status = 'posted' AND inv.doc_date <= @end) AS i
		WHERE i.open_amount <> 0`, args).Scan(&ar).Error; err != nil {
		return out, err
	}
	out.Receivables = dto.ReceivableSummary{Outstanding: ar.Outstanding, Overdue: ar.Overdue, NotDue: ar.NotDue,
		Days1To30: ar.Days1To30, Days31To60: ar.Days31To60, Days61To90: ar.Days61To90, Over90: ar.Over90}

	var top []struct {
		ID   uuid.UUID
		Code string
		Name string
		Net  decimal.Decimal
	}
	if err := db.Raw(`SELECT c.id, c.code, c.name, SUM(d.total_net) AS net
		FROM `+netSales+` JOIN sales_customers AS c ON c.id = d.customer_id
		WHERE d.doc_date BETWEEN @start AND @end
		GROUP BY c.id, c.code, c.name
		ORDER BY net DESC, c.code
		LIMIT 5`, args).Scan(&top).Error; err != nil {
		return out, err
	}
	out.TopCustomers = make([]dto.CustomerAmount, len(top))
	for i, t := range top {
		out.TopCustomers[i] = dto.CustomerAmount{Customer: dto.Ref{ID: t.ID.String(), Code: t.Code, Name: t.Name}, Net: t.Net}
	}
	return out, nil
}

// netSales: ยอดขายสุทธิ = ใบแจ้งหนี้ (บวก) + ใบลดหนี้ (ลบ) ที่ยังไม่ยกเลิก — invoice = true เฉพาะใบแจ้งหนี้ (นับจำนวนใบ)
// ใบรับเงินมัดจำไม่ใช่ยอดขาย (เป็นหนี้สิน) — ไม่นับ
const netSales = `(SELECT customer_id, doc_date, total_net, true AS invoice FROM sales_invoices
		WHERE company_id = @company AND status = 'posted' AND kind <> 'deposit'
	UNION ALL
	SELECT customer_id, doc_date, -total_net, false FROM sales_credit_notes WHERE company_id = @company AND status = 'posted') AS d`
