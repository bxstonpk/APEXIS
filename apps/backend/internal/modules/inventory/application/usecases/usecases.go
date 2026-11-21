// Package usecases คือ business operation ของ inventory
//
// กติกา:
//   - เขียนทุกอย่างใน tx.WithinTransaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - จำนวนทศนิยมของเงิน / จำนวนสินค้ามาจากค่าตั้งของบริษัท (อ่านใน query เดียวกับสถานะบริษัท)
//   - inventory ไม่ลงบัญชีเอง — module ที่เกี่ยวข้อง (ซื้อ / ขาย) เป็นผู้ลงบัญชีโดยใช้ต้นทุนจากเอกสารสต็อก
package usecases

import (
	"context"
	"fmt"
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
	Ledger     ports.Ledger
	Uoms       repositories.UomRepository
	Categories repositories.CategoryRepository
	Warehouses repositories.WarehouseRepository
	Items      repositories.ItemRepository
	Documents  repositories.DocumentRepository
	Stock      repositories.StockRepository
	Settings   repositories.SettingsRepository
	Closings   repositories.ClosingRepository
	Clock      func() time.Time // optional
}

type UseCases struct {
	Uoms       *MasterUseCases[vo.UomID, *entities.Uom]
	Categories *MasterUseCases[vo.CategoryID, *entities.Category]
	Warehouses *WarehouseUseCases
	Items      *ItemUseCases
	Documents  *DocumentUseCases
	Reports    *ReportUseCases
	Settings   *SettingsUseCases
	Closings   *ClosingUseCases
	Facade     *FacadeUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, ledger: d.Ledger, now: d.Clock}
	docs := &DocumentUseCases{base: b, docs: d.Documents, items: d.Items, warehouses: d.Warehouses, stock: d.Stock, closings: d.Closings}
	return &UseCases{
		Uoms: &MasterUseCases[vo.UomID, *entities.Uom]{
			base: b, repo: d.Uoms, read: vo.PermUomRead, manage: vo.PermUomManage,
			newEntity: entities.NewUom, update: (*entities.Uom).Update, id: (*entities.Uom).ID,
		},
		Categories: &MasterUseCases[vo.CategoryID, *entities.Category]{
			base: b, repo: d.Categories, read: vo.PermCategoryRead, manage: vo.PermCategoryManage,
			newEntity: entities.NewCategory, update: (*entities.Category).Update, id: (*entities.Category).ID,
			validate: func(ctx context.Context, companyID vo.CompanyID, c *entities.Category) error {
				return b.checkAccounts(ctx, companyID, c.Data().Accounts.IDs())
			},
		},
		Warehouses: &WarehouseUseCases{base: b, repo: d.Warehouses},
		Items:      &ItemUseCases{base: b, repo: d.Items, uoms: d.Uoms, categories: d.Categories},
		Documents:  docs,
		Reports:    &ReportUseCases{base: b, stock: d.Stock, items: d.Items, warehouses: d.Warehouses},
		Settings:   &SettingsUseCases{base: b, settings: d.Settings},
		Closings:   &ClosingUseCases{base: b, settings: d.Settings, closings: d.Closings, items: d.Items, stock: d.Stock},
		Facade:     &FacadeUseCases{base: b, settings: d.Settings, items: d.Items, docs: docs},
	}
}

type base struct {
	tx     ports.TransactionManager
	auth   ports.Authorizer
	org    ports.Organization
	ledger ports.Ledger
	now    func() time.Time
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

// systemWrite: สำหรับ module อื่นเรียกผ่าน public API — ไม่ตรวจสิทธิ์ของ inventory (ผู้เรียกตรวจแล้ว)
// transaction (เข้าร่วมของผู้เรียก) -> บริษัทต้อง active -> fn
func (b base) systemWrite(ctx context.Context, companyID vo.CompanyID, fn func(ctx context.Context, s ports.CompanySettings) error) error {
	return b.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		s, err := b.org.CompanySettings(ctx, companyID)
		if err != nil {
			return err
		}
		if !s.Active {
			return domain.ErrCompanyInactive
		}
		return fn(ctx, s)
	})
}

// checkAccounts: บัญชีต้องมีอยู่ใน ledger, เป็นบัญชีย่อย (ลงรายการได้) และ active — query เดียว
func (b base) checkAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) error {
	if len(ids) == 0 {
		return nil
	}
	bad, err := b.ledger.InvalidAccounts(ctx, companyID, ids)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", domain.ErrAccountInvalid, bad[0])
	}
	return nil
}

func read[T any](ctx context.Context, b base, a dto.Actor, p vo.Permission, fn func() (T, error)) (T, error) {
	if err := b.authorize(ctx, a, p); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}
