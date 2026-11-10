// Package commands คือ use case ที่เปลี่ยนแปลงข้อมูล (write side)
// ทุก command ที่เกี่ยวกับบริษัทจะตรวจสิทธิ์ผ่าน AccessResolver ภายใน transaction
package commands

import (
	"context"
	"slices"
	"time"

	"account/backend/internal/modules/identity/application/ports"
	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/events"
	"account/backend/internal/modules/identity/domain/repositories"
	"account/backend/internal/modules/identity/domain/services"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type Deps struct {
	Users       repositories.UserRepository
	Companies   repositories.CompanyRepository
	Roles       repositories.RoleRepository
	Memberships repositories.MembershipRepository
	Access      *services.AccessResolver
	Hasher      ports.PasswordHasher
	Tokens      ports.TokenService
	Tx          ports.TxManager
	Events      ports.EventPublisher // optional
	Clock       ports.Clock          // optional, default time.Now().UTC()
}

type Handler struct {
	users       repositories.UserRepository
	companies   repositories.CompanyRepository
	roles       repositories.RoleRepository
	memberships repositories.MembershipRepository
	access      *services.AccessResolver
	hasher      ports.PasswordHasher
	tokens      ports.TokenService
	tx          ports.TxManager
	events      ports.EventPublisher
	now         ports.Clock
}

func NewHandler(d Deps) *Handler {
	h := &Handler{
		users: d.Users, companies: d.Companies, roles: d.Roles, memberships: d.Memberships,
		access: d.Access, hasher: d.Hasher, tokens: d.Tokens, tx: d.Tx, events: d.Events, now: d.Clock,
	}
	if h.now == nil {
		h.now = func() time.Time { return time.Now().UTC() }
	}
	return h
}

// publish ส่ง event หลัง transaction commit แล้วเท่านั้น
func (h *Handler) publish(ctx context.Context, evts ...events.Event) {
	if h.events != nil {
		h.events.Publish(ctx, evts...)
	}
}

// loadRoles โหลด role ตาม id ภายในบริษัท และคืน ErrRoleNotFound ถ้ามี id ใดไม่พบ
func (h *Handler) loadRoles(ctx context.Context, companyID vo.CompanyID, ids []vo.RoleID) ([]*entities.Role, error) {
	unique := slices.Compact(slices.SortedFunc(slices.Values(ids), func(a, b vo.RoleID) int {
		return slices.Compare(a[:], b[:])
	}))
	if len(unique) == 0 {
		return nil, nil
	}
	roles, err := h.roles.FindByIDs(ctx, companyID, unique)
	if err != nil {
		return nil, err
	}
	if len(roles) != len(unique) {
		return nil, domain.ErrRoleNotFound
	}
	return roles, nil
}
