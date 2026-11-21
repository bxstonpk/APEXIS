// Package repositories คือสัญญา persistence ที่ application ของ sales ต้องการ
//
// กติกา: ทุก method รับ CompanyID, ไม่เปิด transaction เอง; Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ
package repositories

import (
	"context"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/domain/entities"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type SettingsRepository interface {
	Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error) // ยังไม่ตั้ง = DefaultSettings
	Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error
}

type CustomerRepository interface {
	Create(ctx context.Context, c *entities.Customer) error
	Update(ctx context.Context, c *entities.Customer) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) error
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (*entities.Customer, error)
	InUse(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (bool, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.CustomerID) (dto.Customer, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.CustomerFilter) (listing.Result[dto.Customer], error)
}

type Sequence interface {
	NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error)
}

type QuotationRepository interface {
	Create(ctx context.Context, q *entities.Quotation) error
	Update(ctx context.Context, q *entities.Quotation) error // แทนที่ draft (หัว + บรรทัด)
	SaveStatus(ctx context.Context, q *entities.Quotation) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) (*entities.Quotation, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) (dto.Quotation, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.SalesDocSummary], error)
}

type OrderRepository interface {
	Create(ctx context.Context, o *entities.Order) error
	Update(ctx context.Context, o *entities.Order) error
	SaveProgress(ctx context.Context, o *entities.Order) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (*entities.Order, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (dto.Order, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.SalesDocSummary], error)
	// Exposure: ยอดค้างของลูกค้า = ลูกหนี้คงค้าง + ใบสั่งขายที่อนุมัติแล้วส่วนที่ยังไม่ออกบิล (ไม่รวม exclude) — query เดียว
	Exposure(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, exclude vo.OrderID, amountDecimals int) (vo.Money, error)
}

type DeliveryRepository interface {
	Create(ctx context.Context, d *entities.Delivery) error
	UpdateStatus(ctx context.Context, d *entities.Delivery) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.DeliveryID) (*entities.Delivery, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.DeliveryID) (dto.Delivery, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.PostedSummary], error)
}

type InvoiceRepository interface {
	Create(ctx context.Context, i *entities.Invoice) error
	UpdateStatus(ctx context.Context, i *entities.Invoice) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (*entities.Invoice, error)
	// LockForReceipt: หัวใบแจ้งหนี้ (ไม่มีบรรทัด) FOR UPDATE เรียง id — query เดียว
	LockForReceipt(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) ([]*entities.Invoice, error)
	SaveReceived(ctx context.Context, invoices []*entities.Invoice) error
	Get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.InvoiceSummary], error)
}

type ReceiptRepository interface {
	Create(ctx context.Context, r *entities.Receipt) error
	UpdateStatus(ctx context.Context, r *entities.Receipt) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (*entities.Receipt, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (dto.Receipt, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.ReceiptSummary], error)
}

type ReportRepository interface {
	Outstanding(ctx context.Context, companyID vo.CompanyID, f dto.OutstandingFilter) (listing.Result[dto.OutstandingRow], error)
	Aging(ctx context.Context, companyID vo.CompanyID, f dto.AgingFilter) (listing.Result[dto.AgingRow], dto.AgingRow, error)
	// OutputVat: ใบแจ้งหนี้ (ยังไม่ยกเลิก) ในช่วงวันที่ — ฐาน = ยอดก่อน VAT, ภาษี = VAT
	OutputVat(ctx context.Context, companyID vo.CompanyID, f dto.TaxFilter) (listing.Result[dto.TaxRow], dto.TaxRow, error)
	// WithholdingReceived: บรรทัดใบเสร็จ (ยังไม่ยกเลิก) ที่ลูกค้าหักภาษีไว้
	WithholdingReceived(ctx context.Context, companyID vo.CompanyID, f dto.TaxFilter) (listing.Result[dto.TaxRow], dto.TaxRow, error)
}
