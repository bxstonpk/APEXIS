package entities

import (
	"slices"

	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

// Membership คือ "พนักงาน X อยู่ในบริษัท Y และถือ role อะไรบ้าง"
// user หนึ่งคนมีได้หลาย membership (หลายบริษัท) แต่ได้แค่หนึ่ง membership ต่อบริษัท
type Membership struct {
	id        vo.MembershipID
	userID    vo.UserID
	companyID vo.CompanyID
	roleIDs   []vo.RoleID
	status    vo.MembershipStatus
}

// NewMembership รับ roles เป็น *Role เพื่อบังคับว่าทุก role ต้องเป็นของบริษัทเดียวกัน
func NewMembership(userID vo.UserID, companyID vo.CompanyID, roles []*Role) (*Membership, error) {
	if userID.IsZero() {
		return nil, domain.ErrInvalidUserID
	}
	if companyID.IsZero() {
		return nil, domain.ErrInvalidCompanyID
	}
	m := &Membership{
		id:        vo.NewMembershipID(),
		userID:    userID,
		companyID: companyID,
		status:    vo.MembershipStatusActive,
	}
	if err := m.AssignRoles(roles); err != nil {
		return nil, err
	}
	return m, nil
}

func RehydrateMembership(id vo.MembershipID, userID vo.UserID, companyID vo.CompanyID, roleIDs []vo.RoleID, status vo.MembershipStatus) *Membership {
	return &Membership{id: id, userID: userID, companyID: companyID, roleIDs: slices.Clone(roleIDs), status: status}
}

func (m *Membership) ID() vo.MembershipID         { return m.id }
func (m *Membership) UserID() vo.UserID           { return m.userID }
func (m *Membership) CompanyID() vo.CompanyID     { return m.companyID }
func (m *Membership) RoleIDs() []vo.RoleID        { return slices.Clone(m.roleIDs) }
func (m *Membership) Status() vo.MembershipStatus { return m.status }
func (m *Membership) IsActive() bool              { return m.status == vo.MembershipStatusActive }
func (m *Membership) HasRole(id vo.RoleID) bool   { return slices.Contains(m.roleIDs, id) }

// AssignRoles แทนที่ role ทั้งหมดของ member (ตัดตัวซ้ำ, ต้องอยู่บริษัทเดียวกัน)
func (m *Membership) AssignRoles(roles []*Role) error {
	ids := make([]vo.RoleID, 0, len(roles))
	for _, r := range roles {
		if r == nil {
			return domain.ErrRoleNotFound
		}
		if r.CompanyID() != m.companyID {
			return domain.ErrRoleCompanyMismatch
		}
		if !slices.Contains(ids, r.ID()) {
			ids = append(ids, r.ID())
		}
	}
	m.roleIDs = ids
	return nil
}

func (m *Membership) Suspend()  { m.status = vo.MembershipStatusSuspended }
func (m *Membership) Activate() { m.status = vo.MembershipStatusActive }

// ChangeStatus ใช้กับ PATCH member — รับเฉพาะสถานะที่กำหนดไว้
func (m *Membership) ChangeStatus(s vo.MembershipStatus) error {
	if _, err := vo.ParseMembershipStatus(string(s)); err != nil {
		return err
	}
	m.status = s
	return nil
}
