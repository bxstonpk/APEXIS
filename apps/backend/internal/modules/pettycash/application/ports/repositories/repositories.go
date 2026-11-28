// Package repositories คือ interface ของ persistence ที่ use case ของ pettycash ต้องการ
// implementation อยู่ใน infrastructure/persistence/gorm/repositories — use case ไม่รู้จัก gorm
package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type SettingsRepository interface {
	Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error)
	Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error
}

// FundRepository: Create / Update คืน domain.ErrCodeExists เมื่อรหัสซ้ำ (unique index)
type FundRepository interface {
	Create(ctx context.Context, companyID vo.CompanyID, f *entities.Fund) error
	Update(ctx context.Context, companyID vo.CompanyID, f *entities.Fund) error
	// FindForUpdate lock แถวกองทุนจน commit (อนุมัติใบสำคัญจ่าย / จ่ายคำขอเรียงคิวต่อกองทุน)
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.FundID) (*entities.Fund, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.FundID) (dto.Fund, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.FundFilter) (listing.Result[dto.Fund], error)
	// Balances: ยอดของกองทุน (asOf nil = ปัจจุบัน) — query เดียวทุกกองทุน
	Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.FundID, asOf *time.Time) (map[vo.FundID]dto.Balance, error)
}

type VoucherRepository interface {
	Create(ctx context.Context, v *entities.Voucher) error
	Update(ctx context.Context, v *entities.Voucher) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.VoucherID) (*entities.Voucher, error)
	// FindManyForUpdate คืนตามลำดับ ids; ไม่พบ = domain.ErrVoucherNotFound (wrap เลข id)
	FindManyForUpdate(ctx context.Context, companyID vo.CompanyID, ids []vo.VoucherID) ([]*entities.Voucher, error)
	ByReplenishmentForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) ([]*entities.Voucher, error)
	// UpdateMany บันทึกสถานะ / คำขอของหลายใบ
	UpdateMany(ctx context.Context, vs []*entities.Voucher) error
	Get(ctx context.Context, companyID vo.CompanyID, id vo.VoucherID) (dto.Voucher, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.VoucherFilter) (listing.Result[dto.VoucherSummary], error)
	Summaries(ctx context.Context, companyID vo.CompanyID, ids []vo.VoucherID) ([]dto.VoucherSummary, error)
}

type ReplenishmentRepository interface {
	Create(ctx context.Context, r *entities.Replenishment) error
	Update(ctx context.Context, r *entities.Replenishment) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) (*entities.Replenishment, error)
	// FindByPaymentForUpdate: คำขอที่ใบจ่ายของการเงิน (documentID) จ่ายไว้
	FindByPaymentForUpdate(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) (*entities.Replenishment, error)
	// HasOpenFunding: กองทุนมีคำขอตั้ง / เพิ่มวงเงินที่ยังรอการเงินจ่าย
	HasOpenFunding(ctx context.Context, companyID vo.CompanyID, fundID vo.FundID) (bool, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) (dto.Replenishment, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.ReplenishmentFilter) (listing.Result[dto.ReplenishmentSummary], error)
	Open(ctx context.Context, companyID vo.CompanyID) ([]dto.OpenRequest, error)
}

type Sequence interface {
	NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error)
}

type ReportRepository interface {
	Register(ctx context.Context, companyID vo.CompanyID, f dto.RegisterFilter) (dto.RegisterReport, error)
	FundRows(ctx context.Context, companyID vo.CompanyID) ([]dto.FundBalanceRow, error)
	InputVat(ctx context.Context, companyID vo.CompanyID, f dto.InputVatFilter) (dto.InputVatReport, error)
	VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error)
}
