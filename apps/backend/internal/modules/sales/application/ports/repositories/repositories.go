// Package repositories คือสัญญา persistence ที่ application ของ sales ต้องการ
//
// กติกา: ทุก method รับ CompanyID, ไม่เปิด transaction เอง; Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ
package repositories

import (
	"apexis/backend/internal/platform/dashperiod"
	"context"
	"time"

	"github.com/google/uuid"

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
	// TaxIDOwner: รหัสลูกค้าอื่นที่ใช้เลขผู้เสียภาษีนี้อยู่ ("" = ไม่มี) — ใช้แจ้ง error ที่อ่านเข้าใจได้ (DB unique index เป็นตัวกันจริง)
	TaxIDOwner(ctx context.Context, companyID vo.CompanyID, taxID string, except *vo.CustomerID) (string, error)
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

// InvoiceRepository: เอกสารขายที่ตั้งลูกหนี้ (ขายสินค้า / ขายบริการ / มัดจำ) — ตารางเดียวแยกด้วย kind
type InvoiceRepository interface {
	Create(ctx context.Context, i *entities.Invoice) error
	UpdateStatus(ctx context.Context, i *entities.Invoice) error
	// SaveState: ยอดที่ตัด / ภาษีที่ถึงกำหนด / มัดจำที่ถูกหัก / ใบกำกับภาษีก่อนรับเงิน ของใบเดียว
	SaveState(ctx context.Context, i *entities.Invoice) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (*entities.Invoice, error)
	// LockForSettlement: หัวเอกสาร (ไม่มีบรรทัด) FOR UPDATE เรียง id — query เดียว
	LockForSettlement(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) ([]*entities.Invoice, error)
	SaveReceived(ctx context.Context, invoices []*entities.Invoice) error
	Get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.InvoiceSummary], error)
	// Open: เอกสารที่ยังค้างรับของลูกค้า เรียงวันครบกำหนด (billingNoteID = เฉพาะใบในใบวางบิล) — query เดียว
	Open(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, billingNoteID *vo.BillingNoteID) ([]dto.OpenInvoice, error)
	// OpenDeposits: มัดจำที่รับเงินครบแล้วและยังหักได้ของลูกค้า — query เดียว
	OpenDeposits(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID) ([]dto.OpenDeposit, error)
	// Billable: ข้อมูลใบที่ใช้ตรวจตอนวางบิล (รวมว่าอยู่ในใบวางบิลอื่นที่ยังเปิดหรือไม่) — query เดียว
	Billable(ctx context.Context, companyID vo.CompanyID, ids []vo.InvoiceID) (map[vo.InvoiceID]entities.BillableInvoice, error)
}

// VatRecognition คือภาษีขายที่ถึงกำหนดของใบแจ้งหนี้บริการ 1 รายการ
type VatRecognition struct {
	Source     string // receipt | tax_invoice
	DocumentID uuid.UUID
	DocumentNo string
	Date       time.Time
	InvoiceID  vo.InvoiceID
	Base       vo.Money
	Vat        vo.Money
}

// VatRecognitionRepository: ประวัติภาษีที่ถึงกำหนด (รายงานภาษีขายตามวันที่ถึงกำหนด)
type VatRecognitionRepository interface {
	Create(ctx context.Context, companyID vo.CompanyID, rows []VatRecognition) error
	// LockDocument: รายการที่ยังมีผลของเอกสาร (FOR UPDATE)
	LockDocument(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) ([]VatRecognition, error)
	Cancel(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error
}

// BillingNoteRepository: ใบวางบิล
type BillingNoteRepository interface {
	Create(ctx context.Context, b *entities.BillingNote) error
	UpdateStatus(ctx context.Context, b *entities.BillingNote) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.BillingNoteID) (*entities.BillingNote, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.BillingNoteID) (dto.BillingNote, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.BillingNoteSummary], error)
}

// CreditNoteRepository: ใบลดหนี้ขาย
type CreditNoteRepository interface {
	Create(ctx context.Context, c *entities.CreditNote) error
	UpdateStatus(ctx context.Context, c *entities.CreditNote) error
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (*entities.CreditNote, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (dto.CreditNote, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.CreditNoteSummary], error)
	// Credited: ยอดที่ลดหนี้แล้วต่อบรรทัดใบแจ้งหนี้ (ใบที่ยังไม่ยกเลิก) — query เดียว
	Credited(ctx context.Context, companyID vo.CompanyID, invoiceID vo.InvoiceID) (map[int]entities.Credited, error)
	// HasPosted: ใบแจ้งหนี้มีใบลดหนี้ที่ยังไม่ยกเลิก
	HasPosted(ctx context.Context, companyID vo.CompanyID, invoiceID vo.InvoiceID) (bool, error)
}

// SettlementRepository: ประวัติการตัดยอดใบแจ้งหนี้จากเอกสารรับเงินของ finance (ใช้คำนวณอายุลูกหนี้ ณ วันที่)
type SettlementRepository interface {
	Create(ctx context.Context, companyID vo.CompanyID, doc dto.SettlementDoc, lines []dto.SettlementLine) error
	// LockDocument: รายการที่ยังมีผลของเอกสาร (FOR UPDATE) — ว่าง = ไม่พบ / ยกเลิกแล้ว
	LockDocument(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) ([]dto.SettlementLine, error)
	Cancel(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error
}

type ReportRepository interface {
	Outstanding(ctx context.Context, companyID vo.CompanyID, f dto.OutstandingFilter) (listing.Result[dto.OutstandingRow], error)
	Aging(ctx context.Context, companyID vo.CompanyID, f dto.AgingFilter) (listing.Result[dto.AgingRow], dto.AgingRow, error)
	// OutputVat: เอกสารที่ภาษีขายเกิดแล้ว (ขายสินค้า / มัดจำ / ใบกำกับภาษี / รับชำระบริการ / ใบลดหนี้ติดลบ) ในช่วงวันที่
	OutputVat(ctx context.Context, companyID vo.CompanyID, f dto.TaxFilter) (listing.Result[dto.TaxRow], dto.TaxRow, error)
	// VatTotals: ยอดรวมของ OutputVat ในช่วงวันที่ — query เดียว
	VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error)
	// UndueVat: ใบแจ้งหนี้บริการที่ภาษียังไม่ถึงกำหนด ณ วันที่ (ตั้งหัก ถึงกำหนด / ลดหนี้ ที่เกิดถึงวันนั้น)
	UndueVat(ctx context.Context, companyID vo.CompanyID, f dto.UndueVatFilter) (listing.Result[dto.UndueVatRow], dto.UndueVatRow, error)
	// Dashboard: ภาพรวมแอปขายในช่วง (aggregate ไม่กี่ query)
	Dashboard(ctx context.Context, companyID vo.CompanyID, p dashperiod.Period) (dto.Dashboard, error)
}
