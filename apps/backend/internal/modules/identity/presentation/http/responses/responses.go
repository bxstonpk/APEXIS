// Package responses คือรูปแบบ JSON ขาออก (แปลงจาก application/dto)
package responses

import (
	"time"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/listing"
)

// envelope กลางอยู่ที่ platform/httpx (ใช้ร่วมกันทุก module)
type (
	Data[T any] = httpx.Data[T]
	List[T any] = httpx.List[T]
	Meta        = httpx.Meta
)

func ToList[D any, T any](r listing.Result[D], fn func(D) T) List[T] { return httpx.ToList(r, fn) }

type User struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"must_change_password"`
}

func FromUser(u dto.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, Status: u.Status, MustChangePassword: u.MustChangePassword}
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

type Member struct {
	User     User      `json:"user"`
	Status   string    `json:"status"`
	RoleIDs  []string  `json:"role_ids"`
	JoinedAt time.Time `json:"joined_at"`
}

func FromMember(m dto.Member) Member {
	return Member{User: FromUser(m.User), Status: m.Status, RoleIDs: m.RoleIDs, JoinedAt: m.JoinedAt}
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
