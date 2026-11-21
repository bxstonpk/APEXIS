package usecases

import (
	"context"
	"errors"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	"apexis/backend/internal/modules/identity/domain/events"
	"apexis/backend/internal/modules/identity/domain/services"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type MemberUseCases struct{ base }

func (uc *MemberUseCases) List(ctx context.Context, in dto.ListMembersInput) (listing.Result[dto.Member], error) {
	if _, err := uc.access.Authorize(ctx, in.ActorID, in.CompanyID, vo.PermMemberRead); err != nil {
		return listing.Result[dto.Member]{}, err
	}
	return uc.deps.Memberships.List(ctx, in.CompanyID, in.Filter)
}

// Get สมาชิกหนึ่งคน (ต้องมีสิทธิ์ identity.member:read)
func (uc *MemberUseCases) Get(ctx context.Context, actor vo.UserID, company vo.CompanyID, user vo.UserID) (dto.Member, error) {
	if _, err := uc.access.Authorize(ctx, actor, company, vo.PermMemberRead); err != nil {
		return dto.Member{}, err
	}
	return uc.deps.Memberships.Get(ctx, company, user)
}

func (uc *MemberUseCases) Add(ctx context.Context, in dto.AddMemberInput) (dto.Membership, error) {
	email, err := vo.NewEmail(in.Email)
	if err != nil {
		return dto.Membership{}, err
	}
	// บัญชีใหม่: ตรวจ / hash รหัสผ่านก่อนเปิด transaction (bcrypt ใช้ CPU นาน — ให้ transaction สั้นที่สุด)
	var newUser *entities.User
	if in.NewUser != nil {
		if err := vo.ValidatePassword(in.NewUser.Password); err != nil {
			return dto.Membership{}, err
		}
		hash, err := uc.deps.Hasher.Hash(in.NewUser.Password)
		if err != nil {
			return dto.Membership{}, err
		}
		if newUser, err = entities.NewUserWithTemporaryPassword(email, in.NewUser.Name, hash); err != nil {
			return dto.Membership{}, err
		}
	}

	var membership *entities.Membership
	err = uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		access, err := uc.access.Authorize(ctx, in.ActorID, in.CompanyID, vo.PermMemberManage)
		if err != nil {
			return err
		}
		user, err := uc.deps.Users.FindByEmail(ctx, email)
		switch {
		case err == nil && newUser != nil:
			// มีบัญชีอยู่แล้ว — ไม่ตั้งรหัสผ่านทับของเจ้าของบัญชี
			return domain.ErrEmailAlreadyExists
		case errors.Is(err, domain.ErrUserNotFound) && newUser != nil:
			// สร้างบัญชีใน transaction เดียวกับสมาชิก (unique email ใน DB กันการสร้างซ้ำพร้อมกัน)
			if err := uc.deps.Users.Create(ctx, newUser); err != nil {
				return err
			}
			user = newUser
		case err != nil:
			return err
		}
		roles, err := uc.loadRoles(ctx, in.CompanyID, in.RoleIDs)
		if err != nil {
			return err
		}
		if err := access.RequireAll(services.UnionPermissions(roles)); err != nil {
			return err
		}
		m, err := entities.NewMembership(user.ID(), in.CompanyID, roles)
		if err != nil {
			return err
		}
		membership = m
		return uc.deps.Memberships.Create(ctx, m)
	})
	if err != nil {
		return dto.Membership{}, err
	}
	if newUser != nil {
		uc.publish(ctx, events.UserRegistered{UserID: newUser.ID(), Email: newUser.Email(), At: uc.now()})
	}
	uc.publish(ctx, events.MemberAdded{
		CompanyID: in.CompanyID, UserID: membership.UserID(), RoleIDs: membership.RoleIDs(),
		By: in.ActorID, At: uc.now(),
	})
	return dto.FromMembership(membership), nil
}

func (uc *MemberUseCases) AssignRoles(ctx context.Context, in dto.AssignMemberRolesInput) (dto.Membership, error) {
	var membership *entities.Membership
	err := uc.withManagedMember(ctx, in.ActorID, in.CompanyID, in.UserID, func(ctx context.Context, access services.AccessContext, m *entities.Membership) error {
		roles, err := uc.loadRoles(ctx, in.CompanyID, in.RoleIDs)
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
		return uc.deps.Memberships.Update(ctx, m)
	})
	if err != nil {
		return dto.Membership{}, err
	}
	uc.publish(ctx, events.MemberRolesChanged{
		CompanyID: in.CompanyID, UserID: in.UserID, RoleIDs: membership.RoleIDs(),
		By: in.ActorID, At: uc.now(),
	})
	return dto.FromMembership(membership), nil
}

// ChangeStatus ใช้กับ PATCH /members/{userID} {"status": "active" | "suspended"}
func (uc *MemberUseCases) ChangeStatus(ctx context.Context, in dto.ChangeMemberStatusInput) (dto.Membership, error) {
	var membership *entities.Membership
	err := uc.withManagedMember(ctx, in.ActorID, in.CompanyID, in.UserID, func(ctx context.Context, _ services.AccessContext, m *entities.Membership) error {
		if err := m.ChangeStatus(in.Status); err != nil {
			return err
		}
		membership = m
		return uc.deps.Memberships.Update(ctx, m)
	})
	if err != nil {
		return dto.Membership{}, err
	}
	uc.publish(ctx, events.MemberStatusChanged{
		CompanyID: in.CompanyID, UserID: in.UserID, Status: string(membership.Status()),
		By: in.ActorID, At: uc.now(),
	})
	return dto.FromMembership(membership), nil
}

// withManagedMember รวมกฎของการจัดการ member คนอื่น (ทำใน transaction เดียว):
//   - actor ต้องมี identity.member:manage
//   - ห้ามแก้ membership ของตัวเอง (กันล็อกตัวเองออก)
//   - actor ต้องมีสิทธิ์ครอบคลุมสิทธิ์ปัจจุบันของเป้าหมาย (เช่น admin แก้ owner ไม่ได้)
func (uc *MemberUseCases) withManagedMember(
	ctx context.Context,
	actor vo.UserID,
	companyID vo.CompanyID,
	target vo.UserID,
	fn func(ctx context.Context, access services.AccessContext, m *entities.Membership) error,
) error {
	if actor == target {
		return domain.ErrCannotModifySelf
	}
	return uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		access, err := uc.access.Authorize(ctx, actor, companyID, vo.PermMemberManage)
		if err != nil {
			return err
		}
		m, err := uc.deps.Memberships.Find(ctx, companyID, target)
		if err != nil {
			return err
		}
		current, err := uc.deps.Roles.FindByIDs(ctx, companyID, m.RoleIDs())
		if err != nil {
			return err
		}
		if err := access.RequireAll(services.UnionPermissions(current)); err != nil {
			return err
		}
		return fn(ctx, access, m)
	})
}
