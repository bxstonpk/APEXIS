package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/ledger/application/dto"
	ports "apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	gp "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.AccountRepository = (*AccountRepository)(nil)

type AccountRepository struct{ db *gorm.DB }

func NewAccountRepository(db *gorm.DB) *AccountRepository { return &AccountRepository{db: db} }

func (r *AccountRepository) Create(ctx context.Context, a *entities.Account) error {
	m := gp.AccountToModel(a)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *AccountRepository) Update(ctx context.Context, a *entities.Account) error {
	m := gp.AccountToModel(a)
	return update(ctx, r.db, &m, m.CompanyID,
		[]string{"code", "name", "name_en", "type", "parent_id", "postable", "status", "description"},
		domain.ErrCodeExists, domain.ErrAccountNotFound)
}

// CreateMany — INSERT แบบ batch (ผังบัญชีมาตรฐาน 245 บัญชี = 3 คำสั่ง) ใน transaction ของ use case
// เรียงบัญชีคุมก่อนบัญชีย่อย FK ของ parent จึงผ่านในทุก batch
func (r *AccountRepository) CreateMany(ctx context.Context, as []*entities.Account) error {
	if len(as) == 0 {
		return nil
	}
	ms := make([]models.AccountModel, len(as))
	for i, a := range as {
		ms[i] = gp.AccountToModel(a)
	}
	if err := database.Conn(ctx, r.db).Omit(clause.Associations).CreateInBatches(&ms, 100).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrCodeExists
		}
		return err
	}
	return nil
}

// HasAny — 1 query: บริษัทมีบัญชี (ที่ยังไม่ถูกลบ) อยู่แล้วหรือไม่
func (r *AccountRepository) HasAny(ctx context.Context, companyID vo.CompanyID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).Raw(
		`SELECT EXISTS (SELECT 1 FROM ledger_accounts WHERE company_id = ? AND deleted_at IS NULL)`,
		uuidOf(companyID)).Scan(&ok).Error
	return ok, err
}

func (r *AccountRepository) PostableIDsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error) {
	out := map[string]vo.AccountID{}
	if len(codes) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   uuid.UUID
		Code string
	}
	err := database.Conn(ctx, r.db).Model(&models.AccountModel{}).Select("id, code").
		Where("company_id = ? AND code IN ? AND postable AND status = ?", uuidOf(companyID), codes, string(vo.StatusActive)).
		Scan(&rows).Error
	for _, row := range rows {
		out[row.Code] = vo.AccountID(row.ID)
	}
	return out, err
}

func (r *AccountRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) error {
	return softDelete[models.AccountModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrAccountNotFound)
}

func (r *AccountRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) (*entities.Account, error) {
	m, err := findByID[models.AccountModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrAccountNotFound)
	if err != nil {
		return nil, err
	}
	return gp.AccountToEntity(m), nil
}

func (r *AccountRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]*entities.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	keys := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		keys[i] = uuidOf(id)
	}
	var ms []models.AccountModel
	if err := database.Conn(ctx, r.db).Where("company_id = ? AND id IN ?", uuidOf(companyID), keys).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.Account, len(ms))
	for i, m := range ms {
		out[i] = gp.AccountToEntity(m)
	}
	return out, nil
}

// Usage — 1 query: มีบัญชีย่อยที่ยังไม่ถูกลบ / มีบรรทัดรายการใช้บัญชีนี้ (ทุกสถานะ)
func (r *AccountRepository) Usage(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) (entities.AccountUsage, error) {
	var u struct{ HasChildren, HasLines bool }
	err := database.Conn(ctx, r.db).Raw(`
		SELECT
			EXISTS (SELECT 1 FROM ledger_accounts WHERE company_id = ? AND parent_id = ? AND deleted_at IS NULL) AS has_children,
			EXISTS (SELECT 1 FROM ledger_journal_lines WHERE account_id = ?) AS has_lines`,
		uuidOf(companyID), uuidOf(id), uuidOf(id)).Scan(&u).Error
	return entities.AccountUsage{HasChildren: u.HasChildren, HasLines: u.HasLines}, err
}

// Ancestry — recursive CTE เดียว จำกัดความลึกกันวนไม่รู้จบถ้าข้อมูลเสีย
func (r *AccountRepository) Ancestry(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) ([]vo.AccountID, error) {
	var rows []struct{ ID uuid.UUID }
	err := database.Conn(ctx, r.db).Raw(`
		WITH RECURSIVE chain AS (
			SELECT id, parent_id, 1 AS depth FROM ledger_accounts
			WHERE id = ? AND company_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT a.id, a.parent_id, c.depth + 1 FROM ledger_accounts AS a
			JOIN chain AS c ON a.id = c.parent_id
			WHERE a.deleted_at IS NULL AND c.depth < 64
		)
		SELECT id FROM chain ORDER BY depth`, uuidOf(id), uuidOf(companyID)).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]vo.AccountID, len(rows))
	for i, row := range rows {
		out[i] = vo.AccountID(row.ID)
	}
	return out, nil
}

type accountRow struct {
	ID         uuid.UUID
	Code       string
	Name       string
	NameEN     string
	Type       string
	ParentID   *uuid.UUID
	ParentCode *string
	ParentName *string
	Postable   bool
	Status     string
	Desc       string `gorm:"column:description"`
	Total      int64
}

func (r accountRow) dto() dto.Account {
	out := dto.Account{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, NameEN: r.NameEN, Type: r.Type, Postable: r.Postable, Status: r.Status,
		Description: r.Desc,
	}
	if r.ParentID != nil && r.ParentCode != nil {
		out.Parent = &dto.Ref{ID: r.ParentID.String(), Code: *r.ParentCode, Name: *r.ParentName}
	}
	return out
}

const accountColumns = "a.id, a.code, a.name, a.name_en, a.type, a.parent_id, p.code AS parent_code, p.name AS parent_name, a.postable, a.status, a.description"

func (r *AccountRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("ledger_accounts AS a").
		Joins("LEFT JOIN ledger_accounts AS p ON p.id = a.parent_id AND p.deleted_at IS NULL").
		Where("a.company_id = ? AND a.deleted_at IS NULL", uuidOf(companyID))
}

// Get — 1 query: accounts LEFT JOIN parent
func (r *AccountRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) (dto.Account, error) {
	var rows []accountRow
	if err := r.base(ctx, companyID).Select(accountColumns).Where("a.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Account{}, err
	}
	if len(rows) == 0 {
		return dto.Account{}, domain.ErrAccountNotFound
	}
	return rows[0].dto(), nil
}

var accountSortColumns = map[string]string{"code": "a.code", "name": "a.name", "type": "a.type"}

// List — 1 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล)
func (r *AccountRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.AccountFilter) (listing.Result[dto.Account], error) {
	order, err := database.OrderBy(f.Sort, accountSortColumns, "a.id")
	if err != nil {
		return listing.Result[dto.Account]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Types) > 0 {
			q = q.Where("a.type IN ?", f.Types)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("a.status IN ?", f.Statuses)
		}
		if f.Postable != nil {
			q = q.Where("a.postable = ?", *f.Postable)
		}
		if f.ParentID != nil {
			q = q.Where("a.parent_id = ?", uuidOf(*f.ParentID))
		}
		return search(q, f.Search, "a.code", "a.name", "a.name_en")
	}
	rows, total, err := database.FetchPage(base, accountColumns, order, f.Page, func(r accountRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, accountRow.dto), err
}

// Tree — 1 query: ทุกบัญชีของบริษัท (ทุกสถานะ) เรียงตามรหัส — หน้าผังบัญชีสร้างต้นไม้จาก parent
// ผังบัญชีมีขนาดจำกัด (หลักร้อย) จึงส่งทั้งชุดแทนการแบ่งหน้า — ใช้ index uq_ledger_accounts_code (company_id, code)
func (r *AccountRepository) Tree(ctx context.Context, companyID vo.CompanyID) ([]dto.Account, error) {
	var rows []accountRow
	if err := r.base(ctx, companyID).Select(accountColumns).Order("a.code").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]dto.Account, len(rows))
	for i, row := range rows {
		out[i] = row.dto()
	}
	return out, nil
}

// Chart — 1 query: ผังบัญชีทั้งหมดของบริษัท (ไม่รวมที่ถูกลบ — บัญชีที่ถูกใช้แล้วลบไม่ได้)
func (r *AccountRepository) Chart(ctx context.Context, companyID vo.CompanyID) ([]services.AccountNode, error) {
	var rows []struct {
		ID       uuid.UUID
		ParentID *uuid.UUID
		Code     string
		Name     string
		Type     string
		Postable bool
	}
	err := database.Conn(ctx, r.db).Table("ledger_accounts").
		Select("id, parent_id, code, name, type, postable").
		Where("company_id = ? AND deleted_at IS NULL", uuidOf(companyID)).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]services.AccountNode, len(rows))
	for i, row := range rows {
		out[i] = services.AccountNode{
			ID: vo.AccountID(row.ID), Code: row.Code, Name: row.Name, Type: vo.AccountType(row.Type), Postable: row.Postable,
		}
		if row.ParentID != nil {
			p := vo.AccountID(*row.ParentID)
			out[i].ParentID = &p
		}
	}
	return out, nil
}
