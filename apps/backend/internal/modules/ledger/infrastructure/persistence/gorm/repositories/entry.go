package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

var _ ports.EntryRepository = (*EntryRepository)(nil)

type EntryRepository struct{ db *gorm.DB }

func NewEntryRepository(db *gorm.DB) *EntryRepository { return &EntryRepository{db: db} }

// Create — 2 statement (หัวรายการ + บรรทัดแบบ batch) ใน transaction ของ use case
func (r *EntryRepository) Create(ctx context.Context, e *entities.Entry) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.EntryToModel(e)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrEntryNoUsed // เลขที่ซ้ำ — เกิดได้เมื่อเปลี่ยนรูปแบบเลขที่ของสมุดจนชนเลขเดิม
		}
		return err
	}
	return tx.Create(&m.Lines).Error
}

// Update แทนที่ draft: แก้หัวรายการ (เฉพาะที่ยังเป็น draft) -> ลบบรรทัดเดิม -> insert บรรทัดใหม่
func (r *EntryRepository) Update(ctx context.Context, e *entities.Entry) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.EntryToModel(e)
	res := tx.Model(&models.EntryModel{}).
		Where("company_id = ? AND id = ? AND status = ?", m.CompanyID, m.ID, vo.EntryDraft).
		Updates(map[string]any{
			"journal_id": m.JournalID, "entry_date": m.EntryDate, "description": m.Description, "reference": m.Reference,
			"total_debit": m.TotalDebit, "total_credit": m.TotalCredit, "updated_at": time.Now().UTC(),
			"adjusting_period": m.AdjustingPeriod, "template": m.Template, "auto_reverse": m.AutoReverse,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrEntryNotDraft
	}
	if err := tx.Where("entry_id = ?", m.ID).Delete(&models.LineModel{}).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

// UpdateStatus เขียนเฉพาะสถานะ / ข้อมูลการผ่านรายการ / การกลับรายการ
func (r *EntryRepository) UpdateStatus(ctx context.Context, e *entities.Entry) error {
	m := gp.EntryToModel(e)
	res := database.Conn(ctx, r.db).Model(&models.EntryModel{}).
		Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{
			"status": m.Status, "entry_no": m.EntryNo, "fiscal_period_id": m.FiscalPeriodID,
			"posted_at": m.PostedAt, "posted_by": m.PostedBy, "reversed_by_id": m.ReversedByID,
			"auto_reversal_id": m.AutoReversalID, "updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrEntryNoUsed
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrEntryNotFound
	}
	return nil
}

// Delete ลบ draft (บรรทัดถูกลบตาม ON DELETE CASCADE)
func (r *EntryRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.EntryID) error {
	res := database.Conn(ctx, r.db).
		Where("company_id = ? AND id = ? AND status = ?", uuidOf(companyID), uuidOf(id), vo.EntryDraft).
		Delete(&models.EntryModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrEntryNotFound
	}
	return nil
}

// FindForUpdate — 2 query: lock แถวหัวรายการ (FOR UPDATE) แล้วโหลดบรรทัด
// แยก query เพราะ lock ต้องอยู่ที่หัวรายการแถวเดียว
func (r *EntryRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.EntryID) (*entities.Entry, error) {
	db := database.Conn(ctx, r.db)
	var m models.EntryModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("entry_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.EntryToEntity(m), nil
}

type entryRow struct {
	ID             uuid.UUID
	EntryNo        *string
	EntryDate      time.Time
	JournalID      uuid.UUID
	JournalCode    string
	JournalName    string
	Description    string
	Reference      string
	Status         string
	TotalDebit     decimal.Decimal
	TotalCredit    decimal.Decimal
	PostedAt       *time.Time
	FiscalPeriodID *uuid.UUID
	ReversalOfID   *uuid.UUID
	ReversedByID   *uuid.UUID
	CreatedBy      uuid.UUID
	PostedBy       *uuid.UUID
	AdjustingPeriod bool
	IsClosing       bool
	Template        string
	AutoReverse     bool
	IsAutoReversal  bool
	RevID           *uuid.UUID
	RevNo           *string
	RevDate         *time.Time
	RevStatus       *string
	Total           int64
}

func (r entryRow) summary() dto.EntrySummary {
	out := dto.EntrySummary{
		ID: r.ID.String(), Date: gp.DateUTC(r.EntryDate),
		Journal:     dto.Ref{ID: r.JournalID.String(), Code: r.JournalCode, Name: r.JournalName},
		Description: r.Description, Reference: r.Reference, Status: r.Status,
		TotalDebit: r.TotalDebit, TotalCredit: r.TotalCredit, PostedAt: r.PostedAt,
		AdjustingPeriod: r.AdjustingPeriod, Closing: r.IsClosing, Template: r.Template,
		AutoReverse: r.AutoReverse, AutoReversal: r.IsAutoReversal,
	}
	if r.EntryNo != nil {
		out.EntryNo = *r.EntryNo
	}
	if r.RevID != nil {
		out.Reversal = &dto.EntryLink{ID: r.RevID.String(), EntryNo: deref(r.RevNo), Status: deref(r.RevStatus)}
		if r.RevDate != nil {
			out.Reversal.Date = gp.DateUTC(*r.RevDate)
		}
	}
	return out
}

const entrySummaryColumns = `e.id, e.entry_no, e.entry_date, e.journal_id, j.code AS journal_code, j.name AS journal_name,
	e.description, e.reference, e.status, e.total_debit, e.total_credit, e.posted_at,
	e.adjusting_period, e.is_closing, e.template, e.auto_reverse, e.is_auto_reversal,
	rv.id AS rev_id, rv.entry_no AS rev_no, rv.entry_date AS rev_date, rv.status AS rev_status`

// base: รายการ JOIN สมุด + LEFT JOIN รายการกลับอัตโนมัติ (1:1)
func (r *EntryRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("ledger_journal_entries AS e").
		Joins("JOIN ledger_journals AS j ON j.id = e.journal_id").
		Joins("LEFT JOIN ledger_journal_entries AS rv ON rv.id = e.auto_reversal_id").
		Where("e.company_id = ?", uuidOf(companyID))
}

// PendingAutoReversals: รายการกลับอัตโนมัติที่ยังรอผ่าน (draft) ลงวันที่ไม่เกิน until — เรียงตามวันที่
func (r *EntryRepository) PendingAutoReversals(ctx context.Context, companyID vo.CompanyID) ([]*entities.Entry, error) {
	var ms []models.EntryModel
	err := database.Conn(ctx, r.db).Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_no") }).
		Where("company_id = ? AND is_auto_reversal AND status = ?", uuidOf(companyID), vo.EntryDraft).
		Order("entry_date, id").Find(&ms).Error
	out := make([]*entities.Entry, len(ms))
	for i, m := range ms {
		out[i] = gp.EntryToEntity(m)
	}
	return out, err
}

// Get — 2 query: หัวรายการ JOIN สมุด + บรรทัด JOIN บัญชี (one-to-many แยก query ไม่ให้หัวรายการซ้ำทุกบรรทัด)
func (r *EntryRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.EntryID) (dto.Entry, error) {
	var heads []entryRow
	err := r.base(ctx, companyID).
		Select(entrySummaryColumns+", e.fiscal_period_id, e.reversal_of_id, e.reversed_by_id, e.created_by, e.posted_by").
		Where("e.id = ?", uuidOf(id)).Limit(1).Scan(&heads).Error
	if err != nil {
		return dto.Entry{}, err
	}
	if len(heads) == 0 {
		return dto.Entry{}, domain.ErrEntryNotFound
	}
	h := heads[0]
	var lines []struct {
		LineNo        int
		AccountID     uuid.UUID
		AccountCode   string
		AccountName   string
		Description   string
		Debit         decimal.Decimal
		Credit        decimal.Decimal
		BranchID      *uuid.UUID
		DepartmentID  *uuid.UUID
		CostCenterID  *uuid.UUID
		ProjectSiteID *uuid.UUID
	}
	err = database.Conn(ctx, r.db).Table("ledger_journal_lines AS l").
		Select(`l.line_no, l.account_id, a.code AS account_code, a.name AS account_name, l.description,
			l.debit, l.credit, l.branch_id, l.department_id, l.cost_center_id, l.project_site_id`).
		Joins("JOIN ledger_accounts AS a ON a.id = l.account_id").
		Where("l.entry_id = ?", h.ID).Order("l.line_no").Scan(&lines).Error
	if err != nil {
		return dto.Entry{}, err
	}
	out := dto.Entry{
		EntrySummary: h.summary(), FiscalPeriodID: optString(h.FiscalPeriodID),
		ReversalOfID: optString(h.ReversalOfID), ReversedByID: optString(h.ReversedByID),
		CreatedBy: h.CreatedBy.String(), PostedBy: optString(h.PostedBy),
		Lines: make([]dto.EntryLine, len(lines)),
	}
	for i, l := range lines {
		out.Lines[i] = dto.EntryLine{
			LineNo: l.LineNo, Account: dto.Ref{ID: l.AccountID.String(), Code: l.AccountCode, Name: l.AccountName},
			Description: l.Description, Debit: l.Debit, Credit: l.Credit,
			BranchID: optString(l.BranchID), DepartmentID: optString(l.DepartmentID),
			CostCenterID: optString(l.CostCenterID), ProjectSiteID: optString(l.ProjectSiteID),
		}
	}
	return out, nil
}

var entrySortColumns = map[string]string{"entry_date": "e.entry_date", "entry_no": "e.entry_no", "created_at": "e.created_at"}

// List — 1 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล) — ยอดรวมอ่านจากหัวรายการ ไม่ต้องรวมบรรทัด
func (r *EntryRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.EntryFilter) (listing.Result[dto.EntrySummary], error) {
	order, err := database.OrderBy(f.Sort, entrySortColumns, "e.id")
	if err != nil {
		return listing.Result[dto.EntrySummary]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("e.status IN ?", f.Statuses)
		}
		if f.JournalID != nil {
			q = q.Where("e.journal_id = ?", uuidOf(*f.JournalID))
		}
		if f.AccountID != nil {
			q = q.Where("EXISTS (SELECT 1 FROM ledger_journal_lines AS l WHERE l.entry_id = e.id AND l.account_id = ?)", uuidOf(*f.AccountID))
		}
		if f.StartDate != nil {
			q = q.Where("e.entry_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("e.entry_date <= ?", *f.EndDate)
		}
		if f.Adjustments {
			q = q.Where("e.template <> ''")
		}
		if len(f.Templates) > 0 {
			q = q.Where("e.template IN ?", f.Templates)
		}
		return search(q, f.Search, "e.entry_no", "e.description", "e.reference")
	}
	rows, total, err := database.FetchPage(base, entrySummaryColumns, order, f.Page, func(r entryRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, entryRow.summary), err
}
