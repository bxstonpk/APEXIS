package valueobjects

import (
	"regexp"
	"slices"
	"strings"

	"account/backend/internal/modules/identity/domain"
)

// Permission มีรูปแบบ "<resource>:<action>"
//
//	accounting.journal:post   — ตรงตัว
//	accounting.journal:*      — ทุก action ของ resource นี้
//	accounting.*:read         — read ทุก resource ใต้ accounting
//	*                         — ทุกอย่าง (ใช้กับ role owner)
//
// แต่ละ module ของ ERP ประกาศ permission ของตัวเองเป็นค่าคงที่แบบด้านล่าง
type Permission string

const PermAll Permission = "*"

// Permission ของ identity module
const (
	PermCompanyRead   Permission = "identity.company:read"
	PermCompanyUpdate Permission = "identity.company:update"
	PermMemberRead    Permission = "identity.member:read"
	PermMemberManage  Permission = "identity.member:manage"
	PermRoleRead      Permission = "identity.role:read"
	PermRoleManage    Permission = "identity.role:manage"
)

var (
	permResourcePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*(\.\*)?$`)
	permActionPattern   = regexp.MustCompile(`^([a-z][a-z0-9_]*|\*)$`)
)

func ParsePermission(raw string) (Permission, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == string(PermAll) {
		return PermAll, nil
	}
	resource, action, ok := strings.Cut(s, ":")
	if !ok || len(s) > 150 || !permResourcePattern.MatchString(resource) || !permActionPattern.MatchString(action) {
		return "", domain.ErrInvalidPermission
	}
	return Permission(s), nil
}

func (p Permission) String() string { return string(p) }

// Covers บอกว่าสิทธิ์ p (ที่ถืออยู่) ครอบคลุม required หรือไม่
// required เป็น wildcard ได้ (ใช้ตอนเช็คว่าผู้มอบสิทธิ์มีสิทธิ์นั้นจริง)
func (p Permission) Covers(required Permission) bool {
	if p == PermAll {
		return true
	}
	if required == PermAll {
		return false
	}
	gRes, gAct, _ := strings.Cut(string(p), ":")
	rRes, rAct, _ := strings.Cut(string(required), ":")

	if gAct != "*" && gAct != rAct {
		return false
	}
	if gRes == rRes {
		return true
	}
	if prefix, ok := strings.CutSuffix(gRes, "*"); ok { // "accounting.*" -> "accounting."
		return strings.HasPrefix(rRes, prefix)
	}
	return false
}

// PermissionSet คือชุดสิทธิ์ที่ไม่ซ้ำกัน
type PermissionSet map[Permission]struct{}

func NewPermissionSet(perms ...Permission) PermissionSet {
	s := make(PermissionSet, len(perms))
	s.Add(perms...)
	return s
}

func ParsePermissionSet(raw []string) (PermissionSet, error) {
	s := make(PermissionSet, len(raw))
	for _, r := range raw {
		p, err := ParsePermission(r)
		if err != nil {
			return nil, err
		}
		s[p] = struct{}{}
	}
	return s, nil
}

func (s PermissionSet) Add(perms ...Permission) {
	for _, p := range perms {
		s[p] = struct{}{}
	}
}

func (s PermissionSet) Merge(other PermissionSet) {
	for p := range other {
		s[p] = struct{}{}
	}
}

// Allows true ถ้ามีสิทธิ์ใดในชุดครอบคลุม required
func (s PermissionSet) Allows(required Permission) bool {
	for p := range s {
		if p.Covers(required) {
			return true
		}
	}
	return false
}

// AllowsAll true ถ้าครอบคลุมทุกสิทธิ์ใน other (ใช้กันการมอบสิทธิ์เกินตัว)
func (s PermissionSet) AllowsAll(other PermissionSet) bool {
	for p := range other {
		if !s.Allows(p) {
			return false
		}
	}
	return true
}

// Sorted คืน slice เรียงลำดับ (deterministic สำหรับ persist / response)
func (s PermissionSet) Sorted() []Permission {
	out := make([]Permission, 0, len(s))
	for p := range s {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

func (s PermissionSet) Strings() []string {
	sorted := s.Sorted()
	out := make([]string, len(sorted))
	for i, p := range sorted {
		out[i] = string(p)
	}
	return out
}

func (s PermissionSet) Clone() PermissionSet {
	out := make(PermissionSet, len(s))
	out.Merge(s)
	return out
}
