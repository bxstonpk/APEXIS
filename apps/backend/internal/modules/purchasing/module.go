// Package purchasing คือจัดซื้อ: ผู้ขาย (จด / ไม่จด VAT), ใบสั่งซื้อ (อนุมัติ), ใบรับสินค้า, ใบแจ้งหนี้ผู้ขาย
// และรายงานใบสั่งซื้อค้างรับ / อายุเจ้าหนี้
//
// กฎ VAT: ใบสั่งซื้อเก็บสถานะจด VAT / อัตรา / ราคารวมหรือแยก VAT ของผู้ขาย ณ ตอนสร้าง —
// ใบรับสินค้าและใบแจ้งหนี้ใช้ค่าในใบสั่งซื้อเสมอ; ต้นทุนสินค้าไม่รวม VAT
//
// การลงบัญชี (ระบบบัญชีตามปีบัญชีที่ inventory กำหนด):
//   - รับสินค้า: Dr สินค้าคงเหลือ (perpetual + stock) หรือบัญชีซื้อ (อื่นๆ) / Cr GR/IR
//   - แจ้งหนี้: Dr GR/IR ± ผลต่างราคา + Dr ภาษีซื้อ / Cr เจ้าหนี้
//   - จ่ายชำระ: Dr เจ้าหนี้ / Cr เงินสด / ธนาคาร (ตามทะเบียน) + Cr ภาษีหัก ณ ที่จ่ายค้างจ่าย (ภ.ง.ด.3 / 53)
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น (ทั้งหมดอยู่ใน transaction เดียวกัน):
//   - identity: ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์
//   - organization: สถานะบริษัท + จำนวนทศนิยม, งวดบัญชี
//   - inventory: สินค้า + บัญชี, ระบบบัญชี, ใบรับเข้าคลัง (FIFO)
//   - ledger: ตรวจบัญชี / สมุดรายวัน, ลงรายการ / กลับรายการ
//   - cashbank: ทะเบียนเงินสด / ธนาคารที่ใช้จ่ายชำระ (บัญชีที่ผูกไว้)
package purchasing

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/purchasing/application/usecases"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/purchasing/infrastructure/services"
	purchasinghttp "apexis/backend/internal/modules/purchasing/presentation/http"
	"apexis/backend/internal/modules/purchasing/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/reports"
)

// Identity คือสิ่งที่ purchasing ต้องการจาก identity module (*identity.Module implement)
type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

// Organization / Ledger / Inventory คือ public API ของ module นั้น (*organization.Module, *ledger.Module, *inventory.Module)
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
		Tx:        gp.NewTransactionManager(db),
		Auth:      services.NewIdentityAuthorizer(identity),
		Org:       services.NewOrganization(org),
		Ledger:    services.NewLedger(ledger),
		Inventory: services.NewInventory(inventory),
		CashBank:  services.NewCashBank(cashbank),
		Settings:  gormrepo.NewSettingsRepository(db),
		Suppliers: gormrepo.NewSupplierRepository(db),
		Sequence:  gormrepo.NewSequenceRepository(),
		Orders:    gormrepo.NewOrderRepository(db),
		Receipts:  gormrepo.NewReceiptRepository(db),
		Invoices:  gormrepo.NewInvoiceRepository(db),
		Reports:   gormrepo.NewReportRepository(db),
		WhtTypes:  gormrepo.NewWhtTypeRepository(db),
		Payments:  gormrepo.NewPaymentRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	purchasinghttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของ purchasing กับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) {
	purchasinghttp.RegisterReports(reg, m.handler)
}

// DocumentTypes คือชนิดเอกสารของจัดซื้อที่ตั้งรูปแบบเลขที่ได้ (ลงทะเบียนกับ organization ตอน New)
func DocumentTypes() []integration.DocumentType {
	out := make([]integration.DocumentType, len(vo.DocTypes))
	for i, t := range vo.DocTypes {
		out[i] = integration.DocumentType{Key: string(t), Name: t.Name(), DefaultPrefix: t.Prefix()}
	}
	return out
}

// Migrate สร้างตารางของ purchasing
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}
