// Package manufacturing คือการผลิต (ใช้ได้เฉพาะบริษัทประเภทธุรกิจผลิต): สูตรการผลิต (BOM), ใบสั่งผลิต
// (เบิก / คืนวัตถุดิบ, ปิดงานรับสินค้าสำเร็จรูปที่ต้นทุนจริง) และรายงานต้นทุนการผลิต / การใช้วัตถุดิบ / งานระหว่างผลิต
//
// ระบบบัญชีสินค้าตามปีบัญชีที่ inventory กำหนด (ค่าเริ่มต้น = สิ้นงวด / periodic ทุกประเภทธุรกิจ):
//   - periodic: มีแค่เอกสารสต็อก (FIFO) และต้นทุนจริงของสินค้าสำเร็จรูป — ไม่ลงบัญชี (ปรับสินค้าคงเหลือตอนปิดงวดของ inventory)
//   - perpetual: ลงบัญชีทุกการเคลื่อนไหว — เบิก Dr WIP / Cr สินค้าคงเหลือ, คืนกลับด้าน,
//     ปิดงาน Dr สินค้าสำเร็จรูป / Cr WIP / Cr ค่าแรงและค่าใช้จ่ายการผลิตปันเข้างาน
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น (ทั้งหมดอยู่ใน transaction เดียวกัน):
//   - identity: ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์
//   - organization: สถานะบริษัท + ทศนิยม, ประเภทธุรกิจ, งวดบัญชี, รูปแบบเลขที่ใบสั่งผลิต
//   - inventory: สินค้า + บัญชี, ระบบบัญชี, คลัง, ใบเบิก / ใบรับเข้า (FIFO), ยกเลิกเอกสารสต็อก
//   - ledger: ตรวจบัญชี / สมุดรายวัน, ชื่อบัญชี, ลงรายการ / กลับรายการ
package manufacturing

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/manufacturing/application/usecases"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	gp "apexis/backend/internal/modules/manufacturing/infrastructure/persistence/gorm"
	gormrepo "apexis/backend/internal/modules/manufacturing/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/manufacturing/infrastructure/services"
	manufacturinghttp "apexis/backend/internal/modules/manufacturing/presentation/http"
	"apexis/backend/internal/modules/manufacturing/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/reports"
)

// Identity คือสิ่งที่ manufacturing ต้องการจาก identity module (*identity.Module implement)
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
)

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger, inventory Inventory) *Module {
	// ใบสั่งผลิตตั้งรูปแบบเลขที่ได้ที่หน้าตั้งค่าเลขที่เอกสารของบริษัท (ค่าเริ่มต้น MO{yyyy}-{seq})
	org.RegisterDocumentTypes(DocumentTypes()...)
	uc := usecases.New(usecases.Deps{
		Tx:        gp.NewTransactionManager(db),
		Auth:      services.NewIdentityAuthorizer(identity),
		Org:       services.NewOrganization(org),
		Ledger:    services.NewLedger(ledger),
		Inventory: services.NewInventory(inventory),
		Settings:  gormrepo.NewSettingsRepository(db),
		Boms:      gormrepo.NewBomRepository(db),
		Orders:    gormrepo.NewOrderRepository(db),
		Sequence:  gormrepo.NewSequenceRepository(),
		Reports:   gormrepo.NewReportRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	manufacturinghttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของ manufacturing กับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) {
	manufacturinghttp.RegisterReports(reg, m.handler)
}

// DocumentTypes คือชนิดเอกสารของการผลิตที่ตั้งรูปแบบเลขที่ได้ (ลงทะเบียนกับ organization ตอน New)
func DocumentTypes() []integration.DocumentType {
	out := make([]integration.DocumentType, len(vo.DocTypes))
	for i, t := range vo.DocTypes {
		out[i] = integration.DocumentType{Key: string(t), Name: t.Name(), DefaultPrefix: t.Prefix()}
	}
	return out
}

// Migrate สร้างตารางของ manufacturing
func Migrate(db *gorm.DB) error { return gp.Migrate(db) }

// ProvisionCompany ตั้งค่าการผลิตเริ่มต้นของบริษัทผลิตที่เพิ่งสร้าง (สมุด GEN, งานระหว่างผลิต, ค่าแรงปันเข้างาน)
// ลงทะเบียนเป็น hook ของ organization.Module.OnCompanyCreated หลัง ledger — ทำงานใน transaction ของการสร้างบริษัท
func (m *Module) ProvisionCompany(ctx context.Context, c integration.NewCompany) error {
	return m.UseCases.Settings.Provision(ctx, vo.CompanyID(c.ID), c.BusinessType)
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}
