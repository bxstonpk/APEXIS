package repositories

import (
	"context"
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

// ---------- ใบลดหนี้ ----------

var _ ports.CreditNoteRepository = (*CreditNoteRepository)(nil)

type CreditNoteRepository struct{ db *gorm.DB }

func NewCreditNoteRepository(db *gorm.DB) *CreditNoteRepository { return &CreditNoteRepository{db: db} }

func (r *CreditNoteRepository) Create(ctx context.Context, c *entities.CreditNote) error {
	m := gp.CreditNoteToModel(c)
	return createDoc(ctx, &m, &m.Lines)
}

func (r *CreditNoteRepository) UpdateStatus(ctx context.Context, c *entities.CreditNote) error {
	m := gp.CreditNoteToModel(c)
	return updateStatus(ctx, r.db, &models.CreditNoteModel{}, m.CompanyID, m.ID, m.PostedModel.Status, m.PostedModel.CancelledBy, m.PostedModel.CancelledAt, domain.ErrCreditNoteNotFound)
}

func (r *CreditNoteRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (*entities.CreditNote, error) {
	m, err := lockHeader[models.CreditNoteModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrCreditNoteNotFound)
	if err != nil {
		return nil, err
	}
	if err := database.Conn(ctx, r.db).Where("credit_note_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.CreditNoteToEntity(m), nil
}

// base: ใบลดหนี้ JOIN ลูกค้า + ใบแจ้งหนี้เดิม (Order ref ของ PostedRow = ใบแจ้งหนี้)
func (r *CreditNoteRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("sales_credit_notes AS d").
		Joins("JOIN sales_customers AS c ON c.id = d.customer_id").
		Joins("JOIN sales_invoices AS iv ON iv.id = d.invoice_id").
		Where("d.company_id = ?", uuidOf(companyID))
}

const creditNoteColumns = `d.id, d.document_no, d.doc_date, iv.id AS order_id, iv.document_no AS order_no, d.customer_id,
	c.code AS customer_code, c.name AS customer_name, d.status, d.entry_id, d.entry_no, d.cancelled_at,
	d.kind, d.reason, d.total_net, d.total_vat, d.total_gross`

type CreditNoteRow struct {
	PostedRow
	Kind       string
	Reason     string
	TotalNet   decimal.Decimal
	TotalVat   decimal.Decimal
	TotalGross decimal.Decimal
}

func (r CreditNoteRow) summary() dto.CreditNoteSummary {
	return dto.CreditNoteSummary{PostedSummary: r.PostedRow.summary(), Kind: r.Kind, Reason: r.Reason,
		Totals: dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross}}
}

// Get — 3 query: หัว (JOIN ลูกค้า + ใบแจ้งหนี้), ใบลดหนี้ก่อนหน้าของใบแจ้งหนี้เดียวกัน, บรรทัด
func (r *CreditNoteRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (dto.CreditNote, error) {
	var rows []struct {
		CreditNoteRow
		InvoiceID        uuid.UUID
		InvoiceDate      time.Time
		InvoiceNet       decimal.Decimal
		InvoiceVat       decimal.Decimal
		InvoiceGross     decimal.Decimal
		WarehouseID      *uuid.UUID
		System           string
		StockDocumentNo  string
		VatRate          decimal.Decimal
		PriceIncludesVat bool
		Description      string
		CreatedBy        uuid.UUID
		PostedAt         time.Time
		BillToName       string
		BillToTaxID      string
		BillToBranchNo   string
		BillToAddress    string
	}
	err := r.base(ctx, companyID).
		Select(creditNoteColumns+`, d.invoice_id, iv.doc_date AS invoice_date, iv.total_net AS invoice_net, iv.total_vat AS invoice_vat,
			iv.total_gross AS invoice_gross, d.warehouse_id, d.system, d.stock_document_no, d.vat_rate, d.price_includes_vat, d.description,
			d.created_by, d.posted_at, d.bill_to_name, d.bill_to_tax_id, d.bill_to_branch_no, d.bill_to_address`).
		Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error
	if err != nil {
		return dto.CreditNote{}, err
	}
	if len(rows) == 0 {
		return dto.CreditNote{}, domain.ErrCreditNoteNotFound
	}
	h := rows[0]
	// ใบลดหนี้ก่อนหน้า (ยังไม่ยกเลิก) ของใบแจ้งหนี้เดียวกัน — มูลค่าที่ถูกต้องตาม ม.86/10 = ใบเดิม - ลดก่อนหน้า - ใบนี้
	var before struct {
		Net   decimal.Decimal
		Vat   decimal.Decimal
		Gross decimal.Decimal
	}
	if err := database.Conn(ctx, r.db).Table("sales_credit_notes").
		Select("COALESCE(SUM(total_net), 0) AS net, COALESCE(SUM(total_vat), 0) AS vat, COALESCE(SUM(total_gross), 0) AS gross").
		Where("company_id = ? AND invoice_id = ? AND status = 'posted' AND id <> ? AND (doc_date < ? OR (doc_date = ? AND posted_at < ?))",
			uuidOf(companyID), h.InvoiceID, h.ID, h.DocDate, h.DocDate, h.PostedAt).
		Scan(&before).Error; err != nil {
		return dto.CreditNote{}, err
	}
	var lines []models.CreditNoteLineModel
	if err := database.Conn(ctx, r.db).Where("credit_note_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.CreditNote{}, err
	}
	out := dto.CreditNote{
		CreditNoteSummary: h.CreditNoteRow.summary(), StockInfo: stockRow{h.WarehouseID, h.System, h.StockDocumentNo}.dto(),
		InvoiceDate: dateUTC(h.InvoiceDate), InvoiceTotals: dto.Totals{Net: h.InvoiceNet, Vat: h.InvoiceVat, Gross: h.InvoiceGross},
		CreditedBefore: dto.Totals{Net: before.Net, Vat: before.Vat, Gross: before.Gross},
		BillTo:         dto.BillTo{Name: h.BillToName, TaxID: h.BillToTaxID, BranchNo: h.BillToBranchNo, Address: h.BillToAddress},
		VatRate:        h.VatRate, PriceIncludesVat: h.PriceIncludesVat, Description: h.Description, CreatedBy: h.CreatedBy.String(),
		Lines: make([]dto.CreditNoteLine, len(lines)),
	}
	if h.StockDocumentNo == "" {
		out.StockInfo.WarehouseID = nil
	}
	for i, l := range lines {
		out.Lines[i] = dto.CreditNoteLine{
			LineNo: l.LineNo, InvoiceLineNo: l.InvoiceLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, AccountID: l.AccountID.String(), Cost: l.Cost,
		}
	}
	return out, nil
}

// List — 1 query (+COUNT ด้วย window function): กรอง / เรียง / แบ่งหน้าใน DB
func (r *CreditNoteRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.CreditNoteSummary], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "d.doc_date", "document_no": "d.document_no", "created_at": "d.created_at"}, "d.id")
	if err != nil {
		return listing.Result[dto.CreditNoteSummary]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.CustomerID != nil {
			q = q.Where("d.customer_id = ?", uuidOf(*f.CustomerID))
		}
		if f.InvoiceID != nil {
			q = q.Where("d.invoice_id = ?", uuidOf(*f.InvoiceID))
		}
		if f.StartDate != nil {
			q = q.Where("d.doc_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("d.doc_date <= ?", *f.EndDate)
		}
		return search(q, f.Search, "d.document_no", "c.code", "c.name", "iv.document_no", "d.reason")
	}
	rows, total, err := database.FetchPage(base, creditNoteColumns, order, f.Page, func(r CreditNoteRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, CreditNoteRow.summary), err
}

// Credited — 1 query: จำนวนที่รับคืน / ยอดก่อน VAT ที่ลดแล้วต่อบรรทัดใบแจ้งหนี้ (ใบลดหนี้ที่ยังไม่ยกเลิก)
func (r *CreditNoteRepository) Credited(ctx context.Context, companyID vo.CompanyID, invoiceID vo.InvoiceID) (map[int]entities.Credited, error) {
	var rows []struct {
		InvoiceLineNo int
		Returned      decimal.Decimal
		Net           decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("sales_credit_note_lines AS l").
		Joins("JOIN sales_credit_notes AS n ON n.id = l.credit_note_id").
		Select("l.invoice_line_no, COALESCE(SUM(l.quantity) FILTER (WHERE n.kind = 'return'), 0) AS returned, SUM(l.net) AS net").
		Where("n.company_id = ? AND n.invoice_id = ? AND n.status = 'posted'", uuidOf(companyID), uuidOf(invoiceID)).
		Group("l.invoice_line_no").Scan(&rows).Error
	out := make(map[int]entities.Credited, len(rows))
	for _, x := range rows {
		out[x.InvoiceLineNo] = entities.Credited{Returned: x.Returned, Net: x.Net}
	}
	return out, err
}

func (r *CreditNoteRepository) HasPosted(ctx context.Context, companyID vo.CompanyID, invoiceID vo.InvoiceID) (bool, error) {
	return exists(ctx, r.db, []string{"SELECT 1 FROM sales_credit_notes WHERE company_id = ? AND invoice_id = ? AND status = 'posted'"},
		uuidOf(companyID), uuidOf(invoiceID))
}
