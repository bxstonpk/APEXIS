package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/organization/application/dto"
	ports "apexis/backend/internal/modules/organization/application/ports/repositories"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	gp "apexis/backend/internal/modules/organization/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.FiscalRepository = (*FiscalRepository)(nil)

type FiscalRepository struct{ db *gorm.DB }

func NewFiscalRepository(db *gorm.DB) *FiscalRepository { return &FiscalRepository{db: db} }

// CreateYear เขียนปีบัญชี + งวด (2 ตาราง) — ต้องอยู่ใน transaction ของ use case
func (r *FiscalRepository) CreateYear(ctx context.Context, fy *entities.FiscalYear) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.FiscalYearToModel(fy)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrFiscalYearExists
		}
		return err
	}
	return tx.Create(&m.Periods).Error
}

func (r *FiscalRepository) ExistsOverlapping(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).Raw(
		"SELECT EXISTS (SELECT 1 FROM organization_fiscal_years WHERE company_id = ? AND start_date <= ? AND end_date >= ?)",
		uuidOf(companyID), end, start).Scan(&ok).Error
	return ok, err
}

// FindYearByPeriodForUpdate — 2 query: lock แถวปีบัญชี (FOR UPDATE) แล้วโหลดงวดทั้งหมดของปี
// แยก query เพราะ lock ต้องอยู่ที่แถวปีบัญชีแถวเดียว ไม่ใช่ทุกแถวงวด
func (r *FiscalRepository) FindYearByPeriodForUpdate(ctx context.Context, companyID vo.CompanyID, periodID vo.FiscalPeriodID) (*entities.FiscalYear, error) {
	var year models.FiscalYearModel
	err := database.Conn(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("company_id = ? AND id = (SELECT fiscal_year_id FROM organization_fiscal_periods WHERE id = ?)",
			uuidOf(companyID), uuidOf(periodID)).
		Take(&year).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrFiscalPeriodNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := database.Conn(ctx, r.db).Where("fiscal_year_id = ?", year.ID).Order("period_no").Find(&year.Periods).Error; err != nil {
		return nil, err
	}
	return gp.FiscalYearToEntity(year), nil
}

func (r *FiscalRepository) UpdatePeriodStatus(ctx context.Context, p *entities.FiscalPeriod) error {
	res := database.Conn(ctx, r.db).Model(&models.FiscalPeriodModel{ID: uuidOf(p.ID())}).
		Updates(map[string]any{"status": string(p.Status()), "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrFiscalPeriodNotFound
	}
	return nil
}

// PeriodAt — 1 query: fiscal_periods JOIN fiscal_years ... FOR SHARE OF p
func (r *FiscalRepository) PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (dto.PeriodAt, error) {
	var rows []struct {
		ID        uuid.UUID
		Status    string
		Year      int
		YearStart time.Time
		StartDate time.Time
		EndDate   time.Time
	}
	err := database.Conn(ctx, r.db).Raw(`
		SELECT p.id, p.status, y.year, y.start_date AS year_start, p.start_date, p.end_date
		FROM organization_fiscal_periods AS p
		JOIN organization_fiscal_years AS y ON y.id = p.fiscal_year_id
		WHERE y.company_id = ? AND p.start_date <= ? AND p.end_date >= ?
		FOR SHARE OF p`, uuidOf(companyID), date, date).
		Scan(&rows).Error
	if err != nil {
		return dto.PeriodAt{}, err
	}
	if len(rows) == 0 {
		return dto.PeriodAt{}, domain.ErrFiscalPeriodNotFound
	}
	p := rows[0]
	return dto.PeriodAt{
		PeriodID: vo.FiscalPeriodID(p.ID), FiscalYear: p.Year, YearStart: dateUTC(p.YearStart),
		Start: dateUTC(p.StartDate), End: dateUTC(p.EndDate), Status: vo.PeriodStatus(p.Status),
	}, nil
}

type fiscalYearRow struct {
	ID        uuid.UUID
	Year      int
	StartDate time.Time
	EndDate   time.Time
	Total     int64
}

func (r fiscalYearRow) dto() dto.FiscalYear {
	return dto.FiscalYear{ID: r.ID.String(), Year: r.Year, StartDate: dateUTC(r.StartDate), EndDate: dateUTC(r.EndDate)}
}

// GetYear — 1 query: fiscal_years JOIN fiscal_periods (12 แถว)
func (r *FiscalRepository) GetYear(ctx context.Context, companyID vo.CompanyID, id vo.FiscalYearID) (dto.FiscalYear, error) {
	var rows []periodRow
	err := r.periodBase(ctx, companyID).Select(periodColumns).Where("y.id = ?", uuidOf(id)).Order("p.period_no").Scan(&rows).Error
	if err != nil {
		return dto.FiscalYear{}, err
	}
	if len(rows) == 0 {
		return dto.FiscalYear{}, domain.ErrFiscalYearNotFound
	}
	out := dto.FiscalYear{
		ID: rows[0].FiscalYearID.String(), Year: rows[0].FiscalYear,
		StartDate: dateUTC(rows[0].YearStart), EndDate: dateUTC(rows[0].YearEnd),
		Periods: make([]dto.FiscalPeriod, len(rows)),
	}
	for i, p := range rows {
		out.Periods[i] = p.dto()
	}
	return out, nil
}

func (r *FiscalRepository) ListYears(ctx context.Context, companyID vo.CompanyID, f dto.FiscalYearFilter) (listing.Result[dto.FiscalYear], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"year": "y.year"}, "y.id")
	if err != nil {
		return listing.Result[dto.FiscalYear]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("organization_fiscal_years AS y").Where("y.company_id = ?", uuidOf(companyID))
		if f.Year != nil {
			q = q.Where("y.year = ?", *f.Year)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, "y.id, y.year, y.start_date, y.end_date", order, f.Page,
		func(r fiscalYearRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, fiscalYearRow.dto), err
}

type periodRow struct {
	ID           uuid.UUID
	FiscalYearID uuid.UUID
	FiscalYear   int
	YearStart    time.Time
	YearEnd      time.Time
	PeriodNo     int
	StartDate    time.Time
	EndDate      time.Time
	Status       string
	Total        int64
}

func (r periodRow) dto() dto.FiscalPeriod {
	return dto.FiscalPeriod{
		ID: r.ID.String(), FiscalYearID: r.FiscalYearID.String(), FiscalYear: r.FiscalYear, PeriodNo: r.PeriodNo,
		StartDate: dateUTC(r.StartDate), EndDate: dateUTC(r.EndDate), Status: r.Status,
	}
}

const periodColumns = `p.id, p.fiscal_year_id, y.year AS fiscal_year, y.start_date AS year_start, y.end_date AS year_end,
	p.period_no, p.start_date, p.end_date, p.status`

// periodBase: fiscal_periods JOIN fiscal_years (กรองบริษัทที่ปีบัญชี)
func (r *FiscalRepository) periodBase(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).
		Table("organization_fiscal_periods AS p").
		Joins("JOIN organization_fiscal_years AS y ON y.id = p.fiscal_year_id").
		Where("y.company_id = ?", uuidOf(companyID))
}

func (r *FiscalRepository) GetPeriod(ctx context.Context, companyID vo.CompanyID, id vo.FiscalPeriodID) (dto.FiscalPeriod, error) {
	row, err := first[periodRow](r.periodBase(ctx, companyID).Select(periodColumns).Where("p.id = ?", uuidOf(id)), domain.ErrFiscalPeriodNotFound)
	return row.dto(), err
}

func (r *FiscalRepository) ListPeriods(ctx context.Context, companyID vo.CompanyID, f dto.FiscalPeriodFilter) (listing.Result[dto.FiscalPeriod], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"start_date": "p.start_date"}, "p.id")
	if err != nil {
		return listing.Result[dto.FiscalPeriod]{}, err
	}
	base := func() *gorm.DB {
		q := r.periodBase(ctx, companyID)
		if f.FiscalYear != nil {
			q = q.Where("y.year = ?", *f.FiscalYear)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("p.status IN ?", f.Statuses)
		}
		if f.Date != nil {
			q = q.Where("p.start_date <= ? AND p.end_date >= ?", *f.Date, *f.Date)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, periodColumns, order, f.Page, func(r periodRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, periodRow.dto), err
}
