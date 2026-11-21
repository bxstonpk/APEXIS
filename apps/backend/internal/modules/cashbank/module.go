// Package cashbank คือทะเบียนเงินสด / บัญชีธนาคาร — แต่ละรายการผูกกับผังบัญชีของ ledger
// เอกสารรับ / จ่ายเงินของ module อื่น (จัดซื้อ, ขาย) เลือกทะเบียนนี้แล้วลงบัญชีที่ผูกไว้
//
// พึ่งพา identity (สิทธิ์), organization (สถานะบริษัท), ledger (ตรวจบัญชี) ผ่าน interface เท่านั้น
package cashbank

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/cashbank/application/usecases"
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
	gp "apexis/backend/internal/modules/cashbank/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/cashbank/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/cashbank/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/cashbank/infrastructure/services"
	"apexis/backend/internal/modules/cashbank/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
)

type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
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
	uc := usecases.New(usecases.Deps{
		Tx:       gp.NewTransactionManager(db),
		Auth:     services.NewAuthorizer(identity),
		Org:      services.NewOrganization(org),
		Ledger:   services.NewLedger(ledger),
		Accounts: gormrepo.NewCashAccountRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	handlers.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// CashAccounts คืนทะเบียนเงินสด / ธนาคารตาม id (ไม่ตรวจสิทธิ์; ที่ไม่พบหรือถูกลบจะไม่อยู่ใน map)
func (m *Module) CashAccounts(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.CashAccount, error) {
	in := make([]vo.CashAccountID, len(ids))
	for i, id := range ids {
		in[i] = vo.CashAccountID(id)
	}
	found, err := m.UseCases.Accounts.Lookup(ctx, vo.CompanyID(companyID), in)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]integration.CashAccount, len(found))
	for _, a := range found {
		d := a.Data()
		out[uuid.UUID(a.ID())] = integration.CashAccount{
			ID: uuid.UUID(a.ID()), Code: d.Code, Name: d.Name, Type: string(d.Type),
			GLAccountID: uuid.UUID(d.GLAccountID), Active: d.Status == vo.StatusActive,
		}
	}
	return out, nil
}

func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
