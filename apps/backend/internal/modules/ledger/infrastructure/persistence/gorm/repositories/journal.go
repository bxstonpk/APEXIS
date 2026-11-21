package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/application/dto"
	ports "apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	gp "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.JournalRepository = (*JournalRepository)(nil)

type JournalRepository struct{ db *gorm.DB }

func NewJournalRepository(db *gorm.DB) *JournalRepository { return &JournalRepository{db: db} }

func (r *JournalRepository) Create(ctx context.Context, j *entities.Journal) error {
	m := gp.JournalToModel(j)
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *JournalRepository) Update(ctx context.Context, j *entities.Journal) error {
	m := gp.JournalToModel(j)
	return update(ctx, r.db, &m, m.CompanyID, []string{"code", "name", "number_template", "status"}, domain.ErrCodeExists, domain.ErrJournalNotFound)
}

func (r *JournalRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) error {
	return softDelete[models.JournalModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrJournalNotFound)
}

func (r *JournalRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (*entities.Journal, error) {
	m, err := findByID[models.JournalModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrJournalNotFound)
	if err != nil {
		return nil, err
	}
	return gp.JournalToEntity(m), nil
}

func (r *JournalRepository) HasEntries(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).Raw(
		"SELECT EXISTS (SELECT 1 FROM ledger_journal_entries WHERE company_id = ? AND journal_id = ?)",
		uuidOf(companyID), uuidOf(id)).Scan(&ok).Error
	return ok, err
}

type journalRow struct {
	ID             uuid.UUID
	Code           string
	Name           string
	NumberTemplate string
	Status         string
	Total          int64
}

func (r journalRow) dto() dto.Journal {
	return dto.Journal{ID: r.ID.String(), Code: r.Code, Name: r.Name, NumberTemplate: r.NumberTemplate, Status: r.Status}
}

const journalColumns = "j.id, j.code, j.name, j.number_template, j.status"

func (r *JournalRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("ledger_journals AS j").
		Where("j.company_id = ? AND j.deleted_at IS NULL", uuidOf(companyID))
}

func (r *JournalRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (dto.Journal, error) {
	var rows []journalRow
	if err := r.base(ctx, companyID).Select(journalColumns).Where("j.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Journal{}, err
	}
	if len(rows) == 0 {
		return dto.Journal{}, domain.ErrJournalNotFound
	}
	return rows[0].dto(), nil
}

func (r *JournalRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.JournalFilter) (listing.Result[dto.Journal], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "j.code", "name": "j.name"}, "j.id")
	if err != nil {
		return listing.Result[dto.Journal]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("j.status IN ?", f.Statuses)
		}
		return search(q, f.Search, "j.code", "j.name")
	}
	rows, total, err := database.FetchPage(base, journalColumns, order, f.Page, func(r journalRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, journalRow.dto), err
}

// NextNumber — statement เดียว: INSERT ... ON CONFLICT DO UPDATE ... RETURNING
// แถวลำดับถูก lock จน transaction commit — การผ่านรายการพร้อมกันในสมุด / ปีเดียวกันจึงต่อคิวกัน
// ถ้า transaction rollback เลขก็ถูก rollback ด้วย จึงไม่มีเลขขาดตอน
func (r *JournalRepository) NextNumber(ctx context.Context, companyID vo.CompanyID, journalID vo.JournalID, periodKey string) (int64, error) {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	err = tx.Raw(`
		INSERT INTO ledger_number_sequences (journal_id, period_key, company_id, last_no) VALUES (?, ?, ?, 1)
		ON CONFLICT (journal_id, period_key) DO UPDATE SET last_no = ledger_number_sequences.last_no + 1
		RETURNING last_no`, uuidOf(journalID), periodKey, uuidOf(companyID)).Scan(&n).Error
	return n, err
}
