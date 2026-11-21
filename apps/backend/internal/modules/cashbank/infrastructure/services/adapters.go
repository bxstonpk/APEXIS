// Package services คือ adapter ไปยัง identity / organization / ledger (public API ด้วย uuid / primitive)
package services

import (
	"context"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/cashbank/application/ports"
	domain "apexis/backend/internal/modules/cashbank/domain/errors"
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
)

type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

var _ ports.Authorizer = (*Authorizer)(nil)

type Authorizer struct{ identity IdentityAccess }

func NewAuthorizer(identity IdentityAccess) *Authorizer { return &Authorizer{identity: identity} }

func (a *Authorizer) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error {
	ok, err := a.identity.Authorize(ctx, uuid.UUID(userID), uuid.UUID(companyID), perm.String())
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

type OrganizationAccess interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
}

var _ ports.Organization = (*Organization)(nil)

type Organization struct{ org OrganizationAccess }

func NewOrganization(org OrganizationAccess) *Organization { return &Organization{org: org} }

func (o *Organization) CompanyActive(ctx context.Context, companyID vo.CompanyID) (bool, error) {
	active, _, _, err := o.org.CompanySettings(ctx, uuid.UUID(companyID))
	return active, err
}

type LedgerAccess interface {
	InvalidPostingAccounts(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID) ([]uuid.UUID, error)
}

var _ ports.Ledger = (*Ledger)(nil)

type Ledger struct{ ledger LedgerAccess }

func NewLedger(l LedgerAccess) *Ledger { return &Ledger{ledger: l} }

func (l *Ledger) InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID) ([]vo.GLAccountID, error) {
	in := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		in[i] = uuid.UUID(id)
	}
	bad, err := l.ledger.InvalidPostingAccounts(ctx, uuid.UUID(companyID), in)
	out := make([]vo.GLAccountID, len(bad))
	for i, id := range bad {
		out[i] = vo.GLAccountID(id)
	}
	return out, err
}
