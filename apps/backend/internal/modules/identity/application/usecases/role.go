package usecases

import (
	"context"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/domain/entities"
	"apexis/backend/internal/modules/identity/domain/events"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type RoleUseCases struct{ base }

func (uc *RoleUseCases) List(ctx context.Context, in dto.ListRolesInput) (listing.Result[dto.Role], error) {
	if _, err := uc.access.Authorize(ctx, in.ActorID, in.CompanyID, vo.PermRoleRead); err != nil {
		return listing.Result[dto.Role]{}, err
	}
	return uc.deps.Roles.List(ctx, in.CompanyID, in.Filter)
}

func (uc *RoleUseCases) Create(ctx context.Context, in dto.CreateRoleInput) (dto.Role, error) {
	perms, err := vo.ParsePermissionSet(in.Permissions)
	if err != nil {
		return dto.Role{}, err
	}
	var role *entities.Role
	err = uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		access, err := uc.access.Authorize(ctx, in.ActorID, in.CompanyID, vo.PermRoleManage)
		if err != nil {
			return err
		}
		// ห้ามสร้าง role ที่มีสิทธิ์เกินกว่าที่ตัวเองมี
		if err := access.RequireAll(perms); err != nil {
			return err
		}
		r, err := entities.NewRole(in.CompanyID, in.Code, in.Name, in.Description, perms)
		if err != nil {
			return err
		}
		role = r
		return uc.deps.Roles.Create(ctx, r)
	})
	if err != nil {
		return dto.Role{}, err
	}
	uc.publish(ctx, events.RoleChanged{CompanyID: in.CompanyID, RoleID: role.ID(), Action: "created", By: in.ActorID, At: uc.now()})
	return dto.FromRole(role), nil
}

func (uc *RoleUseCases) Update(ctx context.Context, in dto.UpdateRoleInput) (dto.Role, error) {
	perms, err := vo.ParsePermissionSet(in.Permissions)
	if err != nil {
		return dto.Role{}, err
	}
	var role *entities.Role
	err = uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		access, err := uc.access.Authorize(ctx, in.ActorID, in.CompanyID, vo.PermRoleManage)
		if err != nil {
			return err
		}
		r, err := uc.deps.Roles.FindByID(ctx, in.CompanyID, in.RoleID)
		if err != nil {
			return err
		}
		// ต้องครอบคลุมทั้งสิทธิ์เดิม (ห้ามแก้ role ที่ใหญ่กว่าตัวเอง) และสิทธิ์ใหม่
		if err := access.RequireAll(r.Permissions()); err != nil {
			return err
		}
		if err := access.RequireAll(perms); err != nil {
			return err
		}
		if err := r.Rename(in.Name, in.Description); err != nil {
			return err
		}
		if err := r.SetPermissions(perms); err != nil {
			return err
		}
		role = r
		return uc.deps.Roles.Update(ctx, r)
	})
	if err != nil {
		return dto.Role{}, err
	}
	uc.publish(ctx, events.RoleChanged{CompanyID: in.CompanyID, RoleID: role.ID(), Action: "updated", By: in.ActorID, At: uc.now()})
	return dto.FromRole(role), nil
}

func (uc *RoleUseCases) Delete(ctx context.Context, in dto.DeleteRoleInput) error {
	err := uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		access, err := uc.access.Authorize(ctx, in.ActorID, in.CompanyID, vo.PermRoleManage)
		if err != nil {
			return err
		}
		r, err := uc.deps.Roles.FindByID(ctx, in.CompanyID, in.RoleID)
		if err != nil {
			return err
		}
		if err := r.EnsureDeletable(); err != nil {
			return err
		}
		if err := access.RequireAll(r.Permissions()); err != nil {
			return err
		}
		return uc.deps.Roles.Delete(ctx, in.CompanyID, in.RoleID)
	})
	if err != nil {
		return err
	}
	uc.publish(ctx, events.RoleChanged{CompanyID: in.CompanyID, RoleID: in.RoleID, Action: "deleted", By: in.ActorID, At: uc.now()})
	return nil
}
