package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	ports "apexis/backend/internal/modules/identity/application/ports/repositories"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	"apexis/backend/internal/modules/identity/domain/services"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/platform/database"
)

var _ ports.AccessRepository = (*AccessRepository)(nil)

// AccessRepository โหลดข้อมูลตัดสินสิทธิ์ (ถูกเรียกทุก request ใต้ /companies/{companyID})
type AccessRepository struct{ db *gorm.DB }

func NewAccessRepository(db *gorm.DB) *AccessRepository { return &AccessRepository{db: db} }

// FindAccessFacts ใช้ query เดียว:
//
//	memberships JOIN users
//	LEFT JOIN membership_roles LEFT JOIN roles LEFT JOIN role_permissions
//
// ได้ 1 แถวต่อ (role, permission) ของ member คนเดียว — จำนวนแถวน้อย จึงคุ้มกว่าแยกหลาย query
func (r *AccessRepository) FindAccessFacts(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (services.AccessFacts, error) {
	var rows []struct {
		MembershipID     uuid.UUID
		MembershipStatus string
		UserStatus       string
		RoleID           *uuid.UUID
		RoleCompanyID    *uuid.UUID
		RoleCode         *string
		RoleName         *string
		RoleDescription  *string
		RoleIsSystem     *bool
		Permission       *string
	}
	err := database.Conn(ctx, r.db).
		Table("identity_memberships AS m").
		Select(`m.id AS membership_id, m.status AS membership_status,
			u.status AS user_status,
			ro.id AS role_id, ro.company_id AS role_company_id, ro.code AS role_code,
			ro.name AS role_name, ro.description AS role_description, ro.is_system AS role_is_system,
			rp.permission`).
		Joins("JOIN identity_users AS u ON u.id = m.user_id").
		Joins("LEFT JOIN identity_membership_roles AS mr ON mr.membership_id = m.id").
		Joins("LEFT JOIN identity_roles AS ro ON ro.id = mr.role_id").
		Joins("LEFT JOIN identity_role_permissions AS rp ON rp.role_id = ro.id").
		Where("m.company_id = ? AND m.user_id = ?", uuid.UUID(companyID), uuid.UUID(userID)).
		Scan(&rows).Error
	if err != nil {
		return services.AccessFacts{}, err
	}
	if len(rows) == 0 {
		return services.AccessFacts{}, domain.ErrMembershipNotFound
	}

	type roleAcc struct {
		companyID               vo.CompanyID
		code, name, description string
		isSystem                bool
		perms                   vo.PermissionSet
	}
	roleIDs := []vo.RoleID{}
	byRole := map[vo.RoleID]*roleAcc{}
	for _, row := range rows {
		if row.RoleID == nil {
			continue
		}
		id := vo.RoleID(*row.RoleID)
		acc, ok := byRole[id]
		if !ok {
			acc = &roleAcc{
				companyID: vo.CompanyID(*row.RoleCompanyID), code: *row.RoleCode, name: *row.RoleName,
				description: *row.RoleDescription, isSystem: *row.RoleIsSystem, perms: vo.NewPermissionSet(),
			}
			byRole[id] = acc
			roleIDs = append(roleIDs, id)
		}
		if row.Permission != nil {
			acc.perms.Add(vo.Permission(*row.Permission))
		}
	}
	roles := make([]*entities.Role, len(roleIDs))
	for i, id := range roleIDs {
		a := byRole[id]
		roles[i] = entities.RehydrateRole(id, a.companyID, a.code, a.name, a.description, a.perms, a.isSystem)
	}

	first := rows[0]
	return services.AccessFacts{
		UserID:     userID,
		UserStatus: vo.UserStatus(first.UserStatus),
		CompanyID:  companyID,
		Membership: entities.RehydrateMembership(
			vo.MembershipID(first.MembershipID), userID, companyID, roleIDs,
			vo.MembershipStatus(first.MembershipStatus),
		),
		Roles: roles,
	}, nil
}
