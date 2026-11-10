// Package responses คือรูปแบบ JSON ขาออก (แปลงจาก application/dto)
package responses

import (
	"time"

	"account/backend/internal/modules/identity/application/dto"
)

type Data[T any] struct {
	Data T `json:"data"`
}

type Error struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func FromUser(u dto.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, Status: u.Status}
}

type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	User        User      `json:"user"`
}

func FromAuthToken(t dto.AuthToken) Token {
	return Token{AccessToken: t.AccessToken, TokenType: "Bearer", ExpiresAt: t.ExpiresAt, User: FromUser(t.User)}
}

type Company struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	TaxID  string `json:"tax_id"`
	Status string `json:"status"`
}

func FromCompany(c dto.Company) Company {
	return Company{ID: c.ID, Code: c.Code, Name: c.Name, TaxID: c.TaxID, Status: c.Status}
}

type MyCompany struct {
	Company
	RoleIDs []string `json:"role_ids"`
}

func FromMyCompanies(cs []dto.MyCompany) []MyCompany {
	out := make([]MyCompany, len(cs))
	for i, c := range cs {
		out[i] = MyCompany{Company: FromCompany(c.Company), RoleIDs: c.RoleIDs}
	}
	return out
}

type Role struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	IsSystem    bool     `json:"is_system"`
}

func FromRole(r dto.Role) Role {
	return Role{
		ID: r.ID, Code: r.Code, Name: r.Name, Description: r.Description,
		Permissions: r.Permissions, IsSystem: r.IsSystem,
	}
}

func FromRoles(rs []dto.Role) []Role {
	out := make([]Role, len(rs))
	for i, r := range rs {
		out[i] = FromRole(r)
	}
	return out
}

type Member struct {
	User    User     `json:"user"`
	Status  string   `json:"status"`
	RoleIDs []string `json:"role_ids"`
}

func FromMembers(ms []dto.Member) []Member {
	out := make([]Member, len(ms))
	for i, m := range ms {
		out[i] = Member{User: FromUser(m.User), Status: m.Status, RoleIDs: m.RoleIDs}
	}
	return out
}

type Membership struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	CompanyID string   `json:"company_id"`
	Status    string   `json:"status"`
	RoleIDs   []string `json:"role_ids"`
}

func FromMembership(m dto.Membership) Membership {
	return Membership{ID: m.ID, UserID: m.UserID, CompanyID: m.CompanyID, Status: m.Status, RoleIDs: m.RoleIDs}
}

type Access struct {
	UserID      string   `json:"user_id"`
	CompanyID   string   `json:"company_id"`
	RoleIDs     []string `json:"role_ids"`
	Permissions []string `json:"permissions"`
}

func FromAccess(a dto.Access) Access {
	return Access{UserID: a.UserID, CompanyID: a.CompanyID, RoleIDs: a.RoleIDs, Permissions: a.Permissions}
}
