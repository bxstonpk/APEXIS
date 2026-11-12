// Package identity ประกอบ (wire) ทุก layer ของ module เข้าด้วยกัน
// main.go เรียกแค่ identity.New(...) แล้ว RegisterRoutes
package identity

import (
	"context"
	"errors"
	"log/slog"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/application/usecases"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	gp "apexis/backend/internal/modules/identity/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/identity/infrastructure/persistence/gorm/repositories"
	infraservices "apexis/backend/internal/modules/identity/infrastructure/services"
	identityhttp "apexis/backend/internal/modules/identity/presentation/http"
	"apexis/backend/internal/modules/identity/presentation/http/handlers"
)

type Config struct {
	JWTSecret  []byte
	TokenTTL   time.Duration
	JWTIssuer  string
	BcryptCost int // 0 = default
	Logger     *slog.Logger
}

type Module struct {
	UseCases   *usecases.UseCases
	Middleware *handlers.Middleware // module อื่นใช้ Authenticate / RequireCompany / RequirePermission
	handler    *handlers.Handler
}

func New(db *gorm.DB, cfg Config) (*Module, error) {
	tokens, err := infraservices.NewJWTTokenService(cfg.JWTSecret, cfg.TokenTTL, cfg.JWTIssuer)
	if err != nil {
		return nil, err
	}

	uc := usecases.New(usecases.Deps{
		Users:       gormrepo.NewUserRepository(db),
		Roles:       gormrepo.NewRoleRepository(db),
		Memberships: gormrepo.NewMembershipRepository(db),
		Access:      gormrepo.NewAccessRepository(db),
		Hasher:      infraservices.NewBcryptHasher(cfg.BcryptCost),
		Tokens:      tokens,
		Tx:          gp.NewTransactionManager(db),
		Events:      infraservices.NewLogEventPublisher(cfg.Logger),
	})

	return &Module{
		UseCases:   uc,
		Middleware: handlers.NewMiddleware(tokens, uc.Access),
		handler:    handlers.NewHandler(uc),
	}, nil
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	identityhttp.RegisterRoutes(mux, m.handler, m.Middleware)
}

// ---------- public API สำหรับ module อื่น ----------
//
// module อื่นพึ่งพา identity ผ่าน method ด้านล่างเท่านั้น (ใช้ uuid / string ไม่ผูกกับ type ภายในของ identity)

// Authenticate คือ middleware ตรวจ Bearer token
func (m *Module) Authenticate(next nethttp.Handler) nethttp.Handler {
	return m.Middleware.Authenticate(next)
}

// CurrentUserID คืน user id ของ request ที่ผ่าน Authenticate แล้ว
func (m *Module) CurrentUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := handlers.UserIDFrom(ctx)
	return uuid.UUID(id), ok
}

// Authorize ตรวจว่า user มี permission ในบริษัทหรือไม่
// คืน (false, nil) เมื่อไม่มีสิทธิ์ทุกกรณี (ไม่ใช่สมาชิก, ถูกระงับ, ผู้ใช้ inactive)
// สถานะของบริษัทไม่ถูกตรวจที่นี่ — เป็นเรื่องของ organization module
// คืน error เฉพาะปัญหาของระบบ — เข้าร่วม transaction ใน ctx ได้
func (m *Module) Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error) {
	_, err := m.UseCases.Access.Authorize(ctx, vo.UserID(userID), vo.CompanyID(companyID), vo.Permission(permission))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrUserInactive),
		errors.Is(err, domain.ErrMembershipInactive),
		errors.Is(err, domain.ErrRoleCompanyMismatch):
		return false, nil
	default:
		return false, err
	}
}

// ProvisionCompanyOwner สร้าง role "owner" ของบริษัทใหม่และให้ ownerID เป็นสมาชิก
// organization เรียกภายใน transaction ที่สร้างบริษัท (เข้าร่วม transaction ใน ctx — rollback ไปด้วยกัน)
// คืน (false, nil) เมื่อผู้ใช้ไม่พบหรือไม่ active
func (m *Module) ProvisionCompanyOwner(ctx context.Context, companyID, ownerID uuid.UUID) (bool, error) {
	err := m.UseCases.Tenants.ProvisionOwner(ctx, dto.ProvisionOwnerInput{
		CompanyID: vo.CompanyID(companyID), OwnerID: vo.UserID(ownerID),
	})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, domain.ErrUserNotFound), errors.Is(err, domain.ErrUserInactive):
		return false, nil
	default:
		return false, err
	}
}

// MemberCompanyIDs คืน id ของบริษัทที่ user เป็นสมาชิก (active)
func (m *Module) MemberCompanyIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := m.UseCases.Tenants.CompanyIDsOf(ctx, vo.UserID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out, nil
}

// SeedUser สร้างผู้ใช้เริ่มต้น (เช่นผู้ดูแลระบบจาก INITIAL_ADMIN_*) ถ้ายังไม่มีอีเมลนี้
// คืน true เมื่อสร้างใหม่, false เมื่อมีอยู่แล้ว (ไม่แก้ไขผู้ใช้เดิม / ไม่รีเซ็ตรหัสผ่าน)
func (m *Module) SeedUser(ctx context.Context, email, name, password string) (bool, error) {
	_, created, err := m.UseCases.Auth.EnsureUser(ctx, dto.RegisterUserInput{Email: email, Name: name, Password: password})
	return created, err
}

// Migrate สร้างตารางของ identity
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
