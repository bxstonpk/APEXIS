// Package repositories คือ implementation ของ cashbank/application/ports/repositories ด้วย gorm
package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/cashbank/application/dto"
	ports "apexis/backend/internal/modules/cashbank/application/ports/repositories"
	"apexis/backend/internal/modules/cashbank/domain/entities"
	domain "apexis/backend/internal/modules/cashbank/domain/errors"
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
	gp "apexis/backend/internal/modules/cashbank/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/cashbank/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.CashAccountRepository = (*CashAccountRepository)(nil)

type CashAccountRepository struct{ db *gorm.DB }

func NewCashAccountRepository(db *gorm.DB) *CashAccountRepository {
	return &CashAccountRepository{db: db}
}

func (r *CashAccountRepository) Create(ctx context.Context, a *entities.CashAccount) error {
	m := gp.ToModel(a)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrCodeExists
		}
		return err
	}
	return nil
}

func (r *CashAccountRepository) Update(ctx context.Context, a *entities.CashAccount) error {
	m := gp.ToModel(a)
	res := database.Conn(ctx, r.db).Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "type", "bank_name", "bank_branch", "account_no", "gl_account_id", "status", "updated_at").
		Updates(&m)
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrCodeExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrCashAccountNotFound
	}
	return nil
}

func (r *CashAccountRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuid.UUID(companyID), uuid.UUID(id)).Delete(&models.CashAccountModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrCashAccountNotFound
	}
	return nil
}

func (r *CashAccountRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) (*entities.CashAccount, error) {
	var m models.CashAccountModel
	err := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuid.UUID(companyID), uuid.UUID(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrCashAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.ToEntity(m), nil
}

// FindByIDs — 1 query (ไม่รวมที่ถูกลบ)
func (r *CashAccountRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.CashAccountID) ([]*entities.CashAccount, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		u[i] = uuid.UUID(id)
	}
	var ms []models.CashAccountModel
	if err := database.Conn(ctx, r.db).Where("company_id = ? AND id IN ?", uuid.UUID(companyID), u).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.CashAccount, len(ms))
	for i, m := range ms {
		out[i] = gp.ToEntity(m)
	}
	return out, nil
}

type row struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Type        string
	BankName    string
	BankBranch  string
	AccountNo   string
	GLAccountID uuid.UUID `gorm:"column:gl_account_id"`
	Status      string
	Total       int64
}

func (r row) dto() dto.CashAccount {
	return dto.CashAccount{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, Type: r.Type, BankName: r.BankName, BankBranch: r.BankBranch,
		AccountNo: r.AccountNo, GLAccountID: r.GLAccountID.String(), Status: r.Status,
	}
}

const columns = "a.id, a.code, a.name, a.type, a.bank_name, a.bank_branch, a.account_no, a.gl_account_id, a.status"

func (r *CashAccountRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("cashbank_accounts AS a").Where("a.company_id = ? AND a.deleted_at IS NULL", uuid.UUID(companyID))
}

func (r *CashAccountRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) (dto.CashAccount, error) {
	var rows []row
	if err := r.base(ctx, companyID).Select(columns).Where("a.id = ?", uuid.UUID(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.CashAccount{}, err
	}
	if len(rows) == 0 {
		return dto.CashAccount{}, domain.ErrCashAccountNotFound
	}
	return rows[0].dto(), nil
}

func (r *CashAccountRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.Filter) (listing.Result[dto.CashAccount], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "a.code", "name": "a.name"}, "a.id")
	if err != nil {
		return listing.Result[dto.CashAccount]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Types) > 0 {
			q = q.Where("a.type IN ?", f.Types)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("a.status IN ?", f.Statuses)
		}
		if f.Search != "" {
			p := database.LikeContains(f.Search)
			cols := []string{"a.code", "a.name", "a.bank_name", "a.account_no"}
			parts := make([]string, len(cols))
			args := make([]any, len(cols))
			for i, c := range cols {
				parts[i], args[i] = "LOWER("+c+") LIKE ?", p
			}
			q = q.Where("("+strings.Join(parts, " OR ")+")", args...)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, columns, order, f.Page, func(r row) int64 { return r.Total })
	items := make([]dto.CashAccount, len(rows))
	for i, x := range rows {
		items[i] = x.dto()
	}
	return listing.Result[dto.CashAccount]{Items: items, Total: total, Page: f.Page}, err
}
