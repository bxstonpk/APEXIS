package valueobjects

import (
	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/identity/domain/errors"
)

// ID แต่ละชนิดเป็น type แยกกัน เพื่อให้ compiler กันการส่ง UserID ไปแทน CompanyID
// ใช้ UUIDv7 (เรียงตามเวลา) เพื่อให้ index ใน DB ทำงานได้ดี
type (
	UserID       uuid.UUID
	CompanyID    uuid.UUID
	RoleID       uuid.UUID
	MembershipID uuid.UUID
)

func newUUID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func parseUUID(s string, errInvalid error) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, errInvalid
	}
	return id, nil
}

func NewUserID() UserID          { return UserID(newUUID()) }
func (id UserID) String() string { return uuid.UUID(id).String() }
func (id UserID) IsZero() bool   { return uuid.UUID(id) == uuid.Nil }
func ParseUserID(s string) (UserID, error) {
	id, err := parseUUID(s, domain.ErrInvalidUserID)
	return UserID(id), err
}

func NewCompanyID() CompanyID       { return CompanyID(newUUID()) }
func (id CompanyID) String() string { return uuid.UUID(id).String() }
func (id CompanyID) IsZero() bool   { return uuid.UUID(id) == uuid.Nil }
func ParseCompanyID(s string) (CompanyID, error) {
	id, err := parseUUID(s, domain.ErrInvalidCompanyID)
	return CompanyID(id), err
}

func NewRoleID() RoleID          { return RoleID(newUUID()) }
func (id RoleID) String() string { return uuid.UUID(id).String() }
func (id RoleID) IsZero() bool   { return uuid.UUID(id) == uuid.Nil }
func ParseRoleID(s string) (RoleID, error) {
	id, err := parseUUID(s, domain.ErrInvalidRoleID)
	return RoleID(id), err
}

func ParseRoleIDs(raw []string) ([]RoleID, error) {
	out := make([]RoleID, len(raw))
	for i, s := range raw {
		id, err := ParseRoleID(s)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

func NewMembershipID() MembershipID    { return MembershipID(newUUID()) }
func (id MembershipID) String() string { return uuid.UUID(id).String() }
func (id MembershipID) IsZero() bool   { return uuid.UUID(id) == uuid.Nil }
