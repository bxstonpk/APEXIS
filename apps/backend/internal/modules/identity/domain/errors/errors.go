// Package domainerrors เก็บ error ของ identity ไว้เป็น package ล่างสุด
// เพื่อให้ entities / value_objects / services import ได้โดยไม่เกิด import cycle
//
// ทุกที่ import ด้วย alias "domain" เช่น domain.ErrUserNotFound
package domainerrors

import "errors"

// User
var (
	ErrInvalidUserID      = errors.New("invalid user id")
	ErrInvalidName        = errors.New("invalid user name")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPassword    = errors.New("password does not meet policy")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserInactive       = errors.New("user is inactive")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthenticated    = errors.New("unauthenticated")
)

// Company (tenant) — ข้อมูลบริษัทอยู่ที่ organization module, identity รู้จักแค่ company id
var ErrInvalidCompanyID = errors.New("invalid company id")

// Role & permission
var (
	ErrInvalidRoleID       = errors.New("invalid role id")
	ErrInvalidRoleCode     = errors.New("invalid role code")
	ErrInvalidRoleName     = errors.New("invalid role name")
	ErrInvalidPermission   = errors.New("invalid permission")
	ErrRoleNotFound        = errors.New("role not found")
	ErrRoleCodeExists      = errors.New("role code already exists in company")
	ErrRoleInUse           = errors.New("role is assigned to members")
	ErrSystemRoleImmutable = errors.New("system role cannot be modified")
	ErrRoleCompanyMismatch = errors.New("role does not belong to company")
)

// Membership & access
var (
	ErrInvalidMembershipID = errors.New("invalid membership id")
	ErrMembershipNotFound  = errors.New("membership not found")
	ErrMembershipExists    = errors.New("user is already a member of company")
	ErrMembershipInactive  = errors.New("membership is inactive")
	ErrInvalidMemberStatus = errors.New("invalid membership status")
	ErrCannotModifySelf    = errors.New("cannot modify your own membership")
	ErrForbidden           = errors.New("permission denied")
)
