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

// ---------- ค่าตั้ง ----------

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

func (r *SettingsRepository) Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error) {
	var m models.SettingsModel
	err := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID)).Take(&m).Error
	if database.IsNotFound(err) {
		return entities.Settings{}, nil
	}
	if err != nil {
		return entities.Settings{}, err
	}
	return gp.SettingsToEntity(m), nil
}

// Save — UPSERT statement เดียว
func (r *SettingsRepository) Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error {
	m := gp.SettingsToModel(companyID, s)
	m.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"journal_id", "wip_account_id", "conversion_account_id", "default_warehouse_id", "updated_at"}),
	}).Create(&m).Error
}

// ---------- สูตรการผลิต ----------

type BomRepository struct{ db *gorm.DB }

func NewBomRepository(db *gorm.DB) *BomRepository { return &BomRepository{db: db} }

func bomUniqueErr(err error) error {
	if database.IsUniqueViolation(err) {
		if database.UniqueConstraint(err) == "uq_mfg_boms_default" {
			return domain.ErrDefaultBomExists
		}
		return domain.ErrCodeExists
	}
	return err
}

// Create: INSERT หัว 1 + บรรทัดวัตถุดิบ 1 + บรรทัดค่าแรง 1 (batch)
func (r *BomRepository) Create(ctx context.Context, b *entities.Bom) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m, components, conversions := gp.BomToModels(b)
	if err := tx.Create(&m).Error; err != nil {
		return bomUniqueErr(err)
	}
	return r.saveLines(tx, m.ID, components, conversions)
}

// Update: UPDATE หัว 1 + DELETE / INSERT บรรทัด (แทนที่ทั้งชุด)
func (r *BomRepository) Update(ctx context.Context, b *entities.Bom) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m, components, conversions := gp.BomToModels(b)
	res := tx.Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "item_id", "output_quantity", "status", "is_default", "updated_at").Updates(&m)
	if res.Error != nil {
		return bomUniqueErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrBomNotFound
	}
	if err := tx.Where("bom_id = ?", m.ID).Delete(&models.BomComponentModel{}).Error; err != nil {
		return err
	}
	if err := tx.Where("bom_id = ?", m.ID).Delete(&models.BomConversionModel{}).Error; err != nil {
		return err
	}
	return r.saveLines(tx, m.ID, components, conversions)
}

func (r *BomRepository) saveLines(tx *gorm.DB, _ uuid.UUID, components []models.BomComponentModel, conversions []models.BomConversionModel) error {
	if len(components) > 0 {
		if err := tx.Create(&components).Error; err != nil {
			return err
		}
	}
	if len(conversions) > 0 {
		return tx.Create(&conversions).Error
	}
	return nil
}

func (r *BomRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.BomID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Delete(&models.BomModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrBomNotFound
	}
	return nil
}

// FindByID: หัว 1 + วัตถุดิบ 1 + ค่าแรง 1 query
func (r *BomRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.BomID) (*entities.Bom, error) {
	db := database.Conn(ctx, r.db)
	var m models.BomModel
	err := db.Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrBomNotFound
	}
	if err != nil {
		return nil, err
	}
	var components []models.BomComponentModel
	if err := db.Where("bom_id = ?", m.ID).Order("line_no").Find(&components).Error; err != nil {
		return nil, err
	}
	var conversions []models.BomConversionModel
	if err := db.Where("bom_id = ?", m.ID).Order("line_no").Find(&conversions).Error; err != nil {
		return nil, err
	}
	return gp.BomToEntity(m, components, conversions), nil
}

var bomSortColumns = map[string]string{"code": "b.code", "name": "b.name", "updated_at": "b.updated_at"}

type bomRow struct {
	ID             uuid.UUID
	Code           string
	Name           string
	ItemID         uuid.UUID
	OutputQuantity decimal.Decimal
	Status         string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Total          int64
}

func (r bomRow) dto() dto.BomRow {
	return dto.BomRow{ID: r.ID.String(), Code: r.Code, Name: r.Name, Item: dto.Ref{ID: r.ItemID.String()}, OutputQuantity: r.OutputQuantity,
		Status: vo.Status(r.Status), IsDefault: r.IsDefault, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

// List: query เดียว (COUNT(*) OVER()) — กรอง / เรียง / แบ่งหน้าใน DB
func (r *BomRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.BomFilter) (listing.Result[dto.BomRow], error) {
	order, err := database.OrderBy(f.Sort, bomSortColumns, "b.id")
	if err != nil {
		return listing.Result[dto.BomRow]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("manufacturing_boms AS b").Where("b.company_id = ? AND b.deleted_at IS NULL", uuidOf(companyID))
		q = search(q, f.Search, "b.code", "b.name")
		if f.ItemID != nil {
			q = q.Where("b.item_id = ?", uuidOf(*f.ItemID))
		}
		if len(f.Status) > 0 {
			q = q.Where("b.status IN ?", strs(f.Status))
		}
		return q
	}
	rows, total, err := database.FetchPage(base, "b.id, b.code, b.name, b.item_id, b.output_quantity, b.status, b.is_default, b.created_at, b.updated_at",
		order, f.Page, func(r bomRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, bomRow.dto), err
}

func (r *BomRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.BomID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).Raw(`SELECT EXISTS (SELECT 1 FROM manufacturing_orders WHERE company_id = ? AND bom_id = ?)`,
		uuidOf(companyID), uuidOf(id)).Scan(&ok).Error
	return ok, err
}

// Refs: รหัส / ชื่อสูตร (รวมที่ลบแล้ว) — query เดียว
func (r *BomRepository) Refs(ctx context.Context, companyID vo.CompanyID, ids []vo.BomID) (map[vo.BomID]dto.Ref, error) {
	out := map[vo.BomID]dto.Ref{}
	if len(ids) == 0 {
		return out, nil
	}
	in := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		in[i] = uuidOf(id)
	}
	var rows []struct {
		ID   uuid.UUID
		Code string
		Name string
	}
	err := database.Conn(ctx, r.db).Unscoped().Model(&models.BomModel{}).Select("id, code, name").
		Where("company_id = ? AND id IN ?", uuidOf(companyID), in).Scan(&rows).Error
	for _, row := range rows {
		out[vo.BomID(row.ID)] = dto.Ref{ID: row.ID.String(), Code: row.Code, Name: row.Name}
	}
	return out, err
}
