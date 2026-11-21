// Package services คือ adapter ไปยัง module อื่น (identity, organization, ledger) — เรียกผ่าน public API (uuid / primitive)
package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/application/ports"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

// IdentityAccess คือ public API ของ identity ที่ inventory ใช้
type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

var _ ports.Authorizer = (*IdentityAuthorizer)(nil)

type IdentityAuthorizer struct{ identity IdentityAccess }

func NewIdentityAuthorizer(identity IdentityAccess) *IdentityAuthorizer {
	return &IdentityAuthorizer{identity: identity}
}

func (a *IdentityAuthorizer) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error {
	ok, err := a.identity.Authorize(ctx, uuid.UUID(userID), uuid.UUID(companyID), perm.String())
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// OrganizationAccess คือ public API ของ organization ที่ inventory ใช้
type OrganizationAccess interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
	FiscalPeriodRange(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, start, end time.Time, open bool, err error)
	MissingDimensions(ctx context.Context, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error)
}

var _ ports.Organization = (*Organization)(nil)

type Organization struct{ org OrganizationAccess }

func NewOrganization(org OrganizationAccess) *Organization { return &Organization{org: org} }

func (o *Organization) CompanySettings(ctx context.Context, companyID vo.CompanyID) (ports.CompanySettings, error) {
	active, amount, quantity, err := o.org.CompanySettings(ctx, uuid.UUID(companyID))
	return ports.CompanySettings{Active: active, AmountDecimals: amount, QuantityDecimals: quantity}, err
}

func (o *Organization) PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	id, year, start, end, open, err := o.org.FiscalPeriodRange(ctx, uuid.UUID(companyID), date)
	if err != nil {
		return ports.Period{}, err
	}
	if id == uuid.Nil {
		return ports.Period{}, domain.ErrNoFiscalPeriod
	}
	return ports.Period{ID: vo.FiscalPeriodID(id), FiscalYear: year, Start: start, End: end, Open: open}, nil
}

func (o *Organization) BranchExists(ctx context.Context, companyID vo.CompanyID, id vo.BranchID) (bool, error) {
	missing, err := o.org.MissingDimensions(ctx, uuid.UUID(companyID), []uuid.UUID{uuid.UUID(id)}, nil, nil, nil)
	return len(missing) == 0, err
}

// LedgerAccess คือ public API ของ ledger ที่ inventory ใช้ (*ledger.Module implement)
type LedgerAccess interface {
	PostEntry(ctx context.Context, companyID, userID, journalID uuid.UUID, date time.Time, description, reference string, lines []integration.JournalLine) (integration.PostedEntry, error)
	ReverseEntry(ctx context.Context, companyID, userID, entryID uuid.UUID, date time.Time, description string) (integration.PostedEntry, error)
	InvalidPostingAccounts(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID) ([]uuid.UUID, error)
	JournalUsable(ctx context.Context, companyID, journalID uuid.UUID) (bool, error)
	AccountBalances(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID, asOf time.Time) (map[uuid.UUID]decimal.Decimal, error)
}

var _ ports.Ledger = (*Ledger)(nil)

type Ledger struct{ ledger LedgerAccess }

func NewLedger(l LedgerAccess) *Ledger { return &Ledger{ledger: l} }

func toUUIDs[T ~[16]byte](ids []T) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out
}

func (l *Ledger) InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error) {
	bad, err := l.ledger.InvalidPostingAccounts(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make([]vo.AccountID, len(bad))
	for i, id := range bad {
		out[i] = vo.AccountID(id)
	}
	return out, err
}

func (l *Ledger) JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error) {
	return l.ledger.JournalUsable(ctx, uuid.UUID(companyID), uuid.UUID(id))
}

func (l *Ledger) Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID, asOf time.Time) (map[vo.AccountID]vo.Money, error) {
	b, err := l.ledger.AccountBalances(ctx, uuid.UUID(companyID), toUUIDs(ids), asOf)
	out := make(map[vo.AccountID]vo.Money, len(b))
	for id, v := range b {
		out[vo.AccountID(id)] = v
	}
	return out, err
}

func (l *Ledger) Post(ctx context.Context, e ports.LedgerEntry) (vo.EntryID, string, error) {
	lines := make([]integration.JournalLine, len(e.Lines))
	for i, x := range e.Lines {
		lines[i] = integration.JournalLine{AccountID: uuid.UUID(x.AccountID), Debit: x.Debit, Credit: x.Credit}
	}
	p, err := l.ledger.PostEntry(ctx, uuid.UUID(e.CompanyID), uuid.UUID(e.UserID), uuid.UUID(e.JournalID), e.Date, e.Description, e.Reference, lines)
	return vo.EntryID(p.EntryID), p.EntryNo, err
}

func (l *Ledger) Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error {
	_, err := l.ledger.ReverseEntry(ctx, uuid.UUID(companyID), uuid.UUID(userID), uuid.UUID(id), date, description)
	return err
}
