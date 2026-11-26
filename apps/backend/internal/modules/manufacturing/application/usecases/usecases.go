// Package usecases คือ business operation ของ manufacturing (ผลิต)
//
// กติกา:
//   - ใช้ได้เฉพาะบริษัทประเภทธุรกิจผลิต (ประเภทอื่น -> domain.ErrNotManufacturing)
//   - เขียนทุกอย่างใน tx.WithinTransaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - เบิก / คืน / ปิดงาน / ยกเลิก: ใบสั่งผลิต + เอกสารสต็อกของ inventory + รายการบัญชีของ ledger อยู่ใน transaction เดียว
//     (ล้มที่จุดใด = rollback ทั้งหมด) — ไม่มีการเรียกบริการภายนอกระหว่าง transaction
//   - ลงบัญชีเฉพาะปีบัญชีที่ใช้ระบบต่อเนื่อง (perpetual); ระบบสิ้นงวด (periodic — ค่าเริ่มต้นทุกประเภทธุรกิจ)
//     มีแค่การเคลื่อนไหวสต็อกและต้นทุน FIFO / ต้นทุนจริงของสินค้าสำเร็จรูป ไม่มีรายการบัญชี
//     (ยอดสินค้าคงเหลือในบัญชีปรับตอนปิดงวดของ inventory)
package usecases

import (
	"context"
	"fmt"
	"time"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/application/ports"
	"apexis/backend/internal/modules/manufacturing/application/ports/repositories"
	"apexis/backend/internal/modules/manufacturing/domain/entities"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
)

type Deps struct {
	Tx        ports.TransactionManager
	Auth      ports.Authorizer
	Org       ports.Organization
	Ledger    ports.Ledger
	Inventory ports.Inventory
	Settings  repositories.SettingsRepository
	Boms      repositories.BomRepository
	Orders    repositories.OrderRepository
	Sequence  repositories.Sequence
	Reports   repositories.ReportRepository
	Clock     func() time.Time // optional
}

type UseCases struct {
	Settings *SettingsUseCases
	Boms     *BomUseCases
	Orders   *OrderUseCases
	Reports  *ReportUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, ledger: d.Ledger, inventory: d.Inventory, settings: d.Settings, seq: d.Sequence, now: d.Clock}
	return &UseCases{
		Settings: &SettingsUseCases{base: b},
		Boms:     &BomUseCases{base: b, boms: d.Boms},
		Orders:   &OrderUseCases{base: b, orders: d.Orders, boms: d.Boms},
		Reports:  &ReportUseCases{base: b, reports: d.Reports},
	}
}

type base struct {
	tx        ports.TransactionManager
	auth      ports.Authorizer
	org       ports.Organization
	ledger    ports.Ledger
	inventory ports.Inventory
	settings  repositories.SettingsRepository
	seq       repositories.Sequence
	now       func() time.Time
}

// manufacturing: แอปผลิตใช้ได้เฉพาะบริษัทประเภทธุรกิจผลิต
func (b base) manufacturing(ctx context.Context, companyID vo.CompanyID) error {
	bt, err := b.org.BusinessType(ctx, companyID)
	if err != nil {
		return err
	}
	if bt != integration.BusinessManufacturing {
		return domain.ErrNotManufacturing
	}
	return nil
}

func (b base) authorize(ctx context.Context, a dto.Actor, p vo.Permission) error {
	if err := b.auth.Authorize(ctx, a.UserID, a.CompanyID, p); err != nil {
		return err
	}
	return b.manufacturing(ctx, a.CompanyID)
}

// writeWith: transaction -> ตรวจสิทธิ์ + ประเภทธุรกิจ -> บริษัทต้อง active -> fn(ทศนิยมของบริษัท)
func (b base) writeWith(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context, dec entities.Decimals) error) error {
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
		return fn(ctx, entities.Decimals{Quantity: s.QuantityDecimals, Amount: s.AmountDecimals})
	})
}

func read[T any](ctx context.Context, b base, a dto.Actor, p vo.Permission, fn func() (T, error)) (T, error) {
	if err := b.authorize(ctx, a, p); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}

func (b base) period(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	return b.org.PeriodAt(ctx, companyID, date)
}

func (b base) openPeriod(ctx context.Context, companyID vo.CompanyID, date time.Time) error {
	p, err := b.org.PeriodAt(ctx, companyID, date)
	if err != nil {
		return err
	}
	if !p.Open {
		return domain.ErrPeriodClosed
	}
	return nil
}

// number ออกเลขที่ถัดไปตามรูปแบบที่บริษัทตั้ง (ค่าเริ่มต้น เช่น MO2026-000001)
func (b base) number(ctx context.Context, companyID vo.CompanyID, t vo.DocType, date time.Time, fiscalYear int) (string, error) {
	f, err := b.org.DocumentFormat(ctx, companyID, t)
	if err != nil {
		return "", err
	}
	nc := numbering.Context{Date: date, FiscalYear: fiscalYear}
	n, err := b.seq.NextNumber(ctx, companyID, t, f.PeriodKey(nc))
	if err != nil {
		return "", err
	}
	return f.Render(nc, n), nil
}

// stockItems ตรวจว่าสินค้าทุกตัวมีอยู่ เป็นชนิด stock และใช้งานอยู่ — query เดียว
func (b base) stockItems(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) (map[vo.ItemID]ports.Item, error) {
	found, err := b.inventory.Items(ctx, companyID, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		i, ok := found[id]
		switch {
		case !ok:
			return nil, fmt.Errorf("%w: %s", domain.ErrItemNotFound, id)
		case !i.Stock:
			return nil, fmt.Errorf("%w: %s", domain.ErrItemNotStock, i.Code)
		case !i.Active:
			return nil, fmt.Errorf("%w: %s", domain.ErrItemInactive, i.Code)
		}
	}
	return found, nil
}

// activeWarehouse ตรวจว่าคลังมีอยู่และใช้งานอยู่
func (b base) activeWarehouse(ctx context.Context, companyID vo.CompanyID, id vo.WarehouseID) error {
	found, err := b.inventory.Warehouses(ctx, companyID, []vo.WarehouseID{id})
	if err != nil {
		return err
	}
	w, ok := found[id]
	switch {
	case !ok:
		return domain.ErrWarehouseNotFound
	case !w.Active:
		return domain.ErrWarehouseInactive
	}
	return nil
}

// itemRefs เติมรหัส / ชื่อ / หน่วยนับของสินค้าจาก inventory — 1 query ต่อ request ไม่ว่ามีกี่บรรทัด
// (ไม่ JOIN ตารางของ module อื่น; สินค้าที่หาไม่พบคงไว้แค่ id)
func (b base) itemRefs(ctx context.Context, companyID vo.CompanyID, refs []*dto.Ref) error {
	ids := make([]vo.ItemID, 0, len(refs))
	seen := map[vo.ItemID]bool{}
	for _, r := range refs {
		if id, err := vo.ParseID[vo.ItemID](r.ID); err == nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	found, err := b.inventory.Items(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, r := range refs {
		if id, err := vo.ParseID[vo.ItemID](r.ID); err == nil {
			if i, ok := found[id]; ok {
				r.Code, r.Name, r.Uom = i.Code, i.Name, i.Uom
			}
		}
	}
	return nil
}

func ref[T ~[16]byte](id T) dto.Ref { return dto.Ref{ID: vo.IDString(id)} }

func optRef[T ~[16]byte](id *T) *dto.Ref {
	if id == nil {
		return nil
	}
	r := ref(*id)
	return &r
}
