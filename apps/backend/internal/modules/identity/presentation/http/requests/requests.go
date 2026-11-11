// Package requests คือรูปแบบ JSON ขาเข้า / URL query และการแปลงเป็น input ของ use case
//
// presentation validation: ตรวจเฉพาะ field ที่ต้องมี, รูปแบบ (UUID, enum), และ query parameter
// กฎธุรกิจ (ความยาวชื่อ, รูปแบบรหัส, policy รหัสผ่าน ฯลฯ) ตรวจใน domain
package requests

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"apexis/backend/internal/modules/identity/application/dto"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

// ErrValidation ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client (HTTP 400)
var ErrValidation = errors.New("validation failed")

func required(fields map[string]string) error {
	var missing []string
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return fmt.Errorf("%w: %s is required", ErrValidation, strings.Join(missing, ", "))
}

// ---------- auth ----------

// POST /api/v1/users
type Register struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (r Register) ToInput() (dto.RegisterUserInput, error) {
	if err := required(map[string]string{"email": r.Email, "name": r.Name, "password": r.Password}); err != nil {
		return dto.RegisterUserInput{}, err
	}
	return dto.RegisterUserInput{Email: r.Email, Name: r.Name, Password: r.Password}, nil
}

// POST /api/v1/auth/tokens
type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r Login) ToInput() (dto.LoginInput, error) {
	if err := required(map[string]string{"email": r.Email, "password": r.Password}); err != nil {
		return dto.LoginInput{}, err
	}
	return dto.LoginInput{Email: r.Email, Password: r.Password}, nil
}

// PUT /api/v1/me/password
type ChangePassword struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (r ChangePassword) ToInput(actor vo.UserID) (dto.ChangePasswordInput, error) {
	if err := required(map[string]string{"old_password": r.OldPassword, "new_password": r.NewPassword}); err != nil {
		return dto.ChangePasswordInput{}, err
	}
	return dto.ChangePasswordInput{UserID: actor, OldPassword: r.OldPassword, NewPassword: r.NewPassword}, nil
}

// ---------- role ----------

type CreateRole struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (r CreateRole) ToInput(actor vo.UserID, company vo.CompanyID) (dto.CreateRoleInput, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return dto.CreateRoleInput{}, err
	}
	return dto.CreateRoleInput{
		ActorID: actor, CompanyID: company, Code: r.Code, Name: r.Name,
		Description: r.Description, Permissions: r.Permissions,
	}, nil
}

type UpdateRole struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (r UpdateRole) ToInput(actor vo.UserID, company vo.CompanyID, role vo.RoleID) (dto.UpdateRoleInput, error) {
	if err := required(map[string]string{"name": r.Name}); err != nil {
		return dto.UpdateRoleInput{}, err
	}
	return dto.UpdateRoleInput{
		ActorID: actor, CompanyID: company, RoleID: role, Name: r.Name,
		Description: r.Description, Permissions: r.Permissions,
	}, nil
}

// ---------- member ----------

type AddMember struct {
	Email   string   `json:"email"`
	RoleIDs []string `json:"role_ids"`
}

func (r AddMember) ToInput(actor vo.UserID, company vo.CompanyID) (dto.AddMemberInput, error) {
	if err := required(map[string]string{"email": r.Email}); err != nil {
		return dto.AddMemberInput{}, err
	}
	ids, err := vo.ParseRoleIDs(r.RoleIDs)
	if err != nil {
		return dto.AddMemberInput{}, err
	}
	return dto.AddMemberInput{ActorID: actor, CompanyID: company, Email: r.Email, RoleIDs: ids}, nil
}

// PUT /api/v1/companies/{companyID}/members/{userID}/roles — แทนที่ role ทั้งชุด
type AssignRoles struct {
	RoleIDs []string `json:"role_ids"`
}

func (r AssignRoles) ToInput(actor vo.UserID, company vo.CompanyID, user vo.UserID) (dto.AssignMemberRolesInput, error) {
	ids, err := vo.ParseRoleIDs(r.RoleIDs)
	if err != nil {
		return dto.AssignMemberRolesInput{}, err
	}
	return dto.AssignMemberRolesInput{ActorID: actor, CompanyID: company, UserID: user, RoleIDs: ids}, nil
}

// PATCH /api/v1/companies/{companyID}/members/{userID} — แก้บางส่วนของ member (ตอนนี้คือ status)
type UpdateMember struct {
	Status string `json:"status"`
}

func (r UpdateMember) ToInput(actor vo.UserID, company vo.CompanyID, user vo.UserID) (dto.ChangeMemberStatusInput, error) {
	if err := required(map[string]string{"status": r.Status}); err != nil {
		return dto.ChangeMemberStatusInput{}, err
	}
	status, err := vo.ParseMembershipStatus(r.Status)
	if err != nil {
		return dto.ChangeMemberStatusInput{}, err
	}
	return dto.ChangeMemberStatusInput{ActorID: actor, CompanyID: company, UserID: user, Status: status}, nil
}
