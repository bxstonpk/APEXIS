// Package identity ประกอบ (wire) ทุก layer ของ module เข้าด้วยกัน
// main.go เรียกแค่ identity.New(...) แล้ว RegisterRoutes
package identity

import (
	"log/slog"
	nethttp "net/http"
	"time"

	"gorm.io/gorm"

	"account/backend/internal/modules/identity/application/commands"
	"account/backend/internal/modules/identity/application/queries"
	domainservices "account/backend/internal/modules/identity/domain/services"
	"account/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
	gormrepo "account/backend/internal/modules/identity/infrastructure/persistence/gorm/repositories"
	infraservices "account/backend/internal/modules/identity/infrastructure/services"
	identityhttp "account/backend/internal/modules/identity/presentation/http"
	"account/backend/internal/modules/identity/presentation/http/handlers"
	"account/backend/internal/platform/database"
)

type Config struct {
	JWTSecret  []byte
	TokenTTL   time.Duration
	JWTIssuer  string
	BcryptCost int // 0 = default
	Logger     *slog.Logger
}

type Module struct {
	Commands   *commands.Handler
	Queries    *queries.Handler
	Middleware *handlers.Middleware // module อื่นใช้ Authenticate / RequireCompany / RequirePermission
	handler    *handlers.Handler
}

func New(db *gorm.DB, cfg Config) (*Module, error) {
	tokens, err := infraservices.NewJWTTokenService(cfg.JWTSecret, cfg.TokenTTL, cfg.JWTIssuer)
	if err != nil {
		return nil, err
	}

	users := gormrepo.NewUserRepository(db)
	companies := gormrepo.NewCompanyRepository(db)
	roles := gormrepo.NewRoleRepository(db)
	memberships := gormrepo.NewMembershipRepository(db)
	access := domainservices.NewAccessResolver(users, companies, roles, memberships)

	cmd := commands.NewHandler(commands.Deps{
		Users: users, Companies: companies, Roles: roles, Memberships: memberships,
		Access: access,
		Hasher: infraservices.NewBcryptHasher(cfg.BcryptCost),
		Tokens: tokens,
		Tx:     database.NewTxManager(db),
		Events: infraservices.NewLogEventPublisher(cfg.Logger),
	})
	qry := queries.NewHandler(queries.Deps{
		Users: users, Companies: companies, Roles: roles, Memberships: memberships, Access: access,
	})

	return &Module{
		Commands:   cmd,
		Queries:    qry,
		Middleware: handlers.NewMiddleware(tokens, qry),
		handler:    handlers.NewHandler(cmd, qry),
	}, nil
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	identityhttp.RegisterRoutes(mux, m.handler, m.Middleware)
}

// Migrate สร้างตารางของ identity
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
