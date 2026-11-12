// Package usecases คือ business operation ของ inventory
//
// กติกา:
//   - เขียนทุกอย่างใน tx.WithinTransaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - จำนวนทศนิยมของเงิน / จำนวนสินค้ามาจากค่าตั้งของบริษัท (อ่านใน query เดียวกับสถานะบริษัท)
//   - inventory ไม่ลงบัญชีเอง — module ที่เกี่ยวข้อง (ซื้อ / ขาย) เป็นผู้ลงบัญชีโดยใช้ต้นทุนจากเอกสารสต็อก
package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

type Deps struct {
	Tx         ports.TransactionManager
	Auth       ports.Authorizer
	Org        ports.Organization
	Uoms       repositories.UomRepository
	Categories repositories.CategoryRepository
	Warehouses repositories.WarehouseRepository
	Items      repositories.ItemRepository
	Documents  repositories.DocumentRepository
	Stock      repositories.StockRepository
	Clock      func() time.Time // optional
}

type UseCases struct {
	Uoms       *MasterUseCases[vo.UomID, *entities.Uom]
	Categories *MasterUseCases[vo.CategoryID, *entities.Category]
	Warehouses *WarehouseUseCases
	Items      *ItemUseCases
	Documents  *DocumentUseCases
	Reports    *ReportUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, now: d.Clock}
	return &UseCases{
		Uoms: &MasterUseCases[vo.UomID, *entities.Uom]{
			base: b, repo: d.Uoms, read: vo.PermUomRead, manage: vo.PermUomManage,
			newEntity: entities.NewUom, update: (*entities.Uom).Update, id: (*entities.Uom).ID,
		},
		Categories: &MasterUseCases[vo.CategoryID, *entities.Category]{
			base: b, repo: d.Categories, read: vo.PermCategoryRead, manage: vo.PermCategoryManage,
			newEntity: entities.NewCategory, update: (*entities.Category).Update, id: (*entities.Category).ID,
		},
		Warehouses: &WarehouseUseCases{base: b, repo: d.Warehouses},
		Items:      &ItemUseCases{base: b, repo: d.Items, uoms: d.Uoms, categories: d.Categories},
		Documents:  &DocumentUseCases{base: b, docs: d.Documents, items: d.Items, warehouses: d.Warehouses, stock: d.Stock},
		Reports:    &ReportUseCases{base: b, stock: d.Stock, items: d.Items, warehouses: d.Warehouses},
	}
}

type base struct {
	tx   ports.TransactionManager
	auth ports.Authorizer
	org  ports.Organization
	now  func() time.Time
}

func (b base) authorize(ctx context.Context, a dto.Actor, p vo.Permission) error {
	return b.auth.Authorize(ctx, a.UserID, a.CompanyID, p)
}

func (b base) write(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context) error) error {
	return b.writeWith(ctx, a, p, func(ctx context.Context, _ ports.CompanySettings) error { return fn(ctx) })
}

// writeWith: transaction -> ตรวจสิทธิ์ -> บริษัทต้อง active -> fn(ค่าตั้งของบริษัท)
func (b base) writeWith(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context, s ports.CompanySettings) error) error {
	return b.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := b.authorize(ctx, a, p); err != nil {
			return err
		}
		s, err := b.org.CompanySettings(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if !s.Active {
			return domain.ErrCompanyInactive
		}
		return fn(ctx, s)
	})
}

func read[T any](ctx context.Context, b base, a dto.Actor, p vo.Permission, fn func() (T, error)) (T, error) {
	if err := b.authorize(ctx, a, p); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}
