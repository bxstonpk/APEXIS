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

// postedTable คือส่วนร่วมของเอกสารขาย / ใบลดหนี้: JOIN ลูกค้า (+ ใบสั่งขาย), กรอง, เรียง, แบ่งหน้า
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
		q = q.Joins("LEFT JOIN sales_orders AS o ON o.id = d.order_id")
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
		if f.Kind != "" && t.table == "sales_invoices" {
			q = q.Where("d.kind = ?", string(f.Kind))
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

type stockRow struct {
	WarehouseID     *uuid.UUID
	System          string
	StockDocumentNo string
}

func (s stockRow) dto() dto.StockInfo {
	return dto.StockInfo{WarehouseID: optString(s.WarehouseID), System: s.System, StockDocumentNo: s.StockDocumentNo}
}

// ---------- เอกสารขาย (ขายสินค้า / ขายบริการ / มัดจำ) ----------

var _ ports.InvoiceRepository = (*InvoiceRepository)(nil)

type InvoiceRepository struct{ db *gorm.DB }

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository { return &InvoiceRepository{db: db} }

var invoiceTable = postedTable{table: "sales_invoices", withOrder: true}

func (r *InvoiceRepository) Create(ctx context.Context, i *entities.Invoice) error {
	m, deps := gp.InvoiceToModel(i)
	children := []any{&m.Lines}
	if len(deps) > 0 {
		children = append(children, &deps)
	}
	if len(m.Lines) == 0 {
		children = children[1:]
	}
	return createDoc(ctx, &m, children...)
}

func (r *InvoiceRepository) UpdateStatus(ctx context.Context, i *entities.Invoice) error {
	m, _ := gp.InvoiceToModel(i)
	return updateStatus(ctx, r.db, &models.InvoiceModel{}, m.CompanyID, m.ID, m.PostedModel.Status, m.PostedModel.CancelledBy, m.PostedModel.CancelledAt, domain.ErrInvoiceNotFound)
}

// SaveState — UPDATE เดียว: ยอดที่ตัด / ภาษีที่ถึงกำหนด / มัดจำที่ถูกหัก / ใบกำกับภาษีก่อนรับเงิน
func (r *InvoiceRepository) SaveState(ctx context.Context, i *entities.Invoice) error {
	m, _ := gp.InvoiceToModel(i)
	return database.Conn(ctx, r.db).Model(&models.InvoiceModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{
			"received_amount": m.ReceivedAmount, "vat_recognized": m.VatRecognized, "vat_credited": m.VatCredited,
			"base_recognized": m.BaseRecognized, "base_credited": m.BaseCredited,
			"deposit_applied": m.DepositApplied, "deposit_applied_vat": m.DepositAppliedVat,
			"tax_invoice_no": m.TaxInvoiceNo, "tax_invoice_date": m.TaxInvoiceDate, "tax_invoice_entry_id": m.TaxInvoiceEntryID,
			"tax_invoice_entry_no": m.TaxInvoiceEntryNo, "tax_invoice_base": m.TaxInvoiceBase, "tax_invoice_vat": m.TaxInvoiceVat,
			"updated_at": time.Now().UTC(),
		}).Error
}

// deposits — 1 query: มัดจำที่หักของใบ (JOIN เลขที่มัดจำ)
func (r *InvoiceRepository) deposits(ctx context.Context, invoiceID uuid.UUID) ([]gp.DepositUseRow, error) {
	var rows []gp.DepositUseRow
	err := database.Conn(ctx, r.db).Table("sales_deposit_uses AS u").Select("u.*, d.document_no AS deposit_no").
		Joins("JOIN sales_invoices AS d ON d.id = u.deposit_id").Where("u.invoice_id = ?", invoiceID).Order("u.position").Scan(&rows).Error
	return rows, err
}

// FindForUpdate — 3 query: หัว (FOR UPDATE) + บรรทัด + มัดจำที่หัก
func (r *InvoiceRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (*entities.Invoice, error) {
	m, err := lockHeader[models.InvoiceModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrInvoiceNotFound)
	if err != nil {
		return nil, err
	}
	if err := database.Conn(ctx, r.db).Where("invoice_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	deps, err := r.deposits(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	return gp.InvoiceToEntity(m, deps), nil
}

// LockForSettlement — 1 query: หัวเอกสาร FOR UPDATE เรียงตาม id (ลำดับ lock คงที่ กัน deadlock)
func (r *InvoiceRepository) LockForSettlement(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) ([]*entities.Invoice, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var ms []models.InvoiceModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id IN ?", uuidOf(companyID), uuids(ids)).Order("id").Find(&ms).Error
	out := make([]*entities.Invoice, len(ms))
	for i, m := range ms {
		out[i] = gp.InvoiceToEntity(m, nil)
	}
	return out, err
}

// SaveReceived — statement เดียว: UPDATE ... FROM (VALUES ...) (ยอดที่ตัด + ภาษีที่ถึงกำหนด)
func (r *InvoiceRepository) SaveReceived(ctx context.Context, invoices []*entities.Invoice) error {
	if len(invoices) == 0 {
		return nil
	}
	values := make([]string, len(invoices))
	args := make([]any, 0, 4*len(invoices)+1)
	for i, inv := range invoices {
		values[i] = "(?::uuid, ?::numeric, ?::numeric, ?::numeric)"
		st := inv.State()
		args = append(args, uuidOf(inv.ID()), st.Received, st.Deferral.VatRecognized, st.Deferral.BaseRecognized)
	}
	args = append(args, uuidOf(invoices[0].State().CompanyID))
	return database.Conn(ctx, r.db).Exec(`UPDATE sales_invoices AS i SET received_amount = v.received, vat_recognized = v.vat,
			base_recognized = v.base, updated_at = now()
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(id, received, vat, base)
		WHERE i.id = v.id AND i.company_id = ?`, args...).Error
}

type InvoiceRow struct {
	PostedRow
	Kind              string
	DueDate           time.Time
	TotalNet          decimal.Decimal
	TotalVat          decimal.Decimal
	TotalGross        decimal.Decimal
	ReceivedAmount    decimal.Decimal
	CreditedAmount    decimal.Decimal
	DepositBase       decimal.Decimal
	DepositVat        decimal.Decimal
	VatDeferred       decimal.Decimal
	VatRecognized     decimal.Decimal
	VatCredited       decimal.Decimal
	TaxInvoiceNo      string
	TaxInvoiceDate    *time.Time
	DepositApplied    decimal.Decimal
	DepositAppliedVat decimal.Decimal
}

// summary: received_amount ในตารางรวมรับเงิน + ใบลดหนี้ + มัดจำที่หัก — แยกยอดรับชำระ = received - credited - มัดจำ
func (r InvoiceRow) summary() dto.InvoiceSummary {
	depGross := r.DepositBase.Add(r.DepositVat)
	out := dto.InvoiceSummary{
		PostedSummary: r.PostedRow.summary(), Kind: r.Kind, DueDate: dateUTC(r.DueDate),
		Totals:   dto.Totals{Net: r.TotalNet, Vat: r.TotalVat, Gross: r.TotalGross},
		Received: r.ReceivedAmount.Sub(r.CreditedAmount).Sub(depGross), Credited: r.CreditedAmount, DepositGross: depGross,
		Outstanding: r.TotalGross.Sub(r.ReceivedAmount), VatStatus: "output", VatRecognized: decimal.Zero, VatUndue: decimal.Zero,
	}
	if r.Kind == string(vo.KindServiceInvoice) {
		undue := r.VatDeferred.Sub(r.VatRecognized).Sub(r.VatCredited)
		out.VatRecognized, out.VatUndue = r.VatRecognized, undue
		switch {
		case r.VatRecognized.IsZero() && undue.IsPositive():
			out.VatStatus = "undue"
		case undue.IsPositive():
			out.VatStatus = "partial"
		}
	} else {
		out.VatRecognized = r.TotalVat
	}
	if r.TaxInvoiceDate != nil {
		out.TaxInvoice = &dto.TaxInvoiceRef{No: r.TaxInvoiceNo, Date: dateUTC(*r.TaxInvoiceDate)}
	}
	if r.Kind == string(vo.KindDeposit) {
		rem, applied := r.TotalNet.Sub(r.DepositApplied), r.DepositApplied
		out.DepositRemaining, out.DepositApplied = &rem, &applied
	}
	return out
}

// invoiceExtra: ยอดใบลดหนี้ (ยังไม่ยกเลิก) เป็น subquery ต่อแถว (index: sales_credit_notes.invoice_id)
const invoiceExtra = `d.kind, d.due_date, d.total_net, d.total_vat, d.total_gross, d.received_amount,
	(SELECT COALESCE(SUM(n.total_gross), 0) FROM sales_credit_notes AS n WHERE n.invoice_id = d.id AND n.status = 'posted') AS credited_amount,
	d.deposit_base, d.deposit_vat, d.vat_deferred, d.vat_recognized, d.vat_credited, d.tax_invoice_no, d.tax_invoice_date,
	d.deposit_applied, d.deposit_applied_vat`

// Get — 3 query: หัว (JOIN ลูกค้า + ใบสั่งขาย) + บรรทัด (พร้อมยอดลดหนี้) + มัดจำที่หัก
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
	var lines []struct {
		models.InvoiceLineModel
		Returned decimal.Decimal
		Credited decimal.Decimal
	}
	// บรรทัด + ยอดที่รับคืน / ลดหนี้แล้ว (ใบลดหนี้ที่ยังไม่ยกเลิก) — query เดียว
	if err := database.Conn(ctx, r.db).Table("sales_invoice_lines AS l").
		Select(`l.*, COALESCE(cr.returned, 0) AS returned, COALESCE(cr.credited, 0) AS credited`).
		Joins(`LEFT JOIN (SELECT cl.invoice_line_no, SUM(cl.quantity) FILTER (WHERE n.kind = 'return') AS returned, SUM(cl.net) AS credited
			FROM sales_credit_note_lines AS cl JOIN sales_credit_notes AS n ON n.id = cl.credit_note_id
			WHERE n.invoice_id = ? AND n.status = 'posted' GROUP BY cl.invoice_line_no) AS cr ON cr.invoice_line_no = l.line_no`, h.ID).
		Where("l.invoice_id = ?", h.ID).Order("l.line_no").Scan(&lines).Error; err != nil {
		return dto.Invoice{}, err
	}
	deps, err := r.deposits(ctx, h.ID)
	if err != nil {
		return dto.Invoice{}, err
	}
	var recs []models.VatRecognitionModel
	if err := database.Conn(ctx, r.db).Where("invoice_id = ? AND status = 'posted'", h.ID).Order("doc_date, created_at").Find(&recs).Error; err != nil {
		return dto.Invoice{}, err
	}
	out := dto.Invoice{
		InvoiceSummary: h.InvoiceRow.summary(), StockInfo: stockRow{h.WarehouseID, h.System, h.StockDocumentNo}.dto(),
		VatRate: h.VatRate, PriceIncludesVat: h.PriceIncludesVat, Reference: h.Reference, Description: h.Description,
		BillTo: dto.BillTo{Name: h.BillToName, TaxID: h.BillToTaxID, BranchNo: h.BillToBranchNo, Address: h.BillToAddress},
		Lines:  make([]dto.InvoiceLine, len(lines)), DepositsApplied: make([]dto.DepositApplied, len(deps)),
	}
	for i, l := range lines {
		out.Lines[i] = dto.InvoiceLine{
			LineNo: l.LineNo, OrderLineNo: l.OrderLineNo, Item: dto.Ref{ID: l.ItemID.String()}, Description: l.Description, Service: l.Service,
			Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: dto.Totals{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, RevenueAccountID: l.RevenueAccountID.String(), Cost: l.Cost,
			Returned: l.Returned, Credited: l.Credited,
		}
	}
	out.VatRecognitions = make([]dto.VatRecognition, len(recs))
	for i, x := range recs {
		out.VatRecognitions[i] = dto.VatRecognition{Kind: x.Source, Document: dto.Ref{ID: x.DocumentID.String(), Code: x.DocumentNo},
			Date: dateUTC(x.DocDate), Base: x.Base, Vat: x.Vat}
	}
	for i, d := range deps {
		out.DepositsApplied[i] = dto.DepositApplied{Deposit: dto.Ref{ID: d.DepositID.String(), Code: d.DepositNo}, Amount: d.Amount, Vat: d.Vat}
	}
	return out, nil
}

func (r *InvoiceRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.InvoiceSummary], error) {
	order, err := invoiceTable.order(f.Sort)
	if err != nil {
		return listing.Result[dto.InvoiceSummary]{}, err
	}
	rows, total, err := database.FetchPage(invoiceTable.filtered(ctx, r.db, companyID, f, "d.reference", "d.tax_invoice_no"), invoiceTable.columns(invoiceExtra), order, f.Page,
		func(r InvoiceRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, InvoiceRow.summary), err
}

// Open — 1 query: เอกสารที่ยังไม่ยกเลิกและยังค้าง (index: company_id + customer_id + kind)
func (r *InvoiceRepository) Open(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, billingNoteID *vo.BillingNoteID) ([]dto.OpenInvoice, error) {
	var rows []struct {
		ID             uuid.UUID
		DocumentNo     string
		Kind           string
		DocDate        time.Time
		DueDate        time.Time
		TotalNet       decimal.Decimal
		TotalGross     decimal.Decimal
		ReceivedAmount decimal.Decimal
		VatUndue       decimal.Decimal
		TaxInvoiceNo   string
	}
	q := database.Conn(ctx, r.db).Table("sales_invoices AS i").
		Select(`i.id, i.document_no, i.kind, i.doc_date, i.due_date, i.total_net, i.total_gross, i.received_amount,
			i.vat_deferred - i.vat_recognized - i.vat_credited AS vat_undue, i.tax_invoice_no`).
		Where("i.company_id = ? AND i.customer_id = ? AND i.status = 'posted' AND i.received_amount < i.total_gross", uuidOf(companyID), uuidOf(customerID))
	if billingNoteID != nil {
		q = q.Where("i.id IN (SELECT invoice_id FROM sales_billing_note_invoices WHERE billing_note_id = ?)", uuidOf(*billingNoteID))
	}
	err := q.Order("i.due_date, i.document_no").Limit(500).Scan(&rows).Error
	out := make([]dto.OpenInvoice, len(rows))
	for i, x := range rows {
		out[i] = dto.OpenInvoice{ID: x.ID.String(), DocumentNo: x.DocumentNo, Kind: x.Kind, Date: gp.DateUTC(x.DocDate), DueDate: gp.DateUTC(x.DueDate),
			Net: x.TotalNet, Gross: x.TotalGross, Outstanding: x.TotalGross.Sub(x.ReceivedAmount), VatUndue: x.VatUndue,
			TaxInvoice: x.TaxInvoiceNo != ""}
	}
	return out, err
}

// OpenDeposits — 1 query: มัดจำที่รับเงินครบ (received = gross) และยังหักได้ (applied < net)
func (r *InvoiceRepository) OpenDeposits(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID) ([]dto.OpenDeposit, error) {
	var rows []struct {
		ID             uuid.UUID
		DocumentNo     string
		DocDate        time.Time
		VatRate        decimal.Decimal
		TotalNet       decimal.Decimal
		TotalVat       decimal.Decimal
		DepositApplied decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("sales_invoices").
		Select("id, document_no, doc_date, vat_rate, total_net, total_vat, deposit_applied").
		Where(`company_id = ? AND customer_id = ? AND kind = 'deposit' AND status = 'posted' AND received_amount = total_gross
			AND deposit_applied < total_net`, uuidOf(companyID), uuidOf(customerID)).
		Order("doc_date, document_no").Limit(200).Scan(&rows).Error
	out := make([]dto.OpenDeposit, len(rows))
	for i, x := range rows {
		out[i] = dto.OpenDeposit{ID: x.ID.String(), DocumentNo: x.DocumentNo, Date: dateUTC(x.DocDate), VatRate: x.VatRate,
			Base: x.TotalNet, Vat: x.TotalVat, Remaining: x.TotalNet.Sub(x.DepositApplied)}
	}
	return out, err
}

// Billable — 1 query: ข้อมูลใบ + อยู่ในใบวางบิลอื่นที่ยังเปิดหรือไม่
func (r *InvoiceRepository) Billable(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) (map[vo.InvoiceID]entities.BillableInvoice, error) {
	out := map[vo.InvoiceID]entities.BillableInvoice{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID             uuid.UUID
		DocumentNo     string
		CustomerID     uuid.UUID
		Kind           string
		Status         string
		TotalGross     decimal.Decimal
		ReceivedAmount decimal.Decimal
		OnOpenNote     bool
	}
	err := database.Conn(ctx, r.db).Table("sales_invoices AS i").
		Select(`i.id, i.document_no, i.customer_id, i.kind, i.status, i.total_gross, i.received_amount,
			EXISTS (SELECT 1 FROM sales_billing_note_invoices AS bi JOIN sales_billing_notes AS b ON b.id = bi.billing_note_id
				WHERE bi.invoice_id = i.id AND b.status = 'open') AS on_open_note`).
		Where("i.company_id = ? AND i.id IN ?", uuidOf(companyID), uuids(ids)).Scan(&rows).Error
	for _, x := range rows {
		out[vo.InvoiceID(x.ID)] = entities.BillableInvoice{
			ID: vo.InvoiceID(x.ID), DocumentNo: x.DocumentNo, CustomerID: vo.CustomerID(x.CustomerID), Kind: vo.InvoiceKind(x.Kind),
			Status: vo.DocStatus(x.Status), Outstanding: x.TotalGross.Sub(x.ReceivedAmount), OnOpenNote: x.OnOpenNote,
		}
	}
	return out, err
}

// ---------- ภาษีขายที่ถึงกำหนด (ขายบริการ) ----------

var _ ports.VatRecognitionRepository = (*VatRecognitionRepository)(nil)

type VatRecognitionRepository struct{ db *gorm.DB }

func NewVatRecognitionRepository(db *gorm.DB) *VatRecognitionRepository {
	return &VatRecognitionRepository{db: db}
}

// Create — INSERT แบบ batch statement เดียว
func (r *VatRecognitionRepository) Create(ctx context.Context, companyID vo.CompanyID, rows []ports.VatRecognition) error {
	if len(rows) == 0 {
		return nil
	}
	now := time.Now().UTC()
	ms := make([]models.VatRecognitionModel, len(rows))
	for i, x := range rows {
		ms[i] = models.VatRecognitionModel{
			ID: uuid.Must(uuid.NewV7()), CompanyID: uuidOf(companyID), DocDate: x.Date, Source: x.Source, DocumentID: x.DocumentID,
			DocumentNo: x.DocumentNo, InvoiceID: uuidOf(x.InvoiceID), Base: x.Base, Vat: x.Vat, Status: "posted", CreatedAt: now,
		}
	}
	return database.Conn(ctx, r.db).Omit(clause.Associations).Create(&ms).Error
}

// LockDocument — 1 query (FOR UPDATE)
func (r *VatRecognitionRepository) LockDocument(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) ([]ports.VatRecognition, error) {
	var ms []models.VatRecognitionModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND document_id = ? AND status = 'posted'", uuidOf(companyID), documentID).Order("id").Find(&ms).Error
	out := make([]ports.VatRecognition, len(ms))
	for i, m := range ms {
		out[i] = ports.VatRecognition{Source: m.Source, DocumentID: m.DocumentID, DocumentNo: m.DocumentNo, Date: gp.DateUTC(m.DocDate),
			InvoiceID: vo.InvoiceID(m.InvoiceID), Base: m.Base, Vat: m.Vat}
	}
	return out, err
}

func (r *VatRecognitionRepository) Cancel(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return database.Conn(ctx, r.db).Model(&models.VatRecognitionModel{}).
		Where("company_id = ? AND document_id = ? AND status = 'posted'", uuidOf(companyID), documentID).
		Update("status", "cancelled").Error
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
