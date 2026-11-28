package repositories

import (
	"context"
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

type ReplenishmentRepository struct {
	db       *gorm.DB
	vouchers *VoucherRepository
}

func NewReplenishmentRepository(db *gorm.DB, vouchers *VoucherRepository) *ReplenishmentRepository {
	return &ReplenishmentRepository{db: db, vouchers: vouchers}
}

func (r *ReplenishmentRepository) Create(ctx context.Context, x *entities.Replenishment) error {
	m := gp.ReplenishmentToModel(x)
	m.UpdatedAt = time.Now().UTC()
	return docErr(database.Conn(ctx, r.db).Create(&m).Error)
}

func (r *ReplenishmentRepository) Update(ctx context.Context, x *entities.Replenishment) error {
	m := gp.ReplenishmentToModel(x)
	return database.Conn(ctx, r.db).Model(&models.ReplenishmentModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{"status": m.Status, "payment_document_id": m.PaymentDocumentID, "payment_document_no": m.PaymentDocumentNo,
			"paid_date": m.PaidDate, "cancelled_at": m.CancelledAt, "updated_at": time.Now().UTC()}).Error
}

func (r *ReplenishmentRepository) find(ctx context.Context, where string, args ...any) (*entities.Replenishment, error) {
	var m models.ReplenishmentModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).Where(where, args...).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrReplenishmentNotFound
	}
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	if err := database.Conn(ctx, r.db).Model(&models.VoucherModel{}).Where("replenishment_id = ?", m.ID).Order("doc_date, document_no").
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	vids := make([]vo.VoucherID, len(ids))
	for i, id := range ids {
		vids[i] = vo.VoucherID(id)
	}
	return gp.ReplenishmentToEntity(m, vids), nil
}

func (r *ReplenishmentRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) (*entities.Replenishment, error) {
	return r.find(ctx, "company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id))
}

func (r *ReplenishmentRepository) FindByPaymentForUpdate(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) (*entities.Replenishment, error) {
	return r.find(ctx, "company_id = ? AND payment_document_id = ? AND status = 'paid'", uuidOf(companyID), documentID)
}

func (r *ReplenishmentRepository) HasOpenFunding(ctx context.Context, companyID vo.CompanyID, fundID vo.FundID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, r.db).Raw(`SELECT EXISTS (SELECT 1 FROM petty_cash_replenishments
		WHERE company_id = ? AND fund_id = ? AND kind IN ('establish', 'increase') AND status = 'requested')`,
		uuidOf(companyID), uuidOf(fundID)).Scan(&ok).Error
	return ok, err
}

type requestRow struct {
	ID                uuid.UUID
	DocumentNo        string
	FundID            uuid.UUID
	FundCode          string
	FundName          string
	Kind              string
	DocDate           time.Time
	Description       string
	Amount            decimal.Decimal
	Status            string
	PaymentDocumentID *uuid.UUID
	PaymentDocumentNo string
	PaidDate          *time.Time
	Vouchers          int
	CreatedBy         uuid.UUID
	CreatedAt         time.Time
	CancelledAt       *time.Time
	Total             int64
}

func (x requestRow) payment() *dto.Payment {
	if x.PaymentDocumentID == nil || x.PaidDate == nil {
		return nil
	}
	return &dto.Payment{DocumentID: x.PaymentDocumentID.String(), DocumentNo: x.PaymentDocumentNo, Date: dateOnly(*x.PaidDate)}
}

func (x requestRow) summary() dto.ReplenishmentSummary {
	return dto.ReplenishmentSummary{ID: x.ID.String(), DocumentNo: x.DocumentNo, Fund: dto.Ref{ID: x.FundID.String(), Code: x.FundCode, Name: x.FundName},
		Kind: vo.RequestKind(x.Kind), Date: dateOnly(x.DocDate), Description: x.Description, Amount: x.Amount, Status: vo.RequestStatus(x.Status),
		Vouchers: x.Vouchers, Payment: x.payment()}
}

const requestColumns = `r.id, r.document_no, r.fund_id, f.code AS fund_code, f.name AS fund_name, r.kind, r.doc_date, r.description, r.amount,
	r.status, r.payment_document_id, r.payment_document_no, r.paid_date, r.created_by, r.created_at, r.cancelled_at,
	(SELECT COUNT(*) FROM petty_cash_vouchers v WHERE v.replenishment_id = r.id) AS vouchers`

func (r *ReplenishmentRepository) baseQuery(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("petty_cash_replenishments AS r").Joins("JOIN petty_cash_funds f ON f.id = r.fund_id").
		Where("r.company_id = ?", uuidOf(companyID))
}

// Get — หัว + กองทุน 1 + ใบสำคัญจ่ายในคำขอ 1
func (r *ReplenishmentRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) (dto.Replenishment, error) {
	var x requestRow
	err := r.baseQuery(ctx, companyID).Select(requestColumns).Where("r.id = ?", uuidOf(id)).Take(&x).Error
	if database.IsNotFound(err) {
		return dto.Replenishment{}, domain.ErrReplenishmentNotFound
	}
	if err != nil {
		return dto.Replenishment{}, err
	}
	out := dto.Replenishment{ID: x.ID.String(), DocumentNo: x.DocumentNo, Fund: dto.Ref{ID: x.FundID.String(), Code: x.FundCode, Name: x.FundName},
		Kind: vo.RequestKind(x.Kind), Date: dateOnly(x.DocDate), Description: x.Description, Amount: x.Amount, Status: vo.RequestStatus(x.Status),
		Payment: x.payment(), CreatedBy: dto.Ref{ID: x.CreatedBy.String()}, CreatedAt: x.CreatedAt, CancelledAt: x.CancelledAt,
		Vouchers: []dto.VoucherSummary{}}
	var rows []voucherRow
	if err := r.vouchers.baseQuery(ctx, companyID).Select(voucherColumns).Where("v.replenishment_id = ?", x.ID).
		Order("v.doc_date, v.document_no").Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, v := range rows {
		out.Vouchers = append(out.Vouchers, v.dto())
	}
	return out, nil
}

var requestSortColumns = map[string]string{"date": "r.doc_date", "document_no": "r.document_no", "created_at": "r.created_at"}

func (r *ReplenishmentRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.ReplenishmentFilter) (listing.Result[dto.ReplenishmentSummary], error) {
	order, err := database.OrderBy(f.Sort, requestSortColumns, "r.id")
	if err != nil {
		return listing.Result[dto.ReplenishmentSummary]{}, err
	}
	base := func() *gorm.DB {
		q := search(r.baseQuery(ctx, companyID), f.Search, "r.document_no", "r.description", "r.payment_document_no")
		if f.FundID != nil {
			q = q.Where("r.fund_id = ?", uuidOf(*f.FundID))
		}
		if len(f.Kind) > 0 {
			q = q.Where("r.kind IN ?", strs(f.Kind))
		}
		if len(f.Status) > 0 {
			q = q.Where("r.status IN ?", strs(f.Status))
		}
		if f.From != nil {
			q = q.Where("r.doc_date >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("r.doc_date <= ?", *f.To)
		}
		return q
	}
	rows, total, err := database.FetchPage(base, requestColumns, order, f.Page, func(x requestRow) int64 { return x.Total })
	return toResult(rows, total, f.Page, requestRow.summary), err
}

// Open — คำขอที่รอการเงินจ่าย (query เดียว, index idx_pc_requests_status)
func (r *ReplenishmentRepository) Open(ctx context.Context, companyID vo.CompanyID) ([]dto.OpenRequest, error) {
	var rows []requestRow
	err := r.baseQuery(ctx, companyID).Select("r.id, r.document_no, f.code AS fund_code, f.name AS fund_name, r.kind, r.doc_date, r.amount").
		Where("r.status = 'requested'").Order("r.doc_date, r.document_no").Scan(&rows).Error
	out := make([]dto.OpenRequest, len(rows))
	for i, x := range rows {
		out[i] = dto.OpenRequest{ID: x.ID, DocumentNo: x.DocumentNo, FundCode: x.FundCode, FundName: x.FundName,
			Kind: vo.RequestKind(x.Kind), Date: dateOnly(x.DocDate), Amount: x.Amount}
	}
	return out, err
}
