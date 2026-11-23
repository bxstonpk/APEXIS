// Package repositories คือสัญญา persistence ที่ application ของ finance ต้องการ
//
// กติกา: ทุก method รับ CompanyID, ไม่เปิด transaction เอง (ใช้ transaction ของ use case ใน ctx),
// Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ, Create คืน domain.ErrCodeExists / ErrDocumentNoUsed เมื่อชน unique
package repositories

import (
	"context"
	"time"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type CashAccountRepository interface {
	Create(ctx context.Context, a *entities.CashAccount) error
	Update(ctx context.Context, a *entities.CashAccount) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) error
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) (*entities.CashAccount, error)
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.CashAccountID) ([]*entities.CashAccount, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.CashAccountID) (dto.CashAccount, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.Filter) (listing.Result[dto.CashAccount], error)
}

type SettingsRepository interface {
	// Get: ยังไม่เคยตั้ง = ค่าว่าง (ไม่ error)
	Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error)
	Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error
}

type WhtTypeRepository interface {
	Create(ctx context.Context, w *entities.WhtType) error
	Update(ctx context.Context, w *entities.WhtType) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.WhtTypeID) error // soft delete
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.WhtTypeID) (*entities.WhtType, error)
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.WhtTypeID) ([]*entities.WhtType, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.WhtTypeFilter) (listing.Result[dto.WhtType], error)
}

// Sequence: เลขลำดับถัดไปของชนิดเอกสาร + ช่วงนับ — ต้องอยู่ใน transaction (แถวถูก lock จน commit)
type Sequence interface {
	NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error)
}

type ReceiptRepository interface {
	// Create เขียนหัว + บรรทัด + ช่องทาง — ต้องอยู่ใน transaction
	Create(ctx context.Context, r *entities.Receipt) error
	UpdateStatus(ctx context.Context, r *entities.Receipt) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (*entities.Receipt, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (dto.Receipt, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error)
}

type PaymentRepository interface {
	Create(ctx context.Context, p *entities.Payment) error
	UpdateStatus(ctx context.Context, p *entities.Payment) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (*entities.Payment, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.PaymentID) (dto.Payment, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error)
}

type TransferRepository interface {
	Create(ctx context.Context, t *entities.Transfer) error
	UpdateStatus(ctx context.Context, t *entities.Transfer) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.TransferID) (*entities.Transfer, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.TransferID) (dto.Transfer, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.Transfer], error)
}

type ReportRepository interface {
	// WithholdingPaid: บรรทัดใบจ่าย (ยังไม่ยกเลิก) ที่หักภาษี + ยอดรวม — query เดียว แบ่งหน้าใน DB
	WithholdingPaid(ctx context.Context, companyID vo.CompanyID, f dto.WithholdingFilter) (listing.Result[dto.WithholdingRow], dto.WithholdingTotals, error)
	// WithholdingReceived: ใบรับ (ยังไม่ยกเลิก) ที่ลูกค้าหักภาษีไว้ + ยอดรวม — query เดียว
	WithholdingReceived(ctx context.Context, companyID vo.CompanyID, f dto.WithholdingFilter) (listing.Result[dto.WithholdingRow], dto.WithholdingTotals, error)
	// ActiveCashAccounts: ทะเบียนเงินที่ใช้งาน (เรียงตามรหัส) พร้อมบัญชีแยกประเภทที่ผูก
	ActiveCashAccounts(ctx context.Context, companyID vo.CompanyID) ([]dto.DashboardCashAccount, error)
	// FlowTotals: ยอดรับ / จ่าย / ภาษี / จำนวนโอนของช่วง — query เดียว (UNION ALL + FILTER)
	FlowTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.FlowTotals, error)
	// MonthlyFlows: ยอดรับ / จ่ายรายเดือนของ [from, to] — query เดียว (GROUP BY เดือน) — เดือนที่ไม่มีรายการไม่อยู่ในผล
	MonthlyFlows(ctx context.Context, companyID vo.CompanyID, from, to time.Time) ([]dto.MonthlyFlow, error)
	// RecentDocuments: เอกสารรับ / จ่าย / โอนที่ยังไม่ยกเลิกล่าสุด — query เดียว (UNION ALL)
	RecentDocuments(ctx context.Context, companyID vo.CompanyID, limit int) ([]dto.RecentDocument, error)
}
