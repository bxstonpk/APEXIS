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

// postedTable คือส่วนร่วมของใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ: JOIN ลูกค้า (+ ใบสั่งขาย), กรอง, เรียง, แบ่งหน้า
type postedTable struct {
	table     string
	withOrder bool
}

// PostedRow: exported เพื่อให้ gorm scan ผ่าน struct ที่ embed ได้
type PostedRow struct {
	ID           uuid.UUID
	DocumentNo   string
	DocDate      time.Time
	OrderID      *uuid.UUID
	OrderNo      *string
	CustomerID   uuid.UUID
	CustomerCode string
	CustomerName string
	Status       string
	EntryID      *uuid.UUID
	EntryNo      string
	CancelledAt  *time.Time
	Total        int64
}

func (r PostedRow) summary() dto.PostedSummary {
	out := dto.PostedSummary{
		ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: dateUTC(r.DocDate),
		Customer: dto.Ref{ID: r.CustomerID.String(), Code: r.CustomerCode, Name: r.CustomerName},
		Status:   r.Status, EntryID: optString(r.EntryID), EntryNo: r.EntryNo, CancelledAt: r.CancelledAt,
	}
	if r.OrderID != nil {
		out.Order.ID = r.OrderID.String()
	}
	if r.OrderNo != nil {
		out.Order.Code = *r.OrderNo
	}
	return out
}

func (t postedTable) columns(extra string) string {
	order := "NULL::uuid AS order_id, NULL::text AS order_no"
	if t.withOrder {
		order = "d.order_id, o.order_no"
	}
	c := `d.id, d.document_no, d.doc_date, ` + order + `, d.customer_id, c.code AS customer_code, c.name AS customer_name,
		d.status, d.entry_id, d.entry_no, d.cancelled_at`
	if extra != "" {
		c += ", " + extra
	}
	return c
}

func (t postedTable) base(ctx context.Context, db *gorm.DB, companyID vo.CompanyID) *gorm.DB {
	q := database.Conn(ctx, db).Table(t.table + " AS d").Joins("JOIN sales_customers AS c ON c.id = d.customer_id")
	if t.withOrder {
		q = q.Joins("JOIN sales_orders AS o ON o.id = d.order_id")
	}
	return q.Where("d.company_id = ?", uuidOf(companyID))
}

func (t postedTable) filtered(ctx context.Context, db *gorm.DB, companyID vo.CompanyID, f dto.DocumentFilter, searchCols ...string) func() *gorm.DB {
	return func() *gorm.DB {
		q := t.base(ctx, db, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.CustomerID != nil {
			q = q.Where("d.customer_id = ?", uuidOf(*f.CustomerID))
		}
		if f.OrderID != nil && t.withOrder {
			q = q.Where("d.order_id = ?", uuidOf(*f.OrderID))
		}
		if f.StartDate != nil {
			q = q.Where("d.doc_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("d.doc_date <= ?", *f.EndDate)
		}
		cols := append([]string{"d.document_no", "c.code", "c.name"}, searchCols...)
		if t.withOrder {
			cols = append(cols, "o.order_no")
		}
		return search(q, f.Search, cols...)
	}
}

func (t postedTable) order(sorts []listing.Sort) (string, error) {
	return database.OrderBy(sorts, map[string]string{"date": "d.doc_date", "document_no": "d.document_no", "created_at": "d.created_at"}, "d.id")
}

func lockHeader[M any](ctx context.Context, db *gorm.DB, companyID, id uuid.UUID, notFound error) (M, error) {
	var m M
	err := database.Conn(ctx, db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", companyID, id).Take(&m).Error
	if database.IsNotFound(err) {
		return m, notFound
	}
	return m, err
}

func createDoc(ctx context.Context, header any, children ...any) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	if err := tx.Omit(clause.Associations).Create(header).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrDocumentNoUsed
		}
		return err
	}
	for _, c := range children {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
	}
	return nil
}

// ---------- ใบส่งของ ----------

var _ ports.DeliveryRepository = (*DeliveryRepository)(nil)

type DeliveryRepository struct{ db *gorm.DB }

func NewDeliveryRepository(db *gorm.DB) *DeliveryRepository { return &DeliveryRepository{db: db} }

var deliveryTable = postedTable{table: "sales_deliveries", withOrder: true}

func (r *DeliveryRepository) Create(ctx context.Context, d *entities.Delivery) error {
	m := gp.DeliveryToModel(d)
	return createDoc(ctx, &m, &m.Lines)
}

func (r *DeliveryRepository) UpdateStatus(ctx context.Context, d *entities.Delivery) error {
	m := gp.DeliveryToModel(d)
	return updateStatus(ctx, r.db, &models.DeliveryModel{}, m.CompanyID, m.ID, m.PostedModel.Status, m.PostedModel.CancelledBy, m.PostedModel.CancelledAt, domain.ErrDeliveryNotFound)
}

func (r *DeliveryRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.DeliveryID) (*entities.Delivery, error) {
	m, err := lockHeader[models.DeliveryModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrDeliveryNotFound)
	if err != nil {
		return nil, err
	}
	if err := database.Conn(ctx, r.db).Where("delivery_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.DeliveryToEntity(m), nil
}

type stockRow struct {
	WarehouseID     *uuid.UUID
	System          string
	StockDocumentNo string
}

func (s stockRow) dto() dto.StockInfo {
	return dto.StockInfo{WarehouseID: optString(s.WarehouseID), System: s.System, StockDocumentNo: s.StockDocumentNo}
}

// Get — 2 query: หัว (JOIN ลูกค้า + ใบสั่งขาย) + บรรทัด
func (r *DeliveryRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.DeliveryID) (dto.Delivery, error) {
	var rows []struct {
		PostedRow
		WarehouseID     *uuid.UUID
		System          string
		StockDocumentNo string
		TotalCost       decimal.Decimal
		Reference       string
		Description     string
	}
	err := deliveryTable.base(ctx, r.db, companyID).
		Select(deliveryTable.columns("d.warehouse_id, d.system, d.stock_document_no, d.total_cost, d.reference, d.description")).
		Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error
	if err != nil {
		return dto.Delivery{}, err
	}
	if len(rows) == 0 {
		return dto.Delivery{}, domain.ErrDeliveryNotFound
	}
	h := rows[0]
	var lines []models.DeliveryLineModel
	if err := database.Conn(ctx, r.db).Where("delivery_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Delivery{}, err
	}
	out := dto.Delivery{
		PostedSummary: h.PostedRow.summary(), StockInfo: stockRow{h.WarehouseID, h.System, h.StockDocumentNo}.dto(),
		TotalCost: h.TotalCost, Reference: h.Reference, Description: h.Description, Lines: make([]dto.DeliveryLine, len(lines)),
	}
	for i, l := range lines {
		out.Lines[i] = dto.DeliveryLine{OrderLineNo: l.OrderLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Quantity: l.Quantity, Cost: l.Cost}
	}
	return out, nil
}

func (r *DeliveryRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.PostedSummary], error) {
	order, err := deliveryTable.order(f.Sort)
	if err != nil {
		return listing.Result[dto.PostedSummary]{}, err
	}
	rows, total, err := database.FetchPage(deliveryTable.filtered(ctx, r.db, companyID, f, "d.reference"), deliveryTable.columns(""), order, f.Page,
		func(r PostedRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, PostedRow.summary), err
}

// ---------- ใบแจ้งหนี้ ----------

var _ ports.InvoiceRepository = (*InvoiceRepository)(nil)

type InvoiceRepository struct{ db *gorm.DB }

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository { return &InvoiceRepository{db: db} }

var invoiceTable = postedTable{table: "sales_invoices", withOrder: true}

func (r *InvoiceRepository) Create(ctx context.Context, i *entities.Invoice) error {
	m := gp.InvoiceToModel(i)
	return createDoc(ctx, &m, &m.Lines)
}

func (r *InvoiceRepository) UpdateStatus(ctx context.Context, i *entities.Invoice) error {
	m := gp.InvoiceToModel(i)
	return updateStatus(ctx, r.db, &models.InvoiceModel{}, m.CompanyID, m.ID, m.PostedModel.Status, m.PostedModel.CancelledBy, m.PostedModel.CancelledAt, domain.ErrInvoiceNotFound)
}

func (r *InvoiceRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (*entities.Invoice, error) {
	m, err := lockHeader[models.InvoiceModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrInvoiceNotFound)
	if err != nil {
		return nil, err
	}
	if err := database.Conn(ctx, r.db).Where("invoice_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.InvoiceToEntity(m), nil
}

// LockForSettlement — 1 query: หัวใบแจ้งหนี้ FOR UPDATE เรียงตาม id (ลำดับ lock คงที่ กัน deadlock)
func (r *InvoiceRepository) LockForSettlement(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) ([]*entities.Invoice, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var ms []models.InvoiceModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id IN ?", uuidOf(companyID), uuids(ids)).Order("id").Find(&ms).Error
	out := make([]*entities.Invoice, len(ms))
	for i, m := range ms {
		out[i] = gp.InvoiceToEntity(m)
	}
	return out, err
}

// SaveReceived — statement เดียว: UPDATE ... FROM (VALUES ...)
func (r *InvoiceRepository) SaveReceived(ctx context.Context, invoices []*entities.Invoice) error {
	if len(invoices) == 0 {
		return nil
	}
	values := make([]string, len(invoices))
	args := make([]any, 0, 2*len(invoices)+1)
	for i, inv := range invoices {
		values[i] = "(?::uuid, ?::numeric)"
		args = append(args, uuidOf(inv.ID()), inv.State().Received)
	}
	args = append(args, uuidOf(invoices[0].State().CompanyID))
	return database.Conn(ctx, r.db).Exec(`UPDATE sales_invoices AS i SET received_amount = v.received, updated_at = now()
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(id, received)
		WHERE i.id = v.id AND i.company_id = ?`, args...).Error
}

type InvoiceRow struct {
	PostedRow
	WithDelivery   bool
	DueDate        time.Time
	TotalNet       decimal.Decimal
	TotalVat       decimal.Decimal
	TotalGross     decimal.Decimal
	ReceivedAmount decimal.Decimal
}

func (r InvoiceRow) summary() dto.InvoiceSummary {
	return dto.InvoiceSummary{
		PostedSummary: r.PostedRow.summary(), WithDelivery: r.WithDelivery, DueDate: dateUTC(r.DueDate),
		Totals:   dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross},
		Received: r.ReceivedAmount, Outstanding: r.TotalGross.Sub(r.ReceivedAmount),
	}
}

const invoiceExtra = "d.with_delivery, d.due_date, d.total_net, d.total_vat, d.total_gross, d.received_amount"

func (r *InvoiceRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error) {
	var rows []struct {
		InvoiceRow
		WarehouseID      *uuid.UUID
		System           string
		StockDocumentNo  string
		VatRate          decimal.Decimal
		PriceIncludesVat bool
		Reference        string
		Description      string
		BillToName       string
		BillToTaxID      string
		BillToBranchNo   string
		BillToAddress    string
	}
	err := invoiceTable.base(ctx, r.db, companyID).
		Select(invoiceTable.columns(invoiceExtra+", d.warehouse_id, d.system, d.stock_document_no, d.vat_rate, d.price_includes_vat, d.reference, d.description, d.bill_to_name, d.bill_to_tax_id, d.bill_to_branch_no, d.bill_to_address")).
		Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error
	if err != nil {
		return dto.Invoice{}, err
	}
	if len(rows) == 0 {
		return dto.Invoice{}, domain.ErrInvoiceNotFound
	}
	h := rows[0]
	var lines []models.InvoiceLineModel
	if err := database.Conn(ctx, r.db).Where("invoice_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Invoice{}, err
	}
	out := dto.Invoice{
		InvoiceSummary: h.InvoiceRow.summary(), StockInfo: stockRow{h.WarehouseID, h.System, h.StockDocumentNo}.dto(),
		VatRate: h.VatRate, PriceIncludesVat: h.PriceIncludesVat, Reference: h.Reference, Description: h.Description,
		BillTo: dto.BillTo{Name: h.BillToName, TaxID: h.BillToTaxID, BranchNo: h.BillToBranchNo, Address: h.BillToAddress},
		Lines:  make([]dto.InvoiceLine, len(lines)),
	}
	if !h.WithDelivery {
		out.StockInfo = dto.StockInfo{}
	}
	for i, l := range lines {
		out.Lines[i] = dto.InvoiceLine{
			OrderLineNo: l.OrderLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, RevenueAccountID: l.RevenueAccountID.String(), Cost: l.Cost,
		}
	}
	return out, nil
}

func (r *InvoiceRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.InvoiceSummary], error) {
	order, err := invoiceTable.order(f.Sort)
	if err != nil {
		return listing.Result[dto.InvoiceSummary]{}, err
	}
	rows, total, err := database.FetchPage(invoiceTable.filtered(ctx, r.db, companyID, f, "d.reference"), invoiceTable.columns(invoiceExtra), order, f.Page,
		func(r InvoiceRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, InvoiceRow.summary), err
}

// Open — 1 query: ใบแจ้งหนี้ที่ยังไม่ยกเลิกและยอดรับ < ยอดรวม (index: company_id + customer_id)
func (r *InvoiceRepository) Open(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID) ([]dto.OpenInvoice, error) {
	var rows []struct {
		ID             uuid.UUID
		DocumentNo     string
		DocDate        time.Time
		DueDate        time.Time
		TotalNet       decimal.Decimal
		TotalGross     decimal.Decimal
		ReceivedAmount decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("sales_invoices").
		Select("id, document_no, doc_date, due_date, total_net, total_gross, received_amount").
		Where("company_id = ? AND customer_id = ? AND status = 'posted' AND received_amount < total_gross", uuidOf(companyID), uuidOf(customerID)).
		Order("due_date, document_no").Limit(500).Scan(&rows).Error
	out := make([]dto.OpenInvoice, len(rows))
	for i, x := range rows {
		out[i] = dto.OpenInvoice{ID: x.ID.String(), DocumentNo: x.DocumentNo, Date: gp.DateUTC(x.DocDate), DueDate: gp.DateUTC(x.DueDate),
			Net: x.TotalNet, Gross: x.TotalGross, Outstanding: x.TotalGross.Sub(x.ReceivedAmount)}
	}
	return out, err
}

// ---------- ประวัติการตัดยอด (เอกสารรับเงินของ finance) ----------

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
