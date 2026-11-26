package repositories

import (
	"apexis/backend/internal/platform/thaiaddress"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/sales/application/dto"
	ports "apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	gp "apexis/backend/internal/modules/sales/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/sales/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

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
		DoUpdates: clause.AssignmentColumns([]string{"vat_registered", "vat_rate", "journal_id", "ar_account_id", "output_vat_account_id",
			"revenue_account_id", "cogs_account_id", "sales_returns_account_id", "updated_at"}),
	}).Create(&m).Error
}

// ---------- ลูกค้า ----------

var _ ports.CustomerRepository = (*CustomerRepository)(nil)

type CustomerRepository struct{ db *gorm.DB }

func NewCustomerRepository(db *gorm.DB) *CustomerRepository { return &CustomerRepository{db: db} }

// TaxIDIndex คือ unique index "เลขผู้เสียภาษี 1 เลข = ลูกค้า 1 รหัส" (สร้างใน Migrate หลังรวมรหัสซ้ำ)
const TaxIDIndex = "uq_sal_customers_tax_id"

func customerUniqueErr(err error) error {
	if !database.IsUniqueViolation(err) {
		return err
	}
	if database.UniqueConstraint(err) == TaxIDIndex {
		return domain.ErrTaxIDExists
	}
	return domain.ErrCodeExists
}

func (r *CustomerRepository) Create(ctx context.Context, c *entities.Customer) error {
	m := gp.CustomerToModel(c)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		return customerUniqueErr(err)
	}
	return r.saveBranches(ctx, c)
}

// saveBranches แทนที่สาขาเพิ่มเติมทั้งชุด (ภายใน transaction ของ use case) — เอกสารอ้างสาขาด้วยเลขสาขา จึงไม่มี FK ไปที่สาขา
func (r *CustomerRepository) saveBranches(ctx context.Context, c *entities.Customer) error {
	conn := database.Conn(ctx, r.db)
	if err := conn.Where("customer_id = ?", uuidOf(c.ID())).Delete(&models.CustomerBranchModel{}).Error; err != nil {
		return err
	}
	ms := gp.CustomerBranchesToModel(c)
	if len(ms) == 0 {
		return nil
	}
	return conn.CreateInBatches(&ms, 200).Error
}

func (r *CustomerRepository) branches(ctx context.Context, id uuid.UUID) ([]models.CustomerBranchModel, error) {
	var ms []models.CustomerBranchModel
	err := database.Conn(ctx, r.db).Where("customer_id = ?", id).Order("position, branch_no").Find(&ms).Error
	return ms, err
}

// TaxIDOwner: รหัสลูกค้า (ที่ยังไม่ถูกลบ) ที่ใช้เลขผู้เสียภาษีนี้อยู่ นอกจาก except — ไม่มี = ""
func (r *CustomerRepository) TaxIDOwner(ctx context.Context, companyID vo.CompanyID, taxID string, except *vo.CustomerID) (string, error) {
	if taxID == "" {
		return "", nil
	}
	q := database.Conn(ctx, r.db).Model(&models.CustomerModel{}).Select("code").
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

func (r *CustomerRepository) Update(ctx context.Context, c *entities.Customer) error {
	m := gp.CustomerToModel(c)
	res := database.Conn(ctx, r.db).Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "tax_id", "branch_no", "address", "addr_building", "addr_room_no", "addr_floor_no", "addr_village", "addr_house_no", "addr_moo", "addr_soi", "addr_yaek", "addr_street", "addr_sub_district", "addr_district", "addr_province", "addr_postal_code", "payment_term_days", "tax_type", "credit_limit", "status", "updated_at").
		Updates(&m)
	if res.Error != nil {
		return customerUniqueErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrCustomerNotFound
	}
	return r.saveBranches(ctx, c)
}

func (r *CustomerRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Delete(&models.CustomerModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrCustomerNotFound
	}
	return nil
}

func (r *CustomerRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (*entities.Customer, error) {
	var m models.CustomerModel
	err := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrCustomerNotFound
	}
	if err != nil {
		return nil, err
	}
	branches, err := r.branches(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	return gp.CustomerToEntity(m, branches...), nil
}

// InUse: มีใบเสนอราคา / ใบสั่งขาย / ใบแจ้งหนี้ของลูกค้านี้ — query เดียว
func (r *CustomerRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (bool, error) {
	c, i := uuidOf(companyID), uuidOf(id)
	return exists(ctx, r.db, []string{
		"SELECT 1 FROM sales_quotations WHERE company_id = ? AND customer_id = ?",
		"SELECT 1 FROM sales_orders WHERE company_id = ? AND customer_id = ?",
		"SELECT 1 FROM sales_invoices WHERE company_id = ? AND customer_id = ?",
		"SELECT 1 FROM sales_credit_notes WHERE company_id = ? AND customer_id = ?",
	}, c, i, c, i, c, i, c, i)
}

type customerRow struct {
	ID              uuid.UUID
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	AddressDetail   models.AddressColumns `gorm:"embedded;embeddedPrefix:addr_"`
	PaymentTermDays int
	TaxType         string
	CreditLimit     *decimal.Decimal
	Status          string
	Total           int64
}

func (r customerRow) dto() dto.Customer {
	return dto.Customer{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, TaxID: r.TaxID, BranchNo: r.BranchNo, Address: r.Address, AddressDetail: thaiaddress.Address(r.AddressDetail),
		PaymentTermDays: r.PaymentTermDays, TaxType: r.TaxType, CreditLimit: r.CreditLimit, Status: r.Status,
	}
}

const customerColumns = "c.id, c.code, c.name, c.tax_id, c.branch_no, c.address, c.addr_building, c.addr_room_no, c.addr_floor_no, c.addr_village, c.addr_house_no, c.addr_moo, c.addr_soi, c.addr_yaek, c.addr_street, c.addr_sub_district, c.addr_district, c.addr_province, c.addr_postal_code, c.payment_term_days, c.tax_type, c.credit_limit, c.status"

func (r *CustomerRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("sales_customers AS c").Where("c.company_id = ? AND c.deleted_at IS NULL", uuidOf(companyID))
}

func (r *CustomerRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (dto.Customer, error) {
	var rows []customerRow
	if err := r.base(ctx, companyID).Select(customerColumns).Where("c.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Customer{}, err
	}
	if len(rows) == 0 {
		return dto.Customer{}, domain.ErrCustomerNotFound
	}
	out := rows[0].dto()
	branches, err := r.branches(ctx, rows[0].ID)
	if err != nil {
		return dto.Customer{}, err
	}
	out.Branches = make([]dto.CustomerBranch, len(branches))
	for i, b := range branches {
		out.Branches[i] = dto.CustomerBranch{BranchNo: b.BranchNo, Name: b.Name, Address: b.Address, AddressDetail: thaiaddress.Address(b.AddressDetail)}
	}
	return out, nil
}

func (r *CustomerRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.CustomerFilter) (listing.Result[dto.Customer], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "c.code", "name": "c.name"}, "c.id")
	if err != nil {
		return listing.Result[dto.Customer]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("c.status IN ?", f.Statuses)
		}
		return search(q, f.Search, "c.code", "c.name", "c.tax_id")
	}
	rows, total, err := database.FetchPage(base, customerColumns, order, f.Page, func(r customerRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, customerRow.dto), err
}
