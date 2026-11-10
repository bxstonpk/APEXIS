package services

import (
	"context"
	"errors"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/repositories"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

// AccessResolver โหลดข้อมูลที่จำเป็นจาก repository แล้วเรียก ResolveAccess
type AccessResolver struct {
	users       repositories.UserRepository
	companies   repositories.CompanyRepository
	roles       repositories.RoleRepository
	memberships repositories.MembershipRepository
}

func NewAccessResolver(
	users repositories.UserRepository,
	companies repositories.CompanyRepository,
	roles repositories.RoleRepository,
	memberships repositories.MembershipRepository,
) *AccessResolver {
	return &AccessResolver{users: users, companies: companies, roles: roles, memberships: memberships}
}

// Resolve คืนสิทธิ์ของ user ในบริษัท
// ถ้าไม่ได้เป็นสมาชิก หรือไม่มีบริษัทนี้ จะคืน ErrForbidden เหมือนกัน
// เพื่อไม่ให้คนนอกเดาได้ว่ามีบริษัทนี้อยู่หรือไม่
func (r *AccessResolver) Resolve(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (AccessContext, error) {
	m, err := r.memberships.Find(ctx, companyID, userID)
	if errors.Is(err, domain.ErrMembershipNotFound) {
		return AccessContext{}, domain.ErrForbidden
	}
	if err != nil {
		return AccessContext{}, err
	}
	user, err := r.users.FindByID(ctx, userID)
	if err != nil {
		return AccessContext{}, err
	}
	company, err := r.companies.FindByID(ctx, companyID)
	if err != nil {
		return AccessContext{}, err
	}
	roles, err := r.roles.FindByIDs(ctx, companyID, m.RoleIDs())
	if err != nil {
		return AccessContext{}, err
	}
	return ResolveAccess(user, company, m, roles)
}

// Authorize = Resolve + Require(perm)
func (r *AccessResolver) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) (AccessContext, error) {
	access, err := r.Resolve(ctx, userID, companyID)
	if err != nil {
		return AccessContext{}, err
	}
	if err := access.Require(perm); err != nil {
		return AccessContext{}, err
	}
	return access, nil
}
