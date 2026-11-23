package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/application/dto"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/dashperiod"
	"apexis/backend/internal/platform/database"
)

// Dashboard — 4 query แบบ aggregate (ไม่มี N+1, ไม่ใช้ transaction):
//  1. มูลค่าคงเหลือต่อคลัง ณ end_date (มูลค่ารวม = ผลรวมของทุกคลัง)  2. สินค้าต่ำกว่าจุดสั่งซื้อ (+ จำนวนทั้งหมดด้วย window function)
//  3. จำนวนสินค้านับสต็อก / เอกสารรับ / จ่ายในช่วง (sub-select ใน query เดียว)  4. มูลค่าเข้า / ออกรายเดือน 12 เดือน
func (r *StockRepository) Dashboard(ctx context.Context, companyID vo.CompanyID, p dashperiod.Period) (dto.Dashboard, error) {
	db := database.Conn(ctx, r.db)
	args := map[string]any{"company": uuidOf(companyID), "start": p.Start, "end": p.End, "trend_start": p.TrendStart(), "trend_end": p.TrendEnd()}
	out := dto.Dashboard{Period: p, StockValue: decimal.Zero}

	var whs []struct {
		ID    uuid.UUID
		Code  string
		Name  string
		Value decimal.Decimal
	}
	if err := db.Raw(`SELECT w.id, w.code, w.name, SUM(m.value) AS value
		FROM inventory_movements AS m JOIN inventory_warehouses AS w ON w.id = m.warehouse_id
		WHERE m.company_id = @company AND m.movement_date <= @end
		GROUP BY w.id, w.code, w.name
		HAVING SUM(m.value) <> 0
		ORDER BY value DESC, w.code`, args).Scan(&whs).Error; err != nil {
		return out, err
	}
	out.Warehouses = make([]dto.WarehouseValue, len(whs))
	for i, w := range whs {
		out.Warehouses[i] = dto.WarehouseValue{Warehouse: dto.Ref{ID: w.ID.String(), Code: w.Code, Name: w.Name}, Value: w.Value}
		out.StockValue = out.StockValue.Add(w.Value)
	}

	var low []struct {
		ID           uuid.UUID
		Code         string
		Name         string
		OnHand       decimal.Decimal
		ReorderPoint decimal.Decimal
		Total        int64
	}
	if err := db.Raw(`SELECT i.id, i.code, i.name, COALESCE(o.qty, 0) AS on_hand, i.reorder_point, COUNT(*) OVER () AS total
		FROM inventory_items AS i
		LEFT JOIN (SELECT item_id, SUM(quantity) AS qty FROM inventory_movements
		            WHERE company_id = @company AND movement_date <= @end GROUP BY item_id) AS o ON o.item_id = i.id
		WHERE i.company_id = @company AND i.deleted_at IS NULL AND i.status = 'active' AND i.type = 'stock'
		  AND i.reorder_point IS NOT NULL AND COALESCE(o.qty, 0) <= i.reorder_point
		ORDER BY i.reorder_point - COALESCE(o.qty, 0) DESC, i.code
		LIMIT 10`, args).Scan(&low).Error; err != nil {
		return out, err
	}
	out.BelowReorder = make([]dto.BelowReorder, len(low))
	for i, x := range low {
		out.BelowReorder[i] = dto.BelowReorder{Item: dto.Ref{ID: x.ID.String(), Code: x.Code, Name: x.Name}, OnHand: x.OnHand, ReorderPoint: x.ReorderPoint}
		out.BelowReorderCount = x.Total
	}

	var counts struct {
		StockItemCount int64
		ReceiptCount   int64
		IssueCount     int64
	}
	if err := db.Raw(`SELECT
			(SELECT COUNT(*) FROM inventory_items WHERE company_id = @company AND deleted_at IS NULL AND status = 'active' AND type = 'stock') AS stock_item_count,
			d.receipt_count, d.issue_count
		FROM (SELECT COUNT(*) FILTER (WHERE doc_type = 'receipt') AS receipt_count, COUNT(*) FILTER (WHERE doc_type = 'issue') AS issue_count
		        FROM inventory_documents
		       WHERE company_id = @company AND status = 'posted' AND doc_date BETWEEN @start AND @end) AS d`, args).Scan(&counts).Error; err != nil {
		return out, err
	}
	out.StockItemCount, out.ReceiptCount, out.IssueCount = counts.StockItemCount, counts.ReceiptCount, counts.IssueCount

	var months []struct {
		Month    string
		InValue  decimal.Decimal
		OutValue decimal.Decimal
	}
	if err := db.Raw(`SELECT to_char(m.movement_date, 'YYYY-MM') AS month,
			COALESCE(SUM(m.value) FILTER (WHERE m.quantity > 0), 0) AS in_value,
			COALESCE(-SUM(m.value) FILTER (WHERE m.quantity < 0), 0) AS out_value
		FROM inventory_movements AS m JOIN inventory_documents AS d ON d.id = m.document_id
		WHERE m.company_id = @company AND m.movement_date BETWEEN @trend_start AND @trend_end
		  AND d.status = 'posted' AND d.doc_type <> 'transfer'
		GROUP BY 1`, args).Scan(&months).Error; err != nil {
		return out, err
	}
	byMonth := make(map[string]dto.MonthFlow, len(months))
	for _, m := range months {
		byMonth[m.Month] = dto.MonthFlow{Month: m.Month, InValue: m.InValue, OutValue: m.OutValue}
	}
	for _, key := range p.Months() {
		f, ok := byMonth[key]
		if !ok {
			f = dto.MonthFlow{Month: key, InValue: decimal.Zero, OutValue: decimal.Zero}
		}
		out.Monthly = append(out.Monthly, f)
	}
	return out, nil
}
