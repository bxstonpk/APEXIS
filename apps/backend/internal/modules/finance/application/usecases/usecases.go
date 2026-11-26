// Package usecases คือ business operation ของ finance (การเงิน)
//
// กติกา:
//   - เขียนทุกอย่างใน tx.WithinTransaction พร้อมตรวจสิทธิ์ + สถานะบริษัท (inactive = อ่านอย่างเดียว)
//   - รับ / จ่ายเงิน: เอกสารการเงิน + ตัดยอดใบแจ้งหนี้ (sales / purchasing) + รายการบัญชี (ledger) อยู่ใน transaction เดียว
//     (ล้มที่จุดใด = rollback ทั้งหมด) — ไม่มีการเรียกบริการภายนอกระหว่าง transaction
//   - GET ไม่เปิด transaction
package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/application/ports"
	"apexis/backend/internal/modules/finance/application/ports/repositories"
	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numbering"
)

type Deps struct {
	Tx          ports.TransactionManager
	Auth        ports.Authorizer
	Org         ports.Organization
	Ledger      ports.Ledger
	Receivables ports.Settlements // sales
	Payables    ports.Settlements // purchasing
	Vat         ports.Vat         // ภาษีขาย (sales) / ภาษีซื้อ (purchasing) — สรุป ภ.พ.30
	Accounts    repositories.CashAccountRepository
	Settings    repositories.SettingsRepository
	WhtTypes    repositories.WhtTypeRepository
	Sequence    repositories.Sequence
	Receipts    repositories.ReceiptRepository
	Payments    repositories.PaymentRepository
	Transfers   repositories.TransferRepository
	Reports     repositories.ReportRepository
	Clock       func() time.Time // optional
}

type UseCases struct {
	Accounts  *AccountUseCases
	Settings  *SettingsUseCases
	WhtTypes  *WhtTypeUseCases
	Receipts  *ReceiptUseCases
	Payments  *PaymentUseCases
	Transfers *TransferUseCases
	Reports   *ReportUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	b := base{tx: d.Tx, auth: d.Auth, org: d.Org, ledger: d.Ledger, settings: d.Settings, seq: d.Sequence, accounts: d.Accounts, now: d.Clock}
	return &UseCases{
		Accounts:  &AccountUseCases{base: b},
		Settings:  &SettingsUseCases{base: b},
		WhtTypes:  &WhtTypeUseCases{base: b, repo: d.WhtTypes},
		Receipts:  &ReceiptUseCases{base: b, receipts: d.Receipts, receivables: d.Receivables},
		Payments:  &PaymentUseCases{base: b, payments: d.Payments, payables: d.Payables, whtTypes: d.WhtTypes},
		Transfers: &TransferUseCases{base: b, transfers: d.Transfers},
		Reports:   &ReportUseCases{base: b, reports: d.Reports, vat: d.Vat},
	}
}

type base struct {
	tx       ports.TransactionManager
	auth     ports.Authorizer
	org      ports.Organization
	ledger   ports.Ledger
	settings repositories.SettingsRepository
	seq      repositories.Sequence
	accounts repositories.CashAccountRepository
	now      func() time.Time
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

// number ออกเลขที่ถัดไปตามรูปแบบที่บริษัทตั้ง (ค่าเริ่มต้น เช่น RV2026-000001)
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

// posting: งวดต้องเปิด -> เลขที่เอกสาร
func (b base) posting(ctx context.Context, a dto.Actor, t vo.DocType, date time.Time) (entities.Posting, error) {
	period, err := b.openPeriod(ctx, a.CompanyID, date)
	if err != nil {
		return entities.Posting{}, err
	}
	no, err := b.number(ctx, a.CompanyID, t, date, period.FiscalYear)
	if err != nil {
		return entities.Posting{}, err
	}
	return entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: a.UserID, At: b.now()}, nil
}

// methods: ทะเบียนเงินสด / ธนาคารต้องมีอยู่และใช้งานอยู่ — ผูกบัญชีแยกประเภท ณ วันเอกสาร (query เดียว)
func (b base) methods(ctx context.Context, companyID vo.CompanyID, in []dto.MethodInput) ([]entities.Method, error) {
	ids := make([]vo.CashAccountID, len(in))
	for i, m := range in {
		ids[i] = m.CashAccountID
	}
	found, err := b.accounts.FindByIDs(ctx, companyID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[vo.CashAccountID]*entities.CashAccount, len(found))
	for _, a := range found {
		byID[a.ID()] = a
	}
	out := make([]entities.Method, len(in))
	for i, m := range in {
		a, ok := byID[m.CashAccountID]
		if !ok {
			return nil, wrapf(domain.ErrCashAccountNotFound, "method %d", i+1)
		}
		d := a.Data()
		if d.Status != vo.StatusActive {
			return nil, wrapf(domain.ErrInactiveRef, "cash / bank account %s", d.Code)
		}
		out[i] = entities.Method{CashAccountID: m.CashAccountID, GLAccountID: d.GLAccountID, Amount: m.Amount, Reference: m.Reference}
	}
	return out, nil
}

// checkAccounts: บัญชีแยกประเภทที่ระบุเองต้องลงรายการได้ (query เดียว)
func (b base) checkAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID) error {
	if len(ids) == 0 {
		return nil
	}
	bad, err := b.ledger.InvalidAccounts(ctx, companyID, ids)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return wrapf(domain.ErrAccountInvalid, "%s", bad[0])
	}
	return nil
}

// post ลงบัญชี — ไม่มีบรรทัด (ยอดหักล้างกันหมด) = ไม่ลงบัญชี
func (b base) post(ctx context.Context, a dto.Actor, journal vo.JournalID, date time.Time, description, reference string, e ports.LedgerEntry) (*vo.EntryID, string, error) {
	if len(e.Lines) == 0 {
		return nil, "", nil
	}
	e.CompanyID, e.UserID, e.JournalID, e.Date, e.Description, e.Reference = a.CompanyID, a.UserID, journal, date, description, reference
	id, no, err := b.ledger.Post(ctx, e)
	if err != nil {
		return nil, "", err
	}
	return &id, no, nil
}

// unpost กลับรายการบัญชีของเอกสาร ณ วันที่เดิม (งวดต้องเปิด)
func (b base) unpost(ctx context.Context, a dto.Actor, p entities.Posting, date time.Time, what string) error {
	if _, err := b.openPeriod(ctx, a.CompanyID, date); err != nil {
		return err
	}
	if p.EntryID == nil {
		return nil
	}
	return b.ledger.Reverse(ctx, a.CompanyID, a.UserID, *p.EntryID, date, "Cancel "+what+" "+p.DocumentNo)
}

// ---------- ทะเบียนเงินสด / ธนาคาร ----------

type AccountUseCases struct{ base }

func (uc *AccountUseCases) Get(ctx context.Context, in dto.ByID) (dto.CashAccount, error) {
	return read(ctx, uc.base, in.Actor, vo.PermAccountRead, func() (dto.CashAccount, error) { return uc.accounts.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *AccountUseCases) List(ctx context.Context, a dto.Actor, f dto.Filter) (listing.Result[dto.CashAccount], error) {
	return read(ctx, uc.base, a, vo.PermAccountRead, func() (listing.Result[dto.CashAccount], error) { return uc.accounts.List(ctx, a.CompanyID, f) })
}

func (uc *AccountUseCases) Create(ctx context.Context, a dto.Actor, d entities.CashAccountData) (dto.CashAccount, error) {
	acc, err := entities.NewCashAccount(a.CompanyID, d)
	if err != nil {
		return dto.CashAccount{}, err
	}
	err = uc.write(ctx, a, vo.PermAccountManage, func(ctx context.Context) error {
		if err := uc.checkGL(ctx, a.CompanyID, d.GLAccountID); err != nil {
			return err
		}
		return uc.accounts.Create(ctx, acc)
	})
	if err != nil {
		return dto.CashAccount{}, err
	}
	return uc.accounts.Get(ctx, a.CompanyID, acc.ID())
}

func (uc *AccountUseCases) Update(ctx context.Context, in dto.ByID, d entities.CashAccountData) (dto.CashAccount, error) {
	err := uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		acc, err := uc.accounts.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := acc.Update(d); err != nil {
			return err
		}
		if err := uc.checkGL(ctx, in.CompanyID, d.GLAccountID); err != nil {
			return err
		}
		return uc.accounts.Update(ctx, acc)
	})
	if err != nil {
		return dto.CashAccount{}, err
	}
	return uc.accounts.Get(ctx, in.CompanyID, in.ID)
}

// Delete: soft delete — เอกสารเดิมยังอ้าง id ได้ (แนะนำปิดใช้งานแทนเมื่อเคยใช้แล้ว)
func (uc *AccountUseCases) Delete(ctx context.Context, in dto.ByID) error {
	return uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		return uc.accounts.Delete(ctx, in.CompanyID, in.ID)
	})
}

func (uc *AccountUseCases) checkGL(ctx context.Context, companyID vo.CompanyID, id vo.GLAccountID) error {
	bad, err := uc.ledger.InvalidAccounts(ctx, companyID, []vo.GLAccountID{id})
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return domain.ErrAccountInvalid
	}
	return nil
}
