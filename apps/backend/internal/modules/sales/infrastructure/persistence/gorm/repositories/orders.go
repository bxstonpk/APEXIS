package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// salesDocTable คือส่วนร่วมของใบเสนอราคา / ใบสั่งขาย (หัว JOIN ลูกค้า)
type salesDocTable struct {
	table, noColumn, lineTable, fk string
	notFound                       error
}

// SalesDocRow: exported เพื่อให้ gorm scan ผ่าน struct ที่ embed ได้
type SalesDocRow struct {
	ID               uuid.UUID
	DocumentNo       *string
	DocDate          time.Time
	CustomerID       uuid.UUID
	CustomerCode     string
	CustomerName     string
	CustomerBranchNo string
	Reference        string
	VatRate          decimal.Decimal
	PriceIncludesVat bool
	TotalNet         decimal.Decimal
	TotalVat         decimal.Decimal
	TotalGross       decimal.Decimal
	Status           string
	Total            int64
}

func (r SalesDocRow) summary() dto.SalesDocSummary {
	out := dto.SalesDocSummary{
		ID: r.ID.String(), Date: dateUTC(r.DocDate), Customer: dto.Ref{ID: r.CustomerID.String(), Code: r.CustomerCode, Name: r.CustomerName},
		CustomerBranchNo: r.CustomerBranchNo, Reference: r.Reference, VatRate: r.VatRate, PriceIncludesVat: r.PriceIncludesVat,
		Totals: dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross}, Status: r.Status,
	}
	if r.DocumentNo != nil {
		out.DocumentNo = *r.DocumentNo
	}
	return out
}

func (t salesDocTable) columns() string {
	return `d.id, d.` + t.noColumn + ` AS document_no, d.doc_date, d.customer_id, c.code AS customer_code, c.name AS customer_name, d.customer_branch_no,
		d.reference, d.vat_rate, d.price_includes_vat, d.total_net, d.total_vat, d.total_gross, d.status`
}

// base: JOIN ลูกค้า (รวมที่ถูก soft delete — เอกสารเดิมต้องแสดงชื่อได้)
func (t salesDocTable) base(ctx context.Context, db *gorm.DB, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, db).Table(t.table+" AS d").Joins("JOIN sales_customers AS c ON c.id = d.customer_id").
		Where("d.company_id = ?", uuidOf(companyID))
}

func (t salesDocTable) list(ctx context.Context, db *gorm.DB, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.SalesDocSummary], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "d.doc_date", "document_no": "d." + t.noColumn, "created_at": "d.created_at"}, "d.id")
	if err != nil {
		return listing.Result[dto.SalesDocSummary]{}, err
	}
	base := func() *gorm.DB {
		q := t.base(ctx, db, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.CustomerID != nil {
			q = q.Where("d.customer_id = ?", uuidOf(*f.CustomerID))
		}
		if f.ItemID != nil {
			q = q.Where("EXISTS (SELECT 1 FROM "+t.lineTable+" AS l WHERE l."+t.fk+" = d.id AND l.item_id = ?)", uuidOf(*f.ItemID))
		}
		if f.StartDate != nil {
			q = q.Where("d.doc_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("d.doc_date <= ?", *f.EndDate)
		}
		return search(q, f.Search, "d."+t.noColumn, "d.reference", "d.description", "c.code", "c.name")
	}
	rows, total, err := database.FetchPage(base, t.columns(), order, f.Page, func(r SalesDocRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, SalesDocRow.summary), err
}

// header — 1 query: หัว + ลูกค้า + คอลัมน์เพิ่มของชนิดเอกสาร (extra ต้อง scan เข้า dst)
func (t salesDocTable) header(ctx context.Context, db *gorm.DB, companyID vo.CompanyID, id uuid.UUID, extra string, dst any) (bool, error) {
	res := t.base(ctx, db, companyID).Select(t.columns()+", "+extra).Where("d.id = ?", id).Limit(1).Scan(dst)
	return res.RowsAffected > 0, res.Error
}

func salesLines(ctx context.Context, db *gorm.DB, table, fk string, id uuid.UUID) ([]dto.SalesLine, error) {
	var rows []struct {
		models.LineModel
		DeliveredQty *decimal.Decimal
		InvoicedQty  *decimal.Decimal
	}
	cols := "line_no, item_id, description, quantity, unit_price, net, vat, gross"
	if table == "sales_order_lines" {
		cols += ", delivered_qty, invoiced_qty"
	}
	if err := database.Conn(ctx, db).Table(table).Select(cols).Where(fk+" = ?", id).Order("line_no").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]dto.SalesLine, len(rows))
	for i, l := range rows {
		line := dto.SalesLine{
			LineNo: l.LineNo, Item: dto.Ref{ID: l.ItemID.String()}, Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, Delivered: decimal.Zero, Invoiced: decimal.Zero,
		}
		if l.DeliveredQty != nil {
			line.Delivered, line.Invoiced = *l.DeliveredQty, *l.InvoicedQty
		}
		out[i] = line
	}
	return out, nil
}

// ---------- ใบเสนอราคา ----------

var _ ports.QuotationRepository = (*QuotationRepository)(nil)

type QuotationRepository struct{ db *gorm.DB }

func NewQuotationRepository(db *gorm.DB) *QuotationRepository { return &QuotationRepository{db: db} }

var quotationTable = salesDocTable{table: "sales_quotations", noColumn: "quotation_no", lineTable: "sales_quotation_lines", fk: "quotation_id", notFound: domain.ErrQuotationNotFound}

func (r *QuotationRepository) Create(ctx context.Context, q *entities.Quotation) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.QuotationToModel(q)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

func (r *QuotationRepository) Update(ctx context.Context, q *entities.Quotation) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.QuotationToModel(q)
	res := tx.Model(&models.QuotationModel{}).Where("company_id = ? AND id = ? AND status = 'draft'", m.CompanyID, m.ID).
		Select("customer_id", "customer_branch_no", "doc_date", "valid_until", "reference", "description", "vat_registered", "vat_rate", "price_includes_vat",
			"payment_term_days", "total_net", "total_vat", "total_gross", "updated_at").Updates(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotDraft
	}
	if err := tx.Where("quotation_id = ?", m.ID).Delete(&models.QuotationLineModel{}).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

func (r *QuotationRepository) SaveStatus(ctx context.Context, q *entities.Quotation) error {
	m := gp.QuotationToModel(q)
	res := database.Conn(ctx, r.db).Model(&models.QuotationModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{"status": m.Status, "quotation_no": m.QuotationNo, "fiscal_year": m.FiscalYear, "order_id": m.OrderID, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrDocumentNoUsed
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrQuotationNotFound
	}
	return nil
}

func (r *QuotationRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ? AND status = 'draft'", uuidOf(companyID), uuidOf(id)).Delete(&models.QuotationModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrQuotationNotFound
	}
	return nil
}

func (r *QuotationRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) (*entities.Quotation, error) {
	db := database.Conn(ctx, r.db)
	var m models.QuotationModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrQuotationNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("quotation_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.QuotationToEntity(m), nil
}

// Get — 2 query: หัว (JOIN ลูกค้า) + บรรทัด
func (r *QuotationRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) (dto.Quotation, error) {
	var h struct {
		SalesDocRow
		ValidUntil      time.Time
		Description     string
		PaymentTermDays int
		OrderID         *uuid.UUID
		CreatedBy       uuid.UUID
	}
	found, err := quotationTable.header(ctx, r.db, companyID, uuidOf(id), "d.valid_until, d.description, d.payment_term_days, d.order_id, d.created_by", &h)
	if err != nil {
		return dto.Quotation{}, err
	}
	if !found {
		return dto.Quotation{}, domain.ErrQuotationNotFound
	}
	lines, err := salesLines(ctx, r.db, "sales_quotation_lines", "quotation_id", h.ID)
	return dto.Quotation{
		SalesDocSummary: h.SalesDocRow.summary(), ValidUntil: dateUTC(h.ValidUntil), Description: h.Description,
		PaymentTermDays: h.PaymentTermDays, OrderID: optString(h.OrderID), CreatedBy: h.CreatedBy.String(), Lines: lines,
	}, err
}

func (r *QuotationRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.SalesDocSummary], error) {
	return quotationTable.list(ctx, r.db, companyID, f)
}

// ---------- ใบสั่งขาย ----------

var _ ports.OrderRepository = (*OrderRepository)(nil)

type OrderRepository struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) *OrderRepository { return &OrderRepository{db: db} }

var orderTable = salesDocTable{table: "sales_orders", noColumn: "order_no", lineTable: "sales_order_lines", fk: "order_id", notFound: domain.ErrOrderNotFound}

func (r *OrderRepository) Create(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModel(o)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrQuotationUsed
		}
		return err
	}
	return tx.Create(&m.Lines).Error
}

func (r *OrderRepository) Update(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModel(o)
	res := tx.Model(&models.OrderModel{}).Where("company_id = ? AND id = ? AND status = 'draft'", m.CompanyID, m.ID).
		Select("customer_id", "customer_branch_no", "doc_date", "delivery_date", "reference", "description", "vat_registered", "vat_rate", "price_includes_vat",
			"payment_term_days", "total_net", "total_vat", "total_gross", "updated_at").Updates(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotDraft
	}
	if err := tx.Where("order_id = ?", m.ID).Delete(&models.OrderLineModel{}).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

// SaveProgress — 2 statement: หัว + ความคืบหน้าทุกบรรทัด (UPDATE ... FROM (VALUES ...))
func (r *OrderRepository) SaveProgress(ctx context.Context, o *entities.Order) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.OrderToModel(o)
	res := tx.Model(&models.OrderModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{
			"status": m.Status, "order_no": m.OrderNo, "fiscal_year": m.FiscalYear, "approved_by": m.ApprovedBy, "approved_at": m.ApprovedAt,
			"closed_by": m.ClosedBy, "closed_at": m.ClosedAt, "cancelled_by": m.CancelledBy, "cancelled_at": m.CancelledAt, "updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrDocumentNoUsed
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrderNotFound
	}
	values := make([]string, len(m.Lines))
	args := make([]any, 0, 3*len(m.Lines)+1)
	for i, l := range m.Lines {
		values[i] = "(?::int, ?::numeric, ?::numeric)"
		args = append(args, l.LineNo, l.DeliveredQty, l.InvoicedQty)
	}
	args = append(args, m.ID)
	return tx.Exec(`UPDATE sales_order_lines AS l SET delivered_qty = v.delivered, invoiced_qty = v.invoiced
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(line_no, delivered, invoiced)
		WHERE l.line_no = v.line_no AND l.order_id = ?`, args...).Error
}

func (r *OrderRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ? AND status = 'draft'", uuidOf(companyID), uuidOf(id)).Delete(&models.OrderModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

func (r *OrderRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (*entities.Order, error) {
	db := database.Conn(ctx, r.db)
	var m models.OrderModel
	err := db.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("order_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.OrderToEntity(m), nil
}

func (r *OrderRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (dto.Order, error) {
	var h struct {
		SalesDocRow
		QuotationID     *uuid.UUID
		DeliveryDate    *time.Time
		Description     string
		PaymentTermDays int
		CreatedBy       uuid.UUID
		ApprovedAt      *time.Time
		ClosedAt        *time.Time
		CancelledAt     *time.Time
	}
	found, err := orderTable.header(ctx, r.db, companyID, uuidOf(id),
		"d.quotation_id, d.delivery_date, d.description, d.payment_term_days, d.created_by, d.approved_at, d.closed_at, d.cancelled_at", &h)
	if err != nil {
		return dto.Order{}, err
	}
	if !found {
		return dto.Order{}, domain.ErrOrderNotFound
	}
	lines, err := salesLines(ctx, r.db, "sales_order_lines", "order_id", h.ID)
	return dto.Order{
		SalesDocSummary: h.SalesDocRow.summary(), QuotationID: optString(h.QuotationID), DeliveryDate: dateOpt(h.DeliveryDate),
		Description: h.Description, PaymentTermDays: h.PaymentTermDays, CreatedBy: h.CreatedBy.String(),
		ApprovedAt: h.ApprovedAt, ClosedAt: h.ClosedAt, CancelledAt: h.CancelledAt, Lines: lines,
	}, err
}

func (r *OrderRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.SalesDocSummary], error) {
	return orderTable.list(ctx, r.db, companyID, f)
}

// Exposure — 1 query: ลูกหนี้คงค้าง + ใบสั่งขาย (อนุมัติ / ปิด) ส่วนที่ยังไม่ออกบิลตามสัดส่วนจำนวน
func (r *OrderRepository) Exposure(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, exclude vo.OrderID, amountDecimals int) (vo.Money, error) {
	var total decimal.Decimal
	c, cu := uuidOf(companyID), uuidOf(customerID)
	err := database.Conn(ctx, r.db).Raw(`
		SELECT COALESCE((SELECT SUM(i.total_gross - i.received_amount) FROM sales_invoices AS i
		                  WHERE i.company_id = ? AND i.customer_id = ? AND i.status = 'posted'), 0)
		     + COALESCE((SELECT SUM(l.gross * (l.quantity - l.invoiced_qty) / l.quantity)
		                   FROM sales_order_lines AS l JOIN sales_orders AS o ON o.id = l.order_id
		                  WHERE o.company_id = ? AND o.customer_id = ? AND o.status IN ('approved', 'closed') AND o.id <> ?), 0)`,
		c, cu, c, cu, uuidOf(exclude)).Scan(&total).Error
	return total.Round(int32(amountDecimals)), err
}
