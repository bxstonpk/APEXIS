package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/application/ports/repositories"
	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	gp "apexis/backend/internal/modules/finance/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
)

// ---------- แบบ ภ.ง.ด.3 / 53 ----------

// whtPaid: บรรทัดใบจ่าย (ยังไม่ยกเลิก) ที่หักภาษีของบริษัทในช่วงวันที่ (alias l = บรรทัด, d = หัว)
func whtPaid(db *gorm.DB, companyID vo.CompanyID, start, end time.Time) *gorm.DB {
	return db.Table("finance_payment_lines AS l").
		Joins("JOIN finance_payments AS d ON d.id = l.payment_id").
		Where("d.company_id = ? AND d.status = 'posted' AND l.wht > 0 AND d.doc_date BETWEEN ? AND ?", uuidOf(companyID), start, end)
}

type whtFormRow struct {
	Seq           int
	PaymentID     uuid.UUID
	DocumentNo    string
	CertificateNo *string
	DocDate       time.Time
	PartyTaxID    string
	PartyBranchNo string
	PartyName     string
	PartyAddress  string
	WhtIncomeType *string
	WhtName       *string
	WhtRate       decimal.Decimal
	WhtBase       decimal.Decimal
	Wht           decimal.Decimal
}

// WhtFormLines — 1 query: บรรทัดใบจ่ายที่หักภาษีของแบบ + ประเภทเงินได้ (LEFT JOIN ประเภทภาษี) +
// ลำดับที่ในใบแนบด้วย DENSE_RANK ต่อใบจ่ายตาม (วันที่, เลขที่) — ใช้ index (company_id, doc_date) ของใบจ่าย
func (r *ReportRepository) WhtFormLines(ctx context.Context, companyID vo.CompanyID, taxType vo.TaxType, start, end time.Time) ([]dto.WhtFormLine, error) {
	var rows []whtFormRow
	err := whtPaid(database.Conn(ctx, r.db), companyID, start, end).
		Joins("LEFT JOIN finance_wht_types AS w ON w.id = l.wht_type_id").
		Where("d.party_tax_type = ?", string(taxType)).
		Select(`DENSE_RANK() OVER (ORDER BY d.doc_date, d.document_no) AS seq, d.id AS payment_id, d.document_no,
			d.wht_certificate_no AS certificate_no, d.doc_date, d.party_tax_id, d.party_branch_no, d.party_name, d.party_address,
			w.income_type AS wht_income_type, w.name AS wht_name, l.wht_rate, l.wht_base, l.wht`).
		Order("d.doc_date, d.document_no, l.line_no").
		Scan(&rows).Error
	out := make([]dto.WhtFormLine, len(rows))
	for i, x := range rows {
		income := "other"
		if x.WhtIncomeType != nil && *x.WhtIncomeType != "" {
			income = *x.WhtIncomeType
		}
		out[i] = dto.WhtFormLine{Seq: x.Seq, PaymentID: x.PaymentID.String(), DocumentNo: x.DocumentNo, Date: gp.DateUTC(x.DocDate),
			TaxID: x.PartyTaxID, BranchNo: x.PartyBranchNo, Name: x.PartyName, Address: x.PartyAddress, IncomeType: income,
			Rate: x.WhtRate, Base: x.WhtBase, Wht: x.Wht}
		if x.CertificateNo != nil {
			out[i].CertificateNo = *x.CertificateNo
		}
		if x.WhtName != nil {
			out[i].WhtTypeName = *x.WhtName
		}
	}
	return out, err
}

type whtMonthRow struct {
	Month        time.Time
	PartyTaxType string
	Payees       int
	Base         decimal.Decimal
	Wht          decimal.Decimal
}

// WhtMonthly — 1 query: GROUP BY เดือน / ประเภทผู้มีเงินได้ (จำนวนราย = จำนวนใบจ่าย)
func (r *ReportRepository) WhtMonthly(ctx context.Context, companyID vo.CompanyID, start, end time.Time) ([]dto.WhtMonthTotal, error) {
	var rows []whtMonthRow
	err := whtPaid(database.Conn(ctx, r.db), companyID, start, end).
		Where("d.party_tax_type IN ?", []string{string(vo.TaxIndividual), string(vo.TaxJuristic)}).
		Select(`date_trunc('month', d.doc_date)::date AS month, d.party_tax_type, COUNT(DISTINCT d.id) AS payees,
			SUM(l.wht_base) AS base, SUM(l.wht) AS wht`).
		Group("1, 2").Order("1, 2").Scan(&rows).Error
	out := make([]dto.WhtMonthTotal, len(rows))
	for i, x := range rows {
		form := vo.FormPnd53
		if vo.TaxType(x.PartyTaxType) == vo.TaxIndividual {
			form = vo.FormPnd3
		}
		out[i] = dto.WhtMonthTotal{Month: vo.TaxMonthOf(x.Month), Form: form, Payees: x.Payees, Base: x.Base, Wht: x.Wht}
	}
	return out, err
}

// WhtSequence — 1 query: นับใบจ่ายที่หักภาษี ประเภทผู้มีเงินได้เดียวกัน เดือนเดียวกัน ที่มาก่อน / เท่ากับใบนี้ (วันที่, เลขที่)
// = DENSE_RANK เดียวกับ WhtFormLines (ลำดับที่ในใบแนบตรงกับหนังสือรับรอง 50 ทวิ)
func (r *ReportRepository) WhtSequence(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (int, error) {
	var seq int
	err := database.Conn(ctx, r.db).Raw(`
		SELECT COUNT(DISTINCT d.id)
		FROM finance_payments AS p
		JOIN finance_payments AS d ON d.company_id = p.company_id AND d.status = 'posted' AND d.party_tax_type = p.party_tax_type
			AND d.doc_date BETWEEN date_trunc('month', p.doc_date)::date AND p.doc_date
			AND (d.doc_date < p.doc_date OR d.document_no <= p.document_no)
		WHERE p.company_id = ? AND p.id = ? AND p.status = 'posted'
			AND EXISTS (SELECT 1 FROM finance_payment_lines AS pl WHERE pl.payment_id = p.id AND pl.wht > 0)
			AND EXISTS (SELECT 1 FROM finance_payment_lines AS l WHERE l.payment_id = d.id AND l.wht > 0)`,
		uuidOf(companyID), uuidOf(id)).Scan(&seq).Error
	return seq, err
}

// ---------- ทะเบียนการยื่น ----------

type TaxFilingRepository struct{ db *gorm.DB }

var _ repositories.TaxFilingRepository = (*TaxFilingRepository)(nil)

func NewTaxFilingRepository(db *gorm.DB) *TaxFilingRepository { return &TaxFilingRepository{db: db} }

func (r *TaxFilingRepository) Create(ctx context.Context, f *entities.TaxFiling) error {
	m := gp.TaxFilingToModel(f)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrTaxFilingExists
		}
		return err
	}
	return nil
}

func (r *TaxFilingRepository) UpdateStatus(ctx context.Context, f *entities.TaxFiling) error {
	s := f.State()
	return database.Conn(ctx, r.db).Model(&models.TaxFilingModel{}).Where("company_id = ? AND id = ?", uuidOf(s.CompanyID), uuidOf(s.ID)).
		Updates(map[string]any{"status": string(s.Status), "cancelled_by": gp.OptUUID(s.CancelledBy), "cancelled_at": s.CancelledAt}).Error
}

func (r *TaxFilingRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.TaxFilingID) (*entities.TaxFiling, error) {
	m, err := lockHeader[models.TaxFilingModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrTaxFilingNotFound)
	if err != nil {
		return nil, err
	}
	return gp.TaxFilingToEntity(m), nil
}

// List — 1 query บน index (company_id, tax_month)
func (r *TaxFilingRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.TaxFilingFilter) ([]*entities.TaxFiling, error) {
	q := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID))
	if f.Year > 0 {
		q = q.Where("tax_month BETWEEN ? AND ?", time.Date(f.Year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(f.Year, 12, 1, 0, 0, 0, 0, time.UTC))
	}
	if f.Form != nil {
		q = q.Where("form = ?", string(*f.Form))
	}
	if f.Status != nil {
		q = q.Where("status = ?", string(*f.Status))
	}
	var rows []models.TaxFilingModel
	if err := q.Order("tax_month, form, created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.TaxFiling, len(rows))
	for i, m := range rows {
		out[i] = gp.TaxFilingToEntity(m)
	}
	return out, nil
}

// Active — 1 query บน unique partial index (company_id, form, tax_month) WHERE status = 'filed'
func (r *TaxFilingRepository) Active(ctx context.Context, companyID vo.CompanyID, form vo.TaxForm, month vo.TaxMonth) (*entities.TaxFiling, error) {
	var m models.TaxFilingModel
	err := database.Conn(ctx, r.db).Where("company_id = ? AND form = ? AND tax_month = ? AND status = 'filed'", uuidOf(companyID), string(form), month.Start()).
		Take(&m).Error
	if database.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return gp.TaxFilingToEntity(m), nil
}
