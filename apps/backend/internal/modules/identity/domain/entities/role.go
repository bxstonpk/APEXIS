package entities

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"account/backend/internal/modules/identity/domain"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,49}$`)

// RoleCodeOwner คือ system role ที่สร้างอัตโนมัติตอนสร้างบริษัท มีสิทธิ์ "*"
const RoleCodeOwner = "owner"

// Role อยู่ภายใต้บริษัทเดียว (tenant-scoped) — แต่ละบริษัทกำหนด role ของตัวเอง
type Role struct {
	id          vo.RoleID
	companyID   vo.CompanyID
	code        string
	name        string
	description string
	permissions vo.PermissionSet
	isSystem    bool
}

func NewRole(companyID vo.CompanyID, code, name, description string, perms vo.PermissionSet) (*Role, error) {
	if companyID.IsZero() {
		return nil, domain.ErrInvalidCompanyID
	}
	c, err := normalizeRoleCode(code)
	if err != nil {
		return nil, err
	}
	n, err := normalizeRoleName(name)
	if err != nil {
		return nil, err
	}
	if perms == nil {
		perms = vo.NewPermissionSet()
	}
	return &Role{
		id:          vo.NewRoleID(),
		companyID:   companyID,
		code:        c,
		name:        n,
		description: strings.TrimSpace(description),
		permissions: perms.Clone(),
	}, nil
}

// NewOwnerRole สร้าง system role "owner" ที่มีสิทธิ์ทุกอย่างในบริษัท (แก้/ลบไม่ได้)
func NewOwnerRole(companyID vo.CompanyID) (*Role, error) {
	r, err := NewRole(companyID, RoleCodeOwner, "Owner", "Full access to the company", vo.NewPermissionSet(vo.PermAll))
	if err != nil {
		return nil, err
	}
	r.isSystem = true
	return r, nil
}

func RehydrateRole(id vo.RoleID, companyID vo.CompanyID, code, name, description string, perms vo.PermissionSet, isSystem bool) *Role {
	if perms == nil {
		perms = vo.NewPermissionSet()
	}
	return &Role{id: id, companyID: companyID, code: code, name: name, description: description, permissions: perms, isSystem: isSystem}
}

func (r *Role) ID() vo.RoleID                 { return r.id }
func (r *Role) CompanyID() vo.CompanyID       { return r.companyID }
func (r *Role) Code() string                  { return r.code }
func (r *Role) Name() string                  { return r.name }
func (r *Role) Description() string           { return r.description }
func (r *Role) Permissions() vo.PermissionSet { return r.permissions.Clone() }
func (r *Role) IsSystem() bool                { return r.isSystem }

func (r *Role) Rename(name, description string) error {
	if r.isSystem {
		return domain.ErrSystemRoleImmutable
	}
	n, err := normalizeRoleName(name)
	if err != nil {
		return err
	}
	r.name, r.description = n, strings.TrimSpace(description)
	return nil
}

func (r *Role) SetPermissions(perms vo.PermissionSet) error {
	if r.isSystem {
		return domain.ErrSystemRoleImmutable
	}
	if perms == nil {
		perms = vo.NewPermissionSet()
	}
	r.permissions = perms.Clone()
	return nil
}

// EnsureDeletable ตรวจกฎของ entity (การตรวจว่ายังมีคนใช้อยู่ทำที่ DB/repository)
func (r *Role) EnsureDeletable() error {
	if r.isSystem {
		return domain.ErrSystemRoleImmutable
	}
	return nil
}

func normalizeRoleCode(code string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(code))
	if !roleCodePattern.MatchString(c) {
		return "", domain.ErrInvalidRoleCode
	}
	return c, nil
}

func normalizeRoleName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || utf8.RuneCountInString(n) > 100 {
		return "", domain.ErrInvalidRoleName
	}
	return n, nil
}
