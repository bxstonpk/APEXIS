// Package services คือ adapter ไปยัง module อื่น (identity, organization)
// แปลง type / error ของ module อื่นเป็นของ ledger — module อื่นถูกเรียกผ่าน public API (uuid / primitive) เท่านั้น
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/ledger/application/ports"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// IdentityAccess คือ public API ของ identity ที่ ledger ใช้ (identity.Module implement)
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

// OrganizationAccess คือ public API ของ organization ที่ ledger ใช้ (organization.Module implement)
type OrganizationAccess interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
	FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
	MissingDimensions(ctx context.Context, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error)
	FiscalPeriodRange(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, start, end time.Time, open bool, err error)
}

var _ ports.Organization = (*Organization)(nil)

type Organization struct{ org OrganizationAccess }

func NewOrganization(org OrganizationAccess) *Organization { return &Organization{org: org} }

func (o *Organization) CompanySettings(ctx context.Context, companyID vo.CompanyID) (ports.CompanySettings, error) {
	active, amountDecimals, _, err := o.org.CompanySettings(ctx, uuid.UUID(companyID))
	return ports.CompanySettings{Active: active, AmountDecimals: amountDecimals}, err
}

func (o *Organization) PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	id, year, yearStart, open, err := o.org.FiscalPeriodAt(ctx, uuid.UUID(companyID), date)
	if err != nil {
		return ports.Period{}, err
	}
	if id == uuid.Nil {
		return ports.Period{}, domain.ErrNoFiscalPeriod
	}
	return ports.Period{ID: vo.FiscalPeriodID(id), FiscalYear: year, YearStart: yearStart, Open: open}, nil
}

func (o *Organization) PeriodRange(ctx context.Context, companyID vo.CompanyID, date time.Time) (*ports.PeriodRange, error) {
	id, year, start, end, open, err := o.org.FiscalPeriodRange(ctx, uuid.UUID(companyID), date)
	if err != nil || id == uuid.Nil {
		return nil, err
	}
	return &ports.PeriodRange{FiscalYear: year, Start: start, End: end, Open: open}, nil
}

func (o *Organization) MissingDimensions(ctx context.Context, companyID vo.CompanyID, d ports.Dimensions) ([]string, error) {
	return o.org.MissingDimensions(ctx, uuid.UUID(companyID),
		uuids(d.Branches), uuids(d.Departments), uuids(d.CostCenters), uuids(d.ProjectSites))
}

func uuids[T ~[16]byte](in []T) []uuid.UUID {
	out := make([]uuid.UUID, len(in))
	for i, id := range in {
		out[i] = uuid.UUID(id)
	}
	return out
}
