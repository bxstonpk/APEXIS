package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/inventory/application/dto"
	ports "apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	gp "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

// ---------- หน่วยนับ ----------

var _ ports.UomRepository = (*UomRepository)(nil)

type UomRepository struct{ db *gorm.DB }

func NewUomRepository(db *gorm.DB) *UomRepository { return &UomRepository{db: db} }

var uomTable = masterTable{table: "inventory_uoms", notFound: domain.ErrUomNotFound}

func (r *UomRepository) Create(ctx context.Context, u *entities.Uom) error {
	m := gp.UomToModel(u)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *UomRepository) Update(ctx context.Context, u *entities.Uom) error {
	m := gp.UomToModel(u)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "status"}, domain.ErrCodeExists, domain.ErrUomNotFound)
}

func (r *UomRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.UomID) error {
	return softDelete[models.UomModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrUomNotFound)
}

func (r *UomRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.UomID) (*entities.Uom, error) {
	m, err := findByID[models.UomModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrUomNotFound)
	if err != nil {
		return nil, err
	}
	return gp.UomToEntity(m), nil
}

func (r *UomRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.UomID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_items WHERE company_id = ? AND uom_id = ? AND deleted_at IS NULL"},
		uuidOf(companyID), uuidOf(id))
}

func (r *UomRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.UomID) (dto.Master, error) {
	return uomTable.get(ctx, r.db, uuidOf(companyID), uuidOf(id))
}

func (r *UomRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.MasterFilter) (listing.Result[dto.Master], error) {
	return uomTable.list(ctx, r.db, uuidOf(companyID), f)
}

// ---------- หมวดสินค้า ----------

var _ ports.CategoryRepository = (*CategoryRepository)(nil)

type CategoryRepository struct{ db *gorm.DB }

func NewCategoryRepository(db *gorm.DB) *CategoryRepository { return &CategoryRepository{db: db} }

var categoryTable = masterTable{table: "inventory_item_categories", notFound: domain.ErrCategoryNotFound}

func (r *CategoryRepository) Create(ctx context.Context, c *entities.Category) error {
	m := gp.CategoryToModel(c)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *CategoryRepository) Update(ctx context.Context, c *entities.Category) error {
	m := gp.CategoryToModel(c)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "status"}, domain.ErrCodeExists, domain.ErrCategoryNotFound)
}

func (r *CategoryRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.CategoryID) error {
	return softDelete[models.CategoryModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrCategoryNotFound)
}

func (r *CategoryRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CategoryID) (*entities.Category, error) {
	m, err := findByID[models.CategoryModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrCategoryNotFound)
	if err != nil {
		return nil, err
	}
	return gp.CategoryToEntity(m), nil
}

func (r *CategoryRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.CategoryID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_items WHERE company_id = ? AND category_id = ? AND deleted_at IS NULL"},
		uuidOf(companyID), uuidOf(id))
}

func (r *CategoryRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.CategoryID) (dto.Master, error) {
	return categoryTable.get(ctx, r.db, uuidOf(companyID), uuidOf(id))
}

func (r *CategoryRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.MasterFilter) (listing.Result[dto.Master], error) {
	return categoryTable.list(ctx, r.db, uuidOf(companyID), f)
}

// ---------- คลังสินค้า ----------

var _ ports.WarehouseRepository = (*WarehouseRepository)(nil)

type WarehouseRepository struct{ db *gorm.DB }

func NewWarehouseRepository(db *gorm.DB) *WarehouseRepository { return &WarehouseRepository{db: db} }

var warehouseTable = masterTable{table: "inventory_warehouses", notFound: domain.ErrWarehouseNotFound, hasBranch: true}

func (r *WarehouseRepository) Create(ctx context.Context, w *entities.Warehouse) error {
	m := gp.WarehouseToModel(w)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *WarehouseRepository) Update(ctx context.Context, w *entities.Warehouse) error {
	m := gp.WarehouseToModel(w)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "branch_id", "status"}, domain.ErrCodeExists, domain.ErrWarehouseNotFound)
}

func (r *WarehouseRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.WarehouseID) error {
	return softDelete[models.WarehouseModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrWarehouseNotFound)
}

func (r *WarehouseRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.WarehouseID) (*entities.Warehouse, error) {
	m, err := findByID[models.WarehouseModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrWarehouseNotFound)
	if err != nil {
		return nil, err
	}
	return gp.WarehouseToEntity(m), nil
}

func (r *WarehouseRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.WarehouseID) ([]*entities.Warehouse, error) {
	ms, err := findByIDs[models.WarehouseModel](ctx, r.db, uuidOf(companyID), uuids(ids))
	out := make([]*entities.Warehouse, len(ms))
	for i, m := range ms {
		out[i] = gp.WarehouseToEntity(m)
	}
	return out, err
}

// InUse: มีเอกสาร (ทุกสถานะ) ที่ใช้คลังนี้เป็นต้นทาง / ปลายทาง — query เดียว
func (r *WarehouseRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.WarehouseID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_documents WHERE company_id = ? AND (warehouse_id = ? OR to_warehouse_id = ?)"},
		uuidOf(companyID), uuidOf(id), uuidOf(id))
}

func (r *WarehouseRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.WarehouseID) (dto.Master, error) {
	return warehouseTable.get(ctx, r.db, uuidOf(companyID), uuidOf(id))
}

func (r *WarehouseRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.MasterFilter) (listing.Result[dto.Master], error) {
	return warehouseTable.list(ctx, r.db, uuidOf(companyID), f)
}

// ---------- สินค้า ----------

var _ ports.ItemRepository = (*ItemRepository)(nil)

type ItemRepository struct{ db *gorm.DB }

func NewItemRepository(db *gorm.DB) *ItemRepository { return &ItemRepository{db: db} }

func (r *ItemRepository) Create(ctx context.Context, i *entities.Item) error {
	m := gp.ItemToModel(i)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *ItemRepository) Update(ctx context.Context, i *entities.Item) error {
	m := gp.ItemToModel(i)
	return update(ctx, r.db, &m, m.CompanyID,
		[]string{"code", "name", "name_en", "type", "uom_id", "category_id", "reorder_point", "status"},
		domain.ErrCodeExists, domain.ErrItemNotFound)
}

func (r *ItemRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.ItemID) error {
	return softDelete[models.ItemModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrItemNotFound)
}

func (r *ItemRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.ItemID) (*entities.Item, error) {
	m, err := findByID[models.ItemModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrItemNotFound)
	if err != nil {
		return nil, err
	}
	return gp.ItemToEntity(m), nil
}

func (r *ItemRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) ([]*entities.Item, error) {
	ms, err := findByIDs[models.ItemModel](ctx, r.db, uuidOf(companyID), uuids(ids))
	out := make([]*entities.Item, len(ms))
	for i, m := range ms {
		out[i] = gp.ItemToEntity(m)
	}
	return out, err
}

// InUse: อยู่ในบรรทัดเอกสารใดๆ (การเคลื่อนไหวมาจากเอกสารเสมอ)
func (r *ItemRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.ItemID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_document_lines WHERE item_id = ?"}, uuidOf(id))
}

func (r *ItemRepository) HasMovements(ctx context.Context, companyID vo.CompanyID, id vo.ItemID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_movements WHERE company_id = ? AND item_id = ?"},
		uuidOf(companyID), uuidOf(id))
}

type itemRow struct {
	ID           uuid.UUID
	Code         string
	Name         string
	NameEN       string
	Type         string
	UomID        uuid.UUID
	UomCode      string
	UomName      string
	CategoryID   *uuid.UUID
	CategoryCode *string
	CategoryName *string
	ReorderPoint *decimal.Decimal
	Status       string
	Total        int64
}

func (r itemRow) dto() dto.Item {
	out := dto.Item{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, NameEN: r.NameEN, Type: r.Type,
		Uom: dto.Ref{ID: r.UomID.String(), Code: r.UomCode, Name: r.UomName}, ReorderPoint: r.ReorderPoint, Status: r.Status,
	}
	if r.CategoryID != nil && r.CategoryCode != nil {
		out.Category = &dto.Ref{ID: r.CategoryID.String(), Code: *r.CategoryCode, Name: *r.CategoryName}
	}
	return out
}

const itemColumns = `i.id, i.code, i.name, i.name_en, i.type, i.uom_id, u.code AS uom_code, u.name AS uom_name,
	i.category_id, c.code AS category_code, c.name AS category_name, i.reorder_point, i.status`

// base: items JOIN หน่วยนับ LEFT JOIN หมวด — 1 query ได้ชื่อของ record ที่อ้างถึง
func (r *ItemRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("inventory_items AS i").
		Joins("JOIN inventory_uoms AS u ON u.id = i.uom_id").
		Joins("LEFT JOIN inventory_item_categories AS c ON c.id = i.category_id AND c.deleted_at IS NULL").
		Where("i.company_id = ? AND i.deleted_at IS NULL", uuidOf(companyID))
}

func (r *ItemRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.ItemID) (dto.Item, error) {
	var rows []itemRow
	if err := r.base(ctx, companyID).Select(itemColumns).Where("i.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Item{}, err
	}
	if len(rows) == 0 {
		return dto.Item{}, domain.ErrItemNotFound
	}
	return rows[0].dto(), nil
}

func (r *ItemRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.ItemFilter) (listing.Result[dto.Item], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "i.code", "name": "i.name", "type": "i.type"}, "i.id")
	if err != nil {
		return listing.Result[dto.Item]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Types) > 0 {
			q = q.Where("i.type IN ?", f.Types)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("i.status IN ?", f.Statuses)
		}
		if f.CategoryID != nil {
			q = q.Where("i.category_id = ?", uuidOf(*f.CategoryID))
		}
		if f.UomID != nil {
			q = q.Where("i.uom_id = ?", uuidOf(*f.UomID))
		}
		return search(q, f.Search, "i.code", "i.name", "i.name_en")
	}
	rows, total, err := database.FetchPage(base, itemColumns, order, f.Page, func(r itemRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, itemRow.dto), err
}
