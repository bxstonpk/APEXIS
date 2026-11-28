package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	gp "apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

type VoucherRepository struct{ db *gorm.DB }

func NewVoucherRepository(db *gorm.DB) *VoucherRepository { return &VoucherRepository{db: db} }

func docErr(err error) error {
	if database.IsUniqueViolation(err) {
		return domain.ErrDocumentNoUsed
	}
	return err
}

// Create: INSERT หัว 1 + บรรทัด 1 (batch)
func (r *VoucherRepository) Create(ctx context.Context, v *entities.Voucher) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m, lines := gp.VoucherToModels(v)
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt = now, now
	if err := tx.Omit("Lines").Create(&m).Error; err != nil {
		return docErr(err)
	}
	return tx.Create(&lines).Error
}

// Update: UPDATE หัว 1 + DELETE / INSERT บรรทัด (แทนที่ทั้งชุด)
func (r *VoucherRepository) Update(ctx context.Context, v *entities.Voucher) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m, lines := gp.VoucherToModels(v)
	if err := r.updateHead(tx, m); err != nil {
		return err
	}
	if err := tx.Where("voucher_id = ?", m.ID).Delete(&models.VoucherLineModel{}).Error; err != nil {
		return err
	}
	return tx.Create(&lines).Error
}

func (r *VoucherRepository) updateHead(tx *gorm.DB, m models.VoucherModel) error {
	return tx.Model(&models.VoucherModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).Updates(map[string]any{
		"fund_id": m.FundID, "doc_date": m.DocDate, "payee_name": m.PayeeName, "payee_tax_id": m.PayeeTaxID,
		"payee_branch_no": m.PayeeBranchNo, "description": m.Description, "vat_rate": m.VatRate,
		"price_includes_vat": m.PriceIncludesVat, "tax_invoice_no": m.TaxInvoiceNo, "tax_invoice_date": m.TaxInvoiceDate,
		"total_net": m.TotalNet, "total_vat": m.TotalVat, "total_gross": m.TotalGross, "status": m.Status,
		"replenishment_id": m.ReplenishmentID, "approved_by": m.ApprovedBy, "approved_at": m.ApprovedAt,
		"cancelled_at": m.CancelledAt, "updated_at": time.Now().UTC(),
	}).Error
}

// UpdateMany: สถานะ / คำขอของหลายใบ (หัวเท่านั้น — บรรทัดไม่เปลี่ยน)
func (r *VoucherRepository) UpdateMany(ctx context.Context, vs []*entities.Voucher) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	for _, v := range vs {
		m, _ := gp.VoucherToModels(v)
		if err := r.updateHead(tx, m); err != nil {
			return err
		}
	}
	return nil
}

func (r *VoucherRepository) lock(ctx context.Context) *gorm.DB {
	return database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "petty_cash_vouchers"}}).
		Preload("Lines", func(q *gorm.DB) *gorm.DB { return q.Order("line_no") })
}

func (r *VoucherRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.VoucherID) (*entities.Voucher, error) {
	var m models.VoucherModel
	err := r.lock(ctx).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrVoucherNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.VoucherToEntity(m), nil
}

// FindManyForUpdate — หัว 1 (FOR UPDATE) + บรรทัด 1; คืนตามลำดับ ids
func (r *VoucherRepository) FindManyForUpdate(ctx context.Context, companyID vo.CompanyID, ids []vo.VoucherID) ([]*entities.Voucher, error) {
	var ms []models.VoucherModel
	if err := r.lock(ctx).Where("company_id = ? AND id IN ?", uuidOf(companyID), uuids(ids)).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]models.VoucherModel, len(ms))
	for _, m := range ms {
		byID[m.ID] = m
	}
	out := make([]*entities.Voucher, len(ids))
	for i, id := range ids {
		m, ok := byID[uuidOf(id)]
		if !ok {
			return nil, fmt.Errorf("%w: %s", domain.ErrVoucherNotFound, id)
		}
		out[i] = gp.VoucherToEntity(m)
	}
	return out, nil
}

func (r *VoucherRepository) ByReplenishmentForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) ([]*entities.Voucher, error) {
	var ms []models.VoucherModel
	err := r.lock(ctx).Where("company_id = ? AND replenishment_id = ?", uuidOf(companyID), uuidOf(id)).Order("doc_date, document_no").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	out := make([]*entities.Voucher, len(ms))
	for i, m := range ms {
		out[i] = gp.VoucherToEntity(m)
	}
	return out, nil
}

// Get — หัว + กองทุน + คำขอ (JOIN) 1 + บรรทัด 1
func (r *VoucherRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.VoucherID) (dto.Voucher, error) {
	var row struct {
		models.VoucherModel
		FundCode string
		FundName string
		ReqNo    *string
	}
	err := database.Conn(ctx, r.db).Table("petty_cash_vouchers AS v").
		Select("v.*, f.code AS fund_code, f.name AS fund_name, rq.document_no AS req_no").
		Joins("JOIN petty_cash_funds f ON f.id = v.fund_id").
		Joins("LEFT JOIN petty_cash_replenishments rq ON rq.id = v.replenishment_id").
		Where("v.company_id = ? AND v.id = ?", uuidOf(companyID), uuidOf(id)).Take(&row).Error
	if database.IsNotFound(err) {
		return dto.Voucher{}, domain.ErrVoucherNotFound
	}
	if err != nil {
		return dto.Voucher{}, err
	}
	var lines []models.VoucherLineModel
	if err := database.Conn(ctx, r.db).Where("voucher_id = ?", row.ID).Order("line_no").Find(&lines).Error; err != nil {
		return dto.Voucher{}, err
	}
	m := row.VoucherModel
	out := dto.Voucher{ID: m.ID.String(), DocumentNo: m.DocumentNo, Fund: dto.Ref{ID: m.FundID.String(), Code: row.FundCode, Name: row.FundName},
		Date: dateOnly(m.DocDate), Payee: entities.Payee{Name: m.PayeeName, TaxID: m.PayeeTaxID, BranchNo: m.PayeeBranchNo},
		Description: m.Description, VatRate: m.VatRate, PriceIncludesVat: m.PriceIncludesVat,
		Totals: entities.Totals{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross}, Status: vo.VoucherStatus(m.Status),
		CreatedBy: dto.Ref{ID: m.CreatedBy.String()}, ApprovedAt: m.ApprovedAt, CancelledAt: m.CancelledAt, CreatedAt: m.CreatedAt}
	if m.TaxInvoiceDate != nil {
		out.TaxInvoice = &dto.TaxInvoice{No: m.TaxInvoiceNo, Date: dateOnly(*m.TaxInvoiceDate)}
	}
	if m.ReplenishmentID != nil && row.ReqNo != nil {
		out.Replenishment = &dto.Ref{ID: m.ReplenishmentID.String(), Code: *row.ReqNo}
	}
	if m.ApprovedBy != nil {
		out.ApprovedBy = &dto.Ref{ID: m.ApprovedBy.String()}
	}
	out.Lines = make([]dto.VoucherLine, len(lines))
	for i, l := range lines {
		out.Lines[i] = dto.VoucherLine{LineNo: l.LineNo, Account: dto.Ref{ID: l.AccountID.String()}, Description: l.Description, Amount: l.Amount, Net: l.Net}
	}
	return out, nil
}

type voucherRow struct {
	ID           uuid.UUID
	DocumentNo   string
	DocDate      time.Time
	FundID       uuid.UUID
	FundCode     string
	FundName     string
	PayeeName    string
	Description  string
	TaxInvoiceNo string
	TotalNet     decimal.Decimal
	TotalVat     decimal.Decimal
	TotalGross   decimal.Decimal
	Status       string
	ReqID        *uuid.UUID
	ReqNo        *string
	Total        int64
}

func (x voucherRow) dto() dto.VoucherSummary {
	out := dto.VoucherSummary{ID: x.ID.String(), DocumentNo: x.DocumentNo, Date: dateOnly(x.DocDate),
		Fund: dto.Ref{ID: x.FundID.String(), Code: x.FundCode, Name: x.FundName}, PayeeName: x.PayeeName, Description: x.Description,
		TaxInvoiceNo: x.TaxInvoiceNo, Totals: entities.Totals{Net: x.TotalNet, Vat: x.TotalVat, Gross: x.TotalGross},
		Status: vo.VoucherStatus(x.Status)}
	if x.ReqID != nil && x.ReqNo != nil {
		out.Replenishment = &dto.Ref{ID: x.ReqID.String(), Code: *x.ReqNo}
	}
	return out
}

const voucherColumns = `v.id, v.document_no, v.doc_date, v.fund_id, f.code AS fund_code, f.name AS fund_name, v.payee_name, v.description,
	v.tax_invoice_no, v.total_net, v.total_vat, v.total_gross, v.status, rq.id AS req_id, rq.document_no AS req_no`

func (r *VoucherRepository) baseQuery(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("petty_cash_vouchers AS v").
		Joins("JOIN petty_cash_funds f ON f.id = v.fund_id").
		Joins("LEFT JOIN petty_cash_replenishments rq ON rq.id = v.replenishment_id").
		Where("v.company_id = ?", uuidOf(companyID))
}

var voucherSortColumns = map[string]string{"date": "v.doc_date", "document_no": "v.document_no", "created_at": "v.created_at"}

// List — query เดียว (JOIN กองทุน + คำขอ, COUNT OVER)
func (r *VoucherRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.VoucherFilter) (listing.Result[dto.VoucherSummary], error) {
	order, err := database.OrderBy(f.Sort, voucherSortColumns, "v.id")
	if err != nil {
		return listing.Result[dto.VoucherSummary]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.baseQuery(ctx, companyID), f.Search, "v.document_no", "v.payee_name", "v.description", "v.tax_invoice_no")
		if f.FundID != nil {
			q = q.Where("v.fund_id = ?", uuidOf(*f.FundID))
		}
		if len(f.Status) > 0 {
			q = q.Where("v.status IN ?", strs(f.Status))
		}
		if f.From != nil {
			q = q.Where("v.doc_date >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("v.doc_date <= ?", *f.To)
		}
		if f.Unrequested {
			q = q.Where("v.status = 'approved' AND v.replenishment_id IS NULL")
		}
		return q
	}
	rows, total, err := database.FetchPage(base, voucherColumns, order, f.Page, func(x voucherRow) int64 { return x.Total })
	return toResult(rows, total, f.Page, voucherRow.dto), err
}

// Summaries: ใบสำคัญจ่ายตาม ids (เรียงวันที่ / เลขที่) — query เดียว
func (r *VoucherRepository) Summaries(ctx context.Context, companyID vo.CompanyID, ids []vo.VoucherID) ([]dto.VoucherSummary, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []voucherRow
	err := r.baseQuery(ctx, companyID).Select(voucherColumns).Where("v.id IN ?", uuids(ids)).Order("v.doc_date, v.document_no").Scan(&rows).Error
	out := make([]dto.VoucherSummary, len(rows))
	for i, x := range rows {
		out[i] = x.dto()
	}
	return out, err
}
