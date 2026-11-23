// Package services คือ adapter ของ taxregistry ไปยัง module อื่น
package services

import (
	"context"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/taxregistry/application/ports"
	domain "apexis/backend/internal/modules/taxregistry/domain/errors"
	vo "apexis/backend/internal/modules/taxregistry/domain/value_objects"
)

type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

// Authorizer: ผ่านเมื่อมีสิทธิ์ข้อใดข้อหนึ่ง (เช่น จัดการลูกค้า หรือ จัดการผู้ขาย) — รายการสิทธิ์มาจากตอนประกอบระบบ
type Authorizer struct {
	identity    IdentityAccess
	permissions []string
}

var _ ports.Authorizer = (*Authorizer)(nil)

func NewAuthorizer(identity IdentityAccess, permissions []string) *Authorizer {
	return &Authorizer{identity: identity, permissions: permissions}
}

func (a *Authorizer) AuthorizeAny(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) error {
	for _, p := range a.permissions {
		ok, err := a.identity.Authorize(ctx, uuid.UUID(userID), uuid.UUID(companyID), p)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return domain.ErrForbidden
}
