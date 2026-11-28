// Package usecases คือ business operation ของ sales
//
// กติกา:
//   - เขียนทุกอย่างใน transaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - ขายสินค้า / ขายบริการ / มัดจำ: เอกสารขาย + ใบเบิกของ inventory + รายการบัญชีของ ledger อยู่ใน transaction เดียว (เงินเข้ารับที่ finance)
//   - เอกสารที่ตามใบเสนอราคา / ใบสั่งขายใช้เงื่อนไข VAT / เครดิตเทอมที่เก็บในใบ ไม่กลับไปดูค่าตั้ง / ลูกค้า
//   - ข้อมูลของ module อื่น (ชื่อสินค้า) เติมผ่าน public API 1 ครั้งต่อ request — ไม่ JOIN ข้าม module
package usecases

import (
	"context"
	"fmt"
	"time"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
)

type Deps struct {
	Tx           ports.TransactionManager
	Auth         ports.Authorizer
	Org          ports.Organization
	Ledger       ports.Ledger
	Inventory    ports.Inventory
	Settings     repositories.SettingsRepository
	Customers    repositories.CustomerRepository
	Sequence     repositories.Sequence
	Quotations   repositories.QuotationRepository
	Orders       repositories.OrderRepository
	Invoices     repositories.InvoiceRepository
	Recognitions repositories.VatRecognitionRepository
	BillingNotes repositories.BillingNoteRepository
	CreditNotes  repositories.CreditNoteRepository
	Settlements  repositories.SettlementRepository
	Reports      repositories.ReportRepository
	Clock        func() time.Time
}

type UseCases struct {
	Settings     *SettingsUseCases
	Customers    *CustomerUseCases
	Quotations   *QuotationUseCases
	Orders       *OrderUseCases
	Invoices     *InvoiceUseCases
	BillingNotes *BillingNoteUseCases
	CreditNotes  *CreditNoteUseCases
	Settlements  *SettlementUseCases
	Reports      *ReportUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, ledger: d.Ledger, inventory: d.Inventory, settings: d.Settings, seq: d.Sequence, now: d.Clock}
	return &UseCases{
		Settings:   &SettingsUseCases{base: b},
		Customers:  &CustomerUseCases{base: b, repo: d.Customers},
		Quotations: &QuotationUseCases{base: b, quotations: d.Quotations, customers: d.Customers},
		Orders:     &OrderUseCases{base: b, orders: d.Orders, quotations: d.Quotations, customers: d.Customers},
		Invoices: &InvoiceUseCases{base: b, invoices: d.Invoices, orders: d.Orders, customers: d.Customers, creditNotes: d.CreditNotes,
			settlements: d.Settlements, recognitions: d.Recognitions},
		BillingNotes: &BillingNoteUseCases{base: b, notes: d.BillingNotes, invoices: d.Invoices, customers: d.Customers},
		CreditNotes:  &CreditNoteUseCases{base: b, notes: d.CreditNotes, invoices: d.Invoices, settlements: d.Settlements, recognitions: d.Recognitions},
		Settlements:  &SettlementUseCases{base: b, invoices: d.Invoices, customers: d.Customers, settlements: d.Settlements, recognitions: d.Recognitions},
		Reports:      &ReportUseCases{base: b, reports: d.Reports},
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

// writeWith: transaction -> สิทธิ์ -> บริษัทต้อง active -> fn(ค่าตั้งของบริษัท)
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
	if !vo.IsDateOnly(date) {
		return ports.Period{}, domain.ErrInvalidDate
	}
	p, err := b.org.PeriodAt(ctx, companyID, date)
	if err != nil {
		return ports.Period{}, err
	}
	if !p.Open {
		return ports.Period{}, domain.ErrPeriodClosed
	}
	return p, nil
}

// number ออกเลขที่ตามรูปแบบที่บริษัทตั้ง
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

// checkItems: สินค้าทุกตัวต้องมีอยู่ (และ active เมื่อ requireActive)
func checkItems(found map[vo.ItemID]ports.Item, ids []vo.ItemID, requireActive bool) error {
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

// itemRefs เติมรหัส / ชื่อสินค้าจาก inventory — 1 query ต่อ request
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
				r.Code, r.Name = i.Code, i.Name
				switch {
				case i.Service:
					r.Type = "service"
				case i.Stock:
					r.Type = "stock"
				default:
					r.Type = "non_stock"
				}
			}
		}
	}
	return nil
}

func lineRefs(lines []dto.SalesLine) []*dto.Ref {
	out := make([]*dto.Ref, len(lines))
	for i := range lines {
		out[i] = &lines[i].Item
	}
	return out
}

func optString[T interface{ String() string }](v *T) *string {
	if v == nil {
		return nil
	}
	s := (*v).String()
	return &s
}
