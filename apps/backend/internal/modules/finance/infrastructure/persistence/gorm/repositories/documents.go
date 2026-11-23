package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/finance/application/dto"
	ports "apexis/backend/internal/modules/finance/application/ports/repositories"
	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	gp "apexis/backend/internal/modules/finance/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

// ---------- ตัวช่วย ----------

func uuidOf[T ~[16]byte](id T) uuid.UUID { return uuid.UUID(id) }

func optString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

func searchCols(q *gorm.DB, text string, columns ...string) *gorm.DB {
	if text == "" {
		return q
	}
	p := database.LikeContains(text)
	parts := make([]string, len(columns))
	args := make([]any, len(columns))
	for i, c := range columns {
		parts[i] = "LOWER(" + c + ") LIKE ?"
		args[i] = p
	}
	return q.Where("("+strings.Join(parts, " OR ")+")", args...)
}

func toResult[R any, T any](rows []R, total int64, page listing.Page, fn func(R) T) listing.Result[T] {
	items := make([]T, len(rows))
	for i, r := range rows {
		items[i] = fn(r)
	}
	return listing.Result[T]{Items: items, Total: total, Page: page}
}

// createDoc — หัว + ลูกแต่ละชุด (batch) ใน transaction ของ use case; เลขที่ซ้ำ = ErrDocumentNoUsed
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
		if err := tx.Omit(clause.Associations).Create(c).Error; err != nil {
			return err
		}
	}
	return nil
}

func updateStatus(ctx context.Context, db *gorm.DB, model any, companyID, id uuid.UUID, status string, c *entities.Cancellation, notFound error) error {
	var by *uuid.UUID
	var at *time.Time
	if c != nil {
		u, t := uuid.UUID(c.By), c.At
		by, at = &u, &t
	}
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

// lockHeader — SELECT ... FOR UPDATE ของหัวเอกสาร
func lockHeader[M any](ctx context.Context, db *gorm.DB, companyID, id uuid.UUID, notFound error) (M, error) {
	var m M
	err := database.Conn(ctx, db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("company_id = ? AND id = ?", companyID, id).Take(&m).Error
	if database.IsNotFound(err) {
		return m, notFound
	}
	return m, err
}

// docFilter: เงื่อนไขร่วมของรายการใบรับ / ใบจ่าย (alias d)
func docFilter(q *gorm.DB, f dto.DocumentFilter, methodTable, methodFK string) *gorm.DB {
	if len(f.Statuses) > 0 {
		q = q.Where("d.status IN ?", f.Statuses)
	}
	if len(f.Kinds) > 0 {
		q = q.Where("d.kind IN ?", f.Kinds)
	}
	if f.PartyID != nil {
		q = q.Where("d.party_id = ?", *f.PartyID)
	}
	if f.CashAccountID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM "+methodTable+" AS m WHERE m."+methodFK+" = d.id AND m.cash_account_id = ?)", uuidOf(*f.CashAccountID))
	}
	if f.StartDate != nil {
		q = q.Where("d.doc_date >= ?", *f.StartDate)
	}
	if f.EndDate != nil {
		q = q.Where("d.doc_date <= ?", *f.EndDate)
	}
	return searchCols(q, f.Search, "d.document_no", "d.party_code", "d.party_name", "d.description")
}

var docSortColumns = map[string]string{"date": "d.doc_date", "document_no": "d.document_no"}

// ---------- ลำดับเลขที่ ----------

var _ ports.Sequence = (*SequenceRepository)(nil)

type SequenceRepository struct{}

func NewSequenceRepository() *SequenceRepository { return &SequenceRepository{} }

func (SequenceRepository) NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error) {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	err = tx.Raw(`
		INSERT INTO finance_number_sequences (company_id, doc_type, period_key, last_no) VALUES (?, ?, ?, 1)
		ON CONFLICT (company_id, doc_type, period_key) DO UPDATE SET last_no = finance_number_sequences.last_no + 1
		RETURNING last_no`, uuidOf(companyID), string(docType), periodKey).Scan(&n).Error
	return n, err
}

// ---------- ค่าตั้ง ----------

var _ ports.SettingsRepository = (*SettingsRepository)(nil)

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

func (r *SettingsRepository) Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error) {
	var ms []models.SettingsModel
	if err := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID)).Limit(1).Find(&ms).Error; err != nil {
		return entities.Settings{}, err
	}
	if len(ms) == 0 {
		return entities.Settings{}, nil
	}
	return gp.SettingsToEntity(ms[0]), nil
}

// Save — UPSERT statement เดียว
func (r *SettingsRepository) Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error {
	m := gp.SettingsToModel(companyID, s)
	m.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"receipt_journal_id", "payment_journal_id", "transfer_journal_id",
			"wht_received_account_id", "wht_pnd3_account_id", "wht_pnd53_account_id", "updated_at"}),
	}).Create(&m).Error
}

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
		q := database.Conn(ctx, r.db).Table("finance_wht_types AS w").Where("w.company_id = ? AND w.deleted_at IS NULL", uuidOf(companyID))
		if len(f.Statuses) > 0 {
			q = q.Where("w.status IN ?", f.Statuses)
		}
		return searchCols(q, f.Search, "w.code", "w.name")
	}
	rows, total, err := database.FetchPage(base, "w.id, w.code, w.name, w.rate, w.status", order, f.Page, func(r whtRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, whtRow.dto), err
}

// ---------- แถวรายการใบรับ / ใบจ่าย ----------

type docRow struct {
	ID            uuid.UUID
	DocumentNo    string
	DocDate       time.Time
	Kind          string
	PartyID       *uuid.UUID
	PartyCode     string
	PartyName     string
	PartyTaxID    string
	PartyBranchNo string
	PartyAddress  string
	PartyTaxType  string
	TotalAmount   decimal.Decimal
	TotalWht      decimal.Decimal
	TotalNet      decimal.Decimal
	Status        string
	EntryID       *uuid.UUID
	EntryNo       string
	CancelledAt   *time.Time
	Description   string
	CreatedBy     uuid.UUID
	Total         int64
}

func (r docRow) summary() dto.DocumentSummary {
	return dto.DocumentSummary{
		ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: gp.DateUTC(r.DocDate), Kind: r.Kind,
		Party: dto.Party{ID: optString(r.PartyID), Code: r.PartyCode, Name: r.PartyName, TaxID: r.PartyTaxID, BranchNo: r.PartyBranchNo,
			Address: r.PartyAddress, TaxType: r.PartyTaxType},
		Amount: r.TotalAmount, Wht: r.TotalWht, Net: r.TotalNet, Status: r.Status, EntryID: optString(r.EntryID), EntryNo: r.EntryNo,
		CancelledAt: r.CancelledAt, Description: r.Description,
	}
}

func docColumns(net string) string {
	return `d.id, d.document_no, d.doc_date, d.kind, d.party_id, d.party_code, d.party_name, d.party_tax_id, d.party_branch_no,
		d.party_address, d.party_tax_type, d.total_amount, d.total_wht, d.` + net + ` AS total_net, d.status, d.entry_id, d.entry_no,
		d.cancelled_at, d.description, d.created_by`
}

// methods — 1 query: ช่องทาง JOIN ทะเบียนเงิน (รหัส / ชื่อ)
func methods(ctx context.Context, db *gorm.DB, table, fk string, id uuid.UUID) ([]dto.Method, error) {
	var rows []struct {
		CashAccountID uuid.UUID
		Code          *string
		Name          *string
		GLAccountID   uuid.UUID
		Amount        decimal.Decimal
		Reference     string
	}
	err := database.Conn(ctx, db).Table(table+" AS m").
		Joins("LEFT JOIN finance_cash_accounts AS a ON a.id = m.cash_account_id").
		Select("m.cash_account_id, a.code, a.name, m.gl_account_id, m.amount, m.reference").
		Where("m."+fk+" = ?", id).Order("m.line_no").Scan(&rows).Error
	out := make([]dto.Method, len(rows))
	for i, r := range rows {
		ref := dto.Ref{ID: r.CashAccountID.String()}
		if r.Code != nil {
			ref.Code, ref.Name = *r.Code, *r.Name
		}
		out[i] = dto.Method{CashAccount: ref, GLAccountID: r.GLAccountID.String(), Amount: r.Amount, Reference: r.Reference}
	}
	return out, err
}

// ---------- ใบรับเงิน ----------

var _ ports.ReceiptRepository = (*ReceiptRepository)(nil)

type ReceiptRepository struct{ db *gorm.DB }

func NewReceiptRepository(db *gorm.DB) *ReceiptRepository { return &ReceiptRepository{db: db} }

// Create — 3 statement: หัว + บรรทัด + ช่องทาง (batch)
func (r *ReceiptRepository) Create(ctx context.Context, rc *entities.Receipt) error {
	m := gp.ReceiptToModel(rc)
	return createDoc(ctx, &m, &m.Lines, &m.Methods)
}

func (r *ReceiptRepository) UpdateStatus(ctx context.Context, rc *entities.Receipt) error {
	s := rc.State()
	return updateStatus(ctx, r.db, &models.ReceiptModel{}, uuidOf(s.CompanyID), uuidOf(s.ID), string(s.Status), s.Cancellation, domain.ErrReceiptNotFound)
}

// FindForUpdate — 3 query: lock หัว แล้วโหลดบรรทัด / ช่องทาง
func (r *ReceiptRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (*entities.Receipt, error) {
	m, err := lockHeader[models.ReceiptModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrReceiptNotFound)
	if err != nil {
		return nil, err
	}
	db := database.Conn(ctx, r.db)
	if err := db.Where("receipt_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	if err := db.Where("receipt_id = ?", m.ID).Order("line_no").Find(&m.Methods).Error; err != nil {
		return nil, err
	}
	return gp.ReceiptToEntity(m), nil
}

func (r *ReceiptRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("finance_receipts AS d").Where("d.company_id = ?", uuidOf(companyID))
}

// Get — 3 query: หัว, บรรทัด, ช่องทาง (JOIN ทะเบียนเงิน) — ข้อมูลคู่ค้า / เลขที่ใบแจ้งหนี้เก็บติดเอกสารแล้ว
func (r *ReceiptRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (dto.Receipt, error) {
	var rows []docRow
	if err := r.base(ctx, companyID).Select(docColumns("total_received")).Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Receipt{}, err
	}
	if len(rows) == 0 {
		return dto.Receipt{}, domain.ErrReceiptNotFound
	}
	h := rows[0]
	out := dto.Receipt{DocumentSummary: h.summary(), CreatedBy: h.CreatedBy.String()}
	var lines []models.ReceiptLineModel
	if err := database.Conn(ctx, r.db).Where("receipt_id = ?", h.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Receipt{}, err
	}
	out.Lines = make([]dto.ReceiptLine, len(lines))
	for i, l := range lines {
		line := dto.ReceiptLine{AccountID: optString(l.AccountID), Description: l.Description, Amount: l.Amount, Wht: l.Wht}
		if l.InvoiceID != nil {
			line.Invoice = &dto.Ref{ID: l.InvoiceID.String(), Code: l.InvoiceNo}
		}
		out.Lines[i] = line
	}
	var err error
	out.Methods, err = methods(ctx, r.db, "finance_receipt_methods", "receipt_id", h.ID)
	return out, err
}

func (r *ReceiptRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error) {
	order, err := database.OrderBy(f.Sort, docSortColumns, "d.id")
	if err != nil {
		return listing.Result[dto.DocumentSummary]{}, err
	}
	base := func() *gorm.DB { return docFilter(r.base(ctx, companyID), f, "finance_receipt_methods", "receipt_id") }
	rows, total, err := database.FetchPage(base, docColumns("total_received"), order, f.Page, func(r docRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, docRow.summary), err
}

// ---------- ใบสำคัญจ่าย ----------

var _ ports.PaymentRepository = (*PaymentRepository)(nil)

type PaymentRepository struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) *PaymentRepository { return &PaymentRepository{db: db} }

func (r *PaymentRepository) Create(ctx context.Context, p *entities.Payment) error {
	m := gp.PaymentToModel(p)
	return createDoc(ctx, &m, &m.Lines, &m.Methods)
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, p *entities.Payment) error {
	s := p.State()
	return updateStatus(ctx, r.db, &models.PaymentModel{}, uuidOf(s.CompanyID), uuidOf(s.ID), string(s.Status), s.Cancellation, domain.ErrPaymentNotFound)
}

func (r *PaymentRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (*entities.Payment, error) {
	m, err := lockHeader[models.PaymentModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrPaymentNotFound)
	if err != nil {
		return nil, err
	}
	db := database.Conn(ctx, r.db)
	if err := db.Where("payment_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	if err := db.Where("payment_id = ?", m.ID).Order("line_no").Find(&m.Methods).Error; err != nil {
		return nil, err
	}
	return gp.PaymentToEntity(m), nil
}

func (r *PaymentRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("finance_payments AS d").Where("d.company_id = ?", uuidOf(companyID))
}

// Get — 3 query: หัว, บรรทัด (JOIN ประเภทภาษี), ช่องทาง (JOIN ทะเบียนเงิน)
func (r *PaymentRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (dto.Payment, error) {
	var rows []docRow
	if err := r.base(ctx, companyID).Select(docColumns("total_paid")).Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Payment{}, err
	}
	if len(rows) == 0 {
		return dto.Payment{}, domain.ErrPaymentNotFound
	}
	h := rows[0]
	out := dto.Payment{DocumentSummary: h.summary(), CreatedBy: h.CreatedBy.String()}
	var lines []struct {
		InvoiceID         *uuid.UUID
		InvoiceNo         string
		SupplierInvoiceNo string
		AccountID         *uuid.UUID
		Description       string
		Amount            decimal.Decimal
		WhtTypeID         *uuid.UUID
		WhtCode           *string
		WhtName           *string
		WhtRate           decimal.Decimal
		WhtBase           decimal.Decimal
		Wht               decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("finance_payment_lines AS l").
		Joins("LEFT JOIN finance_wht_types AS w ON w.id = l.wht_type_id").
		Select(`l.invoice_id, l.invoice_no, l.supplier_invoice_no, l.account_id, l.description, l.amount, l.wht_type_id,
			w.code AS wht_code, w.name AS wht_name, l.wht_rate, l.wht_base, l.wht`).
		Where("l.payment_id = ?", h.ID).Order("l.line_no").Scan(&lines).Error
	if err != nil {
		return dto.Payment{}, err
	}
	out.Lines = make([]dto.PaymentLine, len(lines))
	for i, l := range lines {
		line := dto.PaymentLine{AccountID: optString(l.AccountID), Description: l.Description, Amount: l.Amount,
			WhtRate: l.WhtRate, WhtBase: l.WhtBase, Wht: l.Wht}
		if l.InvoiceID != nil {
			line.Invoice = &dto.Ref{ID: l.InvoiceID.String(), Code: l.InvoiceNo, Name: l.SupplierInvoiceNo}
		}
		if l.WhtTypeID != nil && l.WhtCode != nil {
			line.WhtType = &dto.Ref{ID: l.WhtTypeID.String(), Code: *l.WhtCode, Name: *l.WhtName}
		}
		out.Lines[i] = line
	}
	out.Methods, err = methods(ctx, r.db, "finance_payment_methods", "payment_id", h.ID)
	return out, err
}

func (r *PaymentRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error) {
	order, err := database.OrderBy(f.Sort, docSortColumns, "d.id")
	if err != nil {
		return listing.Result[dto.DocumentSummary]{}, err
	}
	base := func() *gorm.DB { return docFilter(r.base(ctx, companyID), f, "finance_payment_methods", "payment_id") }
	rows, total, err := database.FetchPage(base, docColumns("total_paid"), order, f.Page, func(r docRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, docRow.summary), err
}

// ---------- โอนเงิน ----------

var _ ports.TransferRepository = (*TransferRepository)(nil)

type TransferRepository struct{ db *gorm.DB }

func NewTransferRepository(db *gorm.DB) *TransferRepository { return &TransferRepository{db: db} }

func (r *TransferRepository) Create(ctx context.Context, t *entities.Transfer) error {
	m := gp.TransferToModel(t)
	return createDoc(ctx, &m)
}

func (r *TransferRepository) UpdateStatus(ctx context.Context, t *entities.Transfer) error {
	s := t.State()
	return updateStatus(ctx, r.db, &models.TransferModel{}, uuidOf(s.CompanyID), uuidOf(s.ID), string(s.Status), s.Cancellation, domain.ErrTransferNotFound)
}

func (r *TransferRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.TransferID) (*entities.Transfer, error) {
	m, err := lockHeader[models.TransferModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrTransferNotFound)
	if err != nil {
		return nil, err
	}
	return gp.TransferToEntity(m), nil
}

type transferRow struct {
	ID          uuid.UUID
	DocumentNo  string
	DocDate     time.Time
	FromID      uuid.UUID
	FromCode    string
	FromName    string
	ToID        uuid.UUID
	ToCode      string
	ToName      string
	Amount      decimal.Decimal
	Reference   string
	Description string
	Status      string
	EntryID     *uuid.UUID
	EntryNo     string
	CancelledAt *time.Time
	Total       int64
}

func (r transferRow) dto() dto.Transfer {
	return dto.Transfer{
		ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: gp.DateUTC(r.DocDate),
		From: dto.Ref{ID: r.FromID.String(), Code: r.FromCode, Name: r.FromName}, To: dto.Ref{ID: r.ToID.String(), Code: r.ToCode, Name: r.ToName},
		Amount: r.Amount, Reference: r.Reference, Description: r.Description, Status: r.Status, EntryID: optString(r.EntryID),
		EntryNo: r.EntryNo, CancelledAt: r.CancelledAt,
	}
}

const transferColumns = `d.id, d.document_no, d.doc_date, d.from_account_id AS from_id, fa.code AS from_code, fa.name AS from_name,
	d.to_account_id AS to_id, ta.code AS to_code, ta.name AS to_name, d.amount, d.reference, d.description, d.status, d.entry_id,
	d.entry_no, d.cancelled_at`

// base — JOIN ทะเบียนต้นทาง / ปลายทาง (รวมที่ถูกลบแบบ soft delete — เอกสารเดิมยังแสดงชื่อได้)
func (r *TransferRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("finance_transfers AS d").
		Joins("JOIN finance_cash_accounts AS fa ON fa.id = d.from_account_id").
		Joins("JOIN finance_cash_accounts AS ta ON ta.id = d.to_account_id").
		Where("d.company_id = ?", uuidOf(companyID))
}

// Get — 1 query
func (r *TransferRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.TransferID) (dto.Transfer, error) {
	var rows []transferRow
	if err := r.base(ctx, companyID).Select(transferColumns).Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Transfer{}, err
	}
	if len(rows) == 0 {
		return dto.Transfer{}, domain.ErrTransferNotFound
	}
	return rows[0].dto(), nil
}

func (r *TransferRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.Transfer], error) {
	order, err := database.OrderBy(f.Sort, docSortColumns, "d.id")
	if err != nil {
		return listing.Result[dto.Transfer]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.CashAccountID != nil {
			id := uuidOf(*f.CashAccountID)
			q = q.Where("(d.from_account_id = ? OR d.to_account_id = ?)", id, id)
		}
		if f.StartDate != nil {
			q = q.Where("d.doc_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("d.doc_date <= ?", *f.EndDate)
		}
		return searchCols(q, f.Search, "d.document_no", "d.reference", "d.description")
	}
	rows, total, err := database.FetchPage(base, transferColumns, order, f.Page, func(r transferRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, transferRow.dto), err
}

// ---------- รายงานภาษีหัก ณ ที่จ่าย ----------

var _ ports.ReportRepository = (*ReportRepository)(nil)

type ReportRepository struct{ db *gorm.DB }

func NewReportRepository(db *gorm.DB) *ReportRepository { return &ReportRepository{db: db} }

type whtReportRow struct {
	DocDate       time.Time
	DocumentID    uuid.UUID
	DocumentNo    string
	PartyID       *uuid.UUID
	PartyCode     string
	PartyName     string
	PartyTaxID    string
	PartyBranchNo string
	PartyAddress  string
	PartyTaxType  string
	WhtTypeID     *uuid.UUID
	WhtCode       *string
	WhtName       *string
	Rate          decimal.Decimal
	Base          decimal.Decimal
	Wht           decimal.Decimal
	Total         int64
	TotalBase     decimal.Decimal
	TotalWht      decimal.Decimal
}

func (r whtReportRow) dto() dto.WithholdingRow {
	out := dto.WithholdingRow{
		Date: gp.DateUTC(r.DocDate), DocumentID: r.DocumentID.String(), DocumentNo: r.DocumentNo,
		Party: dto.Party{ID: optString(r.PartyID), Code: r.PartyCode, Name: r.PartyName, TaxID: r.PartyTaxID, BranchNo: r.PartyBranchNo,
			Address: r.PartyAddress, TaxType: r.PartyTaxType},
		TaxForm: vo.TaxType(r.PartyTaxType).Form(), Rate: r.Rate, Base: r.Base, Wht: r.Wht,
	}
	if r.WhtTypeID != nil && r.WhtCode != nil {
		out.WhtType = &dto.Ref{ID: r.WhtTypeID.String(), Code: *r.WhtCode, Name: *r.WhtName}
	}
	return out
}

const partyColumns = "d.party_id, d.party_code, d.party_name, d.party_tax_id, d.party_branch_no, d.party_address, d.party_tax_type"

func (r *ReportRepository) withholding(base func() *gorm.DB, columns, order string, page listing.Page) (listing.Result[dto.WithholdingRow], dto.WithholdingTotals, error) {
	rows, total, err := database.FetchPage(base, columns, order, page, func(r whtReportRow) int64 { return r.Total })
	totals := dto.WithholdingTotals{Base: decimal.Zero, Wht: decimal.Zero}
	if len(rows) > 0 {
		totals = dto.WithholdingTotals{Base: rows[0].TotalBase, Wht: rows[0].TotalWht}
	}
	return toResult(rows, total, page, whtReportRow.dto), totals, err
}

var whtSortColumns = map[string]string{"date": "d.doc_date", "document_no": "d.document_no", "party_name": "d.party_name"}

// WithholdingPaid — 1 query: บรรทัดใบจ่ายที่หักภาษี JOIN หัว + ประเภทภาษี, ยอดรวมด้วย window function
// (ข้อมูลผู้รับเงินเก็บติดใบจ่าย ณ วันจ่าย — ไม่ต้องอ่านทะเบียนผู้ขายข้าม module)
func (r *ReportRepository) WithholdingPaid(ctx context.Context, companyID vo.CompanyID, f dto.WithholdingFilter) (listing.Result[dto.WithholdingRow], dto.WithholdingTotals, error) {
	order, err := database.OrderBy(f.Sort, whtSortColumns, "d.id ASC, l.line_no")
	if err != nil {
		return listing.Result[dto.WithholdingRow]{}, dto.WithholdingTotals{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("finance_payment_lines AS l").
			Joins("JOIN finance_payments AS d ON d.id = l.payment_id").
			Joins("LEFT JOIN finance_wht_types AS w ON w.id = l.wht_type_id").
			Where("d.company_id = ? AND d.status = 'posted' AND l.wht > 0 AND d.doc_date BETWEEN ? AND ?", uuidOf(companyID), f.Start, f.End)
		switch f.Form {
		case "pnd3":
			q = q.Where("d.party_tax_type = ?", string(vo.TaxIndividual))
		case "pnd53":
			q = q.Where("d.party_tax_type = ?", string(vo.TaxJuristic))
		}
		if f.PartyID != nil {
			q = q.Where("d.party_id = ?", *f.PartyID)
		}
		return q
	}
	return r.withholding(base, `d.doc_date, d.id AS document_id, d.document_no, `+partyColumns+`, l.wht_type_id, w.code AS wht_code,
		w.name AS wht_name, l.wht_rate AS rate, l.wht_base AS base, l.wht, SUM(l.wht_base) OVER () AS total_base, SUM(l.wht) OVER () AS total_wht`,
		order, f.Page)
}

// WithholdingReceived — 1 query: ใบรับ (ยังไม่ยกเลิก) ที่ลูกค้าหักภาษี — ฐาน = ยอดตัดหนี้, ภาษี = ยอดที่ถูกหัก
func (r *ReportRepository) WithholdingReceived(ctx context.Context, companyID vo.CompanyID, f dto.WithholdingFilter) (listing.Result[dto.WithholdingRow], dto.WithholdingTotals, error) {
	order, err := database.OrderBy(f.Sort, whtSortColumns, "d.id")
	if err != nil {
		return listing.Result[dto.WithholdingRow]{}, dto.WithholdingTotals{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("finance_receipts AS d").
			Where("d.company_id = ? AND d.status = 'posted' AND d.total_wht > 0 AND d.doc_date BETWEEN ? AND ?", uuidOf(companyID), f.Start, f.End)
		if f.PartyID != nil {
			q = q.Where("d.party_id = ?", *f.PartyID)
		}
		return q
	}
	return r.withholding(base, `d.doc_date, d.id AS document_id, d.document_no, `+partyColumns+`, 0 AS rate, d.total_amount AS base,
		d.total_wht AS wht, SUM(d.total_amount) OVER () AS total_base, SUM(d.total_wht) OVER () AS total_wht`, order, f.Page)
}
