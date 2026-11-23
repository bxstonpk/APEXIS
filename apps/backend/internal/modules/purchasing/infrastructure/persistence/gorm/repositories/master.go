package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
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

// ---------- ค่าตั้ง ----------

var _ ports.SettingsRepository = (*SettingsRepository)(nil)

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

func (r *SettingsRepository) Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error) {
	var ms []models.SettingsModel
	if err := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID)).Limit(1).Find(&ms).Error; err != nil {
		return entities.Settings{}, err
	}
	if len(ms) == 0 {
		return entities.DefaultSettings(), nil
	}
	return gp.SettingsToEntity(ms[0]), nil
}

func (r *SettingsRepository) Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error {
	m := gp.SettingsToModel(companyID, s)
	m.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"vat_rate", "journal_id", "grir_account_id", "ap_account_id", "input_vat_account_id", "price_variance_account_id", "updated_at",
		}),
	}).Create(&m).Error
}

// ---------- ผู้ขาย ----------

var _ ports.SupplierRepository = (*SupplierRepository)(nil)

type SupplierRepository struct{ db *gorm.DB }

func NewSupplierRepository(db *gorm.DB) *SupplierRepository { return &SupplierRepository{db: db} }

func (r *SupplierRepository) Create(ctx context.Context, s *entities.Supplier) error {
	m := gp.SupplierToModel(s)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrCodeExists
		}
		return err
	}
	return nil
}

func (r *SupplierRepository) Update(ctx context.Context, s *entities.Supplier) error {
	m := gp.SupplierToModel(s)
	res := database.Conn(ctx, r.db).Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "tax_id", "branch_no", "address", "payment_term_days", "vat_registered", "tax_type", "status", "updated_at").
		Updates(&m)
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrCodeExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrSupplierNotFound
	}
	return nil
}

func (r *SupplierRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Delete(&models.SupplierModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrSupplierNotFound
	}
	return nil
}

func (r *SupplierRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (*entities.Supplier, error) {
	var m models.SupplierModel
	err := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrSupplierNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.SupplierToEntity(m), nil
}

// InUse: มีใบสั่งซื้อ (ทุกสถานะ) ของผู้ขายนี้
func (r *SupplierRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM purchasing_orders WHERE company_id = ? AND supplier_id = ?"}, uuidOf(companyID), uuidOf(id))
}

type supplierRow struct {
	ID              uuid.UUID
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	PaymentTermDays int
	VatRegistered   bool
	TaxType         string
	Status          string
	Total           int64
}

func (r supplierRow) dto() dto.Supplier {
	return dto.Supplier{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, TaxID: r.TaxID, BranchNo: r.BranchNo, Address: r.Address,
		PaymentTermDays: r.PaymentTermDays, VatRegistered: r.VatRegistered, TaxType: r.TaxType, Status: r.Status,
	}
}

const supplierColumns = "s.id, s.code, s.name, s.tax_id, s.branch_no, s.address, s.payment_term_days, s.vat_registered, s.tax_type, s.status"

func (r *SupplierRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("purchasing_suppliers AS s").Where("s.company_id = ? AND s.deleted_at IS NULL", uuidOf(companyID))
}

func (r *SupplierRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (dto.Supplier, error) {
	var rows []supplierRow
	if err := r.base(ctx, companyID).Select(supplierColumns).Where("s.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Supplier{}, err
	}
	if len(rows) == 0 {
		return dto.Supplier{}, domain.ErrSupplierNotFound
	}
	return rows[0].dto(), nil
}

func (r *SupplierRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.SupplierFilter) (listing.Result[dto.Supplier], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "s.code", "name": "s.name"}, "s.id")
	if err != nil {
		return listing.Result[dto.Supplier]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("s.status IN ?", f.Statuses)
		}
		if f.VatRegistered != nil {
			q = q.Where("s.vat_registered = ?", *f.VatRegistered)
		}
		return search(q, f.Search, "s.code", "s.name", "s.tax_id")
	}
	rows, total, err := database.FetchPage(base, supplierColumns, order, f.Page, func(r supplierRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, supplierRow.dto), err
}
