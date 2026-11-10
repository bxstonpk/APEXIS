// Package dto คือข้อมูลขาออกของ use case — เป็น plain struct ไม่ผูกกับ entity
// presentation layer แปลง dto -> response JSON อีกที
package dto

import (
	"time"

	"account/backend/internal/modules/identity/domain/entities"
	"account/backend/internal/modules/identity/domain/services"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type User struct {
	ID     string
	Email  string
	Name   string
	Status string
}

type AuthToken struct {
	AccessToken string
	ExpiresAt   time.Time
	User        User
}

type Company struct {
	ID     string
	Code   string
	Name   string
	TaxID  string
	Status string
}

type MyCompany struct {
	Company Company
	RoleIDs []string
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
	User    User
	Status  string
	RoleIDs []string
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

// ---------- mapping ----------

func FromUser(u *entities.User) User {
	return User{ID: u.ID().String(), Email: u.Email().String(), Name: u.Name(), Status: string(u.Status())}
}

func FromCompany(c *entities.Company) Company {
	return Company{
		ID: c.ID().String(), Code: c.Code(), Name: c.Name(), TaxID: c.TaxID(),
		Status: string(c.Status()),
	}
}

func FromRole(r *entities.Role) Role {
	return Role{
		ID: r.ID().String(), CompanyID: r.CompanyID().String(), Code: r.Code(), Name: r.Name(),
		Description: r.Description(), Permissions: r.Permissions().Strings(), IsSystem: r.IsSystem(),
	}
}

func FromRoles(roles []*entities.Role) []Role {
	out := make([]Role, len(roles))
	for i, r := range roles {
		out[i] = FromRole(r)
	}
	return out
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
