// Package requests คือรูปแบบ JSON ขาเข้า และการแปลงเป็น command ของ application
package requests

import (
	"account/backend/internal/modules/identity/application/commands"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type Register struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (r Register) ToCommand() commands.RegisterUser {
	return commands.RegisterUser{Email: r.Email, Name: r.Name, Password: r.Password}
}

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r Login) ToCommand() commands.Login {
	return commands.Login{Email: r.Email, Password: r.Password}
}

type ChangePassword struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (r ChangePassword) ToCommand(actor vo.UserID) commands.ChangePassword {
	return commands.ChangePassword{UserID: actor, OldPassword: r.OldPassword, NewPassword: r.NewPassword}
}

type CreateCompany struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	TaxID string `json:"tax_id"`
}

func (r CreateCompany) ToCommand(actor vo.UserID) commands.CreateCompany {
	return commands.CreateCompany{ActorID: actor, Code: r.Code, Name: r.Name, TaxID: r.TaxID}
}

type UpdateCompany struct {
	Name  string `json:"name"`
	TaxID string `json:"tax_id"`
}

func (r UpdateCompany) ToCommand(actor vo.UserID, company vo.CompanyID) commands.UpdateCompany {
	return commands.UpdateCompany{ActorID: actor, CompanyID: company, Name: r.Name, TaxID: r.TaxID}
}

type CreateRole struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (r CreateRole) ToCommand(actor vo.UserID, company vo.CompanyID) commands.CreateRole {
	return commands.CreateRole{
		ActorID: actor, CompanyID: company, Code: r.Code, Name: r.Name,
		Description: r.Description, Permissions: r.Permissions,
	}
}

type UpdateRole struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (r UpdateRole) ToCommand(actor vo.UserID, company vo.CompanyID, role vo.RoleID) commands.UpdateRole {
	return commands.UpdateRole{
		ActorID: actor, CompanyID: company, RoleID: role, Name: r.Name,
		Description: r.Description, Permissions: r.Permissions,
	}
}

type AddMember struct {
	Email   string   `json:"email"`
	RoleIDs []string `json:"role_ids"`
}

func (r AddMember) ToCommand(actor vo.UserID, company vo.CompanyID) (commands.AddMember, error) {
	ids, err := vo.ParseRoleIDs(r.RoleIDs)
	if err != nil {
		return commands.AddMember{}, err
	}
	return commands.AddMember{ActorID: actor, CompanyID: company, Email: r.Email, RoleIDs: ids}, nil
}

type AssignRoles struct {
	RoleIDs []string `json:"role_ids"`
}

func (r AssignRoles) ToCommand(actor vo.UserID, company vo.CompanyID, user vo.UserID) (commands.AssignMemberRoles, error) {
	ids, err := vo.ParseRoleIDs(r.RoleIDs)
	if err != nil {
		return commands.AssignMemberRoles{}, err
	}
	return commands.AssignMemberRoles{ActorID: actor, CompanyID: company, UserID: user, RoleIDs: ids}, nil
}
