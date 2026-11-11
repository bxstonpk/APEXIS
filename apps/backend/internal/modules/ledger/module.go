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
	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/application/usecases"
	gp "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/ledger/infrastructure/services"
	ledgerhttp "apexis/backend/internal/modules/ledger/presentation/http"
	"apexis/backend/internal/modules/ledger/presentation/http/handlers"
)

// Identity คือสิ่งที่ ledger ต้องการจาก identity module (*identity.Module implement)
type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

// Organization คือสิ่งที่ ledger ต้องการจาก organization module (*organization.Module implement)
type Organization interface {
	CompanyActive(ctx context.Context, companyID uuid.UUID) (bool, error)
	FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
	MissingDimensions(ctx context.Context, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error)
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
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	ledgerhttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// Migrate สร้างตารางของ ledger
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
