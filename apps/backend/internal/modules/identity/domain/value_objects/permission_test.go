package valueobjects

import (
	"errors"
	"testing"

	domain "apexis/backend/internal/modules/identity/domain/errors"
)

func TestParsePermission(t *testing.T) {
	valid := []string{"*", "sales.order:create", " Sales.Order:CREATE ", "accounting.*:read", "accounting.journal:*", "a:b"}
	for _, s := range valid {
		if _, err := ParsePermission(s); err != nil {
			t.Errorf("ParsePermission(%q) unexpected error %v", s, err)
		}
	}
	invalid := []string{"", "sales", "sales:", ":create", "sales.*.order:read", "sales order:read", "1sales:read", "*:read"}
	for _, s := range invalid {
		if _, err := ParsePermission(s); !errors.Is(err, domain.ErrInvalidPermission) {
			t.Errorf("ParsePermission(%q) = %v, want ErrInvalidPermission", s, err)
		}
	}
}

func TestPermissionCovers(t *testing.T) {
	cases := []struct {
		granted, required Permission
		want              bool
	}{
		{"*", "sales.order:create", true},
		{"*", "*", true},
		{"sales.order:*", "*", false},
		{"sales.order:create", "sales.order:create", true},
		{"sales.order:create", "sales.order:delete", false},
		{"sales.order:*", "sales.order:delete", true},
		{"sales.*:read", "sales.order:read", true},
		{"sales.*:read", "sales.order:create", false},
		{"sales.*:read", "salesx.order:read", false},
		{"sales.*:*", "sales.*:read", true},
		{"sales.order:create", "sales.order:*", false}, // ให้สิทธิ์ wildcard ไม่ได้ถ้าถือแค่บาง action
		{"sales.order:read", "purchase.order:read", false},
	}
	for _, c := range cases {
		if got := c.granted.Covers(c.required); got != c.want {
			t.Errorf("%q.Covers(%q) = %v, want %v", c.granted, c.required, got, c.want)
		}
	}
}

func TestPermissionSetAllowsAll(t *testing.T) {
	admin := NewPermissionSet("identity.*:*")
	if !admin.AllowsAll(NewPermissionSet(PermMemberManage, PermRoleRead)) {
		t.Fatal("identity.*:* should cover identity permissions")
	}
	if admin.AllowsAll(NewPermissionSet(PermMemberManage, "accounting.journal:post")) {
		t.Fatal("identity.*:* must not cover accounting permission")
	}
	if admin.AllowsAll(NewPermissionSet(PermAll)) {
		t.Fatal("only * covers *")
	}
}
