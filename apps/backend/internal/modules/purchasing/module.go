// Package purchasing คือจัดซื้อ: ผู้ขาย (จด / ไม่จด VAT), ใบสั่งซื้อ (อนุมัติ), ใบรับสินค้า, ใบแจ้งหนี้ผู้ขาย
// และรายงานใบสั่งซื้อค้างรับ / อายุเจ้าหนี้
//
// กฎ VAT: ใบสั่งซื้อเก็บสถานะจด VAT / อัตรา / ราคารวมหรือแยก VAT ของผู้ขาย ณ ตอนสร้าง —
// ใบรับสินค้าและใบแจ้งหนี้ใช้ค่าในใบสั่งซื้อเสมอ; ต้นทุนสินค้าไม่รวม VAT
//
// การลงบัญชี (ระบบบัญชีตามปีบัญชีที่ inventory กำหนด):
//   - รับสินค้า: Dr สินค้าคงเหลือ (perpetual + stock) หรือบัญชีซื้อ (อื่นๆ) / Cr GR/IR
//   - แจ้งหนี้: Dr GR/IR ± ผลต่างราคา + Dr ภาษีซื้อ / Cr เจ้าหนี้
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น (ทั้งหมดอยู่ใน transaction เดียวกัน):
//   - identity: ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์
//   - organization: สถานะบริษัท + จำนวนทศนิยม, งวดบัญชี
//   - inventory: สินค้า + บัญชี, ระบบบัญชี, ใบรับเข้าคลัง (FIFO)
//   - ledger: ตรวจบัญชี / สมุดรายวัน, ลงรายการ / กลับรายการ
//
// ใบจ่ายชำระเป็นของ finance: ตัดยอดใบแจ้งหนี้ผ่าน SettlePayables / UnsettlePayables ใน transaction เดียวกัน
package purchasing

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/usecases"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
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
)

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger, inventory Inventory) *Module {
	org.RegisterDocumentTypes(DocumentTypes()...)
	uc := usecases.New(usecases.Deps{
		Tx:          gp.NewTransactionManager(db),
		Auth:        services.NewIdentityAuthorizer(identity),
		Org:         services.NewOrganization(org),
		Ledger:      services.NewLedger(ledger),
		Inventory:   services.NewInventory(inventory),
		Settings:    gormrepo.NewSettingsRepository(db),
		Suppliers:   gormrepo.NewSupplierRepository(db),
		Sequence:    gormrepo.NewSequenceRepository(),
		Orders:      gormrepo.NewOrderRepository(db),
		Receipts:    gormrepo.NewReceiptRepository(db),
		Invoices:    gormrepo.NewInvoiceRepository(db),
		Reports:     gormrepo.NewReportRepository(db),
		Settlements: gormrepo.NewSettlementRepository(db),
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
// Migrate: คืนผู้ขายรหัสซ้ำที่ถูกรวม (finance ต้องปรับ id คู่ค้าในใบจ่ายเงินตาม — ดู main.go)
func Migrate(db *gorm.DB) ([]integration.PartyMerge, error) { return gp.Migrate(db) }

// ---------- public API สำหรับ finance (เรียกภายใน transaction ของ finance) ----------

// SettlePayables ตัดยอดใบแจ้งหนี้ผู้ขายตามเอกสารจ่ายเงิน — คืนผู้ขาย ณ วันนี้ (ประเภทผู้เสียภาษี) + บัญชีเจ้าหนี้ + ใบแจ้งหนี้
// กฎธุรกิจของจัดซื้อ (เช่นเกินยอดค้าง, ไม่ได้ตั้งบัญชีเจ้าหนี้) ถูกห่อเป็น integration.ErrRejected
func (m *Module) SettlePayables(ctx context.Context, companyID, supplierID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error) {
	in := make([]dto.SettlementLine, len(lines))
	for i, l := range lines {
		in[i] = dto.SettlementLine{InvoiceID: vo.InvoiceID(l.InvoiceID), Amount: l.Amount}
	}
	res, err := m.UseCases.Settlements.Settle(ctx, vo.CompanyID(companyID), vo.SupplierID(supplierID),
		dto.SettlementDoc{ID: doc.DocumentID, No: doc.DocumentNo, Date: doc.Date}, in)
	if err != nil {
		return integration.SettlementResult{}, reject(err)
	}
	s := res.Supplier
	out := integration.SettlementResult{
		Party: integration.Counterparty{ID: uuid.UUID(s.ID), Code: s.Code, Name: s.Name, TaxID: s.TaxID, BranchNo: s.BranchNo,
			Address: s.Address, TaxType: string(s.TaxType)},
		ControlAccountID: uuid.UUID(res.APAccountID), Invoices: make([]integration.SettledInvoice, len(res.Invoices)),
	}
	for i, x := range res.Invoices {
		out.Invoices[i] = integration.SettledInvoice{InvoiceID: uuid.UUID(x.ID), DocumentNo: x.DocumentNo, PartnerRef: x.SupplierInvoiceNo, Net: x.Net, Gross: x.Gross}
	}
	return out, nil
}

// UnsettlePayables คืนยอดค้างให้ใบแจ้งหนี้ของเอกสารจ่ายเงิน (ยกเลิกใบจ่าย)
func (m *Module) UnsettlePayables(ctx context.Context, companyID, documentID uuid.UUID) error {
	return reject(m.UseCases.Settlements.Unsettle(ctx, vo.CompanyID(companyID), documentID))
}

// OpenPayables: ใบแจ้งหนี้ค้างจ่ายของผู้ขาย (ไม่ตรวจสิทธิ์ — finance ตรวจ finance.payment:read แล้ว)
func (m *Module) OpenPayables(ctx context.Context, companyID, supplierID uuid.UUID) ([]integration.OpenInvoice, error) {
	rows, err := m.UseCases.Settlements.Open(ctx, vo.CompanyID(companyID), vo.SupplierID(supplierID))
	if err != nil {
		return nil, err
	}
	out := make([]integration.OpenInvoice, len(rows))
	for i, r := range rows {
		id, _ := uuid.Parse(r.ID)
		out[i] = integration.OpenInvoice{ID: id, DocumentNo: r.DocumentNo, PartnerRef: r.SupplierInvoiceNo, Date: r.Date, DueDate: r.DueDate,
			Net: r.Net, Gross: r.Gross, Outstanding: r.Outstanding}
	}
	return out, nil
}

func reject(err error) error {
	if err != nil && domain.IsBusiness(err) {
		return integration.Reject("purchasing", err)
	}
	return err
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}
