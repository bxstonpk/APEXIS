package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

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

// ---------- ใบวางบิล ----------

var _ ports.BillingNoteRepository = (*BillingNoteRepository)(nil)

type BillingNoteRepository struct{ db *gorm.DB }

func NewBillingNoteRepository(db *gorm.DB) *BillingNoteRepository {
	return &BillingNoteRepository{db: db}
}

func (r *BillingNoteRepository) Create(ctx context.Context, b *entities.BillingNote) error {
	m := gp.BillingNoteToModel(b)
	m.UpdatedAt = m.CreatedAt
	return createDoc(ctx, &m, &m.Invoices)
}

func (r *BillingNoteRepository) UpdateStatus(ctx context.Context, b *entities.BillingNote) error {
	m := gp.BillingNoteToModel(b)
	res := database.Conn(ctx, r.db).Model(&models.BillingNoteModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{"status": m.Status, "cancelled_by": m.CancelledBy, "cancelled_at": m.CancelledAt, "updated_at": time.Now().UTC()})
	if res.Error == nil && res.RowsAffected == 0 {
		return domain.ErrBillingNoteNotFound
	}
	return res.Error
}

// FindForUpdate — 2 query: หัว (FOR UPDATE) + ใบแจ้งหนี้ในใบวางบิล
func (r *BillingNoteRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.BillingNoteID) (*entities.BillingNote, error) {
	m, err := lockHeader[models.BillingNoteModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrBillingNoteNotFound)
	if err != nil {
		return nil, err
	}
	if err := database.Conn(ctx, r.db).Where("billing_note_id = ?", m.ID).Order("position").Find(&m.Invoices).Error; err != nil {
		return nil, err
	}
	return gp.BillingNoteToEntity(m), nil
}

// billingTotals: ยอดรวม / ยอดค้างของใบแจ้งหนี้ในใบวางบิล (subquery ต่อแถว — index: sales_billing_note_invoices PK)
const billingColumns = `b.id, b.document_no, b.doc_date, b.due_date, b.customer_id, c.code AS customer_code, c.name AS customer_name,
	b.status, b.cancelled_at, b.description, b.created_by,
	(SELECT COALESCE(SUM(i.total_gross), 0) FROM sales_billing_note_invoices AS bi JOIN sales_invoices AS i ON i.id = bi.invoice_id
		WHERE bi.billing_note_id = b.id) AS total,
	` + billingOutstanding + ` AS outstanding`

const billingOutstanding = `(SELECT COALESCE(SUM(CASE WHEN i.status = 'posted' THEN i.total_gross - i.received_amount ELSE 0 END), 0)
		FROM sales_billing_note_invoices AS bi JOIN sales_invoices AS i ON i.id = bi.invoice_id WHERE bi.billing_note_id = b.id)`

type billingRow struct {
	ID           uuid.UUID
	DocumentNo   string
	DocDate      time.Time
	DueDate      time.Time
	CustomerID   uuid.UUID
	CustomerCode string
	CustomerName string
	Status       string
	CancelledAt  *time.Time
	Description  string
	CreatedBy    uuid.UUID
	Total        decimal.Decimal
	Outstanding  decimal.Decimal
	RowTotal     int64 `gorm:"column:row_total"`
}

// summary: open + ไม่มียอดค้าง = paid (คำนวณ ไม่เก็บ)
func (r billingRow) summary() dto.BillingNoteSummary {
	status := r.Status
	if status == string(vo.BillingOpen) && !r.Outstanding.IsPositive() {
		status = string(vo.BillingPaid)
	}
	return dto.BillingNoteSummary{
		ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: dateUTC(r.DocDate), DueDate: dateUTC(r.DueDate),
		Customer: dto.Ref{ID: r.CustomerID.String(), Code: r.CustomerCode, Name: r.CustomerName},
		Status:   status, Total: r.Total, Outstanding: r.Outstanding, CancelledAt: r.CancelledAt,
	}
}

func (r *BillingNoteRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("sales_billing_notes AS b").Joins("JOIN sales_customers AS c ON c.id = b.customer_id").
		Where("b.company_id = ?", uuidOf(companyID))
}

// Get — 2 query: หัว (+ ยอดรวม) + ใบแจ้งหนี้
func (r *BillingNoteRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.BillingNoteID) (dto.BillingNote, error) {
	var rows []billingRow
	if err := r.base(ctx, companyID).Select(billingColumns).Where("b.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.BillingNote{}, err
	}
	if len(rows) == 0 {
		return dto.BillingNote{}, domain.ErrBillingNoteNotFound
	}
	h := rows[0]
	var invs []struct {
		ID             uuid.UUID
		DocumentNo     string
		Kind           string
		DocDate        time.Time
		DueDate        time.Time
		Status         string
		TotalGross     decimal.Decimal
		ReceivedAmount decimal.Decimal
	}
	if err := database.Conn(ctx, r.db).Table("sales_billing_note_invoices AS bi").
		Select("i.id, i.document_no, i.kind, i.doc_date, i.due_date, i.status, i.total_gross, i.received_amount").
		Joins("JOIN sales_invoices AS i ON i.id = bi.invoice_id").Where("bi.billing_note_id = ?", h.ID).
		Order("bi.position").Scan(&invs).Error; err != nil {
		return dto.BillingNote{}, err
	}
	out := dto.BillingNote{BillingNoteSummary: h.summary(), Description: h.Description, CreatedBy: h.CreatedBy.String(),
		Invoices: make([]dto.BillingNoteInvoice, len(invs))}
	for i, x := range invs {
		outstanding := x.TotalGross.Sub(x.ReceivedAmount)
		if x.Status != string(vo.DocPosted) {
			outstanding = decimal.Zero
		}
		out.Invoices[i] = dto.BillingNoteInvoice{ID: x.ID.String(), DocumentNo: x.DocumentNo, Kind: x.Kind, Date: dateUTC(x.DocDate),
			DueDate: dateUTC(x.DueDate), Gross: x.TotalGross, Outstanding: outstanding}
	}
	return out, nil
}

// List — 1 query (+ COUNT ในหน้าเดียวกันด้วย window function)
func (r *BillingNoteRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.BillingNoteSummary], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "b.doc_date", "document_no": "b.document_no", "created_at": "b.created_at"}, "b.id")
	if err != nil {
		return listing.Result[dto.BillingNoteSummary]{}, err
	}
	q := r.base(ctx, companyID)
	// สถานะ paid / open คำนวณจากยอดค้าง (ในตารางทั้งสองเป็น status = 'open')
	var conds []string
	for _, st := range f.Statuses {
		switch st {
		case string(vo.BillingOpen):
			conds = append(conds, "(b.status = 'open' AND "+billingOutstanding+" > 0)")
		case string(vo.BillingPaid):
			conds = append(conds, "(b.status = 'open' AND "+billingOutstanding+" <= 0)")
		case string(vo.BillingCancelled):
			conds = append(conds, "b.status = 'cancelled'")
		}
	}
	if len(conds) > 0 {
		q = q.Where(strings.Join(conds, " OR "))
	}
	if f.CustomerID != nil {
		q = q.Where("b.customer_id = ?", uuidOf(*f.CustomerID))
	}
	if f.StartDate != nil {
		q = q.Where("b.doc_date >= ?", *f.StartDate)
	}
	if f.EndDate != nil {
		q = q.Where("b.doc_date <= ?", *f.EndDate)
	}
	q = search(q, f.Search, "b.document_no", "c.code", "c.name")
	var rows []billingRow
	if err := q.Select(billingColumns + ", COUNT(*) OVER () AS row_total").Order(order).
		Limit(f.Page.Limit()).Offset(f.Page.Offset()).Scan(&rows).Error; err != nil {
		return listing.Result[dto.BillingNoteSummary]{}, err
	}
	res := listing.Result[dto.BillingNoteSummary]{Items: make([]dto.BillingNoteSummary, len(rows)), Page: f.Page}
	for i, x := range rows {
		res.Items[i] = x.summary()
	}
	if len(rows) > 0 {
		res.Total = rows[0].RowTotal
	}
	return res, nil
}
