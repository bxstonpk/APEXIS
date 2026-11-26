// Package usecases คือ business operation ของ purchasing
//
// กติกา:
//   - เขียนทุกอย่างใน tx.WithinTransaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - รับสินค้า / แจ้งหนี้: เอกสารจัดซื้อ + ใบรับเข้าของ inventory + รายการบัญชีของ ledger อยู่ใน transaction เดียว
//     (ล้มที่จุดใด = rollback ทั้งหมด) — ไม่มีการเรียกบริการภายนอกระหว่าง transaction
//   - เอกสารที่ตามใบสั่งซื้อใช้เงื่อนไข VAT / เครดิตเทอมที่ใบสั่งซื้อเก็บไว้ ไม่กลับไปดูผู้ขาย
package usecases

import (
	"context"

	"fmt"
	"github.com/shopspring/decimal"
	"time"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
	"apexis/backend/internal/platform/numeric"
)

type Deps struct {
	Tx          ports.TransactionManager
	Auth        ports.Authorizer
	Org         ports.Organization
	Ledger      ports.Ledger
	Inventory   ports.Inventory
	Settings    repositories.SettingsRepository
	Suppliers   repositories.SupplierRepository
	Sequence    repositories.Sequence
	Orders      repositories.OrderRepository
	Receipts    repositories.ReceiptRepository
	Invoices    repositories.InvoiceRepository
	Reports     repositories.ReportRepository
	Settlements repositories.SettlementRepository
	CreditNotes repositories.CreditNoteRepository
	Clock       func() time.Time // optional
}

type UseCases struct {
	Settings    *SettingsUseCases
	Suppliers   *SupplierUseCases
	Orders      *OrderUseCases
	Receipts    *ReceiptUseCases
	Invoices    *InvoiceUseCases
	Reports     *ReportUseCases
	Settlements *SettlementUseCases
	CreditNotes *CreditNoteUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, ledger: d.Ledger, inventory: d.Inventory, settings: d.Settings, seq: d.Sequence, now: d.Clock}
	return &UseCases{
		Settings:    &SettingsUseCases{base: b},
		Suppliers:   &SupplierUseCases{base: b, repo: d.Suppliers},
		Orders:      &OrderUseCases{base: b, orders: d.Orders, suppliers: d.Suppliers},
		Receipts:    &ReceiptUseCases{base: b, receipts: d.Receipts, orders: d.Orders},
		Invoices:    &InvoiceUseCases{base: b, invoices: d.Invoices, orders: d.Orders, suppliers: d.Suppliers, credits: d.CreditNotes},
		Reports:     &ReportUseCases{base: b, reports: d.Reports},
		Settlements: &SettlementUseCases{base: b, invoices: d.Invoices, suppliers: d.Suppliers, settlements: d.Settlements},
		CreditNotes: &CreditNoteUseCases{base: b, notes: d.CreditNotes, invoices: d.Invoices, settlements: d.Settlements},
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

func (b base) authorize(ctx context.Context, a dto.Actor, p vo.Permission) error {
	return b.auth.Authorize(ctx, a.UserID, a.CompanyID, p)
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

func (b base) write(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context) error) error {
	return b.writeWith(ctx, a, p, func(ctx context.Context, _ ports.CompanySettings) error { return fn(ctx) })
}

func read[T any](ctx context.Context, b base, a dto.Actor, p vo.Permission, fn func() (T, error)) (T, error) {
	if err := b.authorize(ctx, a, p); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}

func (b base) openPeriod(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	p, err := b.org.PeriodAt(ctx, companyID, date)
	if err != nil {
		return ports.Period{}, err
	}
	if !p.Open {
		return ports.Period{}, domain.ErrPeriodClosed
	}
	return p, nil
}

// number ออกเลขที่ถัดไปตามรูปแบบที่บริษัทตั้ง (ค่าเริ่มต้น เช่น PO2026-000001)
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

// items โหลดสินค้าจาก inventory — ทุกตัวต้องมีอยู่ (และ active เมื่อ requireActive)
func items(found map[vo.ItemID]ports.Item, ids []vo.ItemID, requireActive bool) error {
	for _, id := range ids {
		i, ok := found[id]
		switch {
		case !ok:
			return fmt.Errorf("%w: %s", domain.ErrItemNotFound, id)
		case requireActive && !i.Active:
			return fmt.Errorf("%w: %s", domain.ErrItemInactive, i.Code)
		}
	}
	return nil
}

// itemRefs เติมรหัส / ชื่อสินค้าจาก inventory — 1 query ต่อ request ไม่ว่ามีกี่บรรทัด
// (ไม่ JOIN ตารางของ module อื่น; สินค้าที่หาไม่พบคงไว้แค่ id)
func (b base) itemRefs(ctx context.Context, companyID vo.CompanyID, refs []*dto.Ref) error {
	ids := make([]vo.ItemID, 0, len(refs))
	seen := map[vo.ItemID]bool{}
	for _, r := range refs {
		id, err := vo.ParseID[vo.ItemID](r.ID)
		if err == nil && !seen[id] {
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
				r.Code, r.Name = i.Code, i.Name
			}
		}
	}
	return nil
}

var (
	zero       = decimal.Zero
	decimalOne = decimal.NewFromInt(1)
)

// numericAmount: ยอดเงิน > 0, อยู่ในช่วง NUMERIC(24,6) และทศนิยมไม่เกินที่บริษัทกำหนด
func numericAmount(v vo.Money, amountDecimals int) bool {
	return v.IsPositive() && numeric.InRange(v) && numeric.FitsScale(v, amountDecimals)
}
