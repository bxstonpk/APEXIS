// Package usecases คือ business operation ของ ledger
//
// กติกา:
//   - CREATE / UPDATE / DELETE / ผ่านรายการ / กลับรายการ ทำใน tx.WithinTransaction
//     และตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว) ใน transaction เดียวกัน
//   - GET และรายงานไม่เปิด transaction
//   - ข้อมูลของ module อื่น (สิทธิ์, งวดบัญชี, มิติวิเคราะห์) เข้าถึงผ่าน ports เท่านั้น
package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

type Deps struct {
	Tx       ports.TransactionManager
	Auth     ports.Authorizer
	Org      ports.Organization
	Accounts repositories.AccountRepository
	Journals repositories.JournalRepository
	Entries  repositories.EntryRepository
	Reports  repositories.ReportRepository
	Clock    func() time.Time // optional, default time.Now().UTC()
}

type UseCases struct {
	Accounts *AccountUseCases
	Journals *JournalUseCases
	Entries  *EntryUseCases
	Reports  *ReportUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, now: d.Clock}
	return &UseCases{
		Accounts: &AccountUseCases{base: b, accounts: d.Accounts, journals: d.Journals},
		Journals: &JournalUseCases{base: b, journals: d.Journals},
		Entries:  &EntryUseCases{base: b, entries: d.Entries, accounts: d.Accounts, journals: d.Journals},
		Reports:  &ReportUseCases{base: b, accounts: d.Accounts, reports: d.Reports},
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

// write เปิด transaction -> ตรวจสิทธิ์ -> บริษัทต้อง active -> fn  (error ใดๆ = rollback ทั้งหมด)
func (b base) write(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context) error) error {
	return b.writeWith(ctx, a, p, func(ctx context.Context, _ ports.CompanySettings) error { return fn(ctx) })
}

// writeWith เหมือน write แต่ส่งค่าตั้งของบริษัท (เช่นจำนวนทศนิยม) ให้ fn — อ่านมาใน query เดียวกับสถานะบริษัท
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

// read ตรวจสิทธิ์แล้วอ่าน (ไม่เปิด transaction)
func read[T any](ctx context.Context, b base, a dto.Actor, p vo.Permission, fn func() (T, error)) (T, error) {
	if err := b.authorize(ctx, a, p); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}
