// Package events คือ domain event ของ identity — module อื่น (เช่น HR, audit log)
// subscribe ได้ผ่าน ports.EventPublisher โดยไม่ต้องผูกกับ identity โดยตรง
package events

import (
	"time"

	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type UserRegistered struct {
	UserID vo.UserID
	Email  vo.Email
	At     time.Time
}

func (e UserRegistered) EventName() string     { return "identity.user.registered" }
func (e UserRegistered) OccurredAt() time.Time { return e.At }

type MemberAdded struct {
	CompanyID vo.CompanyID
	UserID    vo.UserID
	RoleIDs   []vo.RoleID
	By        vo.UserID
	At        time.Time
}

func (e MemberAdded) EventName() string     { return "identity.member.added" }
func (e MemberAdded) OccurredAt() time.Time { return e.At }

type MemberRolesChanged struct {
	CompanyID vo.CompanyID
	UserID    vo.UserID
	RoleIDs   []vo.RoleID
	By        vo.UserID
	At        time.Time
}

func (e MemberRolesChanged) EventName() string     { return "identity.member.roles_changed" }
func (e MemberRolesChanged) OccurredAt() time.Time { return e.At }

type MemberStatusChanged struct {
	CompanyID vo.CompanyID
	UserID    vo.UserID
	Status    string
	By        vo.UserID
	At        time.Time
}

func (e MemberStatusChanged) EventName() string     { return "identity.member.status_changed" }
func (e MemberStatusChanged) OccurredAt() time.Time { return e.At }

type RoleChanged struct {
	CompanyID vo.CompanyID
	RoleID    vo.RoleID
	Action    string // created | updated | deleted
	By        vo.UserID
	At        time.Time
}

func (e RoleChanged) EventName() string     { return "identity.role.changed" }
func (e RoleChanged) OccurredAt() time.Time { return e.At }
