package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/domain/entities"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	gp "apexis/backend/internal/modules/manufacturing/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/manufacturing/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

type OrderRepository struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) *OrderRepository { return &OrderRepository{db: db} }

// Create: INSERT หัว 1 + วัตถุดิบ 1 + ค่าแรง 1 (batch)
func (r *OrderRepository) Create(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModels(o)
	m.Order.UpdatedAt = time.Now().UTC()
	if err := tx.Create(&m.Order).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrDocumentNoUsed
		}
		return err
	}
	return saveChildren(tx, m)
}

// Save: UPDATE หัว 1 + แทนที่วัตถุดิบ / ค่าแรง / การเคลื่อนไหวทั้งชุด (DELETE 3 + INSERT batch)
func (r *OrderRepository) Save(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModels(o)
	m.Order.UpdatedAt = time.Now().UTC()
	res := tx.Model(&m.Order).Where("company_id = ?", m.Order.CompanyID).
		Select("order_date", "item_id", "bom_id", "quantity", "output_quantity", "warehouse_id", "planned_start", "planned_end", "status",
			"description", "completed_quantity", "completed_at", "conversion_applied", "cancelled_at", "cancel_reason", "updated_at").
		Updates(&m.Order)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrderNotFound
	}
	for _, child := range []any{&models.OrderMaterialModel{}, &models.OrderConversionModel{}, &models.MovementModel{}} {
		if err := tx.Where("order_id = ?", m.Order.ID).Delete(child).Error; err != nil {
			return err
		}
	}
	return saveChildren(tx, m)
}

func saveChildren(tx *gorm.DB, m gp.OrderModels) error {
	if len(m.Materials) > 0 {
		if err := tx.Create(&m.Materials).Error; err != nil {
			return err
		}
	}
	if len(m.Conversions) > 0 {
		if err := tx.Create(&m.Conversions).Error; err != nil {
			return err
		}
	}
	if len(m.Movements) > 0 {
		return tx.Create(&m.Movements).Error
	}
	return nil
}

// FindByID: หัว 1 (+ FOR UPDATE) + วัตถุดิบ 1 + ค่าแรง 1 + การเคลื่อนไหว 1 query
func (r *OrderRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.OrderID, forUpdate bool) (*entities.Order, error) {
	db := database.Conn(ctx, r.db)
	q := db.Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id))
	if forUpdate {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var m gp.OrderModels
	err := q.Take(&m.Order).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("order_id = ?", m.Order.ID).Order("line_no").Find(&m.Materials).Error; err != nil {
		return nil, err
	}
	if err := db.Where("order_id = ?", m.Order.ID).Order("line_no").Find(&m.Conversions).Error; err != nil {
		return nil, err
	}
	if err := db.Where("order_id = ?", m.Order.ID).Order("position").Find(&m.Movements).Error; err != nil {
		return nil, err
	}
	return gp.OrderToEntity(m), nil
}

var orderSortColumns = map[string]string{"date": "o.order_date", "document_no": "o.document_no", "created_at": "o.created_at"}

type orderRow struct {
	ID                uuid.UUID
	DocumentNo        string
	OrderDate         time.Time
	ItemID            uuid.UUID
	BomID             uuid.UUID
	BomCode           string
	BomName           string
	Quantity          decimal.Decimal
	WarehouseID       uuid.UUID
	PlannedStart      *time.Time
	PlannedEnd        *time.Time
	Status            string
	CompletedQuantity decimal.Decimal
	CompletedAt       *time.Time
	TotalCost         decimal.Decimal
	CreatedAt         time.Time
	Total             int64
}

func (r orderRow) dto() dto.OrderRow {
	return dto.OrderRow{ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: r.OrderDate, Item: dto.Ref{ID: r.ItemID.String()},
		Bom: dto.Ref{ID: r.BomID.String(), Code: r.BomCode, Name: r.BomName}, Quantity: r.Quantity, Warehouse: dto.Ref{ID: r.WarehouseID.String()},
		PlannedStart: r.PlannedStart, PlannedEnd: r.PlannedEnd, Status: vo.OrderStatus(r.Status), CompletedQuantity: r.CompletedQuantity,
		CompletedAt: r.CompletedAt, TotalCost: r.TotalCost, CreatedAt: r.CreatedAt}
}

// ต้นทุนรวม = วัตถุดิบสุทธิ (subquery ตาม PK ของวัตถุดิบ order_id, line_no) + ค่าแรงที่ปันแล้ว
const orderColumns = `o.id, o.document_no, o.order_date, o.item_id, o.bom_id, b.code AS bom_code, b.name AS bom_name, o.quantity,
	o.warehouse_id, o.planned_start, o.planned_end, o.status, o.completed_quantity, o.completed_at, o.created_at,
	COALESCE((SELECT SUM(m.issued_cost - m.returned_cost) FROM manufacturing_order_materials AS m WHERE m.order_id = o.id), 0)
		+ o.conversion_applied AS total_cost`

// List: query เดียว (JOIN สูตร + ต้นทุนวัตถุดิบรวม, COUNT(*) OVER()) — กรอง / เรียง / แบ่งหน้าใน DB
func (r *OrderRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.OrderFilter) (listing.Result[dto.OrderRow], error) {
	order, err := database.OrderBy(f.Sort, orderSortColumns, "o.id")
	if err != nil {
		return listing.Result[dto.OrderRow]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("manufacturing_orders AS o").
			Joins("JOIN manufacturing_boms AS b ON b.id = o.bom_id").
			Where("o.company_id = ?", uuidOf(companyID))
		q = search(q, f.Search, "o.document_no", "o.description", "b.code", "b.name")
		if len(f.Status) > 0 {
			q = q.Where("o.status IN ?", strs(f.Status))
		}
		if f.ItemID != nil {
			q = q.Where("o.item_id = ?", uuidOf(*f.ItemID))
		}
		if f.From != nil {
			q = q.Where("o.order_date >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("o.order_date <= ?", *f.To)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, orderColumns, order, f.Page, func(r orderRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, orderRow.dto), err
}
