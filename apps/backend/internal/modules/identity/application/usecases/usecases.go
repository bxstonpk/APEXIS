// Package usecases คือ business operation ของ identity (ทั้งฝั่งเขียนและฝั่งอ่าน)
//
// กติกา:
//   - CREATE / UPDATE / DELETE ทุกตัวทำใน tx.WithinTransaction (repository ห้ามเปิด transaction เอง)
//   - GET ไม่เปิด transaction
//   - use case ที่เกี่ยวกับบริษัทตรวจสิทธิ์ผ่าน AccessResolver (ฝั่งเขียนตรวจภายใน transaction เดียวกับการแก้ข้อมูล)
//   - publish domain event หลัง commit แล้วเท่านั้น
package usecases

import (
	"context"
	"slices"
	"time"

	"apexis/backend/internal/modules/identity/application/ports"
	"apexis/backend/internal/modules/identity/application/ports/repositories"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	"apexis/backend/internal/modules/identity/domain/events"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

type Deps struct {
	Users       repositories.UserRepository
	Roles       repositories.RoleRepository
	Memberships repositories.MembershipRepository
	Access      repositories.AccessRepository
	Hasher      ports.PasswordHasher
	Tokens      ports.TokenService
	Tx          ports.TransactionManager
	Events      ports.EventPublisher // optional
	Clock       ports.Clock          // optional, default time.Now().UTC()
}

// UseCases รวม use case ทั้งหมดของ module แยกตาม resource
type UseCases struct {
	Access  *AccessResolver
	Auth    *AuthUseCases
	Tenants *TenantUseCases
	Roles   *RoleUseCases
	Members *MemberUseCases
}

func New(d Deps) *UseCases {
	if d.Clock == nil {
		d.Clock = func() time.Time { return time.Now().UTC() }
	}
	access := &AccessResolver{access: d.Access}
	b := base{deps: d, access: access}
	return &UseCases{
		Access:  access,
		Auth:    &AuthUseCases{b},
		Tenants: &TenantUseCases{b},
		Roles:   &RoleUseCases{b},
		Members: &MemberUseCases{b},
	}
}

// base คือ dependency และ helper ที่ use case ทุกกลุ่มใช้ร่วมกัน
type base struct {
	deps   Deps
	access *AccessResolver
}

func (b base) now() time.Time { return b.deps.Clock() }

// publish ส่ง event หลัง transaction commit แล้วเท่านั้น
func (b base) publish(ctx context.Context, evts ...events.Event) {
	if b.deps.Events != nil {
		b.deps.Events.Publish(ctx, evts...)
	}
}

// loadRoles โหลด role ตาม id ภายในบริษัท และคืน ErrRoleNotFound ถ้ามี id ใดไม่พบ
func (b base) loadRoles(ctx context.Context, companyID vo.CompanyID, ids []vo.RoleID) ([]*entities.Role, error) {
	unique := slices.Compact(slices.SortedFunc(slices.Values(ids), func(a, b vo.RoleID) int {
		return slices.Compare(a[:], b[:])
	}))
	if len(unique) == 0 {
		return nil, nil
	}
	roles, err := b.deps.Roles.FindByIDs(ctx, companyID, unique)
	if err != nil {
		return nil, err
	}
	if len(roles) != len(unique) {
		return nil, domain.ErrRoleNotFound
	}
	return roles, nil
}
