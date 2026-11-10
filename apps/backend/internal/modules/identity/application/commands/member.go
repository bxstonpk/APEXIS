package commands

import (
	"context"

	"account/backend/internal/modules/identity/application/dto"
	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/events"
	"account/backend/internal/modules/identity/domain/services"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type AddMember struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	Email     string // user ต้องลงทะเบียนไว้ก่อนแล้ว
	RoleIDs   []vo.RoleID
}

func (h *Handler) AddMember(ctx context.Context, cmd AddMember) (dto.Membership, error) {
	email, err := vo.NewEmail(cmd.Email)
	if err != nil {
		return dto.Membership{}, err
	}
	var membership *entities.Membership
	err = h.tx.WithinTx(ctx, func(ctx context.Context) error {
		access, err := h.access.Authorize(ctx, cmd.ActorID, cmd.CompanyID, vo.PermMemberManage)
		if err != nil {
			return err
		}
		user, err := h.users.FindByEmail(ctx, email)
		if err != nil {
			return err
		}
		roles, err := h.loadRoles(ctx, cmd.CompanyID, cmd.RoleIDs)
		if err != nil {
			return err
		}
		if err := access.RequireAll(services.UnionPermissions(roles)); err != nil {
			return err
		}
		m, err := entities.NewMembership(user.ID(), cmd.CompanyID, roles)
		if err != nil {
			return err
		}
		membership = m
		return h.memberships.Create(ctx, m)
	})
	if err != nil {
		return dto.Membership{}, err
	}
	h.publish(ctx, events.MemberAdded{
		CompanyID: cmd.CompanyID, UserID: membership.UserID(), RoleIDs: membership.RoleIDs(),
		By: cmd.ActorID, At: h.now(),
	})
	return dto.FromMembership(membership), nil
}

type AssignMemberRoles struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	UserID    vo.UserID
	RoleIDs   []vo.RoleID
}

func (h *Handler) AssignMemberRoles(ctx context.Context, cmd AssignMemberRoles) (dto.Membership, error) {
	var membership *entities.Membership
	err := h.withManagedMember(ctx, cmd.ActorID, cmd.CompanyID, cmd.UserID, func(ctx context.Context, access services.AccessContext, m *entities.Membership) error {
		roles, err := h.loadRoles(ctx, cmd.CompanyID, cmd.RoleIDs)
		if err != nil {
			return err
		}
		if err := access.RequireAll(services.UnionPermissions(roles)); err != nil {
			return err
		}
		if err := m.AssignRoles(roles); err != nil {
			return err
		}
		membership = m
		return h.memberships.Update(ctx, m)
	})
	if err != nil {
		return dto.Membership{}, err
	}
	h.publish(ctx, events.MemberRolesChanged{
		CompanyID: cmd.CompanyID, UserID: cmd.UserID, RoleIDs: membership.RoleIDs(),
		By: cmd.ActorID, At: h.now(),
	})
	return dto.FromMembership(membership), nil
}

type ChangeMemberStatus struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	UserID    vo.UserID
}

func (h *Handler) SuspendMember(ctx context.Context, cmd ChangeMemberStatus) error {
	return h.changeMemberStatus(ctx, cmd, (*entities.Membership).Suspend)
}

func (h *Handler) ActivateMember(ctx context.Context, cmd ChangeMemberStatus) error {
	return h.changeMemberStatus(ctx, cmd, (*entities.Membership).Activate)
}

func (h *Handler) changeMemberStatus(ctx context.Context, cmd ChangeMemberStatus, apply func(*entities.Membership)) error {
	var membership *entities.Membership
	err := h.withManagedMember(ctx, cmd.ActorID, cmd.CompanyID, cmd.UserID, func(ctx context.Context, _ services.AccessContext, m *entities.Membership) error {
		apply(m)
		membership = m
		return h.memberships.Update(ctx, m)
	})
	if err != nil {
		return err
	}
	h.publish(ctx, events.MemberStatusChanged{
		CompanyID: cmd.CompanyID, UserID: cmd.UserID, Status: string(membership.Status()),
		By: cmd.ActorID, At: h.now(),
	})
	return nil
}

// withManagedMember รวมกฎของการจัดการ member คนอื่น:
//   - actor ต้องมี identity.member:manage
//   - ห้ามแก้ membership ของตัวเอง (กันล็อกตัวเองออก)
//   - actor ต้องมีสิทธิ์ครอบคลุมสิทธิ์ปัจจุบันของเป้าหมาย (เช่น admin แก้ owner ไม่ได้)
func (h *Handler) withManagedMember(
	ctx context.Context,
	actor vo.UserID,
	companyID vo.CompanyID,
	target vo.UserID,
	fn func(ctx context.Context, access services.AccessContext, m *entities.Membership) error,
) error {
	if actor == target {
		return domain.ErrCannotModifySelf
	}
	return h.tx.WithinTx(ctx, func(ctx context.Context) error {
		access, err := h.access.Authorize(ctx, actor, companyID, vo.PermMemberManage)
		if err != nil {
			return err
		}
		m, err := h.memberships.Find(ctx, companyID, target)
		if err != nil {
			return err
		}
		current, err := h.roles.FindByIDs(ctx, companyID, m.RoleIDs())
		if err != nil {
			return err
		}
		if err := access.RequireAll(services.UnionPermissions(current)); err != nil {
			return err
		}
		return fn(ctx, access, m)
	})
}
