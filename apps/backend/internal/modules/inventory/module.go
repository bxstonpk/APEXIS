// Package inventory คือคลังสินค้า: หน่วยนับ, หมวดสินค้า, สินค้า, คลัง, เอกสารเคลื่อนไหวสต็อก
// (รับเข้า / เบิกออก / โอน / ปรับปรุง) ต้นทุนแบบ FIFO ห้ามสต็อกติดลบ และรายงานยอดคงเหลือ / stock card
//
// inventory ไม่ลงบัญชีเอง — module ที่เกี่ยวข้อง (ซื้อ / ขาย) ลงบัญชีโดยใช้ต้นทุนจากเอกสารสต็อก
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น:
//   - identity: ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์
//   - organization: สถานะบริษัท + จำนวนทศนิยม, งวดบัญชี, สาขา
package inventory

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/inventory/application/usecases"
	gp "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/inventory/infrastructure/services"
	inventoryhttp "apexis/backend/internal/modules/inventory/presentation/http"
	"apexis/backend/internal/modules/inventory/presentation/http/handlers"
	"apexis/backend/internal/platform/reports"
)

// Identity คือสิ่งที่ inventory ต้องการจาก identity module (*identity.Module implement)
type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

// Organization คือสิ่งที่ inventory ต้องการจาก organization module (*organization.Module implement)
type Organization interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
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
		Tx:         gp.NewTransactionManager(db),
		Auth:       services.NewIdentityAuthorizer(identity),
		Org:        services.NewOrganization(org),
		Uoms:       gormrepo.NewUomRepository(db),
		Categories: gormrepo.NewCategoryRepository(db),
		Warehouses: gormrepo.NewWarehouseRepository(db),
		Items:      gormrepo.NewItemRepository(db),
		Documents:  gormrepo.NewDocumentRepository(db),
		Stock:      gormrepo.NewStockRepository(db),
	})
	return &Module{UseCases: uc, identity: identity, handler: handlers.NewHandler(uc, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	inventoryhttp.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}

// RegisterReports ลงทะเบียน book ของ inventory กับ GET /reports?book=
func (m *Module) RegisterReports(reg *reports.Registry) {
	inventoryhttp.RegisterReports(reg, m.handler)
}

// Migrate สร้างตารางของ inventory
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
