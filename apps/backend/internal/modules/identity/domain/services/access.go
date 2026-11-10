package services

import (
	"slices"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

// AccessContext คือสิทธิ์ที่ resolve แล้วของ user หนึ่งคนในบริษัทหนึ่ง
// ส่งต่อให้ทุก module ของ ERP: ทุก query ต้อง filter ด้วย CompanyID()
// และทุก action ต้องเรียก Require(permission) ก่อน
type AccessContext struct {
	userID      vo.UserID
	companyID   vo.CompanyID
	roleIDs     []vo.RoleID
	permissions vo.PermissionSet
}

func (a AccessContext) UserID() vo.UserID             { return a.userID }
func (a AccessContext) CompanyID() vo.CompanyID       { return a.companyID }
func (a AccessContext) RoleIDs() []vo.RoleID          { return slices.Clone(a.roleIDs) }
func (a AccessContext) Permissions() vo.PermissionSet { return a.permissions.Clone() }
func (a AccessContext) Can(p vo.Permission) bool      { return a.permissions.Allows(p) }

func (a AccessContext) Require(p vo.Permission) error {
	if !a.Can(p) {
		return domain.ErrForbidden
	}
	return nil
}

// RequireAll ใช้ตอนจะมอบสิทธิ์ให้คนอื่น: ห้ามมอบสิทธิ์ที่ตัวเองไม่มี (กัน privilege escalation)
func (a AccessContext) RequireAll(perms vo.PermissionSet) error {
	if !a.permissions.AllowsAll(perms) {
		return domain.ErrForbidden
	}
	return nil
}

// ResolveAccess รวมกฎว่า user จะเข้าบริษัทได้เมื่อใด และรวม permission จากทุก role
// เป็น pure function — ไม่แตะ DB จึง test ได้ง่าย
func ResolveAccess(user *entities.User, company *entities.Company, m *entities.Membership, roles []*entities.Role) (AccessContext, error) {
	switch {
	case user == nil:
		return AccessContext{}, domain.ErrUserNotFound
	case company == nil:
		return AccessContext{}, domain.ErrCompanyNotFound
	case m == nil || m.UserID() != user.ID() || m.CompanyID() != company.ID():
		return AccessContext{}, domain.ErrMembershipNotFound
	case !user.IsActive():
		return AccessContext{}, domain.ErrUserInactive
	case !company.IsActive():
		return AccessContext{}, domain.ErrCompanyInactive
	case !m.IsActive():
		return AccessContext{}, domain.ErrMembershipInactive
	}

	perms := vo.NewPermissionSet()
	for _, r := range roles {
		if r == nil || !m.HasRole(r.ID()) {
			continue
		}
		if r.CompanyID() != company.ID() {
			return AccessContext{}, domain.ErrRoleCompanyMismatch
		}
		perms.Merge(r.Permissions())
	}
	return AccessContext{
		userID:      user.ID(),
		companyID:   company.ID(),
		roleIDs:     m.RoleIDs(),
		permissions: perms,
	}, nil
}

// UnionPermissions รวมสิทธิ์จากหลาย role
func UnionPermissions(roles []*entities.Role) vo.PermissionSet {
	set := vo.NewPermissionSet()
	for _, r := range roles {
		set.Merge(r.Permissions())
	}
	return set
}
