// Package services คือ adapter ไปยัง module อื่น (identity, organization, ledger)
// เรียกผ่าน public API (uuid / primitive / platform/integration) เท่านั้น
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/pettycash/application/ports"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
)

func toUUIDs[T ~[16]byte](ids []T) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out
}

// ---------- identity ----------

type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
	MemberNames(ctx context.Context, companyID uuid.UUID, userIDs []uuid.UUID) (map[uuid.UUID]string, error)
}

var (
	_ ports.Authorizer = (*Identity)(nil)
	_ ports.Members    = (*Identity)(nil)
)

type Identity struct{ identity IdentityAccess }

func NewIdentity(identity IdentityAccess) *Identity { return &Identity{identity: identity} }

func (a *Identity) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error {
	ok, err := a.identity.Authorize(ctx, uuid.UUID(userID), uuid.UUID(companyID), perm.String())
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (a *Identity) Names(ctx context.Context, companyID vo.CompanyID, ids []vo.UserID) (map[vo.UserID]string, error) {
	found, err := a.identity.MemberNames(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make(map[vo.UserID]string, len(found))
	for id, n := range found {
		out[vo.UserID(id)] = n
	}
	return out, err
}

// ---------- organization ----------

type OrganizationAccess interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
	FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
	RegisterDocumentTypes(types ...integration.DocumentType)
	DocumentFormat(ctx context.Context, companyID uuid.UUID, docType string) (numbering.Format, error)
}

var _ ports.Organization = (*Organization)(nil)

type Organization struct{ org OrganizationAccess }

func NewOrganization(org OrganizationAccess) *Organization { return &Organization{org: org} }

func (o *Organization) CompanySettings(ctx context.Context, companyID vo.CompanyID) (ports.CompanySettings, error) {
	active, amount, _, err := o.org.CompanySettings(ctx, uuid.UUID(companyID))
	return ports.CompanySettings{Active: active, AmountDecimals: amount}, err
}

func (o *Organization) PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	id, year, _, open, err := o.org.FiscalPeriodAt(ctx, uuid.UUID(companyID), date)
	if err != nil {
		return ports.Period{}, err
	}
	if id == uuid.Nil {
		return ports.Period{}, domain.ErrNoFiscalPeriod
	}
	return ports.Period{FiscalYear: year, Open: open}, nil
}

func (o *Organization) DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error) {
	return o.org.DocumentFormat(ctx, uuid.UUID(companyID), string(docType))
}

// ---------- ledger ----------

type LedgerAccess interface {
	InvalidPostingAccounts(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID) ([]uuid.UUID, error)
	AccountIDsByCode(ctx context.Context, companyID uuid.UUID, codes []string) (map[string]uuid.UUID, error)
	AccountRefs(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.Ref, error)
}

var _ ports.Ledger = (*Ledger)(nil)

type Ledger struct{ ledger LedgerAccess }

func NewLedger(l LedgerAccess) *Ledger { return &Ledger{ledger: l} }

func (l *Ledger) InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error) {
	bad, err := l.ledger.InvalidPostingAccounts(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make([]vo.AccountID, len(bad))
	for i, id := range bad {
		out[i] = vo.AccountID(id)
	}
	return out, err
}

func (l *Ledger) AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error) {
	found, err := l.ledger.AccountIDsByCode(ctx, uuid.UUID(companyID), codes)
	out := make(map[string]vo.AccountID, len(found))
	for code, id := range found {
		out[code] = vo.AccountID(id)
	}
	return out, err
}

func (l *Ledger) AccountRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) (map[vo.AccountID]ports.Ref, error) {
	if len(ids) == 0 {
		return map[vo.AccountID]ports.Ref{}, nil
	}
	found, err := l.ledger.AccountRefs(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make(map[vo.AccountID]ports.Ref, len(found))
	for id, r := range found {
		out[vo.AccountID(id)] = ports.Ref{Code: r.Code, Name: r.Name}
	}
	return out, err
}
