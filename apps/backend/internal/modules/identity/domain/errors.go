// Package domain (root) เก็บ error ของ identity ไว้เป็น package ล่างสุด
// เพื่อให้ entities / valueobjects / services import ได้โดยไม่เกิด import cycle
package domain

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

// Company (tenant)
var (
	ErrInvalidCompanyID   = errors.New("invalid company id")
	ErrInvalidCompanyCode = errors.New("invalid company code")
	ErrInvalidCompanyName = errors.New("invalid company name")
	ErrInvalidTaxID       = errors.New("invalid tax id")
	ErrCompanyNotFound    = errors.New("company not found")
	ErrCompanyCodeExists  = errors.New("company code already exists")
	ErrCompanyInactive    = errors.New("company is inactive")
)

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
	ErrCannotModifySelf    = errors.New("cannot modify your own membership")
	ErrForbidden           = errors.New("permission denied")
)
