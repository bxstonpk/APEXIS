package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/purchasing/application/dto"
	ports "apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.OrderRepository = (*OrderRepository)(nil)

type OrderRepository struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) *OrderRepository { return &OrderRepository{db: db} }

// Create — 2 statement (หัว + บรรทัดแบบ batch)
func (r *OrderRepository) Create(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModel(o)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

// Update แทนที่ draft: แก้หัว (เฉพาะ draft) -> ลบบรรทัดเดิม -> insert บรรทัดใหม่
func (r *OrderRepository) Update(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModel(o)
	res := tx.Model(&models.OrderModel{}).
		Where("company_id = ? AND id = ? AND status = ?", m.CompanyID, m.ID, vo.OrderDraft).
		Updates(map[string]any{
			"supplier_id": m.SupplierID, "order_date": m.OrderDate, "delivery_date": m.DeliveryDate, "reference": m.Reference,
			"description": m.Description, "vat_registered": m.VatRegistered, "vat_rate": m.VatRate,
			"price_includes_vat": m.PriceIncludesVat, "payment_term_days": m.PaymentTermDays,
			"total_net": m.TotalNet, "total_vat": m.TotalVat, "total_gross": m.TotalGross, "updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrderNotDraft
	}
	if err := tx.Where("order_id = ?", m.ID).Delete(&models.OrderLineModel{}).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

// SaveProgress — 2 statement: หัว (สถานะ / อนุมัติ / ปิด / ยกเลิก) + ความคืบหน้าของทุกบรรทัด (UPDATE ... FROM (VALUES ...))
func (r *OrderRepository) SaveProgress(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModel(o)
	res := tx.Model(&models.OrderModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{
			"status": m.Status, "order_no": m.OrderNo, "fiscal_year": m.FiscalYear, "approved_by": m.ApprovedBy, "approved_at": m.ApprovedAt,
			"closed_by": m.ClosedBy, "closed_at": m.ClosedAt, "cancelled_by": m.CancelledBy, "cancelled_at": m.CancelledAt,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrderNotFound
	}
	values := make([]string, len(m.Lines))
	args := make([]any, 0, 5*len(m.Lines)+1)
	for i, l := range m.Lines {
		values[i] = "(?::int, ?::numeric, ?::numeric, ?::numeric, ?::numeric)"
		args = append(args, l.LineNo, l.ReceivedQty, l.InvoicedQty, l.ReceivedValue, l.ClearedValue)
	}
	args = append(args, m.ID)
	return tx.Exec(`UPDATE purchasing_order_lines AS l
		SET received_qty = v.received_qty, invoiced_qty = v.invoiced_qty, received_value = v.received_value, cleared_value = v.cleared_value
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(line_no, received_qty, invoiced_qty, received_value, cleared_value)
		WHERE l.line_no = v.line_no AND l.order_id = ?`, args...).Error
}

func (r *OrderRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ? AND status = ?", uuidOf(companyID), uuidOf(id), vo.OrderDraft).
		Delete(&models.OrderModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

// FindForUpdate — 2 query: lock หัว (FOR UPDATE) แล้วโหลดบรรทัด
func (r *OrderRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (*entities.Order, error) {
	db := database.Conn(ctx, r.db)
	var m models.OrderModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("order_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.OrderToEntity(m), nil
}

type orderRow struct {
	ID               uuid.UUID
	OrderNo          *string
	OrderDate        time.Time
	DeliveryDate     *time.Time
	SupplierID       uuid.UUID
	SupplierCode     string
	SupplierName     string
	Reference        string
	Description      string
	VatRegistered    bool
	VatRate          decimal.Decimal
	PriceIncludesVat bool
	PaymentTermDays  int
	TotalNet         decimal.Decimal
	TotalVat         decimal.Decimal
	TotalGross       decimal.Decimal
	Status           string
	CreatedBy        uuid.UUID
	ApprovedBy       *uuid.UUID
	ApprovedAt       *time.Time
	ClosedAt         *time.Time
	CancelledAt      *time.Time
	Total            int64
}

func dateOpt(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := gp.DateUTC(*t)
	return &d
}

func (r orderRow) summary() dto.OrderSummary {
	out := dto.OrderSummary{
		ID: r.ID.String(), Date: gp.DateUTC(r.OrderDate), DeliveryDate: dateOpt(r.DeliveryDate),
		Supplier: dto.Ref{ID: r.SupplierID.String(), Code: r.SupplierCode, Name: r.SupplierName}, Reference: r.Reference,
		VatRate: r.VatRate, PriceIncludesVat: r.PriceIncludesVat,
		Totals: dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross}, Status: r.Status,
	}
	if r.OrderNo != nil {
		out.OrderNo = *r.OrderNo
	}
	return out
}

const orderSummaryColumns = `o.id, o.order_no, o.order_date, o.delivery_date, o.supplier_id, s.code AS supplier_code, s.name AS supplier_name,
	o.reference, o.vat_rate, o.price_includes_vat, o.total_net, o.total_vat, o.total_gross, o.status`

const orderColumns = orderSummaryColumns + `, o.description, o.vat_registered, o.payment_term_days, o.created_by,
	o.approved_by, o.approved_at, o.closed_at, o.cancelled_at`

// base: orders JOIN ผู้ขาย (รวมผู้ขายที่ถูก soft delete — เอกสารเดิมต้องแสดงชื่อได้)
func (r *OrderRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("purchasing_orders AS o").
		Joins("JOIN purchasing_suppliers AS s ON s.id = o.supplier_id").
		Where("o.company_id = ?", uuidOf(companyID))
}

// Get — 2 query: หัว + ผู้ขาย (JOIN), บรรทัด (one-to-many แยก query เพื่อไม่ทำหัวซ้ำทุกบรรทัด)
func (r *OrderRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (dto.Order, error) {
	var rows []orderRow
	if err := r.base(ctx, companyID).Select(orderColumns).Where("o.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Order{}, err
	}
	if len(rows) == 0 {
		return dto.Order{}, domain.ErrOrderNotFound
	}
	h := rows[0]
	out := dto.Order{
		OrderSummary: h.summary(), Description: h.Description, VatRegistered: h.VatRegistered, PaymentTermDays: h.PaymentTermDays,
		CreatedBy: h.CreatedBy.String(), ApprovedBy: optString(h.ApprovedBy), ApprovedAt: h.ApprovedAt, ClosedAt: h.ClosedAt,
		CancelledAt: h.CancelledAt,
	}
	var lines []models.OrderLineModel
	if err := database.Conn(ctx, r.db).Where("order_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Order{}, err
	}
	out.Lines = make([]dto.OrderLine, len(lines))
	for i, l := range lines {
		out.Lines[i] = dto.OrderLine{
			LineNo: l.LineNo, Item: dto.Ref{ID: l.ItemID.String()}, Description: l.Description, Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross},
			Received: l.ReceivedQty, Invoiced: l.InvoicedQty,
		}
	}
	return out, nil
}

func (r *OrderRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.OrderSummary], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "o.order_date", "order_no": "o.order_no", "created_at": "o.created_at"}, "o.id")
	if err != nil {
		return listing.Result[dto.OrderSummary]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("o.status IN ?", f.Statuses)
		}
		if f.SupplierID != nil {
			q = q.Where("o.supplier_id = ?", uuidOf(*f.SupplierID))
		}
		if f.ItemID != nil {
			q = q.Where("EXISTS (SELECT 1 FROM purchasing_order_lines AS l WHERE l.order_id = o.id AND l.item_id = ?)", uuidOf(*f.ItemID))
		}
		if f.StartDate != nil {
			q = q.Where("o.order_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("o.order_date <= ?", *f.EndDate)
		}
		return search(q, f.Search, "o.order_no", "o.reference", "o.description", "s.code", "s.name")
	}
	rows, total, err := database.FetchPage(base, orderSummaryColumns, order, f.Page, func(r orderRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, orderRow.summary), err
}
