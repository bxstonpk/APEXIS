// Package pettycash คือเงินสดย่อย (ระบบวงเงินคงที่ / imprest): กองทุน, ใบสำคัญจ่ายเงินสดย่อย, ใบขอเบิกชดเชย
//
// เงินสดย่อยบันทึกเอกสารเท่านั้น — เงินเข้ากองทุน (ตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย) การเงินเป็นคนยืนยันจ่าย:
// finance เรียก PayRequest / UnpayRequest ใน transaction เดียวกับใบสำคัญจ่ายของการเงิน และลงบัญชีด้วยบรรทัดเดบิตที่ได้
//   - ตั้ง / เพิ่มวงเงิน: Dr เงินสดย่อย / Cr เงินสด / ธนาคาร
//   - เบิกชดเชย: Dr ค่าใช้จ่ายตามใบสำคัญจ่าย (ก่อน VAT) + Dr ภาษีซื้อ (ใบที่มีใบกำกับภาษี) / Cr เงินสด / ธนาคาร
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น: identity (สิทธิ์ / ชื่อสมาชิก), organization (บริษัท / งวด / เลขที่), ledger (ตรวจบัญชี)
package pettycash

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/pettycash/application/usecases"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	gp "apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm"
	gormrepo "apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/pettycash/infrastructure/services"
	pettycashhttp "apexis/backend/internal/modules/pettycash/presentation/http"
	"apexis/backend/internal/modules/pettycash/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/reports"
)

// Identity คือสิ่งที่ pettycash ต้องการจาก identity module (*identity.Module implement)
type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
	MemberNames(ctx context.Context, companyID uuid.UUID, userIDs []uuid.UUID) (map[uuid.UUID]string, error)
}

type (
	Organization = services.OrganizationAccess
	Ledger       = services.LedgerAccess
)

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger) *Module {
	org.RegisterDocumentTypes(DocumentTypes()...)
	vouchers := gormrepo.NewVoucherRepository(db)
	idAdapter := services.NewIdentity(identity)
	uc := usecases.New(usecases.Deps{
		Tx:             gp.NewTransactionManager(db),
		Auth:           idAdapter,
		Members:        idAdapter,
		Org:            services.NewOrganization(org),
		Ledger:         services.NewLedger(ledger),
		Settings:       gormrepo.NewSettingsRepository(db),
		Funds:          gormrepo.NewFundRepository(db),
		Vouchers:       vouchers,
		Replenishments: gormrepo.NewReplenishmentRepository(db, vouchers),
		Sequence:       gormrepo.NewSequenceRepository(),
		Reports:        gormrepo.NewReportRepository(db, vouchers),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	pettycashhttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของเงินสดย่อยกับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) {
	pettycashhttp.RegisterReports(reg, m.handler)
}

// DocumentTypes คือชนิดเอกสารของเงินสดย่อยที่ตั้งรูปแบบเลขที่ได้ (ลงทะเบียนกับ organization ตอน New)
func DocumentTypes() []integration.DocumentType {
	out := make([]integration.DocumentType, len(vo.DocTypes))
	for i, t := range vo.DocTypes {
		out[i] = integration.DocumentType{Key: string(t), Name: t.Name(), DefaultPrefix: t.Prefix()}
	}
	return out
}

// Migrate สร้างตารางของ pettycash
func Migrate(db *gorm.DB) error { return gp.Migrate(db) }

// ProvisionCompany ตั้งค่าเงินสดย่อยเริ่มต้นของบริษัทใหม่ (บัญชีภาษีซื้อจากผังมาตรฐาน)
// ลงทะเบียนเป็น hook ของ organization.Module.OnCompanyCreated หลัง ledger — ทำงานใน transaction ของการสร้างบริษัท
func (m *Module) ProvisionCompany(ctx context.Context, c integration.NewCompany) error {
	return m.UseCases.Settings.Provision(ctx, vo.CompanyID(c.ID))
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}

// ---------- public API ของการเงิน ----------

// reject ห่อ error กฎธุรกิจของเงินสดย่อยเป็น integration.ErrRejected (การเงินตอบ 422 พร้อมข้อความเดิม)
func reject(err error) error {
	if err != nil && domain.IsBusiness(err) {
		return integration.Reject("petty cash", err)
	}
	return err
}

// OpenRequests: คำขอที่รอการเงินจ่าย (การเงินเลือกในหน้าจ่ายเงิน)
func (m *Module) OpenRequests(ctx context.Context, companyID uuid.UUID) ([]integration.PettyCashRequest, error) {
	rows, err := m.UseCases.Finance.Open(ctx, vo.CompanyID(companyID))
	if err != nil {
		return nil, reject(err)
	}
	out := make([]integration.PettyCashRequest, len(rows))
	for i, r := range rows {
		out[i] = integration.PettyCashRequest{ID: r.ID, DocumentNo: r.DocumentNo, FundCode: r.FundCode, FundName: r.FundName,
			Kind: string(r.Kind), Date: r.Date, Amount: r.Amount}
	}
	return out, nil
}

// PayRequest: การเงินจ่ายคำขอ requestID ด้วยใบสำคัญจ่าย doc — ทำงานใน transaction ของการเงิน (ctx)
func (m *Module) PayRequest(ctx context.Context, companyID, requestID uuid.UUID, doc integration.SettlementDoc) (integration.PettyCashPayment, error) {
	res, err := m.UseCases.Finance.Pay(ctx, vo.CompanyID(companyID), vo.ReplenishmentID(requestID),
		entities.Payment{DocumentID: doc.DocumentID, DocumentNo: doc.DocumentNo, Date: doc.Date})
	if errors.Is(err, domain.ErrReplenishmentNotFound) {
		return integration.PettyCashPayment{}, integration.Reject("petty cash", fmt.Errorf("%w: %s", err, requestID))
	}
	if err != nil {
		return integration.PettyCashPayment{}, reject(err)
	}
	out := integration.PettyCashPayment{DocumentNo: res.DocumentNo, FundCode: res.FundCode, FundName: res.FundName, Amount: res.Amount,
		Debits: make([]integration.PettyCashDebit, len(res.Debits))}
	for i, d := range res.Debits {
		out.Debits[i] = integration.PettyCashDebit{AccountID: uuid.UUID(d.AccountID), Description: d.Description, Amount: d.Amount}
	}
	return out, nil
}

// UnpayRequest: การเงินยกเลิกใบจ่าย documentID — คำขอกลับเป็นรอจ่าย
func (m *Module) UnpayRequest(ctx context.Context, companyID, documentID uuid.UUID) error {
	return reject(m.UseCases.Finance.Unpay(ctx, vo.CompanyID(companyID), documentID))
}

// InputVat: ภาษีซื้อของใบสำคัญจ่ายที่ชดเชยแล้ว (ตามวันที่ใบกำกับภาษี) — finance รวมกับภาษีซื้อของจัดซื้อใน ภ.พ.30
func (m *Module) InputVat(ctx context.Context, companyID uuid.UUID, start, end time.Time) (integration.VatTotals, error) {
	t, err := m.UseCases.Finance.VatTotals(ctx, vo.CompanyID(companyID), start, end)
	if err != nil {
		return integration.VatTotals{}, reject(err)
	}
	return integration.VatTotals{Base: t.Base, Tax: t.Tax, Documents: t.Documents}, nil
}
