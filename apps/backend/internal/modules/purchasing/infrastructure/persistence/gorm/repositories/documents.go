package repositories

import (
	"context"
	"strings"
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

// docTable คือส่วนร่วมของใบรับสินค้า / ใบแจ้งหนี้: JOIN ใบสั่งซื้อ + ผู้ขาย, กรอง, เรียง, แบ่งหน้า
type docTable struct {
	table    string // ชื่อตาราง (alias d)
	date     string // คอลัมน์วันที่ของเอกสาร
	notFound error
}

// DocRow: exported เพื่อให้ gorm scan ผ่าน struct ที่ embed ได้
type DocRow struct {
	ID           uuid.UUID
	DocumentNo   string
	DocDate      time.Time
	OrderID      uuid.UUID
	OrderNo      *string
	SupplierID   uuid.UUID
	SupplierCode string
	SupplierName string
	Status       string
	EntryID      *uuid.UUID
	EntryNo      string
	CancelledAt  *time.Time
}

func (r DocRow) summary() dto.DocumentSummary {
	out := dto.DocumentSummary{
		ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: gp.DateUTC(r.DocDate), Order: dto.Ref{ID: r.OrderID.String()},
		Supplier: dto.Ref{ID: r.SupplierID.String(), Code: r.SupplierCode, Name: r.SupplierName},
		Status:   r.Status, EntryID: optString(r.EntryID), EntryNo: r.EntryNo, CancelledAt: r.CancelledAt,
	}
	if r.OrderNo != nil {
		out.Order.Code = *r.OrderNo
	}
	return out
}

func (t docTable) columns(extra string) string {
	return `d.id, d.document_no, d.` + t.date + ` AS doc_date, d.order_id, o.order_no, d.supplier_id,
		s.code AS supplier_code, s.name AS supplier_name, d.status, d.entry_id, d.entry_no, d.cancelled_at, ` + extra
}

func (t docTable) base(ctx context.Context, db *gorm.DB, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, db).Table(t.table+" AS d").
		Joins("JOIN purchasing_orders AS o ON o.id = d.order_id").
		Joins("JOIN purchasing_suppliers AS s ON s.id = d.supplier_id").
		Where("d.company_id = ?", uuidOf(companyID))
}

func (t docTable) filtered(ctx context.Context, db *gorm.DB, companyID vo.CompanyID, f dto.DocumentFilter, searchCols ...string) func() *gorm.DB {
	return func() *gorm.DB {
		q := t.base(ctx, db, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.SupplierID != nil {
			q = q.Where("d.supplier_id = ?", uuidOf(*f.SupplierID))
		}
		if f.OrderID != nil {
			q = q.Where("d.order_id = ?", uuidOf(*f.OrderID))
		}
		if f.StartDate != nil {
			q = q.Where("d."+t.date+" >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("d."+t.date+" <= ?", *f.EndDate)
		}
		return search(q, f.Search, append([]string{"d.document_no", "o.order_no", "s.code", "s.name"}, searchCols...)...)
	}
}

func (t docTable) order(sorts []listing.Sort) (string, error) {
	return database.OrderBy(sorts, map[string]string{"date": "d." + t.date, "document_no": "d.document_no"}, "d.id")
}

// ---------- ใบรับสินค้า ----------

var _ ports.ReceiptRepository = (*ReceiptRepository)(nil)

type ReceiptRepository struct{ db *gorm.DB }

func NewReceiptRepository(db *gorm.DB) *ReceiptRepository { return &ReceiptRepository{db: db} }

var receiptTable = docTable{table: "purchasing_receipts", date: "receipt_date", notFound: domain.ErrReceiptNotFound}

func (r *ReceiptRepository) Create(ctx context.Context, rc *entities.Receipt) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.ReceiptToModel(rc)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrDocumentNoUsed
		}
		return err
	}
	return tx.Create(&m.Lines).Error
}

func (r *ReceiptRepository) UpdateStatus(ctx context.Context, rc *entities.Receipt) error {
	m := gp.ReceiptToModel(rc)
	return updateStatus(ctx, r.db, &models.ReceiptModel{}, m.CompanyID, m.ID, m.Status, m.CancelledBy, m.CancelledAt, domain.ErrReceiptNotFound)
}

func (r *ReceiptRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (*entities.Receipt, error) {
	db := database.Conn(ctx, r.db)
	var m models.ReceiptModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("receipt_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.ReceiptToEntity(m), nil
}

type receiptRow struct {
	DocRow
	WarehouseID     *uuid.UUID
	StockDocumentNo string
	System          string
	TotalValue      decimal.Decimal
	Reference       string
	Description     string
	CreatedBy       uuid.UUID
	Total           int64
}

func (r receiptRow) summary() dto.ReceiptSummary {
	return dto.ReceiptSummary{
		DocumentSummary: r.DocRow.summary(), WarehouseID: optString(r.WarehouseID), StockDocumentNo: r.StockDocumentNo,
		System: r.System, TotalValue: r.TotalValue,
	}
}

var (
	receiptSummaryColumns = receiptTable.columns("d.warehouse_id, d.stock_document_no, d.system, d.total_value")
	receiptColumns        = receiptSummaryColumns + ", d.reference, d.description, d.created_by"
)

// Get — 2 query: หัว (JOIN ใบสั่งซื้อ + ผู้ขาย) + บรรทัด
func (r *ReceiptRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (dto.Receipt, error) {
	var rows []receiptRow
	if err := receiptTable.base(ctx, r.db, companyID).Select(receiptColumns).Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Receipt{}, err
	}
	if len(rows) == 0 {
		return dto.Receipt{}, domain.ErrReceiptNotFound
	}
	h := rows[0]
	out := dto.Receipt{ReceiptSummary: h.summary(), Reference: h.Reference, Description: h.Description, CreatedBy: h.CreatedBy.String()}
	var lines []models.ReceiptLineModel
	if err := database.Conn(ctx, r.db).Where("receipt_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Receipt{}, err
	}
	out.Lines = make([]dto.ReceiptLine, len(lines))
	for i, l := range lines {
		out.Lines[i] = dto.ReceiptLine{
			OrderLineNo: l.OrderLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Quantity: l.Quantity, UnitCost: l.UnitCost,
			Value: l.Value, AccountID: l.AccountID.String(),
		}
	}
	return out, nil
}

func (r *ReceiptRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.ReceiptSummary], error) {
	order, err := receiptTable.order(f.Sort)
	if err != nil {
		return listing.Result[dto.ReceiptSummary]{}, err
	}
	base := receiptTable.filtered(ctx, r.db, companyID, f, "d.reference", "d.stock_document_no")
	rows, total, err := database.FetchPage(base, receiptSummaryColumns, order, f.Page, func(r receiptRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, receiptRow.summary), err
}

// ---------- ใบแจ้งหนี้ ----------

var _ ports.InvoiceRepository = (*InvoiceRepository)(nil)

type InvoiceRepository struct{ db *gorm.DB }

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository { return &InvoiceRepository{db: db} }

var invoiceTable = docTable{table: "purchasing_invoices", date: "posting_date", notFound: domain.ErrInvoiceNotFound}

func (r *InvoiceRepository) Create(ctx context.Context, inv *entities.Invoice) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.InvoiceToModel(inv)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrDocumentNoUsed
		}
		return err
	}
	return tx.Create(&m.Lines).Error
}

// LockForSettlement — 1 query: หัวใบแจ้งหนี้ FOR UPDATE เรียงตาม id (ลำดับ lock คงที่ กัน deadlock)
func (r *InvoiceRepository) LockForSettlement(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) ([]*entities.Invoice, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		u[i] = uuidOf(id)
	}
	var ms []models.InvoiceModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id IN ?", uuidOf(companyID), u).Order("id").Find(&ms).Error
	out := make([]*entities.Invoice, len(ms))
	for i, m := range ms {
		out[i] = gp.InvoiceToEntity(m)
	}
	return out, err
}

// SavePaid — statement เดียว: UPDATE ... FROM (VALUES ...)
func (r *InvoiceRepository) SavePaid(ctx context.Context, invoices []*entities.Invoice) error {
	if len(invoices) == 0 {
		return nil
	}
	values := make([]string, len(invoices))
	args := make([]any, 0, 2*len(invoices)+1)
	for i, inv := range invoices {
		values[i] = "(?::uuid, ?::numeric)"
		args = append(args, uuidOf(inv.ID()), inv.State().Paid)
	}
	args = append(args, uuidOf(invoices[0].State().CompanyID))
	return database.Conn(ctx, r.db).Exec(`UPDATE purchasing_invoices AS i SET paid_amount = v.paid, updated_at = now()
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(id, paid)
		WHERE i.id = v.id AND i.company_id = ?`, args...).Error
}

func (r *InvoiceRepository) SupplierInvoiceNoUsed(ctx context.Context, companyID vo.CompanyID, supplierID vo.SupplierID, no string) (bool, error) {
	return exists(ctx, r.db, []string{`SELECT 1 FROM purchasing_invoices
		WHERE company_id = ? AND supplier_id = ? AND supplier_invoice_no = ? AND status = 'posted'`},
		uuidOf(companyID), uuidOf(supplierID), no)
}

func (r *InvoiceRepository) UpdateStatus(ctx context.Context, inv *entities.Invoice) error {
	m := gp.InvoiceToModel(inv)
	return updateStatus(ctx, r.db, &models.InvoiceModel{}, m.CompanyID, m.ID, m.Status, m.CancelledBy, m.CancelledAt, domain.ErrInvoiceNotFound)
}

func (r *InvoiceRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (*entities.Invoice, error) {
	db := database.Conn(ctx, r.db)
	var m models.InvoiceModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrInvoiceNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("invoice_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.InvoiceToEntity(m), nil
}

type invoiceRow struct {
	DocRow
	SupplierInvoiceNo string
	InvoiceDate       time.Time
	DueDate           time.Time
	TotalNet          decimal.Decimal
	TotalVat          decimal.Decimal
	TotalGross        decimal.Decimal
	PaidAmount        decimal.Decimal
	CreditedAmount    decimal.Decimal
	Description       string
	VatRate           decimal.Decimal
	PriceIncludesVat  bool
	CreatedBy         uuid.UUID
	BillFromName      string
	BillFromTaxID     string
	BillFromBranchNo  string
	BillFromAddress   string
	Total             int64
}

func (r invoiceRow) summary() dto.InvoiceSummary {
	return dto.InvoiceSummary{
		DocumentSummary: r.DocRow.summary(), SupplierInvoiceNo: r.SupplierInvoiceNo, InvoiceDate: gp.DateUTC(r.InvoiceDate),
		DueDate: gp.DateUTC(r.DueDate), Totals: dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross},
		// paid_amount รวมยอดตัดจากใบลดหนี้ (ตัดยอดผ่าน purchasing_invoice_settlements เหมือนใบจ่าย) — แยกให้เห็นเป็นยอดจ่าย / ลดหนี้
		Paid: r.PaidAmount.Sub(r.CreditedAmount), Credited: r.CreditedAmount, Outstanding: r.TotalGross.Sub(r.PaidAmount),
	}
}

var (
	invoiceSummaryColumns = invoiceTable.columns(`d.supplier_invoice_no, d.invoice_date, d.due_date, d.total_net, d.total_vat, d.total_gross, d.paid_amount,
		COALESCE((SELECT SUM(c.total_gross) FROM purchasing_credit_notes AS c WHERE c.invoice_id = d.id AND c.status = 'posted'), 0) AS credited_amount`)
	invoiceColumns = invoiceSummaryColumns + ", d.description, d.vat_rate, d.price_includes_vat, d.created_by, d.bill_from_name, d.bill_from_tax_id, d.bill_from_branch_no, d.bill_from_address"
)

func (r *InvoiceRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error) {
	var rows []invoiceRow
	if err := invoiceTable.base(ctx, r.db, companyID).Select(invoiceColumns).Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Invoice{}, err
	}
	if len(rows) == 0 {
		return dto.Invoice{}, domain.ErrInvoiceNotFound
	}
	h := rows[0]
	out := dto.Invoice{
		InvoiceSummary: h.summary(), Description: h.Description, VatRate: h.VatRate, PriceIncludesVat: h.PriceIncludesVat,
		CreatedBy: h.CreatedBy.String(),
		BillFrom:  dto.BillFrom{Name: h.BillFromName, TaxID: h.BillFromTaxID, BranchNo: h.BillFromBranchNo, Address: h.BillFromAddress},
	}
	var lines []models.InvoiceLineModel
	if err := database.Conn(ctx, r.db).Where("invoice_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Invoice{}, err
	}
	credits, err := lineCredits(ctx, r.db, uuidOf(companyID), h.ID)
	if err != nil {
		return dto.Invoice{}, err
	}
	out.Lines = make([]dto.InvoiceLine, len(lines))
	for i, l := range lines {
		c := credits[l.LineNo]
		line := dto.InvoiceLine{
			LineNo: l.LineNo, OrderLineNo: l.OrderLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, Cleared: l.Cleared, Variance: l.Net.Sub(l.Cleared),
			Returned: c.Returned, CreditedNet: c.Net,
		}
		if l.VarianceAccountID != nil {
			line.VarianceAccountID = l.VarianceAccountID.String()
		}
		out.Lines[i] = line
	}
	return out, nil
}

func (r *InvoiceRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.InvoiceSummary], error) {
	order, err := invoiceTable.order(f.Sort)
	if err != nil {
		return listing.Result[dto.InvoiceSummary]{}, err
	}
	base := invoiceTable.filtered(ctx, r.db, companyID, f, "d.supplier_invoice_no")
	rows, total, err := database.FetchPage(base, invoiceSummaryColumns, order, f.Page, func(r invoiceRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, invoiceRow.summary), err
}

func updateStatus(ctx context.Context, db *gorm.DB, model any, companyID, id uuid.UUID, status string, by *uuid.UUID, at *time.Time, notFound error) error {
	res := database.Conn(ctx, db).Model(model).Where("company_id = ? AND id = ?", companyID, id).
		Updates(map[string]any{"status": status, "cancelled_by": by, "cancelled_at": at, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound
	}
	return nil
}
