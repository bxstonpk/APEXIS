package repositories

import (
	"apexis/backend/internal/platform/thaiaddress"
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
			"vat_rate", "journal_id", "grir_account_id", "ap_account_id", "input_vat_account_id", "price_variance_account_id", "purchase_returns_account_id", "undue_input_vat_account_id", "service_expense_account_id", "updated_at",
		}),
	}).Create(&m).Error
}

// ---------- ผู้ขาย ----------

var _ ports.SupplierRepository = (*SupplierRepository)(nil)

type SupplierRepository struct{ db *gorm.DB }

func NewSupplierRepository(db *gorm.DB) *SupplierRepository { return &SupplierRepository{db: db} }

// TaxIDIndex คือ unique index "เลขผู้เสียภาษี 1 เลข = ผู้ขาย 1 รหัส" (สร้างใน Migrate หลังรวมรหัสซ้ำ)
const TaxIDIndex = "uq_pur_suppliers_tax_id"

func supplierUniqueErr(err error) error {
	if !database.IsUniqueViolation(err) {
		return err
	}
	if database.UniqueConstraint(err) == TaxIDIndex {
		return domain.ErrTaxIDExists
	}
	return domain.ErrCodeExists
}

func (r *SupplierRepository) Create(ctx context.Context, s *entities.Supplier) error {
	m := gp.SupplierToModel(s)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		return supplierUniqueErr(err)
	}
	return r.saveBranches(ctx, s)
}

// saveBranches แทนที่สาขาเพิ่มเติมทั้งชุด (ภายใน transaction ของ use case) — เอกสารอ้างสาขาด้วยเลขสาขา จึงไม่มี FK ไปที่สาขา
func (r *SupplierRepository) saveBranches(ctx context.Context, s *entities.Supplier) error {
	conn := database.Conn(ctx, r.db)
	if err := conn.Where("supplier_id = ?", uuidOf(s.ID())).Delete(&models.SupplierBranchModel{}).Error; err != nil {
		return err
	}
	ms := gp.SupplierBranchesToModel(s)
	if len(ms) == 0 {
		return nil
	}
	return conn.CreateInBatches(&ms, 200).Error
}

func (r *SupplierRepository) branches(ctx context.Context, id uuid.UUID) ([]models.SupplierBranchModel, error) {
	var ms []models.SupplierBranchModel
	err := database.Conn(ctx, r.db).Where("supplier_id = ?", id).Order("position, branch_no").Find(&ms).Error
	return ms, err
}

// TaxIDOwner: รหัสผู้ขาย (ที่ยังไม่ถูกลบ) ที่ใช้เลขผู้เสียภาษีนี้อยู่ นอกจาก except — ไม่มี = ""
func (r *SupplierRepository) TaxIDOwner(ctx context.Context, companyID vo.CompanyID, taxID string, except *vo.SupplierID) (string, error) {
	if taxID == "" {
		return "", nil
	}
	q := database.Conn(ctx, r.db).Model(&models.SupplierModel{}).Select("code").
		Where("company_id = ? AND tax_id = ?", uuidOf(companyID), taxID)
	if except != nil {
		q = q.Where("id <> ?", uuidOf(*except))
	}
	var codes []string
	if err := q.Limit(1).Pluck("code", &codes).Error; err != nil || len(codes) == 0 {
		return "", err
	}
	return codes[0], nil
}

func (r *SupplierRepository) Update(ctx context.Context, s *entities.Supplier) error {
	m := gp.SupplierToModel(s)
	res := database.Conn(ctx, r.db).Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "tax_id", "branch_no", "address", "addr_building", "addr_room_no", "addr_floor_no", "addr_village", "addr_house_no", "addr_moo", "addr_soi", "addr_yaek", "addr_street", "addr_sub_district", "addr_district", "addr_province", "addr_postal_code", "payment_term_days", "vat_registered", "tax_type", "status", "updated_at").
		Updates(&m)
	if res.Error != nil {
		return supplierUniqueErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrSupplierNotFound
	}
	return r.saveBranches(ctx, s)
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
	branches, err := r.branches(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	return gp.SupplierToEntity(m, branches...), nil
}

// InUse: มีใบสั่งซื้อ (ทุกสถานะ) หรือใบลดหนี้ของผู้ขายนี้
func (r *SupplierRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (bool, error) {
	return exists(ctx, r.db, []string{
		"SELECT 1 FROM purchasing_orders WHERE company_id = ? AND supplier_id = ?",
		"SELECT 1 FROM purchasing_credit_notes WHERE company_id = ? AND supplier_id = ?",
	}, uuidOf(companyID), uuidOf(id), uuidOf(companyID), uuidOf(id))
}

type supplierRow struct {
	ID              uuid.UUID
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	AddressDetail   models.AddressColumns `gorm:"embedded;embeddedPrefix:addr_"`
	PaymentTermDays int
	VatRegistered   bool
	TaxType         string
	Status          string
	Total           int64
}

func (r supplierRow) dto() dto.Supplier {
	return dto.Supplier{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, TaxID: r.TaxID, BranchNo: r.BranchNo, Address: r.Address, AddressDetail: thaiaddress.Address(r.AddressDetail),
		PaymentTermDays: r.PaymentTermDays, VatRegistered: r.VatRegistered, TaxType: r.TaxType, Status: r.Status,
	}
}

const supplierColumns = "s.id, s.code, s.name, s.tax_id, s.branch_no, s.address, s.addr_building, s.addr_room_no, s.addr_floor_no, s.addr_village, s.addr_house_no, s.addr_moo, s.addr_soi, s.addr_yaek, s.addr_street, s.addr_sub_district, s.addr_district, s.addr_province, s.addr_postal_code, s.payment_term_days, s.vat_registered, s.tax_type, s.status"

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
	out := rows[0].dto()
	branches, err := r.branches(ctx, rows[0].ID)
	if err != nil {
		return dto.Supplier{}, err
	}
	out.Branches = make([]dto.SupplierBranch, len(branches))
	for i, b := range branches {
		out.Branches[i] = dto.SupplierBranch{BranchNo: b.BranchNo, Name: b.Name, Address: b.Address, AddressDetail: thaiaddress.Address(b.AddressDetail)}
	}
	return out, nil
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
