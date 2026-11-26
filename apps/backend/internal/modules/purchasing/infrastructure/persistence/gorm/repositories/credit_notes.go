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

// ---------- ใบลดหนี้ผู้ขาย ----------

var _ ports.CreditNoteRepository = (*CreditNoteRepository)(nil)

type CreditNoteRepository struct{ db *gorm.DB }

func NewCreditNoteRepository(db *gorm.DB) *CreditNoteRepository { return &CreditNoteRepository{db: db} }

// ใบลดหนี้เก็บ order_id ของใบแจ้งหนี้เดิม จึงใช้ส่วนร่วม (JOIN ใบสั่งซื้อ + ผู้ขาย) เดียวกับใบรับ / ใบแจ้งหนี้
var creditNoteTable = docTable{table: "purchasing_credit_notes", date: "posting_date", notFound: domain.ErrCreditNoteNotFound}

func (r *CreditNoteRepository) Create(ctx context.Context, c *entities.CreditNote) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.CreditNoteToModel(c)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			if database.UniqueConstraint(err) == "uq_pur_credit_notes_supplier_no" {
				return domain.ErrCreditNoteNoExists
			}
			return domain.ErrDocumentNoUsed
		}
		return err
	}
	return tx.Create(&m.Lines).Error
}

func (r *CreditNoteRepository) UpdateStatus(ctx context.Context, c *entities.CreditNote) error {
	m := gp.CreditNoteToModel(c)
	return updateStatus(ctx, r.db, &models.CreditNoteModel{}, m.CompanyID, m.ID, m.Status, m.CancelledBy, m.CancelledAt, domain.ErrCreditNoteNotFound)
}

func (r *CreditNoteRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (*entities.CreditNote, error) {
	db := database.Conn(ctx, r.db)
	var m models.CreditNoteModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrCreditNoteNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("credit_note_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.CreditNoteToEntity(m), nil
}

func (r *CreditNoteRepository) SupplierNoUsed(ctx context.Context, companyID vo.CompanyID, supplierID vo.SupplierID, no string) (bool, error) {
	return exists(ctx, r.db, []string{`SELECT 1 FROM purchasing_credit_notes
		WHERE company_id = ? AND supplier_id = ? AND supplier_credit_note_no = ? AND status = 'posted'`},
		uuidOf(companyID), uuidOf(supplierID), no)
}

func (r *CreditNoteRepository) InvoiceCredits(ctx context.Context, companyID vo.CompanyID, invoiceID vo.InvoiceID) (map[int]dto.LineCredit, error) {
	return lineCredits(ctx, r.db, uuidOf(companyID), uuidOf(invoiceID))
}

func (r *CreditNoteRepository) HasPosted(ctx context.Context, companyID vo.CompanyID, invoiceID vo.InvoiceID) (bool, error) {
	return exists(ctx, r.db, []string{`SELECT 1 FROM purchasing_credit_notes WHERE company_id = ? AND invoice_id = ? AND status = 'posted'`},
		uuidOf(companyID), uuidOf(invoiceID))
}

// lineCredits — 1 query: ยอดส่งคืน / ลดหนี้ (ก่อน VAT) ต่อ line_no ของใบแจ้งหนี้ จากใบลดหนี้ที่ยังไม่ยกเลิก
func lineCredits(ctx context.Context, db *gorm.DB, companyID, invoiceID uuid.UUID) (map[int]dto.LineCredit, error) {
	var rows []struct {
		InvoiceLineNo int
		Returned      decimal.Decimal
		Net           decimal.Decimal
	}
	err := database.Conn(ctx, db).Table("purchasing_credit_note_lines AS l").
		Joins("JOIN purchasing_credit_notes AS c ON c.id = l.credit_note_id").
		Where("c.company_id = ? AND c.invoice_id = ? AND c.status = 'posted'", companyID, invoiceID).
		Select(`l.invoice_line_no, SUM(CASE WHEN c.kind = 'return' THEN l.quantity ELSE 0 END) AS returned, SUM(l.net) AS net`).
		Group("l.invoice_line_no").Scan(&rows).Error
	out := make(map[int]dto.LineCredit, len(rows))
	for _, x := range rows {
		out[x.InvoiceLineNo] = dto.LineCredit{Returned: x.Returned, Net: x.Net}
	}
	return out, err
}

type creditNoteRow struct {
	DocRow
	InvoiceID            uuid.UUID
	InvoiceNo            string
	SupplierInvoiceNo    string
	Kind                 string
	SupplierCreditNoteNo string
	CreditNoteDate       time.Time
	Reason               string
	TotalNet             decimal.Decimal
	TotalVat             decimal.Decimal
	TotalGross           decimal.Decimal
	Description          string
	VatRate              decimal.Decimal
	PriceIncludesVat     bool
	WarehouseID          *uuid.UUID
	System               string
	StockDocumentNo      string
	CreatedBy            uuid.UUID
	BillFromName         string
	BillFromTaxID        string
	BillFromBranchNo     string
	BillFromAddress      string
	Total                int64
}

func (r creditNoteRow) summary() dto.CreditNoteSummary {
	return dto.CreditNoteSummary{
		DocumentSummary: r.DocRow.summary(), Invoice: dto.Ref{ID: r.InvoiceID.String(), Code: r.InvoiceNo, Name: r.SupplierInvoiceNo},
		Kind: r.Kind, SupplierCreditNoteNo: r.SupplierCreditNoteNo, CreditNoteDate: gp.DateUTC(r.CreditNoteDate), Reason: r.Reason,
		Totals: dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross},
	}
}

var (
	creditNoteSummaryColumns = creditNoteTable.columns(`d.invoice_id, i.document_no AS invoice_no, i.supplier_invoice_no, d.kind,
		d.supplier_credit_note_no, d.credit_note_date, d.reason, d.total_net, d.total_vat, d.total_gross`)
	creditNoteColumns = creditNoteSummaryColumns + `, d.description, d.vat_rate, d.price_includes_vat, d.warehouse_id, d.system,
		d.stock_document_no, d.created_by, d.bill_from_name, d.bill_from_tax_id, d.bill_from_branch_no, d.bill_from_address`
)

func creditNoteBase(ctx context.Context, db *gorm.DB, companyID vo.CompanyID) *gorm.DB {
	return creditNoteTable.base(ctx, db, companyID).Joins("JOIN purchasing_invoices AS i ON i.id = d.invoice_id")
}

// Get — 2 query: หัว (JOIN ใบสั่งซื้อ + ผู้ขาย + ใบแจ้งหนี้เดิม) + บรรทัด
func (r *CreditNoteRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (dto.CreditNote, error) {
	var rows []creditNoteRow
	if err := creditNoteBase(ctx, r.db, companyID).Select(creditNoteColumns).Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.CreditNote{}, err
	}
	if len(rows) == 0 {
		return dto.CreditNote{}, domain.ErrCreditNoteNotFound
	}
	h := rows[0]
	out := dto.CreditNote{
		CreditNoteSummary: h.summary(), Description: h.Description, VatRate: h.VatRate, PriceIncludesVat: h.PriceIncludesVat,
		WarehouseID: optString(h.WarehouseID), System: h.System, StockDocumentNo: h.StockDocumentNo, CreatedBy: h.CreatedBy.String(),
		BillFrom: dto.BillFrom{Name: h.BillFromName, TaxID: h.BillFromTaxID, BranchNo: h.BillFromBranchNo, Address: h.BillFromAddress},
	}
	var lines []models.CreditNoteLineModel
	if err := database.Conn(ctx, r.db).Where("credit_note_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.CreditNote{}, err
	}
	out.Lines = make([]dto.CreditNoteLine, len(lines))
	for i, l := range lines {
		out.Lines[i] = dto.CreditNoteLine{
			LineNo: l.LineNo, InvoiceLineNo: l.InvoiceLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, AccountID: l.AccountID.String(), Stock: l.Stock, Cost: l.Cost,
			InventoryAccountID: optString(l.InventoryAccountID), Variance: l.Variance,
		}
	}
	return out, nil
}

func (r *CreditNoteRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.CreditNoteSummary], error) {
	order, err := creditNoteTable.order(f.Sort)
	if err != nil {
		return listing.Result[dto.CreditNoteSummary]{}, err
	}
	filtered := creditNoteTable.filtered(ctx, r.db, companyID, f, "d.supplier_credit_note_no", "i.supplier_invoice_no", "i.document_no")
	base := func() *gorm.DB {
		q := filtered().Joins("JOIN purchasing_invoices AS i ON i.id = d.invoice_id")
		if f.InvoiceID != nil {
			q = q.Where("d.invoice_id = ?", uuidOf(*f.InvoiceID))
		}
		return q
	}
	rows, total, err := database.FetchPage(base, creditNoteSummaryColumns, order, f.Page, func(r creditNoteRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, creditNoteRow.summary), err
}
