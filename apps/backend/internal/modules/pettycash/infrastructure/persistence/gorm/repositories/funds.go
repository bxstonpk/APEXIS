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

type FundRepository struct{ db *gorm.DB }

func NewFundRepository(db *gorm.DB) *FundRepository { return &FundRepository{db: db} }

func fundErr(err error) error {
	if database.IsUniqueViolation(err) {
		return domain.ErrCodeExists
	}
	return err
}

func (r *FundRepository) Create(ctx context.Context, companyID vo.CompanyID, f *entities.Fund) error {
	m := gp.FundToModel(companyID, f)
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt = now, now
	return fundErr(database.Conn(ctx, r.db).Create(&m).Error)
}

func (r *FundRepository) Update(ctx context.Context, companyID vo.CompanyID, f *entities.Fund) error {
	m := gp.FundToModel(companyID, f)
	return fundErr(database.Conn(ctx, r.db).Model(&models.FundModel{}).Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{"name": m.Name, "custodian_id": m.CustodianID, "gl_account_id": m.GLAccountID,
			"imprest_amount": m.ImprestAmount, "status": m.Status, "updated_at": time.Now().UTC()}).Error)
}

// FindForUpdate — SELECT ... FOR UPDATE
func (r *FundRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.FundID) (*entities.Fund, error) {
	var m models.FundModel
	err := database.Conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrFundNotFound
	}
	if err != nil {
		return nil, err
	}
	return gp.FundToEntity(m), nil
}

type fundRow struct {
	ID            uuid.UUID
	Code          string
	Name          string
	CustodianID   uuid.UUID
	GLAccountID   uuid.UUID
	ImprestAmount decimal.Decimal
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Total         int64
}

func (x fundRow) dto() dto.Fund {
	return dto.Fund{ID: x.ID.String(), Code: x.Code, Name: x.Name, Custodian: dto.Ref{ID: x.CustodianID.String()},
		GLAccount: dto.Ref{ID: x.GLAccountID.String()}, ImprestAmount: x.ImprestAmount, Status: vo.Status(x.Status),
		CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt}
}

const fundColumns = "f.id, f.code, f.name, f.custodian_id, f.gl_account_id, f.imprest_amount, f.status, f.created_at, f.updated_at"

// Get — กองทุน 1 + ยอด 1
func (r *FundRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.FundID) (dto.Fund, error) {
	var row fundRow
	err := database.Conn(ctx, r.db).Table("petty_cash_funds AS f").Select(fundColumns).
		Where("f.company_id = ? AND f.id = ?", uuidOf(companyID), uuidOf(id)).Take(&row).Error
	if database.IsNotFound(err) {
		return dto.Fund{}, domain.ErrFundNotFound
	}
	if err != nil {
		return dto.Fund{}, err
	}
	out := row.dto()
	bal, err := r.Balances(ctx, companyID, []vo.FundID{id}, nil)
	out.Balance = bal[id]
	return out, err
}

var fundSortColumns = map[string]string{"code": "f.code", "name": "f.name", "updated_at": "f.updated_at"}

// List — หน้า 1 (+COUNT) + ยอดของกองทุนในหน้า 1
func (r *FundRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.FundFilter) (listing.Result[dto.Fund], error) {
	order, err := database.OrderBy(f.Sort, fundSortColumns, "f.id")
	if err != nil {
		return listing.Result[dto.Fund]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("petty_cash_funds AS f").Where("f.company_id = ?", uuidOf(companyID))
		q = search(q, f.Search, "f.code", "f.name")
		if len(f.Status) > 0 {
			q = q.Where("f.status IN ?", strs(f.Status))
		}
		return q
	}
	rows, total, err := database.FetchPage(base, fundColumns, order, f.Page, func(x fundRow) int64 { return x.Total })
	if err != nil {
		return listing.Result[dto.Fund]{}, err
	}
	res := toResult(rows, total, f.Page, fundRow.dto)
	ids := make([]vo.FundID, len(rows))
	for i, x := range rows {
		ids[i] = vo.FundID(x.ID)
	}
	bal, err := r.Balances(ctx, companyID, ids, nil)
	for i := range res.Items {
		res.Items[i].Balance = bal[ids[i]]
	}
	return res, err
}

// farFuture: "ณ ปัจจุบัน" = ทุกเอกสาร
var farFuture = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// Balances — query เดียว (sub-query ต่อกองทุนใช้ index idx_pc_requests_fund / idx_pc_vouchers_fund):
//
//	เงินเข้า = คำขอที่การเงินจ่ายแล้ว (วันที่จ่าย <= asOf), ใช้ไป = ใบสำคัญจ่ายที่อนุมัติแล้ว / ชดเชยแล้ว (วันที่เอกสาร <= asOf)
//	รอชดเชย = ใช้ไปที่คำขอยังไม่ได้รับเงิน ณ asOf, รอการเงินจ่าย = คำขอที่ยังไม่จ่าย (วันที่เอกสาร <= asOf)
func (r *FundRepository) Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.FundID, asOf *time.Time) (map[vo.FundID]dto.Balance, error) {
	out := make(map[vo.FundID]dto.Balance, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	at := farFuture
	if asOf != nil {
		at = dateOnly(*asOf)
	}
	var rows []struct {
		ID        uuid.UUID
		Funded    bool
		PaidIn    decimal.Decimal
		Spent     decimal.Decimal
		Pending   decimal.Decimal
		Requested decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Raw(`
		SELECT f.id,
			EXISTS (SELECT 1 FROM petty_cash_replenishments r
				WHERE r.fund_id = f.id AND r.kind = 'establish' AND r.status = 'paid' AND r.paid_date <= @at) AS funded,
			COALESCE((SELECT SUM(r.amount) FROM petty_cash_replenishments r
				WHERE r.fund_id = f.id AND r.status = 'paid' AND r.paid_date <= @at), 0) AS paid_in,
			COALESCE((SELECT SUM(v.total_gross) FROM petty_cash_vouchers v
				WHERE v.fund_id = f.id AND v.status IN ('approved', 'replenished') AND v.doc_date <= @at), 0) AS spent,
			COALESCE((SELECT SUM(v.total_gross) FROM petty_cash_vouchers v
				WHERE v.fund_id = f.id AND v.status IN ('approved', 'replenished') AND v.doc_date <= @at
				AND NOT EXISTS (SELECT 1 FROM petty_cash_replenishments r
					WHERE r.id = v.replenishment_id AND r.status = 'paid' AND r.paid_date <= @at)), 0) AS pending,
			COALESCE((SELECT SUM(r.amount) FROM petty_cash_replenishments r
				WHERE r.fund_id = f.id AND r.status = 'requested' AND r.doc_date <= @at), 0) AS requested
		FROM petty_cash_funds f
		WHERE f.company_id = @company AND f.id IN @ids`,
		map[string]any{"at": at, "company": uuidOf(companyID), "ids": uuids(ids)}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, x := range rows {
		out[vo.FundID(x.ID)] = dto.Balance{Funded: x.Funded, CashOnHand: x.PaidIn.Sub(x.Spent), Pending: x.Pending, Requested: x.Requested}
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = dto.Balance{CashOnHand: decimal.Zero, Pending: decimal.Zero, Requested: decimal.Zero}
		}
	}
	return out, nil
}
