package entities

import (
	"strings"
	"unicode/utf8"

	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
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
	status       vo.UserStatus
	// mustChangePassword: ผู้ดูแลสร้างบัญชีให้พร้อมรหัสผ่านชั่วคราว — ต้องเปลี่ยนก่อนเข้าใช้งานบริษัท
	mustChangePassword bool
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
		status:       vo.UserStatusActive,
	}, nil
}

// NewUserWithTemporaryPassword: บัญชีที่ผู้ดูแลบริษัทสร้างให้ — รหัสผ่านเป็นแบบชั่วคราว ต้องเปลี่ยนเมื่อเข้าสู่ระบบครั้งแรก
func NewUserWithTemporaryPassword(email vo.Email, name, passwordHash string) (*User, error) {
	u, err := NewUser(email, name, passwordHash)
	if err != nil {
		return nil, err
	}
	u.mustChangePassword = true
	return u, nil
}

// RehydrateUser ใช้โดย repository เพื่อสร้าง entity กลับจาก DB (ไม่ validate ซ้ำ)
func RehydrateUser(id vo.UserID, email vo.Email, name, passwordHash string, status vo.UserStatus, mustChangePassword bool) *User {
	return &User{id: id, email: email, name: name, passwordHash: passwordHash, status: status, mustChangePassword: mustChangePassword}
}

func (u *User) ID() vo.UserID            { return u.id }
func (u *User) Email() vo.Email          { return u.email }
func (u *User) Name() string             { return u.name }
func (u *User) PasswordHash() string     { return u.passwordHash }
func (u *User) Status() vo.UserStatus    { return u.status }
func (u *User) IsActive() bool           { return u.status == vo.UserStatusActive }
func (u *User) MustChangePassword() bool { return u.mustChangePassword }

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
	u.mustChangePassword = false // ตั้งรหัสผ่านเองแล้ว
	return nil
}

func (u *User) Disable() { u.status = vo.UserStatusDisabled }
func (u *User) Enable()  { u.status = vo.UserStatusActive }

func normalizeUserName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || utf8.RuneCountInString(n) > 200 {
		return "", domain.ErrInvalidName
	}
	return n, nil
}
