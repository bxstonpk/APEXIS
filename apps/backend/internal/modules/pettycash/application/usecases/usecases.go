// Package usecases คือ business operation ของ pettycash (เงินสดย่อยระบบวงเงินคงที่ / imprest)
//
// กติกา:
//   - เขียนทุกอย่างใน tx.WithinTransaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - เงินสดย่อยบันทึกเอกสารเท่านั้น ไม่ลงบัญชีเอง: เงินเข้ากองทุน (ตั้ง / เพิ่มวงเงิน / เบิกชดเชย) การเงินเป็นคนยืนยันจ่าย
//     และลงบัญชีในใบสำคัญจ่ายของการเงิน (finance เรียก Pay / Unpay ใน transaction เดียวกับใบจ่าย)
//   - อนุมัติใบสำคัญจ่าย lock แถวกองทุนก่อนตรวจเงินคงเหลือ (อนุมัติพร้อมกันไม่ทำให้เงินติดลบ)
package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/application/ports"
	"apexis/backend/internal/modules/pettycash/application/ports/repositories"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/numbering"
)

type Deps struct {
	Tx             ports.TransactionManager
	Auth           ports.Authorizer
	Org            ports.Organization
	Ledger         ports.Ledger
	Members        ports.Members
	Settings       repositories.SettingsRepository
	Funds          repositories.FundRepository
	Vouchers       repositories.VoucherRepository
	Replenishments repositories.ReplenishmentRepository
	Sequence       repositories.Sequence
	Reports        repositories.ReportRepository
	Clock          func() time.Time // optional
}

type UseCases struct {
	Settings       *SettingsUseCases
	Funds          *FundUseCases
	Vouchers       *VoucherUseCases
	Replenishments *ReplenishmentUseCases
	Reports        *ReportUseCases
	Finance        *FinanceUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, ledger: d.Ledger, members: d.Members, settings: d.Settings, funds: d.Funds,
		vouchers: d.Vouchers, requests: d.Replenishments, seq: d.Sequence, now: d.Clock}
	return &UseCases{
		Settings:       &SettingsUseCases{base: b},
		Funds:          &FundUseCases{base: b},
		Vouchers:       &VoucherUseCases{base: b},
		Replenishments: &ReplenishmentUseCases{base: b},
		Reports:        &ReportUseCases{base: b, reports: d.Reports},
		Finance:        &FinanceUseCases{base: b, reports: d.Reports},
	}
}

type base struct {
	tx       ports.TransactionManager
	auth     ports.Authorizer
	org      ports.Organization
	ledger   ports.Ledger
	members  ports.Members
	settings repositories.SettingsRepository
	funds    repositories.FundRepository
	vouchers repositories.VoucherRepository
	requests repositories.ReplenishmentRepository
	seq      repositories.Sequence
	now      func() time.Time
}

// writeWith: transaction -> ตรวจสิทธิ์ -> บริษัทต้อง active -> fn(ทศนิยมของเงิน)
func (b base) writeWith(ctx context.Context, a dto.Actor, p vo.Permission, fn func(ctx context.Context, dec int) error) error {
	return b.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := b.auth.Authorize(ctx, a.UserID, a.CompanyID, p); err != nil {
			return err
		}
		s, err := b.org.CompanySettings(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if !s.Active {
			return domain.ErrCompanyInactive
		}
		return fn(ctx, s.AmountDecimals)
	})
}

func read[T any](ctx context.Context, b base, a dto.Actor, p vo.Permission, fn func() (T, error)) (T, error) {
	if err := b.auth.Authorize(ctx, a.UserID, a.CompanyID, p); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}

// number ออกเลขที่ถัดไปตามรูปแบบที่บริษัทตั้ง (ค่าเริ่มต้น เช่น PCV2026-000001) — วันที่ต้องอยู่ในปีบัญชี
func (b base) number(ctx context.Context, companyID vo.CompanyID, t vo.DocType, date time.Time) (string, error) {
	p, err := b.org.PeriodAt(ctx, companyID, date)
	if err != nil {
		return "", err
	}
	f, err := b.org.DocumentFormat(ctx, companyID, t)
	if err != nil {
		return "", err
	}
	nc := numbering.Context{Date: date, FiscalYear: p.FiscalYear}
	n, err := b.seq.NextNumber(ctx, companyID, t, f.PeriodKey(nc))
	if err != nil {
		return "", err
	}
	return f.Render(nc, n), nil
}

// checkAccounts: บัญชีต้องลงรายการได้และใช้งานอยู่ (query เดียว)
func (b base) checkAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) error {
	if len(ids) == 0 {
		return nil
	}
	bad, err := b.ledger.InvalidAccounts(ctx, companyID, ids)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return wrap(domain.ErrAccountInvalid, bad[0].String())
	}
	return nil
}

// accountRefs เติมรหัส / ชื่อบัญชีจาก ledger — 1 query ต่อ request
func (b base) accountRefs(ctx context.Context, companyID vo.CompanyID, refs []*dto.Ref) error {
	ids := make([]vo.AccountID, 0, len(refs))
	seen := map[vo.AccountID]bool{}
	for _, r := range refs {
		if id, err := vo.ParseID[vo.AccountID](r.ID); err == nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	found, err := b.ledger.AccountRefs(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, r := range refs {
		if id, err := vo.ParseID[vo.AccountID](r.ID); err == nil {
			if x, ok := found[id]; ok {
				r.Code, r.Name = x.Code, x.Name
			}
		}
	}
	return nil
}

// userRefs เติมชื่อผู้ใช้ (สมาชิกของบริษัท) — 1 query ต่อ request; ไม่ใช่สมาชิกแล้วคงไว้แค่ id
func (b base) userRefs(ctx context.Context, companyID vo.CompanyID, refs []*dto.Ref) error {
	ids := make([]vo.UserID, 0, len(refs))
	seen := map[vo.UserID]bool{}
	for _, r := range refs {
		if r == nil {
			continue
		}
		if id, err := vo.ParseID[vo.UserID](r.ID); err == nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	found, err := b.members.Names(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, r := range refs {
		if r == nil {
			continue
		}
		if id, err := vo.ParseID[vo.UserID](r.ID); err == nil {
			r.Name = found[id]
		}
	}
	return nil
}
