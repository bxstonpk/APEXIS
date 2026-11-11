// Package services คือ adapter ไปยังระบบภายนอก module (ตอนนี้คือ identity module)
package services

import (
	"context"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/organization/application/ports"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// IdentityAccess คือ public API ของ identity ที่ organization ใช้ (identity.Module implement)
// ใช้ uuid / string เพื่อไม่ผูกกับ type ภายในของ identity
type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
	ProvisionCompanyOwner(ctx context.Context, companyID, ownerID uuid.UUID) (bool, error)
	MemberCompanyIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

var (
	_ ports.Authorizer = (*Identity)(nil)
	_ ports.Membership = (*Identity)(nil)
)

// Identity แปลงผลจาก identity module เป็น type / error ของ organization
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

func (a *Identity) ProvisionOwner(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) error {
	ok, err := a.identity.ProvisionCompanyOwner(ctx, uuid.UUID(companyID), uuid.UUID(userID))
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (a *Identity) CompanyIDsOf(ctx context.Context, userID vo.UserID) ([]vo.CompanyID, error) {
	ids, err := a.identity.MemberCompanyIDs(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]vo.CompanyID, len(ids))
	for i, id := range ids {
		out[i] = vo.CompanyID(id)
	}
	return out, nil
}
