// Package inventory คือคลังสินค้า: หน่วยนับ, หมวดสินค้า, สินค้า, คลัง, เอกสารเคลื่อนไหวสต็อก
// (รับเข้า / เบิกออก / โอน / ปรับปรุง) ต้นทุนแบบ FIFO ห้ามสต็อกติดลบ และรายงานยอดคงเหลือ / stock card
//
// เอกสารสต็อกไม่ลงบัญชีเอง — module ที่เกี่ยวข้อง (ซื้อ / ขาย) ลงบัญชีโดยใช้ต้นทุนจากเอกสารสต็อก
// inventory เก็บค่าตั้งบัญชีสินค้า (ระบบ perpetual / periodic ต่อปีบัญชี, บัญชีตามสินค้า / หมวด / ค่าเริ่มต้น)
// และลงบัญชีเองเฉพาะรายการปรับปรุงสินค้าคงเหลือสิ้นงวด (periodic) ผ่าน ledger
//
// พึ่งพา module อื่นผ่าน interface เท่านั้น:
//   - identity: ตรวจ token, user ปัจจุบัน, ตรวจสิทธิ์
//   - organization: สถานะบริษัท + จำนวนทศนิยม, งวดบัญชี, สาขา
//   - ledger: ตรวจบัญชี / สมุดรายวัน, ยอดบัญชี, ลงรายการปรับปรุงสิ้นงวด
package inventory

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/usecases"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	gp "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/models"
	gormrepo "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/repositories"
	"apexis/backend/internal/modules/inventory/infrastructure/services"
	inventoryhttp "apexis/backend/internal/modules/inventory/presentation/http"
	"apexis/backend/internal/modules/inventory/presentation/http/handlers"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
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
	RegisterDocumentTypes(types ...integration.DocumentType)
	DocumentFormat(ctx context.Context, companyID uuid.UUID, docType string) (numbering.Format, error)
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
	FiscalPeriodRange(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, start, end time.Time, open bool, err error)
	MissingDimensions(ctx context.Context, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error)
}

// Ledger คือสิ่งที่ inventory ต้องการจาก ledger module (*ledger.Module implement)
type Ledger = services.LedgerAccess

type Module struct {
	UseCases *usecases.UseCases
	identity Identity
	handler  *handlers.Handler
}

func New(db *gorm.DB, identity Identity, org Organization, ledger Ledger) *Module {
	org.RegisterDocumentTypes(DocumentTypes()...)
	uc := usecases.New(usecases.Deps{
		Tx:         gp.NewTransactionManager(db),
		Auth:       services.NewIdentityAuthorizer(identity),
		Org:        services.NewOrganization(org),
		Ledger:     services.NewLedger(ledger),
		Uoms:       gormrepo.NewUomRepository(db),
		Categories: gormrepo.NewCategoryRepository(db),
		Warehouses: gormrepo.NewWarehouseRepository(db),
		Items:      gormrepo.NewItemRepository(db),
		Documents:  gormrepo.NewDocumentRepository(db),
		Stock:      gormrepo.NewStockRepository(db),
		Settings:   gormrepo.NewSettingsRepository(db),
		Closings:   gormrepo.NewClosingRepository(db),
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

// ---------- public API สำหรับ module อื่น (จัดซื้อ / ขาย) ----------
//
// ไม่ตรวจสิทธิ์ของ inventory (ผู้เรียกตรวจสิทธิ์ของเอกสารตัวเองแล้ว) แต่บังคับกฎของ inventory ครบ
// ทำงานใน transaction ของผู้เรียก (ctx); error กฎธุรกิจถูกห่อเป็น integration.ErrRejected

func reject(err error) error {
	if err != nil && domain.IsBusiness(err) {
		return integration.Reject("inventory", err)
	}
	return err
}

// PurchaseContext คืนระบบบัญชีของปีบัญชีที่ date อยู่ และสินค้าพร้อมบัญชีที่ใช้จริง (สินค้าที่ไม่พบจะไม่อยู่ใน map)
// lock ค่าตั้งไว้จน transaction ของผู้เรียก commit — ระบบบัญชีเปลี่ยนระหว่างนั้นไม่ได้
func (m *Module) PurchaseContext(ctx context.Context, companyID uuid.UUID, date time.Time, itemIDs []uuid.UUID) (integration.PurchaseContext, error) {
	ids := make([]vo.ItemID, len(itemIDs))
	for i, id := range itemIDs {
		ids[i] = vo.ItemID(id)
	}
	system, items, err := m.UseCases.Facade.PurchaseContext(ctx, vo.CompanyID(companyID), date, ids)
	if err != nil {
		return integration.PurchaseContext{}, reject(err)
	}
	return integration.PurchaseContext{System: integration.AccountingSystem(system), Items: itemMap(items)}, nil
}

// PurchaseItems คืนสินค้าพร้อมบัญชีที่ใช้จริง (ไม่ผูกกับปีบัญชี) — สินค้าที่ไม่พบจะไม่อยู่ใน map
func (m *Module) PurchaseItems(ctx context.Context, companyID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]integration.Item, error) {
	ids := make([]vo.ItemID, len(itemIDs))
	for i, id := range itemIDs {
		ids[i] = vo.ItemID(id)
	}
	items, err := m.UseCases.Facade.PurchaseItems(ctx, vo.CompanyID(companyID), ids)
	if err != nil {
		return nil, reject(err)
	}
	return itemMap(items), nil
}

func itemMap(items []dto.PurchaseItem) map[uuid.UUID]integration.Item {
	out := make(map[uuid.UUID]integration.Item, len(items))
	for _, i := range items {
		out[uuid.UUID(i.ID)] = integration.Item{
			ID: uuid.UUID(i.ID), Code: i.Code, Name: i.Name, Stock: i.Stock, Active: i.Active,
			PurchaseAccountID: accountUUID(i.Accounts.Purchase), InventoryAccountID: accountUUID(i.Accounts.Inventory),
			RevenueAccountID: accountUUID(i.Accounts.Revenue),
		}
	}
	return out
}

func accountUUID(id *vo.AccountID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return uuid.UUID(*id)
}

// PostReceipt ออกใบรับเข้า (receipt) แล้วผ่านทันที — ต้นทุน FIFO ตามราคาก่อน VAT ที่ส่งมา
func (m *Module) PostReceipt(ctx context.Context, companyID, userID, warehouseID uuid.UUID, date time.Time, reference, description string, lines []integration.ReceiptLine) (integration.Receipt, error) {
	data := entities.DocumentData{
		Type: vo.DocReceipt, Date: date, WarehouseID: vo.WarehouseID(warehouseID), Reference: reference, Description: description,
		Lines: make([]entities.LineData, len(lines)),
	}
	for i, l := range lines {
		cost := l.UnitCost
		data.Lines[i] = entities.LineData{ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity, UnitCost: &cost}
	}
	r, err := m.UseCases.Facade.PostReceipt(ctx, vo.CompanyID(companyID), vo.UserID(userID), data)
	if err != nil {
		return integration.Receipt{}, reject(err)
	}
	return integration.Receipt{DocumentID: uuid.UUID(r.DocumentID), DocumentNo: r.DocumentNo, LineValues: r.LineValues}, nil
}

// PostIssue ออกใบเบิก (issue) แล้วผ่านทันที — ตัดสต็อก FIFO ห้ามติดลบ; LineValues = ต้นทุน FIFO ต่อบรรทัด
func (m *Module) PostIssue(ctx context.Context, companyID, userID, warehouseID uuid.UUID, date time.Time, reference, description string, lines []integration.IssueLine) (integration.Receipt, error) {
	data := entities.DocumentData{
		Date: date, WarehouseID: vo.WarehouseID(warehouseID), Reference: reference, Description: description,
		Lines: make([]entities.LineData, len(lines)),
	}
	for i, l := range lines {
		data.Lines[i] = entities.LineData{ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity}
	}
	r, err := m.UseCases.Facade.PostIssue(ctx, vo.CompanyID(companyID), vo.UserID(userID), data)
	if err != nil {
		return integration.Receipt{}, reject(err)
	}
	return integration.Receipt{DocumentID: uuid.UUID(r.DocumentID), DocumentNo: r.DocumentNo, LineValues: r.LineValues}, nil
}

// CancelDocument ยกเลิกเอกสารสต็อกที่ผ่านแล้ว (คืนต้นทุน FIFO) — ของที่รับเข้าถูกใช้ไปแล้วยกเลิกไม่ได้
func (m *Module) CancelDocument(ctx context.Context, companyID, userID, documentID uuid.UUID) error {
	return reject(m.UseCases.Facade.CancelDocument(ctx, vo.CompanyID(companyID), vo.UserID(userID), vo.DocumentID(documentID)))
}

// DocumentTypes คือชนิดเอกสารของ inventory ที่ตั้งรูปแบบเลขที่ได้ (ลงทะเบียนกับ organization ตอน New)
func DocumentTypes() []integration.DocumentType {
	out := make([]integration.DocumentType, len(vo.DocumentTypes))
	for i, t := range vo.DocumentTypes {
		out[i] = integration.DocumentType{Key: t.NumberingKey(), Name: t.Name(), DefaultPrefix: t.Prefix()}
	}
	return out
}

// Migrate สร้างตารางของ inventory
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }

// SalesItems / SalesContext คือ PurchaseItems / PurchaseContext สำหรับ module ขาย
// (ข้อมูลเดียวกัน: สินค้า + บัญชีที่ใช้จริง และระบบบัญชีของปีบัญชี)
func (m *Module) SalesItems(ctx context.Context, companyID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]integration.Item, error) {
	return m.PurchaseItems(ctx, companyID, itemIDs)
}

func (m *Module) SalesContext(ctx context.Context, companyID uuid.UUID, date time.Time, itemIDs []uuid.UUID) (integration.PurchaseContext, error) {
	return m.PurchaseContext(ctx, companyID, date, itemIDs)
}

// Permissions คือรหัสสิทธิ์ทั้งหมดของ module นี้ — main.go ลงทะเบียนกับ platform/permissions (GET /api/v1/permissions)
func (m *Module) Permissions() []string {
	out := make([]string, len(vo.AllPermissions))
	for i, p := range vo.AllPermissions {
		out[i] = string(p)
	}
	return out
}
