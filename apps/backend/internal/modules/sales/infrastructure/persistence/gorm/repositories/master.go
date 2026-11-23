package repositories

import (
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
			"revenue_account_id", "cogs_account_id", "updated_at"}),
	}).Create(&m).Error
}

// ---------- ลูกค้า ----------

var _ ports.CustomerRepository = (*CustomerRepository)(nil)

type CustomerRepository struct{ db *gorm.DB }

func NewCustomerRepository(db *gorm.DB) *CustomerRepository { return &CustomerRepository{db: db} }

func (r *CustomerRepository) Create(ctx context.Context, c *entities.Customer) error {
	m := gp.CustomerToModel(c)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrCodeExists
		}
		return err
	}
	return nil
}

func (r *CustomerRepository) Update(ctx context.Context, c *entities.Customer) error {
	m := gp.CustomerToModel(c)
	res := database.Conn(ctx, r.db).Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "tax_id", "branch_no", "address", "payment_term_days", "tax_type", "credit_limit", "status", "updated_at").
		Updates(&m)
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrCodeExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrCustomerNotFound
	}
	return nil
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
	return gp.CustomerToEntity(m), nil
}

// InUse: มีใบเสนอราคา / ใบสั่งขาย / ใบแจ้งหนี้ของลูกค้านี้ — query เดียว
func (r *CustomerRepository) InUse(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (bool, error) {
	c, i := uuidOf(companyID), uuidOf(id)
	return exists(ctx, r.db, []string{
		"SELECT 1 FROM sales_quotations WHERE company_id = ? AND customer_id = ?",
		"SELECT 1 FROM sales_orders WHERE company_id = ? AND customer_id = ?",
		"SELECT 1 FROM sales_invoices WHERE company_id = ? AND customer_id = ?",
	}, c, i, c, i, c, i)
}

type customerRow struct {
	ID              uuid.UUID
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	PaymentTermDays int
	TaxType         string
	CreditLimit     *decimal.Decimal
	Status          string
	Total           int64
}

func (r customerRow) dto() dto.Customer {
	return dto.Customer{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, TaxID: r.TaxID, BranchNo: r.BranchNo, Address: r.Address,
		PaymentTermDays: r.PaymentTermDays, TaxType: r.TaxType, CreditLimit: r.CreditLimit, Status: r.Status,
	}
}

const customerColumns = "c.id, c.code, c.name, c.tax_id, c.branch_no, c.address, c.payment_term_days, c.tax_type, c.credit_limit, c.status"

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
	return rows[0].dto(), nil
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
