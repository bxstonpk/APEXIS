// Package sales คือขาย: ลูกค้า (วงเงินเครดิต), ใบเสนอราคา, ใบสั่งขาย (อนุมัติ + เตือนวงเงิน),
// ใบส่งของ / ใบแจ้งหนี้-ใบกำกับภาษี (แยกใบหรือรวมใบ), ใบเสร็จรับเงิน และรายงานค้างส่ง / อายุลูกหนี้ / ภาษีขาย / ภาษีถูกหัก ณ ที่จ่าย
//
// VAT ตามค่าตั้งขาย (บริษัทจด / ไม่จด VAT) เก็บในใบเสนอราคา / ใบสั่งขายตอนสร้าง — เอกสารที่ตามมาใช้ค่าในใบ
//
// การลงบัญชี (ระบบบัญชีตามปีบัญชีที่ inventory กำหนด) — 1 ผังบัญชี = 1 บรรทัด:
//   - ส่งของ: perpetual + สินค้า stock = Dr ต้นทุนขาย / Cr สินค้าคงเหลือ (ต้นทุน FIFO); periodic ไม่มีรายการ
//   - ออกบิล: Dr ลูกหนี้ / Cr รายได้ (สินค้า -> หมวด -> ค่าตั้ง) + Cr ภาษีขาย [+ ต้นทุนขายเมื่อรวมใบ]
//   - รับเงิน: Dr เงินสด / ธนาคาร (cashbank) + Dr ภาษีถูกหัก ณ ที่จ่าย / Cr ลูกหนี้
package sales

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/sales/application/usecases"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	gp "apexis/backend/internal/modules/sales/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/sales/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/sales/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/sales/infrastructure/services"
	"apexis/backend/internal/modules/sales/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/reports"
)

type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

type (
	Organization = services.OrganizationAccess
	Ledger       = services.LedgerAccess
	Inventory    = services.InventoryAccess
	CashBank     = services.CashBankAccess
)

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger, inventory Inventory, cashbank CashBank) *Module {
	org.RegisterDocumentTypes(DocumentTypes()...)
	uc := usecases.New(usecases.Deps{
		Tx:         gp.NewTransactionManager(db),
		Auth:       services.NewAuthorizer(identity),
		Org:        services.NewOrganization(org),
		Ledger:     services.NewLedger(ledger),
		Inventory:  services.NewInventory(inventory),
		CashBank:   services.NewCashBank(cashbank),
		Settings:   gormrepo.NewSettingsRepository(db),
		Customers:  gormrepo.NewCustomerRepository(db),
		Sequence:   gormrepo.NewSequenceRepository(),
		Quotations: gormrepo.NewQuotationRepository(db),
		Orders:     gormrepo.NewOrderRepository(db),
		Deliveries: gormrepo.NewDeliveryRepository(db),
		Invoices:   gormrepo.NewInvoiceRepository(db),
		Receipts:   gormrepo.NewReceiptRepository(db),
		Reports:    gormrepo.NewReportRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	handlers.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของ sales กับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) { handlers.RegisterReports(reg, m.handler) }

// DocumentTypes คือชนิดเอกสารของขายที่ตั้งรูปแบบเลขที่ได้ (ลงทะเบียนกับ organization ตอน New)
func DocumentTypes() []integration.DocumentType {
	out := make([]integration.DocumentType, len(vo.DocTypes))
	for i, t := range vo.DocTypes {
		out[i] = integration.DocumentType{Key: string(t), Name: t.Name(), DefaultPrefix: t.Prefix()}
	}
	return out
}

func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
