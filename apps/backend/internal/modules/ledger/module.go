// Package ledger คือบัญชีแยกประเภททั่วไป (General Ledger):
// ผังบัญชี, สมุดรายวัน, รายการบัญชี (draft -> posted -> reversed) และรายงาน
// (งบทดลอง, บัญชีแยกประเภท, งบแสดงฐานะการเงิน, งบกำไรขาดทุน)
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น:
//   - identity: ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์
//   - organization: สถานะบริษัท, งวดบัญชี, มิติวิเคราะห์ (สาขา / แผนก / ศูนย์ต้นทุน / โครงการ)
package ledger

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/usecases"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	gp "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/ledger/infrastructure/services"
	ledgerhttp "apexis/backend/internal/modules/ledger/presentation/http"
	"apexis/backend/internal/modules/ledger/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/reports"
)

// Identity คือสิ่งที่ ledger ต้องการจาก identity module (*identity.Module implement)
type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

// Organization คือสิ่งที่ ledger ต้องการจาก organization module (*organization.Module implement)
type Organization interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
	FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
	MissingDimensions(ctx context.Context, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error)
	FiscalPeriodRange(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, start, end time.Time, open bool, err error)
	CompanyBusinessType(ctx context.Context, companyID uuid.UUID) (integration.BusinessType, error)
	FiscalYears(ctx context.Context, companyID uuid.UUID) ([]integration.FiscalYearStatus, error)
}

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization) *Module {
	uc := usecases.New(usecases.Deps{
		Tx:       gp.NewTransactionManager(db),
		Auth:     services.NewIdentityAuthorizer(identity),
		Org:      services.NewOrganization(org),
		Accounts: gormrepo.NewAccountRepository(db),
		Journals: gormrepo.NewJournalRepository(db),
		Entries:  gormrepo.NewEntryRepository(db),
		Reports:  gormrepo.NewReportRepository(db),
		YearEnds: gormrepo.NewYearEndRepository(db),
		Settings: gormrepo.NewSettingsRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	ledgerhttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของ ledger กับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) {
	ledgerhttp.RegisterReports(reg, m.handler)
}

// ---------- public API สำหรับ module อื่น (จัดซื้อ / ขาย / คลัง) ----------
//
// ไม่ตรวจสิทธิ์ของ ledger (ผู้เรียกตรวจสิทธิ์ของเอกสารตัวเองแล้ว) แต่บังคับกฎบัญชีครบ
// ทำงานใน transaction ของผู้เรียก (ctx); error กฎบัญชีถูกห่อเป็น integration.ErrRejected

func reject(err error) error {
	if err != nil && domain.IsBusiness(err) {
		return integration.Reject("ledger", err)
	}
	return err
}

// PostEntry สร้างรายการบัญชีแล้วผ่านทันที
func (m *Module) PostEntry(ctx context.Context, companyID, userID, journalID uuid.UUID, date time.Time, description, reference string, lines []integration.JournalLine) (integration.PostedEntry, error) {
	data := entities.EntryData{
		JournalID: vo.JournalID(journalID), Date: date, Description: description, Reference: reference,
		Lines: make([]entities.LineData, len(lines)),
	}
	for i, l := range lines {
		data.Lines[i] = entities.LineData{AccountID: vo.AccountID(l.AccountID), Description: l.Description, Debit: l.Debit, Credit: l.Credit}
	}
	id, no, err := m.UseCases.Entries.PostSystem(ctx, dto.Actor{UserID: vo.UserID(userID), CompanyID: vo.CompanyID(companyID)}, data)
	if err != nil {
		return integration.PostedEntry{}, reject(err)
	}
	return integration.PostedEntry{EntryID: uuid.UUID(id), EntryNo: no}, nil
}

// ReverseEntry กลับรายการที่ลงไว้ ณ วันที่ date (งวดต้องเปิด)
func (m *Module) ReverseEntry(ctx context.Context, companyID, userID, entryID uuid.UUID, date time.Time, description string) (integration.PostedEntry, error) {
	id, no, err := m.UseCases.Entries.ReverseSystem(ctx, dto.Actor{UserID: vo.UserID(userID), CompanyID: vo.CompanyID(companyID)},
		vo.EntryID(entryID), date, description)
	if err != nil {
		return integration.PostedEntry{}, reject(err)
	}
	return integration.PostedEntry{EntryID: uuid.UUID(id), EntryNo: no}, nil
}

// InvalidPostingAccounts คืนบัญชีที่ใช้ลงรายการไม่ได้ (ไม่พบ / บัญชีคุม / inactive) — ว่าง = ใช้ได้ทั้งหมด
func (m *Module) InvalidPostingAccounts(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID) ([]uuid.UUID, error) {
	ids := make([]vo.AccountID, len(accountIDs))
	for i, id := range accountIDs {
		ids[i] = vo.AccountID(id)
	}
	bad, err := m.UseCases.Accounts.InvalidPostingAccounts(ctx, vo.CompanyID(companyID), ids)
	out := make([]uuid.UUID, len(bad))
	for i, id := range bad {
		out[i] = uuid.UUID(id)
	}
	return out, err
}

// AccountBalances คือยอด (เดบิต - เครดิต) สะสมถึงวันที่ ของบัญชีที่ขอ
func (m *Module) AccountBalances(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID, asOf time.Time) (map[uuid.UUID]decimal.Decimal, error) {
	ids := make([]vo.AccountID, len(accountIDs))
	for i, id := range accountIDs {
		ids[i] = vo.AccountID(id)
	}
	b, err := m.UseCases.Reports.Balances(ctx, vo.CompanyID(companyID), ids, asOf)
	out := make(map[uuid.UUID]decimal.Decimal, len(b))
	for id, v := range b {
		out[uuid.UUID(id)] = v
	}
	return out, err
}

// JournalUsable: สมุดรายวันมีอยู่และ active
func (m *Module) JournalUsable(ctx context.Context, companyID, journalID uuid.UUID) (bool, error) {
	return m.UseCases.Journals.JournalUsable(ctx, vo.CompanyID(companyID), vo.JournalID(journalID))
}

// Migrate สร้างตารางของ ledger แล้วเติมบัญชีมาตรฐานที่เพิ่มภายหลังให้บริษัทเดิม
func Migrate(db *gorm.DB) error {
	if err := models.AutoMigrate(db); err != nil {
		return err
	}
	return gp.TopUpDefaultChart(db)
}

// ProvisionCompany สร้างผังบัญชีมาตรฐานตามประเภทธุรกิจ + สมุดรายวันมาตรฐานให้บริษัทใหม่
// ลงทะเบียนเป็น hook ของ organization.Module.OnCompanyCreated — ทำงานใน transaction ของการสร้างบริษัท
func (m *Module) ProvisionCompany(ctx context.Context, c integration.NewCompany) error {
	bt, err := vo.ParseBusinessType(string(c.BusinessType))
	if err != nil {
		return err
	}
	_, err = m.UseCases.Accounts.Provision(ctx, vo.CompanyID(c.ID), bt)
	return err
}

// FiscalYearCreated: hook ของ organization หลังสร้างปีบัญชี — ผ่านรายการกลับอัตโนมัติที่รอ (งวดต้องเปิด)
// ทำงานใน transaction ของการสร้างปีบัญชี
func (m *Module) FiscalYearCreated(ctx context.Context, companyID uuid.UUID, _ int) error {
	_, err := m.UseCases.Entries.PostPendingReversals(ctx, vo.CompanyID(companyID))
	return err
}

// AccountIDsByCode คืน id ของบัญชีที่ลงรายการได้ตามรหัส (รหัสที่ไม่พบ / บัญชีคุม / inactive ไม่อยู่ในผล)
// module อื่นใช้ตั้งค่าเริ่มต้นจากผังบัญชีมาตรฐานตอนสร้างบริษัท — query เดียว, เข้าร่วม transaction ใน ctx
func (m *Module) AccountIDsByCode(ctx context.Context, companyID uuid.UUID, codes []string) (map[string]uuid.UUID, error) {
	found, err := m.UseCases.Accounts.PostableIDsByCode(ctx, vo.CompanyID(companyID), codes)
	out := make(map[string]uuid.UUID, len(found))
	for code, id := range found {
		out[code] = uuid.UUID(id)
	}
	return out, err
}

// JournalIDsByCode คืน id ของสมุดรายวันที่ active ตามรหัส (เช่น GEN / CRJ / CDJ)
func (m *Module) JournalIDsByCode(ctx context.Context, companyID uuid.UUID, codes []string) (map[string]uuid.UUID, error) {
	found, err := m.UseCases.Accounts.JournalIDsByCode(ctx, vo.CompanyID(companyID), codes)
	out := make(map[string]uuid.UUID, len(found))
	for code, id := range found {
		out[code] = uuid.UUID(id)
	}
	return out, err
}

// AccountRefs / JournalRefs คืนรหัส / ชื่อของบัญชี / สมุดรายวันตาม id (รวมที่ลบแล้ว) ให้ module อื่นแสดงในเอกสาร — query เดียว
func (m *Module) AccountRefs(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.Ref, error) {
	in := make([]vo.AccountID, len(ids))
	for i, id := range ids {
		in[i] = vo.AccountID(id)
	}
	found, err := m.UseCases.Accounts.AccountRefs(ctx, vo.CompanyID(companyID), in)
	out := make(map[uuid.UUID]integration.Ref, len(found))
	for id, r := range found {
		out[uuid.UUID(id)] = integration.Ref{ID: uuid.UUID(id), Code: r.Code, Name: r.Name}
	}
	return out, err
}

func (m *Module) JournalRefs(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.Ref, error) {
	in := make([]vo.JournalID, len(ids))
	for i, id := range ids {
		in[i] = vo.JournalID(id)
	}
	found, err := m.UseCases.Accounts.JournalRefs(ctx, vo.CompanyID(companyID), in)
	out := make(map[uuid.UUID]integration.Ref, len(found))
	for id, r := range found {
		out[uuid.UUID(id)] = integration.Ref{ID: uuid.UUID(id), Code: r.Code, Name: r.Name}
	}
	return out, err
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}
