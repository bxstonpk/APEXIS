// Package sales คือขาย: ลูกค้า (วงเงินเครดิต), ใบเสนอราคา, ใบสั่งขาย (อนุมัติ + เตือนวงเงิน),
// ขายสินค้า / ขายบริการ / ใบรับเงินมัดจำ / ใบกำกับภาษี / ใบวางบิล / ใบลดหนี้ และรายงานค้างส่ง / อายุลูกหนี้ / ภาษีขาย
//
// จุดเกิดภาษีขาย (ประมวลรัษฎากร): ขายสินค้า ม.78 = ส่งมอบ (ออกใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษีพร้อมตัดสต็อก),
// ขายบริการ ม.78/1 = รับชำระ (ใบแจ้งหนี้ลงภาษีขายยังไม่ถึงกำหนด แล้วถึงกำหนดตอน finance รับเงิน / ออกใบกำกับก่อนรับเงิน),
// มัดจำ = วันที่ใบรับเงินมัดจำ / ใบกำกับภาษี; ประเภทธุรกิจกำหนดว่าออกขายสินค้า (ซื้อขาย / ผลิต) หรือขายบริการ (บริการ / ก่อสร้าง)
//
// การลงบัญชี (ระบบบัญชีตามปีบัญชีที่ inventory กำหนด) — 1 ผังบัญชี = 1 บรรทัด:
//   - ขายสินค้า: Dr ลูกหนี้ / Cr รายได้ + Cr ภาษีขาย [+ Dr ต้นทุนขาย / Cr สินค้าคงเหลือ เมื่อ perpetual]
//   - ขายบริการ: Dr ลูกหนี้ / Cr รายได้ + Cr ภาษีขายยังไม่ถึงกำหนด
//   - มัดจำ: Dr ลูกหนี้ / Cr เงินมัดจำรับ + Cr ภาษีขาย; หักในเอกสารขายภายหลัง (Dr เงินมัดจำรับ, ภาษีของเอกสารลดลง)
//
// เงินเข้าทั้งหมดรับที่ finance: ตัดยอดผ่าน SettleReceivables / UnsettleReceivables ใน transaction เดียวกัน
package sales

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/usecases"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	gp "apexis/backend/internal/modules/sales/infrastructure/persistence/gorm"
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
)

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger, inventory Inventory) *Module {
	org.RegisterDocumentTypes(DocumentTypes()...)
	uc := usecases.New(usecases.Deps{
		Tx:           gp.NewTransactionManager(db),
		Auth:         services.NewAuthorizer(identity),
		Org:          services.NewOrganization(org),
		Ledger:       services.NewLedger(ledger),
		Inventory:    services.NewInventory(inventory),
		Settings:     gormrepo.NewSettingsRepository(db),
		Customers:    gormrepo.NewCustomerRepository(db),
		Sequence:     gormrepo.NewSequenceRepository(),
		Quotations:   gormrepo.NewQuotationRepository(db),
		Orders:       gormrepo.NewOrderRepository(db),
		Recognitions: gormrepo.NewVatRecognitionRepository(db),
		BillingNotes: gormrepo.NewBillingNoteRepository(db),
		Invoices:     gormrepo.NewInvoiceRepository(db),
		CreditNotes:  gormrepo.NewCreditNoteRepository(db),
		Settlements:  gormrepo.NewSettlementRepository(db),
		Reports:      gormrepo.NewReportRepository(db),
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

// Migrate: คืนลูกค้ารหัสซ้ำที่ถูกรวม (finance ต้องปรับ id คู่ค้าในใบรับเงินตาม — ดู main.go)
func Migrate(db *gorm.DB) ([]integration.PartyMerge, error) { return gp.Migrate(db) }

// ---------- public API สำหรับ finance (เรียกภายใน transaction ของ finance) ----------

// SettleReceivables ตัดยอดใบแจ้งหนี้ขายตามเอกสารรับเงิน — คืนลูกค้า ณ วันนี้ + บัญชีลูกหนี้ + ใบแจ้งหนี้
// กฎธุรกิจของขาย (เช่นเกินยอดค้าง, ไม่ได้ตั้งบัญชีลูกหนี้) ถูกห่อเป็น integration.ErrRejected
func (m *Module) SettleReceivables(ctx context.Context, companyID, customerID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error) {
	in := make([]dto.SettlementLine, len(lines))
	for i, l := range lines {
		in[i] = dto.SettlementLine{InvoiceID: vo.InvoiceID(l.InvoiceID), Amount: l.Amount}
	}
	res, err := m.UseCases.Settlements.Settle(ctx, vo.CompanyID(companyID), vo.CustomerID(customerID),
		dto.SettlementDoc{ID: doc.DocumentID, No: doc.DocumentNo, Date: doc.Date}, in)
	if err != nil {
		return integration.SettlementResult{}, reject(err)
	}
	c := res.Customer
	out := integration.SettlementResult{
		Party:            integration.Counterparty{ID: uuid.UUID(c.ID), Code: c.Code, Name: c.Name, TaxID: c.TaxID, BranchNo: c.BranchNo, Address: c.Address},
		ControlAccountID: uuid.UUID(res.ARAccountID), Invoices: make([]integration.SettledInvoice, len(res.Invoices)),
	}
	for i, x := range res.Invoices {
		out.Invoices[i] = integration.SettledInvoice{InvoiceID: uuid.UUID(x.ID), DocumentNo: x.DocumentNo, Kind: string(x.Kind), Net: x.Net, Gross: x.Gross,
			VatBase: x.VatBase, Vat: x.Vat}
	}
	for _, l := range res.ExtraJournal {
		out.ExtraJournal = append(out.ExtraJournal, integration.JournalLine{AccountID: uuid.UUID(l.AccountID), Debit: l.Debit, Credit: l.Credit,
			Description: "Output VAT due on receipt " + doc.DocumentNo})
	}
	return out, nil
}

// UnsettleReceivables คืนยอดค้างให้ใบแจ้งหนี้ของเอกสารรับเงิน (ยกเลิกใบเสร็จ)
func (m *Module) UnsettleReceivables(ctx context.Context, companyID, documentID uuid.UUID) error {
	return reject(m.UseCases.Settlements.Unsettle(ctx, vo.CompanyID(companyID), documentID))
}

// OpenReceivables: เอกสารขายค้างรับของลูกค้า (ขายสินค้า / ขายบริการ / มัดจำ) — billingNoteID = เฉพาะใบในใบวางบิล
// ไม่ตรวจสิทธิ์ (finance ตรวจ finance.receipt:read แล้ว)
func (m *Module) OpenReceivables(ctx context.Context, companyID, customerID uuid.UUID, billingNoteID *uuid.UUID) ([]integration.OpenInvoice, error) {
	var bn *vo.BillingNoteID
	if billingNoteID != nil {
		b := vo.BillingNoteID(*billingNoteID)
		bn = &b
	}
	rows, err := m.UseCases.Settlements.Open(ctx, vo.CompanyID(companyID), vo.CustomerID(customerID), bn)
	if err != nil {
		return nil, err
	}
	out := make([]integration.OpenInvoice, len(rows))
	for i, r := range rows {
		id, _ := uuid.Parse(r.ID)
		out[i] = integration.OpenInvoice{ID: id, DocumentNo: r.DocumentNo, Date: r.Date, DueDate: r.DueDate, Net: r.Net, Gross: r.Gross, Outstanding: r.Outstanding,
			Kind: r.Kind, VatUndue: r.VatUndue, TaxInvoice: r.TaxInvoice}
	}
	return out, nil
}

func reject(err error) error {
	if err != nil && domain.IsBusiness(err) {
		return integration.Reject("sales", err)
	}
	return err
}

// ProvisionCompany ตั้งค่าเริ่มต้นของ module ให้บริษัทใหม่ตามประเภทธุรกิจ (บัญชี / สมุดรายวันจากผังบัญชีมาตรฐาน)
// ลงทะเบียนเป็น hook ของ organization.Module.OnCompanyCreated หลัง ledger — ทำงานใน transaction ของการสร้างบริษัท
func (m *Module) ProvisionCompany(ctx context.Context, c integration.NewCompany) error {
	return m.UseCases.Settings.Provision(ctx, vo.CompanyID(c.ID), c.BusinessType)
}

// PermissionMigration: แปลงสิทธิ์เดิมของบทบาท (From -> To, Keep = คงสิทธิ์เดิมไว้ด้วย; To ว่าง = ลบสิทธิ์)
type PermissionMigration struct {
	From string
	To   []string
	Keep bool
}

// PermissionMigrations: ขายแบบใหม่ (2026-11-26) — ไม่มีใบส่งของแยก จึงลบสิทธิ์ใบส่งของออกจากทุกบทบาท (ทำซ้ำได้)
// สิทธิ์ใบวางบิล (sales.billing_note:*) ให้ผู้ดูแลบทบาทตั้งเอง — ไม่แปลงอัตโนมัติ (จะถูกเพิ่มกลับทุกครั้งที่เริ่มระบบ)
func PermissionMigrations() []PermissionMigration {
	return []PermissionMigration{
		{From: "sales.delivery:*"},
		{From: "sales.delivery:read"},
		{From: "sales.delivery:post"},
	}
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}

// OutputVat: ยอดภาษีขายของช่วงวันที่ (ใบแจ้งหนี้ที่ผ่านรายการ หักใบลดหนี้) — finance ใช้ทำสรุป ภ.พ.30
// ไม่ตรวจสิทธิ์ของขาย (finance ตรวจ finance.report:read แล้ว); start / end เป็นวันที่ (UTC, ไม่มีเวลา)
func (m *Module) OutputVat(ctx context.Context, companyID uuid.UUID, start, end time.Time) (integration.VatTotals, error) {
	t, err := m.UseCases.Reports.VatTotals(ctx, vo.CompanyID(companyID), start, end)
	if err != nil {
		return integration.VatTotals{}, reject(err)
	}
	return integration.VatTotals{Base: t.Base, Tax: t.Tax, Documents: t.Documents}, nil
}
