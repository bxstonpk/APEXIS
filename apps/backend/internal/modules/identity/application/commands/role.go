package commands

import (
	"context"

	"account/backend/internal/modules/identity/application/dto"
	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/events"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type CreateRole struct {
	ActorID     vo.UserID
	CompanyID   vo.CompanyID
	Code        string
	Name        string
	Description string
	Permissions []string
}

func (h *Handler) CreateRole(ctx context.Context, cmd CreateRole) (dto.Role, error) {
	perms, err := vo.ParsePermissionSet(cmd.Permissions)
	if err != nil {
		return dto.Role{}, err
	}
	var role *entities.Role
	err = h.tx.WithinTx(ctx, func(ctx context.Context) error {
		access, err := h.access.Authorize(ctx, cmd.ActorID, cmd.CompanyID, vo.PermRoleManage)
		if err != nil {
			return err
		}
		// ห้ามสร้าง role ที่มีสิทธิ์เกินกว่าที่ตัวเองมี
		if err := access.RequireAll(perms); err != nil {
			return err
		}
		r, err := entities.NewRole(cmd.CompanyID, cmd.Code, cmd.Name, cmd.Description, perms)
		if err != nil {
			return err
		}
		role = r
		return h.roles.Create(ctx, r)
	})
	if err != nil {
		return dto.Role{}, err
	}
	h.publish(ctx, events.RoleChanged{CompanyID: cmd.CompanyID, RoleID: role.ID(), Action: "created", By: cmd.ActorID, At: h.now()})
	return dto.FromRole(role), nil
}

type UpdateRole struct {
	ActorID     vo.UserID
	CompanyID   vo.CompanyID
	RoleID      vo.RoleID
	Name        string
	Description string
	Permissions []string
}

func (h *Handler) UpdateRole(ctx context.Context, cmd UpdateRole) (dto.Role, error) {
	perms, err := vo.ParsePermissionSet(cmd.Permissions)
	if err != nil {
		return dto.Role{}, err
	}
	var role *entities.Role
	err = h.tx.WithinTx(ctx, func(ctx context.Context) error {
		access, err := h.access.Authorize(ctx, cmd.ActorID, cmd.CompanyID, vo.PermRoleManage)
		if err != nil {
			return err
		}
		r, err := h.roles.FindByID(ctx, cmd.CompanyID, cmd.RoleID)
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
		if err := r.Rename(cmd.Name, cmd.Description); err != nil {
			return err
		}
		if err := r.SetPermissions(perms); err != nil {
			return err
		}
		role = r
		return h.roles.Update(ctx, r)
	})
	if err != nil {
		return dto.Role{}, err
	}
	h.publish(ctx, events.RoleChanged{CompanyID: cmd.CompanyID, RoleID: role.ID(), Action: "updated", By: cmd.ActorID, At: h.now()})
	return dto.FromRole(role), nil
}

type DeleteRole struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	RoleID    vo.RoleID
}

func (h *Handler) DeleteRole(ctx context.Context, cmd DeleteRole) error {
	err := h.tx.WithinTx(ctx, func(ctx context.Context) error {
		access, err := h.access.Authorize(ctx, cmd.ActorID, cmd.CompanyID, vo.PermRoleManage)
		if err != nil {
			return err
		}
		r, err := h.roles.FindByID(ctx, cmd.CompanyID, cmd.RoleID)
		if err != nil {
			return err
		}
		if err := r.EnsureDeletable(); err != nil {
			return err
		}
		if err := access.RequireAll(r.Permissions()); err != nil {
			return err
		}
		return h.roles.Delete(ctx, cmd.CompanyID, cmd.RoleID)
	})
	if err != nil {
		return err
	}
	h.publish(ctx, events.RoleChanged{CompanyID: cmd.CompanyID, RoleID: cmd.RoleID, Action: "deleted", By: cmd.ActorID, At: h.now()})
	return nil
}
