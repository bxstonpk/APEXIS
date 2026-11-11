package services

import (
	"slices"

	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
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

// AccessFacts คือข้อมูลขั้นต่ำที่ต้องใช้ตัดสินสิทธิ์ของ user ในบริษัท
// repository โหลดมาได้ใน query เดียว (memberships JOIN users JOIN roles)
// โดยไม่ต้องโหลด User entity เต็มตัว
//
// สถานะของบริษัท (active / inactive) เป็นเรื่องของ organization module — identity ไม่ตรวจ
type AccessFacts struct {
	UserID     vo.UserID
	UserStatus vo.UserStatus
	CompanyID  vo.CompanyID
	Membership *entities.Membership
	Roles      []*entities.Role // role ที่ membership ถืออยู่
}

// ResolveAccess รวมกฎว่า user จะเข้าบริษัทได้เมื่อใด และรวม permission จากทุก role
// เป็น pure function — ไม่แตะ DB จึง test ได้ง่าย
func ResolveAccess(f AccessFacts) (AccessContext, error) {
	m := f.Membership
	switch {
	case m == nil || m.UserID() != f.UserID || m.CompanyID() != f.CompanyID:
		return AccessContext{}, domain.ErrMembershipNotFound
	case f.UserStatus != vo.UserStatusActive:
		return AccessContext{}, domain.ErrUserInactive
	case !m.IsActive():
		return AccessContext{}, domain.ErrMembershipInactive
	}

	perms := vo.NewPermissionSet()
	for _, r := range f.Roles {
		if r == nil || !m.HasRole(r.ID()) {
			continue
		}
		if r.CompanyID() != f.CompanyID {
			return AccessContext{}, domain.ErrRoleCompanyMismatch
		}
		perms.Merge(r.Permissions())
	}
	return AccessContext{
		userID:      f.UserID,
		companyID:   f.CompanyID,
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
