package services

import (
	"errors"
	"testing"

	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

func fixture(t *testing.T) (*entities.User, vo.CompanyID, *entities.Role, *entities.Role) {
	t.Helper()
	email, _ := vo.NewEmail("staff@example.com")
	user, err := entities.NewUser(email, "Staff", "hash")
	if err != nil {
		t.Fatal(err)
	}
	company := vo.NewCompanyID() // บริษัทอยู่ที่ organization module — identity รู้จักแค่ id
	sales, err := entities.NewRole(company, "sales", "Sales", "", vo.NewPermissionSet("sales.order:create", "sales.order:read"))
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := entities.NewRole(company, "viewer", "Viewer", "", vo.NewPermissionSet("sales.*:read", "organization.company:read"))
	if err != nil {
		t.Fatal(err)
	}
	return user, company, sales, viewer
}

func facts(u *entities.User, c vo.CompanyID, m *entities.Membership, roles []*entities.Role) AccessFacts {
	return AccessFacts{
		UserID: u.ID(), UserStatus: u.Status(),
		CompanyID:  c,
		Membership: m, Roles: roles,
	}
}

func TestResolveAccessUnionsRolePermissions(t *testing.T) {
	user, company, sales, viewer := fixture(t)
	m, err := entities.NewMembership(user.ID(), company, []*entities.Role{sales, viewer})
	if err != nil {
		t.Fatal(err)
	}
	access, err := ResolveAccess(facts(user, company, m, []*entities.Role{sales, viewer}))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []vo.Permission{"sales.order:create", "sales.invoice:read", "organization.company:read"} {
		if !access.Can(p) {
			t.Errorf("expected access to %q", p)
		}
	}
	if err := access.Require("sales.order:delete"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("Require(delete) = %v, want ErrForbidden", err)
	}
	if access.CompanyID() != company {
		t.Error("company id mismatch")
	}
}

func TestResolveAccessIgnoresRolesNotOnMembership(t *testing.T) {
	user, company, sales, viewer := fixture(t)
	m, _ := entities.NewMembership(user.ID(), company, []*entities.Role{viewer})
	access, err := ResolveAccess(facts(user, company, m, []*entities.Role{sales, viewer}))
	if err != nil {
		t.Fatal(err)
	}
	if access.Can("sales.order:create") {
		t.Error("must not get permission from a role the member does not hold")
	}
}

func TestResolveAccessDeniesInactive(t *testing.T) {
	cases := map[string]struct {
		mutate func(*entities.User, vo.CompanyID, *entities.Membership)
		want   error
	}{
		"user disabled":        {func(u *entities.User, _ vo.CompanyID, _ *entities.Membership) { u.Disable() }, domain.ErrUserInactive},
		"membership suspended": {func(_ *entities.User, _ vo.CompanyID, m *entities.Membership) { m.Suspend() }, domain.ErrMembershipInactive},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			user, company, sales, _ := fixture(t)
			m, _ := entities.NewMembership(user.ID(), company, []*entities.Role{sales})
			c.mutate(user, company, m)
			if _, err := ResolveAccess(facts(user, company, m, []*entities.Role{sales})); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestResolveAccessRejectsForeignMembership(t *testing.T) {
	user, company, sales, _ := fixture(t)
	other := vo.NewCompanyID()
	m, _ := entities.NewMembership(user.ID(), company, []*entities.Role{sales})
	f := facts(user, other, m, []*entities.Role{sales})
	if _, err := ResolveAccess(f); !errors.Is(err, domain.ErrMembershipNotFound) {
		t.Fatalf("got %v, want ErrMembershipNotFound", err)
	}
}

func TestMembershipChangeStatus(t *testing.T) {
	user, company, sales, _ := fixture(t)
	m, _ := entities.NewMembership(user.ID(), company, []*entities.Role{sales})
	if err := m.ChangeStatus(vo.MembershipStatusSuspended); err != nil || m.IsActive() {
		t.Fatalf("suspend: err=%v active=%v", err, m.IsActive())
	}
	if err := m.ChangeStatus("deleted"); !errors.Is(err, domain.ErrInvalidMemberStatus) {
		t.Fatalf("got %v, want ErrInvalidMemberStatus", err)
	}
}

func TestMembershipRejectsRoleFromOtherCompany(t *testing.T) {
	user, company, _, _ := fixture(t)
	other := vo.NewCompanyID()
	foreign, _ := entities.NewRole(other, "sales", "Sales", "", nil)
	if _, err := entities.NewMembership(user.ID(), company, []*entities.Role{foreign}); !errors.Is(err, domain.ErrRoleCompanyMismatch) {
		t.Fatalf("got %v, want ErrRoleCompanyMismatch", err)
	}
}

func TestOwnerRoleIsImmutable(t *testing.T) {
	_, company, _, _ := fixture(t)
	owner, _ := entities.NewOwnerRole(company)
	if err := owner.SetPermissions(vo.NewPermissionSet()); !errors.Is(err, domain.ErrSystemRoleImmutable) {
		t.Fatalf("got %v", err)
	}
	if err := owner.EnsureDeletable(); !errors.Is(err, domain.ErrSystemRoleImmutable) {
		t.Fatalf("got %v", err)
	}
}

// บัญชีที่ผู้ดูแลสร้างให้ (รหัสผ่านชั่วคราว) เข้าบริษัทไม่ได้จนกว่าจะเปลี่ยนรหัสผ่าน
func TestResolveAccessRequiresPasswordChange(t *testing.T) {
	_, company, sales, _ := fixture(t)
	email, _ := vo.NewEmail("new@example.com")
	user, err := entities.NewUserWithTemporaryPassword(email, "New", "hash")
	if err != nil {
		t.Fatal(err)
	}
	m, err := entities.NewMembership(user.ID(), company, []*entities.Role{sales})
	if err != nil {
		t.Fatal(err)
	}
	f := facts(user, company, m, []*entities.Role{sales})
	f.MustChangePassword = user.MustChangePassword()
	if _, err := ResolveAccess(f); !errors.Is(err, domain.ErrPasswordChangeRequired) {
		t.Fatalf("ResolveAccess = %v, want ErrPasswordChangeRequired", err)
	}

	if err := user.ChangePasswordHash("new-hash"); err != nil || user.MustChangePassword() {
		t.Fatalf("after change: must=%v err=%v", user.MustChangePassword(), err)
	}
	f.MustChangePassword = user.MustChangePassword()
	if _, err := ResolveAccess(f); err != nil {
		t.Fatalf("after change = %v", err)
	}
}
