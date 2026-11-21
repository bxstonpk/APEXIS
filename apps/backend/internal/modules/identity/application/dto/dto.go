// Package dto คือข้อมูลขาเข้า (Input / Filter) และขาออกของ use case
// เป็น plain struct ไม่ผูกกับ entity หรือ HTTP — presentation แปลง request -> input และ dto -> response
package dto

import (
	"time"

	"apexis/backend/internal/modules/identity/domain/entities"
	"apexis/backend/internal/modules/identity/domain/services"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- Output ----------

type User struct {
	ID                 string
	Email              string
	Name               string
	Status             string
	MustChangePassword bool
}

type AuthToken struct {
	AccessToken string
	ExpiresAt   time.Time
	User        User
}

type Role struct {
	ID          string
	CompanyID   string
	Code        string
	Name        string
	Description string
	Permissions []string
	IsSystem    bool
}

type Member struct {
	User     User
	Status   string
	RoleIDs  []string
	JoinedAt time.Time
}

type Membership struct {
	ID        string
	UserID    string
	CompanyID string
	Status    string
	RoleIDs   []string
}

type Access struct {
	UserID      string
	CompanyID   string
	RoleIDs     []string
	Permissions []string
}

// ---------- Input (write) ----------

type RegisterUserInput struct {
	Email    string
	Name     string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type ChangePasswordInput struct {
	UserID      vo.UserID
	OldPassword string
	NewPassword string
}

// ProvisionOwnerInput: ตั้งสิทธิ์เริ่มต้นของบริษัทที่เพิ่งสร้าง (บริษัทถูกสร้างที่ organization module)
type ProvisionOwnerInput struct {
	CompanyID vo.CompanyID
	OwnerID   vo.UserID
}

type CreateRoleInput struct {
	ActorID     vo.UserID
	CompanyID   vo.CompanyID
	Code        string
	Name        string
	Description string
	Permissions []string
}

type UpdateRoleInput struct {
	ActorID     vo.UserID
	CompanyID   vo.CompanyID
	RoleID      vo.RoleID
	Name        string
	Description string
	Permissions []string
}

type DeleteRoleInput struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	RoleID    vo.RoleID
}

type AddMemberInput struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	Email     string
	RoleIDs   []vo.RoleID
	// NewUser: ยังไม่มีบัญชี — สร้างบัญชีพร้อมรหัสผ่านชั่วคราวให้ (ต้องเปลี่ยนเมื่อเข้าสู่ระบบครั้งแรก)
	// nil = เพิ่มเฉพาะผู้ที่มีบัญชีแล้ว (ไม่พบ = ErrUserNotFound)
	NewUser *NewUserInput
}

type NewUserInput struct {
	Name     string
	Password string
}

type AssignMemberRolesInput struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	UserID    vo.UserID
	RoleIDs   []vo.RoleID
}

type ChangeMemberStatusInput struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	UserID    vo.UserID
	Status    vo.MembershipStatus
}

// ---------- Input (list / search จาก URL query) ----------
//
// *SortKeys คือคีย์สาธารณะที่ client ส่งใน ?sort= ได้ (presentation ใช้ตรวจ,
// repository map เป็น column ผ่าน whitelist ของตัวเอง)

var (
	MemberSortKeys    = []string{"name", "email", "status", "joined_at"}
	DefaultMemberSort = []listing.Sort{{Key: "name"}}
)

// MemberFilter: GET /companies/{companyID}/members?q=&status=&role_id=&sort=&page=&page_size=
type MemberFilter struct {
	Search   string // ค้นจากชื่อหรืออีเมล
	Statuses []vo.MembershipStatus
	RoleID   *vo.RoleID
	Sort     []listing.Sort
	Page     listing.Page
}

type ListMembersInput struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	Filter    MemberFilter
}

var (
	RoleSortKeys    = []string{"code", "name", "is_system"}
	DefaultRoleSort = []listing.Sort{{Key: "is_system", Desc: true}, {Key: "code"}}
)

// RoleFilter: GET /companies/{companyID}/roles?q=&is_system=&sort=&page=&page_size=
type RoleFilter struct {
	Search   string // ค้นจาก code หรือชื่อ
	IsSystem *bool
	Sort     []listing.Sort
	Page     listing.Page
}

type ListRolesInput struct {
	ActorID   vo.UserID
	CompanyID vo.CompanyID
	Filter    RoleFilter
}

// ---------- mapping ----------

func FromUser(u *entities.User) User {
	return User{ID: u.ID().String(), Email: u.Email().String(), Name: u.Name(), Status: string(u.Status()), MustChangePassword: u.MustChangePassword()}
}

func FromRole(r *entities.Role) Role {
	return Role{
		ID: r.ID().String(), CompanyID: r.CompanyID().String(), Code: r.Code(), Name: r.Name(),
		Description: r.Description(), Permissions: r.Permissions().Strings(), IsSystem: r.IsSystem(),
	}
}

func FromMembership(m *entities.Membership) Membership {
	return Membership{
		ID: m.ID().String(), UserID: m.UserID().String(), CompanyID: m.CompanyID().String(),
		Status: string(m.Status()), RoleIDs: RoleIDStrings(m.RoleIDs()),
	}
}

func FromAccess(a services.AccessContext) Access {
	return Access{
		UserID: a.UserID().String(), CompanyID: a.CompanyID().String(),
		RoleIDs: RoleIDStrings(a.RoleIDs()), Permissions: a.Permissions().Strings(),
	}
}

func RoleIDStrings(ids []vo.RoleID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
