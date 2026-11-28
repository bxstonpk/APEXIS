package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

// ReportRepository คือรายงานของการผลิต — ทุกรายงานกรอง / แบ่งหน้าใน DB
// ยอดรวมทั้งรายงาน (ไม่ใช่แค่หน้าที่แสดง) ใช้ query รวมแยกอีก 1 ครั้ง (aggregation คนละระดับกับแถว)
type ReportRepository struct{ db *gorm.DB }

func NewReportRepository(db *gorm.DB) *ReportRepository { return &ReportRepository{db: db} }

const materialCost = `COALESCE((SELECT SUM(m.issued_cost - m.returned_cost) FROM manufacturing_order_materials AS m WHERE m.order_id = o.id), 0)`

func (r *ReportRepository) completed(ctx context.Context, companyID vo.CompanyID, f dto.CostFilter) *gorm.DB {
	q := database.Conn(ctx, r.db).Table("manufacturing_orders AS o").
		Where("o.company_id = ? AND o.status = ? AND o.completed_at BETWEEN ? AND ?", uuidOf(companyID), string(vo.OrderCompleted), f.Start, f.End)
	if f.ItemID != nil {
		q = q.Where("o.item_id = ?", uuidOf(*f.ItemID))
	}
	return q
}

type costRow struct {
	ID                uuid.UUID
	DocumentNo        string
	CompletedAt       time.Time
	ItemID            uuid.UUID
	CompletedQuantity decimal.Decimal
	Materials         decimal.Decimal
	ConversionApplied decimal.Decimal
	Total             int64
}

// ProductionCost: แถว 1 query (หน้า + จำนวน) + ยอดรวม 1 query
func (r *ReportRepository) ProductionCost(ctx context.Context, companyID vo.CompanyID, f dto.CostFilter) (dto.CostReport, error) {
	base := func() *gorm.DB { return r.completed(ctx, companyID, f) }
	rows, total, err := database.FetchPage(base,
		"o.id, o.document_no, o.completed_at, o.item_id, o.completed_quantity, "+materialCost+" AS materials, o.conversion_applied",
		"o.completed_at ASC, o.document_no ASC, o.id ASC", f.Page, func(r costRow) int64 { return r.Total })
	if err != nil {
		return dto.CostReport{}, err
	}
	out := dto.CostReport{Rows: toResult(rows, total, f.Page, func(x costRow) dto.CostRow {
		t := x.Materials.Add(x.ConversionApplied)
		unit := decimal.Zero
		if x.CompletedQuantity.IsPositive() {
			unit = t.DivRound(x.CompletedQuantity, 6)
		}
		return dto.CostRow{Order: dto.Ref{ID: x.ID.String(), Code: x.DocumentNo}, Date: x.CompletedAt, Item: dto.Ref{ID: x.ItemID.String()},
			Quantity: x.CompletedQuantity, Materials: x.Materials, Conversion: x.ConversionApplied, Total: t, UnitCost: unit}
	})}
	var sums struct {
		Materials  decimal.Decimal
		Conversion decimal.Decimal
	}
	if err := base().Select("COALESCE(SUM(" + materialCost + "), 0) AS materials, COALESCE(SUM(o.conversion_applied), 0) AS conversion").
		Scan(&sums).Error; err != nil {
		return out, err
	}
	out.Totals = dto.CostTotals{Materials: sums.Materials, Conversion: sums.Conversion, Total: sums.Materials.Add(sums.Conversion)}
	return out, nil
}

type usageRow struct {
	ID                uuid.UUID
	DocumentNo        string
	CompletedAt       time.Time
	ItemID            uuid.UUID
	MaterialID        uuid.UUID
	Quantity          decimal.Decimal
	CompletedQuantity decimal.Decimal
	RequiredQuantity  decimal.Decimal
	NetQuantity       decimal.Decimal
	NetCost           decimal.Decimal
	Total             int64
}

// MaterialUsage: query เดียว (ใบสั่งผลิต × วัตถุดิบ, หน้า + จำนวน)
// ตามแผน = ปริมาณที่ต้องใช้ × (จำนวนที่ผลิตได้ / จำนวนสั่งผลิต); ผลต่างมูลค่า = ผลต่างจำนวน × ต้นทุนเฉลี่ยที่เบิกจริง
func (r *ReportRepository) MaterialUsage(ctx context.Context, companyID vo.CompanyID, f dto.CostFilter) (listing.Result[dto.UsageRow], error) {
	base := func() *gorm.DB {
		return r.completed(ctx, companyID, f).Joins("JOIN manufacturing_order_materials AS m ON m.order_id = o.id")
	}
	rows, total, err := database.FetchPage(base,
		`o.id, o.document_no, o.completed_at, o.item_id, m.item_id AS material_id, o.quantity, o.completed_quantity, m.required_quantity,
		m.issued_quantity - m.returned_quantity AS net_quantity, m.issued_cost - m.returned_cost AS net_cost`,
		"o.completed_at ASC, o.document_no ASC, o.id ASC, m.line_no ASC", f.Page, func(r usageRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, func(x usageRow) dto.UsageRow {
		standard := decimal.Zero
		if x.Quantity.IsPositive() {
			standard = x.RequiredQuantity.Mul(x.CompletedQuantity).Div(x.Quantity).Round(int32(f.QuantityDecimals))
		}
		variance := x.NetQuantity.Sub(standard)
		value := decimal.Zero
		if x.NetQuantity.IsPositive() {
			value = variance.Mul(x.NetCost).Div(x.NetQuantity).Round(int32(f.AmountDecimals))
		}
		return dto.UsageRow{Order: dto.Ref{ID: x.ID.String(), Code: x.DocumentNo}, Date: x.CompletedAt, Item: dto.Ref{ID: x.ItemID.String()},
			Material: dto.Ref{ID: x.MaterialID.String()}, Standard: standard, Actual: x.NetQuantity, Variance: variance,
			ActualCost: x.NetCost, VarianceValue: value}
	}), err
}

type wipRow struct {
	ID         uuid.UUID
	DocumentNo string
	OrderDate  time.Time
	ItemID     uuid.UUID
	Quantity   decimal.Decimal
	Materials  decimal.Decimal
	Total      int64
}

// วัตถุดิบสุทธิ ณ วันที่ = เบิก − คืน ที่ยังไม่ยกเลิกและลงวันที่ไม่เกินวันที่รายงาน
const wipMaterials = `COALESCE((SELECT SUM(CASE WHEN mv.kind = 'issue' THEN mv.value ELSE -mv.value END) FROM manufacturing_movements AS mv
	WHERE mv.order_id = o.id AND mv.kind IN ('issue','return') AND NOT mv.cancelled AND mv.movement_date <= ?), 0)`

// Wip: ใบสั่งผลิตที่ปล่อยผลิตแล้วแต่ยังไม่ปิดงาน ณ วันที่ — แถว 1 query + ยอดรวม 1 query
func (r *ReportRepository) Wip(ctx context.Context, companyID vo.CompanyID, f dto.WipFilter) (dto.WipReport, error) {
	base := func() *gorm.DB {
		return database.Conn(ctx, r.db).Table("manufacturing_orders AS o").
			Where("o.company_id = ? AND o.order_date <= ? AND o.status IN ?", uuidOf(companyID), f.AsOf,
				[]string{string(vo.OrderReleased), string(vo.OrderCompleted)}).
			Where("(o.completed_at IS NULL OR o.completed_at > ?)", f.AsOf)
	}
	var rows []wipRow
	err := base().Select("o.id, o.document_no, o.order_date, o.item_id, o.quantity, "+wipMaterials+" AS materials, COUNT(*) OVER() AS total", f.AsOf).
		Order("o.order_date ASC, o.document_no ASC, o.id ASC").Limit(f.Page.Limit()).Offset(f.Page.Offset()).Scan(&rows).Error
	if err != nil {
		return dto.WipReport{}, err
	}
	var count int64
	if len(rows) > 0 {
		count = rows[0].Total
	} else if f.Page.Offset() > 0 {
		if err := base().Count(&count).Error; err != nil {
			return dto.WipReport{}, err
		}
	}
	out := dto.WipReport{Rows: toResult(rows, count, f.Page, func(x wipRow) dto.WipRow {
		return dto.WipRow{Order: dto.Ref{ID: x.ID.String(), Code: x.DocumentNo}, Date: x.OrderDate, Item: dto.Ref{ID: x.ItemID.String()},
			Quantity: x.Quantity, Materials: x.Materials}
	})}
	var sum decimal.Decimal
	if err := base().Select("COALESCE(SUM("+wipMaterials+"), 0)", f.AsOf).Scan(&sum).Error; err != nil {
		return out, err
	}
	out.Total = sum
	return out, nil
}
