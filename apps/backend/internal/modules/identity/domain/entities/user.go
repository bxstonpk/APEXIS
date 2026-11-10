package entities

import (
	"strings"
	"unicode/utf8"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/enums"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

// User คือบัญชีผู้ใช้ระดับระบบ (global) — ล็อกอินครั้งเดียวแล้วเข้าได้หลายบริษัท
// สิทธิ์ต่อบริษัทอยู่ที่ Membership ไม่ได้อยู่ที่ User
//
// ไม่มี created_at / updated_at เพราะเป็น metadata ของ persistence ไม่ใช่ business rule
// (gorm จัดการเองใน infrastructure)
type User struct {
	id           vo.UserID
	email        vo.Email
	name         string
	passwordHash string
	status       enums.UserStatus
}

func NewUser(email vo.Email, name, passwordHash string) (*User, error) {
	if email == "" {
		return nil, domain.ErrInvalidEmail
	}
	n, err := normalizeUserName(name)
	if err != nil {
		return nil, err
	}
	if passwordHash == "" {
		return nil, domain.ErrInvalidPassword
	}
	return &User{
		id:           vo.NewUserID(),
		email:        email,
		name:         n,
		passwordHash: passwordHash,
		status:       enums.UserStatusActive,
	}, nil
}

// RehydrateUser ใช้โดย repository เพื่อสร้าง entity กลับจาก DB (ไม่ validate ซ้ำ)
func RehydrateUser(id vo.UserID, email vo.Email, name, passwordHash string, status enums.UserStatus) *User {
	return &User{id: id, email: email, name: name, passwordHash: passwordHash, status: status}
}

func (u *User) ID() vo.UserID            { return u.id }
func (u *User) Email() vo.Email          { return u.email }
func (u *User) Name() string             { return u.name }
func (u *User) PasswordHash() string     { return u.passwordHash }
func (u *User) Status() enums.UserStatus { return u.status }
func (u *User) IsActive() bool           { return u.status == enums.UserStatusActive }

func (u *User) Rename(name string) error {
	n, err := normalizeUserName(name)
	if err != nil {
		return err
	}
	u.name = n
	return nil
}

func (u *User) ChangePasswordHash(hash string) error {
	if hash == "" {
		return domain.ErrInvalidPassword
	}
	u.passwordHash = hash
	return nil
}

func (u *User) Disable() { u.status = enums.UserStatusDisabled }
func (u *User) Enable()  { u.status = enums.UserStatusActive }

func normalizeUserName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || utf8.RuneCountInString(n) > 200 {
		return "", domain.ErrInvalidName
	}
	return n, nil
}
