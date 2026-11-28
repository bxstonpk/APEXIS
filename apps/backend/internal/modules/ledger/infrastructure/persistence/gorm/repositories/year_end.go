package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/ledger/application/dto"
	ports "apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	gp "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
)

// ---------- ค่าตั้ง ----------

var _ ports.SettingsRepository = (*SettingsRepository)(nil)

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

func (r *SettingsRepository) Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, bool, error) {
	var m models.SettingsModel
	err := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID)).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entities.Settings{}, false, nil
	}
	if err != nil {
		return entities.Settings{}, false, err
	}
	return gp.SettingsToEntity(m), true, nil
}

// Save — upsert 1 แถวต่อบริษัท
func (r *SettingsRepository) Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error {
	m := gp.SettingsToModel(companyID, s, time.Now())
	return database.Conn(ctx, r.db).Omit(clause.Associations).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"retained_earnings_account_id", "income_summary_account_id", "closing_journal_id", "updated_at"}),
	}).Create(&m).Error
}

// ---------- ปิดบัญชีสิ้นปี ----------

var _ ports.YearEndRepository = (*YearEndRepository)(nil)

type YearEndRepository struct{ db *gorm.DB }

func NewYearEndRepository(db *gorm.DB) *YearEndRepository { return &YearEndRepository{db: db} }

func (r *YearEndRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, year int) (*entities.YearEnd, error) {
	var m models.YearEndClosingModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("company_id = ? AND fiscal_year = ?", uuidOf(companyID), year).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return gp.YearEndToEntity(m), nil
}

// Save — upsert ตาม (company_id, fiscal_year)
func (r *YearEndRepository) Save(ctx context.Context, y *entities.YearEnd) error {
	m := gp.YearEndToModel(y, time.Now())
	return database.Conn(ctx, r.db).Omit(clause.Associations).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "company_id"}, {Name: "fiscal_year"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "net_income", "entry_id", "reversal_id",
			"closed_by", "closed_at", "reopened_by", "reopened_at", "updated_at"}),
	}).Create(&m).Error
}

// IsClosed — ภายใน transaction อ่านแบบ FOR SHARE: การปิด / เปิดปีพร้อมกัน (UPDATE แถวเดียวกัน) ต้องรอรายการที่กำลังลง
func (r *YearEndRepository) IsClosed(ctx context.Context, companyID vo.CompanyID, year int) (bool, error) {
	q := database.Conn(ctx, r.db).Model(&models.YearEndClosingModel{}).Select("id").
		Where("company_id = ? AND fiscal_year = ? AND status = ?", uuidOf(companyID), year, string(entities.YearClosed))
	if _, err := database.TxConn(ctx); err == nil {
		q = q.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var ids []uuid.UUID
	err := q.Limit(1).Pluck("id", &ids).Error
	return len(ids) > 0, err
}

// List — 1 query: แถวการปิดบัญชี LEFT JOIN เลขที่รายการปิด / รายการกลับ
func (r *YearEndRepository) List(ctx context.Context, companyID vo.CompanyID) ([]dto.YearEndRecord, error) {
	var rows []struct {
		FiscalYear int
		Status     string
		NetIncome  vo.Money
		EntryID    *uuid.UUID
		EntryNo    *string
		ReversalNo *string
		ClosedAt   time.Time
		ClosedBy   uuid.UUID
		ReopenedAt *time.Time
	}
	err := database.Conn(ctx, r.db).Table("ledger_year_end_closings AS y").
		Select("y.fiscal_year, y.status, y.net_income, y.entry_id, e.entry_no, rv.entry_no AS reversal_no, y.closed_at, y.closed_by, y.reopened_at").
		Joins("LEFT JOIN ledger_journal_entries AS e ON e.id = y.entry_id").
		Joins("LEFT JOIN ledger_journal_entries AS rv ON rv.id = y.reversal_id").
		Where("y.company_id = ?", uuidOf(companyID)).Order("y.fiscal_year").Scan(&rows).Error
	out := make([]dto.YearEndRecord, len(rows))
	for i, x := range rows {
		out[i] = dto.YearEndRecord{
			FiscalYear: x.FiscalYear, Status: x.Status, NetIncome: x.NetIncome, EntryID: gp.OptEntryID(x.EntryID),
			EntryNo: deref(x.EntryNo), ReversalNo: deref(x.ReversalNo), ClosedAt: x.ClosedAt, ClosedBy: vo.UserID(x.ClosedBy), ReopenedAt: x.ReopenedAt,
		}
	}
	return out, err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
