// Package finance คือการเงิน: ทะเบียนเงินสด / ธนาคาร และเอกสารเงินเข้า-ออกทั้งหมดของบริษัท
//
//   - รับเงิน: รับชำระหนี้ลูกค้า (ตัดยอดใบแจ้งหนี้ขาย + ภาษีถูกหัก ณ ที่จ่าย) หรือรับเงินอื่น (ระบุบัญชีเอง)
//   - จ่ายเงิน: จ่ายชำระหนี้ผู้ขาย (ตัดยอดใบแจ้งหนี้ซื้อ + ภาษีหัก ณ ที่จ่าย ภ.ง.ด.3 / 53) หรือจ่ายเงินอื่น
//   - โอนเงินระหว่างบัญชี, ประเภทภาษีหัก ณ ที่จ่าย, รายงานภาษีหัก ณ ที่จ่าย / ภาษีถูกหัก
//
// การลงบัญชี (ตามทะเบียนเงินที่ผูกผังบัญชีไว้ — 1 ผังบัญชี = 1 บรรทัด):
//   - รับชำระหนี้: Dr เงินสด / ธนาคาร + Dr ภาษีถูกหัก ณ ที่จ่าย / Cr ลูกหนี้ (ค่าตั้งขาย)
//   - จ่ายชำระหนี้: Dr เจ้าหนี้ (ค่าตั้งจัดซื้อ) / Cr เงินสด / ธนาคาร + Cr ภาษีหัก ณ ที่จ่ายค้างจ่าย
//   - โอนเงิน: Dr ปลายทาง / Cr ต้นทาง
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น (ทั้งหมดอยู่ใน transaction เดียวกัน):
// identity (สิทธิ์), organization (บริษัท / งวด / เลขที่), ledger (ลงบัญชี), sales / purchasing (ตัดยอดใบแจ้งหนี้)
package finance

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/finance/application/usecases"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	gp "apexis/backend/internal/modules/finance/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/finance/infrastructure/services"
	"apexis/backend/internal/modules/finance/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/reports"
)

type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

// Organization / Ledger / Receivables / Payables / PettyCash คือ public API ของ *organization.Module, *ledger.Module,
// *sales.Module (ลูกหนี้), *purchasing.Module (เจ้าหนี้) และ *pettycash.Module (คำขอเงินสดย่อย)
type (
	Organization = services.OrganizationAccess
	Ledger       = services.LedgerAccess
	Receivables  = services.ReceivablesAccess
	Payables     = services.PayablesAccess
	PettyCash    = services.PettyCashAccess
)

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

// New: pettyCash = การเงินจ่ายคำขอตั้ง / เพิ่มวงเงิน / เบิกชดเชยของเงินสดย่อย และรวมภาษีซื้อของใบสำคัญจ่ายเงินสดย่อยใน ภ.พ.30
func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger, receivables Receivables, payables Payables, pettyCash PettyCash) *Module {
	org.RegisterDocumentTypes(DocumentTypes()...)
	uc := usecases.New(usecases.Deps{
		Tx:          gp.NewTransactionManager(db),
		Auth:        services.NewAuthorizer(identity),
		Org:         services.NewOrganization(org),
		Ledger:      services.NewLedger(ledger),
		Receivables: services.NewReceivables(receivables),
		Payables:    services.NewPayables(payables),
		PettyCash:   services.NewPettyCash(pettyCash),
		Vat:         services.NewVat(receivables, payables, services.VatSource{Totals: pettyCash.InputVat, Rows: pettyCash.InputVatRows}),
		Accounts:    gormrepo.NewCashAccountRepository(db),
		Settings:    gormrepo.NewSettingsRepository(db),
		WhtTypes:    gormrepo.NewWhtTypeRepository(db),
		Sequence:    gormrepo.NewSequenceRepository(),
		Receipts:    gormrepo.NewReceiptRepository(db),
		Payments:    gormrepo.NewPaymentRepository(db),
		Transfers:   gormrepo.NewTransferRepository(db),
		Reports:     gormrepo.NewReportRepository(db),
		TaxFilings:  gormrepo.NewTaxFilingRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	handlers.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของ finance กับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) { handlers.RegisterReports(reg, m.handler) }

// DocumentTypes คือชนิดเอกสารของการเงินที่ตั้งรูปแบบเลขที่ได้ (ลงทะเบียนกับ organization ตอน New)
func DocumentTypes() []integration.DocumentType {
	out := make([]integration.DocumentType, len(vo.DocTypes))
	for i, t := range vo.DocTypes {
		out[i] = integration.DocumentType{Key: string(t), Name: t.Name(), DefaultPrefix: t.Prefix()}
	}
	return out
}

func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }

// RemapParties: ลูกค้า / ผู้ขายรหัสซ้ำที่ sales / purchasing รวมตอน migrate — ใบรับเงิน (customer) / ใบจ่ายเงิน (supplier)
// ชี้ไปรหัสที่เก็บไว้ (ชื่อ / เลขผู้เสียภาษี ณ วันเอกสารใน party_* ไม่เปลี่ยน)
func RemapParties(db *gorm.DB, kind string, merges []integration.PartyMerge) error {
	table := map[string]string{"customer": "finance_receipts", "supplier": "finance_payments"}[kind]
	if table == "" || len(merges) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, m := range merges {
			if err := tx.Exec("UPDATE "+table+" SET party_id = ? WHERE party_id = ?", m.To, m.From).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ProvisionCompany ตั้งค่าเริ่มต้นของ module ให้บริษัทใหม่ตามประเภทธุรกิจ (บัญชี / สมุดรายวันจากผังบัญชีมาตรฐาน)
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

// PermissionMigration คือการแปลงสิทธิ์เดิมของบทบาท (main.go ส่งให้ identity ทำตอนเริ่มโปรแกรม — ทำซ้ำได้)
type PermissionMigration = vo.PermissionMigration

// PermissionMigrations: สิทธิ์รับ / จ่ายเงินเดิมของ sales / purchasing / cashbank -> finance
func PermissionMigrations() []PermissionMigration { return vo.PermissionMigrations }
