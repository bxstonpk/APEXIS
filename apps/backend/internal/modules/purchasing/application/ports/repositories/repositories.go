// Package repositories คือสัญญา (port) ที่ application ของ purchasing ต้องการจาก persistence
//
// กติกา: ทุก method รับ CompanyID, ไม่เปิด transaction เอง; Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ
package repositories

import (
	"context"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type SettingsRepository interface {
	// Get: ยังไม่เคยตั้ง = entities.DefaultSettings()
	Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error)
	Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error
}

type SupplierRepository interface {
	Create(ctx context.Context, s *entities.Supplier) error // domain.ErrCodeExists เมื่อรหัสซ้ำ
	Update(ctx context.Context, s *entities.Supplier) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) error // soft delete
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (*entities.Supplier, error)
	InUse(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (bool, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.SupplierID) (dto.Supplier, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.SupplierFilter) (listing.Result[dto.Supplier], error)
}

// Sequence: เลขที่เอกสารถัดไปต่อ prefix + ปีบัญชี — lock แถวลำดับจน commit (ไม่ซ้ำ / ไม่ขาดตอน)
type Sequence interface {
	NextNumber(ctx context.Context, companyID vo.CompanyID, prefix string, fiscalYear int) (int64, error)
}

type OrderRepository interface {
	// Create / Update เขียนหัว + บรรทัด (Update แทนที่บรรทัดของ draft) — ต้องอยู่ใน transaction
	Create(ctx context.Context, o *entities.Order) error
	Update(ctx context.Context, o *entities.Order) error
	// SaveProgress เขียนสถานะ / การอนุมัติ / ปิด / ยกเลิก และความคืบหน้าของทุกบรรทัด
	SaveProgress(ctx context.Context, o *entities.Order) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) error // ลบจริง (เฉพาะ draft)
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (*entities.Order, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (dto.Order, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.OrderSummary], error)
}

type ReceiptRepository interface {
	Create(ctx context.Context, r *entities.Receipt) error
	UpdateStatus(ctx context.Context, r *entities.Receipt) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (*entities.Receipt, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (dto.Receipt, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.ReceiptSummary], error)
}

type InvoiceRepository interface {
	// Create คืน domain.ErrInvoiceNoExists เมื่อเลขที่ใบแจ้งหนี้ของผู้ขายซ้ำ (ที่ยังไม่ยกเลิก)
	Create(ctx context.Context, i *entities.Invoice) error
	UpdateStatus(ctx context.Context, i *entities.Invoice) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (*entities.Invoice, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.InvoiceSummary], error)
}

type ReportRepository interface {
	Outstanding(ctx context.Context, companyID vo.CompanyID, f dto.OutstandingFilter) (listing.Result[dto.OutstandingRow], error)
	// Aging: ใบแจ้งหนี้ที่ลงบัญชีถึงวันที่ asOf (ยังไม่ยกเลิก) ตามอายุนับจากวันครบกำหนด — แบ่งหน้าใน DB
	Aging(ctx context.Context, companyID vo.CompanyID, f dto.AgingFilter) (listing.Result[dto.AgingRow], dto.AgingRow, error)
}
