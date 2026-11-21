package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/purchasing/application/dto"
	ports "apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

// ---------- ประเภทภาษีหัก ณ ที่จ่าย ----------

var _ ports.WhtTypeRepository = (*WhtTypeRepository)(nil)

type WhtTypeRepository struct{ db *gorm.DB }

func NewWhtTypeRepository(db *gorm.DB) *WhtTypeRepository { return &WhtTypeRepository{db: db} }

func (r *WhtTypeRepository) Create(ctx context.Context, w *entities.WhtType) error {
	m := gp.WhtTypeToModel(w)
	if err := database.Conn(ctx, r.db).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrCodeExists
		}
		return err
	}
	return nil
}

func (r *WhtTypeRepository) Update(ctx context.Context, w *entities.WhtType) error {
	m := gp.WhtTypeToModel(w)
	res := database.Conn(ctx, r.db).Model(&m).Where("company_id = ?", m.CompanyID).
		Select("code", "name", "rate", "status", "updated_at").Updates(&m)
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrCodeExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrWhtTypeNotFound
	}
	return nil
}

func (r *WhtTypeRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.WhtTypeID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Delete(&models.WhtTypeModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrWhtTypeNotFound
	}
	return nil
}

func (r *WhtTypeRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.WhtTypeID) (*entities.WhtType, error) {
	var m models.WhtTypeModel
	err := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrWhtTypeNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.WhtTypeToEntity(m), nil
}

func (r *WhtTypeRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.WhtTypeID) ([]*entities.WhtType, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		u[i] = uuidOf(id)
	}
	var ms []models.WhtTypeModel
	if err := database.Conn(ctx, r.db).Where("company_id = ? AND id IN ?", uuidOf(companyID), u).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.WhtType, len(ms))
	for i, m := range ms {
		out[i] = gp.WhtTypeToEntity(m)
	}
	return out, nil
}

type whtRow struct {
	ID     uuid.UUID
	Code   string
	Name   string
	Rate   decimal.Decimal
	Status string
	Total  int64
}

func (r whtRow) dto() dto.WhtType {
	return dto.WhtType{ID: r.ID.String(), Code: r.Code, Name: r.Name, Rate: r.Rate, Status: r.Status}
}

func (r *WhtTypeRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.WhtTypeFilter) (listing.Result[dto.WhtType], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "w.code", "name": "w.name"}, "w.id")
	if err != nil {
		return listing.Result[dto.WhtType]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("purchasing_wht_types AS w").Where("w.company_id = ? AND w.deleted_at IS NULL", uuidOf(companyID))
		if len(f.Statuses) > 0 {
			q = q.Where("w.status IN ?", f.Statuses)
		}
		return search(q, f.Search, "w.code", "w.name")
	}
	rows, total, err := database.FetchPage(base, "w.id, w.code, w.name, w.rate, w.status", order, f.Page, func(r whtRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, whtRow.dto), err
}

// ---------- ใบจ่ายชำระ ----------

var _ ports.PaymentRepository = (*PaymentRepository)(nil)

type PaymentRepository struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) *PaymentRepository { return &PaymentRepository{db: db} }

// Create — 3 statement: หัว + บรรทัด + ช่องทางจ่าย (batch)
func (r *PaymentRepository) Create(ctx context.Context, p *entities.Payment) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.PaymentToModel(p)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrDocumentNoUsed
		}
		return err
	}
	if err := tx.Create(&m.Lines).Error; err != nil {
		return err
	}
	return tx.Create(&m.Methods).Error
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, p *entities.Payment) error {
	m := gp.PaymentToModel(p)
	return updateStatus(ctx, r.db, &models.PaymentModel{}, m.CompanyID, m.ID, m.Status, m.CancelledBy, m.CancelledAt, domain.ErrPaymentNotFound)
}

// FindForUpdate — 3 query: lock หัว แล้วโหลดบรรทัด / ช่องทางจ่าย
func (r *PaymentRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (*entities.Payment, error) {
	db := database.Conn(ctx, r.db)
	var m models.PaymentModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("payment_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	if err := db.Where("payment_id = ?", m.ID).Order("line_no").Find(&m.Methods).Error; err != nil {
		return nil, err
	}
	return gp.PaymentToEntity(m), nil
}

type paymentRow struct {
	ID           uuid.UUID
	DocumentNo   string
	PaymentDate  time.Time
	SupplierID   uuid.UUID
	SupplierCode string
	SupplierName string
	TaxType      string
	TotalAmount  decimal.Decimal
	TotalWht     decimal.Decimal
	TotalPaid    decimal.Decimal
	Status       string
	EntryID      *uuid.UUID
	EntryNo      string
	CancelledAt  *time.Time
	Description  string
	CreatedBy    uuid.UUID
	Total        int64
}

func (r paymentRow) summary() dto.PaymentSummary {
	return dto.PaymentSummary{
		ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: gp.DateUTC(r.PaymentDate),
		Supplier: dto.Ref{ID: r.SupplierID.String(), Code: r.SupplierCode, Name: r.SupplierName},
		TaxForm:  vo.TaxType(r.TaxType).Form(), Amount: r.TotalAmount, Wht: r.TotalWht, Paid: r.TotalPaid, Status: r.Status,
		EntryID: optString(r.EntryID), EntryNo: r.EntryNo, CancelledAt: r.CancelledAt,
	}
}

const (
	paymentSummaryColumns = `p.id, p.document_no, p.payment_date, p.supplier_id, s.code AS supplier_code, s.name AS supplier_name,
		p.tax_type, p.total_amount, p.total_wht, p.total_paid, p.status, p.entry_id, p.entry_no, p.cancelled_at`
	paymentColumns = paymentSummaryColumns + ", p.description, p.created_by"
)

func (r *PaymentRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("purchasing_payments AS p").
		Joins("JOIN purchasing_suppliers AS s ON s.id = p.supplier_id").
		Where("p.company_id = ?", uuidOf(companyID))
}

// Get — 3 query: หัว (JOIN ผู้ขาย), บรรทัด (JOIN ใบแจ้งหนี้ + ประเภทภาษี), ช่องทางจ่าย
func (r *PaymentRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (dto.Payment, error) {
	var rows []paymentRow
	if err := r.base(ctx, companyID).Select(paymentColumns).Where("p.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Payment{}, err
	}
	if len(rows) == 0 {
		return dto.Payment{}, domain.ErrPaymentNotFound
	}
	h := rows[0]
	out := dto.Payment{PaymentSummary: h.summary(), Description: h.Description, CreatedBy: h.CreatedBy.String()}
	var lines []struct {
		InvoiceID         uuid.UUID
		DocumentNo        string
		SupplierInvoiceNo string
		Amount            decimal.Decimal
		WhtTypeID         *uuid.UUID
		WhtCode           *string
		WhtName           *string
		WhtRate           decimal.Decimal
		WhtBase           decimal.Decimal
		Wht               decimal.Decimal
	}
	db := database.Conn(ctx, r.db)
	err := db.Table("purchasing_payment_lines AS l").
		Joins("JOIN purchasing_invoices AS i ON i.id = l.invoice_id").
		Joins("LEFT JOIN purchasing_wht_types AS w ON w.id = l.wht_type_id").
		Select(`l.invoice_id, i.document_no, i.supplier_invoice_no, l.amount, l.wht_type_id, w.code AS wht_code, w.name AS wht_name,
			l.wht_rate, l.wht_base, l.wht`).
		Where("l.payment_id = ?", h.ID).Order("l.line_no").Scan(&lines).Error
	if err != nil {
		return dto.Payment{}, err
	}
	out.Lines = make([]dto.PaymentLine, len(lines))
	for i, l := range lines {
		line := dto.PaymentLine{
			Invoice: dto.Ref{ID: l.InvoiceID.String(), Code: l.DocumentNo, Name: l.SupplierInvoiceNo},
			Amount:  l.Amount, WhtRate: l.WhtRate, WhtBase: l.WhtBase, Wht: l.Wht,
		}
		if l.WhtTypeID != nil && l.WhtCode != nil {
			line.WhtType = &dto.Ref{ID: l.WhtTypeID.String(), Code: *l.WhtCode, Name: *l.WhtName}
		}
		out.Lines[i] = line
	}
	var methods []models.PaymentMethodModel
	if err := db.Where("payment_id = ?", h.ID).Order("line_no").Find(&methods).Error; err != nil {
		return dto.Payment{}, err
	}
	out.Methods = make([]dto.PaymentMethod, len(methods))
	for i, m := range methods {
		out.Methods[i] = dto.PaymentMethod{CashAccountID: m.CashAccountID.String(), GLAccountID: m.GLAccountID.String(), Amount: m.Amount, Reference: m.Reference}
	}
	return out, nil
}

func (r *PaymentRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.PaymentSummary], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "p.payment_date", "document_no": "p.document_no"}, "p.id")
	if err != nil {
		return listing.Result[dto.PaymentSummary]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("p.status IN ?", f.Statuses)
		}
		if f.SupplierID != nil {
			q = q.Where("p.supplier_id = ?", uuidOf(*f.SupplierID))
		}
		if f.StartDate != nil {
			q = q.Where("p.payment_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("p.payment_date <= ?", *f.EndDate)
		}
		return search(q, f.Search, "p.document_no", "s.code", "s.name", "p.description")
	}
	rows, total, err := database.FetchPage(base, paymentSummaryColumns, order, f.Page, func(r paymentRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, paymentRow.summary), err
}
