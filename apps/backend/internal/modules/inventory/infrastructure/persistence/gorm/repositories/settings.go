package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// ---------- ค่าตั้ง ----------

var _ ports.SettingsRepository = (*SettingsRepository)(nil)

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

// Get — 2 query (ค่าตั้ง 1 แถว + ระบบบัญชีตามปี) เป็นข้อมูลอิสระต่อกัน
// lock: สร้างแถวถ้ายังไม่มี (ON CONFLICT DO NOTHING) แล้ว SELECT ... FOR SHARE / FOR UPDATE
func (r *SettingsRepository) Get(ctx context.Context, companyID vo.CompanyID, lock ports.Lock) (entities.Settings, []entities.SystemChange, error) {
	conn := database.Conn(ctx, r.db)
	q := conn.Where("company_id = ?", uuidOf(companyID))
	if lock != ports.NoLock {
		tx, err := database.TxConn(ctx)
		if err != nil {
			return entities.Settings{}, nil, err
		}
		row := models.SettingsModel{CompanyID: uuidOf(companyID), UpdatedAt: time.Now().UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return entities.Settings{}, nil, err
		}
		strength := clause.LockingStrengthShare
		if lock == ports.ForUpdate {
			strength = clause.LockingStrengthUpdate
		}
		q = tx.Clauses(clause.Locking{Strength: strength}).Where("company_id = ?", uuidOf(companyID))
	}
	var ms []models.SettingsModel
	if err := q.Limit(1).Find(&ms).Error; err != nil {
		return entities.Settings{}, nil, err
	}
	var s entities.Settings
	if len(ms) > 0 {
		s = gp.SettingsToEntity(ms[0])
	}
	var systems []models.SystemModel
	if err := conn.Where("company_id = ?", uuidOf(companyID)).Order("from_year").Find(&systems).Error; err != nil {
		return entities.Settings{}, nil, err
	}
	out := make([]entities.SystemChange, len(systems))
	for i, m := range systems {
		out[i] = entities.SystemChange{FromYear: m.FromYear, System: vo.AccountingSystem(m.System)}
	}
	return s, out, nil
}

func (r *SettingsRepository) Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error {
	m := gp.SettingsToModel(companyID, s)
	m.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"default_purchase_account_id", "default_inventory_account_id", "cogs_account_id", "journal_id", "updated_at"}),
	}).Create(&m).Error
}

func (r *SettingsRepository) SaveSystem(ctx context.Context, companyID vo.CompanyID, c entities.SystemChange) error {
	m := models.SystemModel{CompanyID: uuidOf(companyID), FromYear: c.FromYear, System: string(c.System), UpdatedAt: time.Now().UTC()}
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}, {Name: "from_year"}},
		DoUpdates: clause.AssignmentColumns([]string{"system", "updated_at"}),
	}).Create(&m).Error
}

func (r *SettingsRepository) HasPostedFrom(ctx context.Context, companyID vo.CompanyID, year int) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_documents WHERE company_id = ? AND status = 'posted' AND fiscal_year >= ?"},
		uuidOf(companyID), year)
}

// ---------- ปรับปรุงสินค้าคงเหลือสิ้นงวด ----------

var _ ports.ClosingRepository = (*ClosingRepository)(nil)

type ClosingRepository struct{ db *gorm.DB }

func NewClosingRepository(db *gorm.DB) *ClosingRepository { return &ClosingRepository{db: db} }

func (r *ClosingRepository) Create(ctx context.Context, c *entities.PeriodClosing) error {
	m := gp.ClosingToModel(c)
	return create(ctx, r.db, &m, domain.ErrClosingExists)
}

func (r *ClosingRepository) UpdateStatus(ctx context.Context, c *entities.PeriodClosing) error {
	m := gp.ClosingToModel(c)
	res := database.Conn(ctx, r.db).Model(&models.ClosingModel{}).
		Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{
			"entry_id": m.EntryID, "entry_no": m.EntryNo, "adjustment": m.Adjustment, "status": m.Status,
			"cancelled_by": m.CancelledBy, "cancelled_at": m.CancelledAt, "updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrClosingExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrClosingNotFound
	}
	return nil
}

func (r *ClosingRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ClosingID) (*entities.PeriodClosing, error) {
	var m models.ClosingModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrClosingNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.ClosingToEntity(m), nil
}

func (r *ClosingRepository) PostedOnOrAfter(ctx context.Context, companyID vo.CompanyID, date time.Time) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM inventory_period_closings WHERE company_id = ? AND status = 'posted' AND period_end >= ?"},
		uuidOf(companyID), date)
}

type closingRow struct {
	ID             uuid.UUID
	FiscalPeriodID uuid.UUID
	FiscalYear     int
	PeriodEnd      time.Time
	EntryID        *uuid.UUID
	EntryNo        string
	Adjustment     decimal.Decimal
	Status         string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	CancelledBy    *uuid.UUID
	CancelledAt    *time.Time
	Total          int64
}

func (r closingRow) dto() dto.Closing {
	return dto.Closing{
		ID: r.ID.String(), PeriodID: r.FiscalPeriodID.String(), FiscalYear: r.FiscalYear, PeriodEnd: gp.DateUTC(r.PeriodEnd),
		EntryID: optString(r.EntryID), EntryNo: r.EntryNo, Adjustment: r.Adjustment, Status: r.Status,
		CreatedBy: r.CreatedBy.String(), CreatedAt: r.CreatedAt, CancelledBy: optString(r.CancelledBy), CancelledAt: r.CancelledAt,
	}
}

const closingColumns = `x.id, x.fiscal_period_id, x.fiscal_year, x.period_end, x.entry_id, x.entry_no, x.adjustment,
	x.status, x.created_by, x.created_at, x.cancelled_by, x.cancelled_at`

func (r *ClosingRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("inventory_period_closings AS x").Where("x.company_id = ?", uuidOf(companyID))
}

func (r *ClosingRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.ClosingID) (dto.Closing, error) {
	var rows []closingRow
	if err := r.base(ctx, companyID).Select(closingColumns).Where("x.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Closing{}, err
	}
	if len(rows) == 0 {
		return dto.Closing{}, domain.ErrClosingNotFound
	}
	return rows[0].dto(), nil
}

func (r *ClosingRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.ClosingFilter) (listing.Result[dto.Closing], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"period_end": "x.period_end", "created_at": "x.created_at"}, "x.id")
	if err != nil {
		return listing.Result[dto.Closing]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if f.FiscalYear != nil {
			q = q.Where("x.fiscal_year = ?", *f.FiscalYear)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("x.status IN ?", f.Statuses)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, closingColumns, order, f.Page, func(r closingRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, closingRow.dto), err
}
