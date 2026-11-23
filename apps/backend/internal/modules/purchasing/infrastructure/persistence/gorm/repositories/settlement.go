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
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
)

// Open — 1 query: ใบแจ้งหนี้ที่ยังไม่ยกเลิกและยอดจ่าย < ยอดรวม
func (r *InvoiceRepository) Open(ctx context.Context, companyID vo.CompanyID, supplierID vo.SupplierID) ([]dto.OpenInvoice, error) {
	var rows []struct {
		ID                uuid.UUID
		DocumentNo        string
		SupplierInvoiceNo string
		InvoiceDate       time.Time
		DueDate           time.Time
		TotalNet          decimal.Decimal
		TotalGross        decimal.Decimal
		PaidAmount        decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("purchasing_invoices").
		Select("id, document_no, supplier_invoice_no, invoice_date, due_date, total_net, total_gross, paid_amount").
		Where("company_id = ? AND supplier_id = ? AND status = 'posted' AND paid_amount < total_gross", uuidOf(companyID), uuidOf(supplierID)).
		Order("due_date, document_no").Limit(500).Scan(&rows).Error
	out := make([]dto.OpenInvoice, len(rows))
	for i, x := range rows {
		out[i] = dto.OpenInvoice{ID: x.ID.String(), DocumentNo: x.DocumentNo, SupplierInvoiceNo: x.SupplierInvoiceNo,
			Date: gp.DateUTC(x.InvoiceDate), DueDate: gp.DateUTC(x.DueDate), Net: x.TotalNet, Gross: x.TotalGross, Outstanding: x.TotalGross.Sub(x.PaidAmount)}
	}
	return out, err
}

// ---------- ประวัติการตัดยอด (เอกสารจ่ายเงินของ finance) ----------

var _ ports.SettlementRepository = (*SettlementRepository)(nil)

type SettlementRepository struct{ db *gorm.DB }

func NewSettlementRepository(db *gorm.DB) *SettlementRepository { return &SettlementRepository{db: db} }

// Create — INSERT แบบ batch statement เดียว
func (r *SettlementRepository) Create(ctx context.Context, companyID vo.CompanyID, doc dto.SettlementDoc, lines []dto.SettlementLine) error {
	if len(lines) == 0 {
		return nil
	}
	now := time.Now().UTC()
	ms := make([]models.SettlementModel, len(lines))
	for i, l := range lines {
		ms[i] = models.SettlementModel{
			ID: uuid.Must(uuid.NewV7()), CompanyID: uuidOf(companyID), DocumentID: doc.ID, DocumentNo: doc.No, SettledOn: doc.Date,
			InvoiceID: uuidOf(l.InvoiceID), Amount: l.Amount, Status: "posted", CreatedAt: now,
		}
	}
	return database.Conn(ctx, r.db).Omit(clause.Associations).Create(&ms).Error
}

// LockDocument — 1 query (FOR UPDATE): ยอดที่ยังมีผลของเอกสาร
func (r *SettlementRepository) LockDocument(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) ([]dto.SettlementLine, error) {
	var ms []models.SettlementModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND document_id = ? AND status = 'posted'", uuidOf(companyID), documentID).Order("id").Find(&ms).Error
	out := make([]dto.SettlementLine, len(ms))
	for i, m := range ms {
		out[i] = dto.SettlementLine{InvoiceID: vo.InvoiceID(m.InvoiceID), Amount: m.Amount}
	}
	return out, err
}

func (r *SettlementRepository) Cancel(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return database.Conn(ctx, r.db).Model(&models.SettlementModel{}).
		Where("company_id = ? AND document_id = ? AND status = 'posted'", uuidOf(companyID), documentID).
		Update("status", "cancelled").Error
}
