// Package queries คือ use case ฝั่งอ่าน (read side) — ไม่แก้ไขข้อมูล
package queries

import (
	"context"

	"account/backend/internal/modules/identity/application/dto"
	"account/backend/internal/modules/identity/domain/entities"
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
}

type Handler struct {
	users       repositories.UserRepository
	companies   repositories.CompanyRepository
	roles       repositories.RoleRepository
	memberships repositories.MembershipRepository
	access      *services.AccessResolver
}

func NewHandler(d Deps) *Handler {
	return &Handler{users: d.Users, companies: d.Companies, roles: d.Roles, memberships: d.Memberships, access: d.Access}
}

// ResolveAccess ใช้โดย HTTP middleware ของทุก module เพื่อแนบ AccessContext กับ request
func (h *Handler) ResolveAccess(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (services.AccessContext, error) {
	return h.access.Resolve(ctx, userID, companyID)
}

func (h *Handler) GetMyAccess(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (dto.Access, error) {
	a, err := h.access.Resolve(ctx, userID, companyID)
	if err != nil {
		return dto.Access{}, err
	}
	return dto.FromAccess(a), nil
}

func (h *Handler) GetMe(ctx context.Context, userID vo.UserID) (dto.User, error) {
	u, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return dto.User{}, err
	}
	return dto.FromUser(u), nil
}

// ListMyCompanies คืนเฉพาะบริษัทที่ active และ membership active (ใช้หน้าเลือกบริษัทหลังล็อกอิน)
func (h *Handler) ListMyCompanies(ctx context.Context, userID vo.UserID) ([]dto.MyCompany, error) {
	memberships, err := h.memberships.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]vo.CompanyID, 0, len(memberships))
	byCompany := make(map[vo.CompanyID]*entities.Membership, len(memberships))
	for _, m := range memberships {
		if m.IsActive() {
			ids = append(ids, m.CompanyID())
			byCompany[m.CompanyID()] = m
		}
	}
	out := []dto.MyCompany{}
	if len(ids) == 0 {
		return out, nil
	}
	companies, err := h.companies.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, c := range companies {
		if c.IsActive() {
			out = append(out, dto.MyCompany{Company: dto.FromCompany(c), RoleIDs: dto.RoleIDStrings(byCompany[c.ID()].RoleIDs())})
		}
	}
	return out, nil
}

func (h *Handler) GetCompany(ctx context.Context, actor vo.UserID, companyID vo.CompanyID) (dto.Company, error) {
	if _, err := h.access.Authorize(ctx, actor, companyID, vo.PermCompanyRead); err != nil {
		return dto.Company{}, err
	}
	c, err := h.companies.FindByID(ctx, companyID)
	if err != nil {
		return dto.Company{}, err
	}
	return dto.FromCompany(c), nil
}

func (h *Handler) ListRoles(ctx context.Context, actor vo.UserID, companyID vo.CompanyID) ([]dto.Role, error) {
	if _, err := h.access.Authorize(ctx, actor, companyID, vo.PermRoleRead); err != nil {
		return nil, err
	}
	roles, err := h.roles.ListByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	return dto.FromRoles(roles), nil
}

func (h *Handler) ListMembers(ctx context.Context, actor vo.UserID, companyID vo.CompanyID) ([]dto.Member, error) {
	if _, err := h.access.Authorize(ctx, actor, companyID, vo.PermMemberRead); err != nil {
		return nil, err
	}
	memberships, err := h.memberships.ListByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]vo.UserID, len(memberships))
	for i, m := range memberships {
		userIDs[i] = m.UserID()
	}
	users, err := h.users.FindByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[vo.UserID]*entities.User, len(users))
	for _, u := range users {
		byID[u.ID()] = u
	}
	out := make([]dto.Member, 0, len(memberships))
	for _, m := range memberships {
		if u, ok := byID[m.UserID()]; ok {
			out = append(out, dto.Member{
				User: dto.FromUser(u), Status: string(m.Status()),
				RoleIDs: dto.RoleIDStrings(m.RoleIDs()),
			})
		}
	}
	return out, nil
}
