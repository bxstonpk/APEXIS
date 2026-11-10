package services

import (
	"errors"
	"testing"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/entities"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

func fixture(t *testing.T) (*entities.User, *entities.Company, *entities.Role, *entities.Role) {
	t.Helper()
	email, _ := vo.NewEmail("staff@example.com")
	user, err := entities.NewUser(email, "Staff", "hash")
	if err != nil {
		t.Fatal(err)
	}
	company, err := entities.NewCompany("acme", "ACME Co., Ltd.", "0105556000000")
	if err != nil {
		t.Fatal(err)
	}
	sales, err := entities.NewRole(company.ID(), "sales", "Sales", "", vo.NewPermissionSet("sales.order:create", "sales.order:read"))
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := entities.NewRole(company.ID(), "viewer", "Viewer", "", vo.NewPermissionSet("sales.*:read", vo.PermCompanyRead))
	if err != nil {
		t.Fatal(err)
	}
	return user, company, sales, viewer
}

func TestResolveAccessUnionsRolePermissions(t *testing.T) {
	user, company, sales, viewer := fixture(t)
	m, err := entities.NewMembership(user.ID(), company.ID(), []*entities.Role{sales, viewer})
	if err != nil {
		t.Fatal(err)
	}
	access, err := ResolveAccess(user, company, m, []*entities.Role{sales, viewer})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []vo.Permission{"sales.order:create", "sales.invoice:read", vo.PermCompanyRead} {
		if !access.Can(p) {
			t.Errorf("expected access to %q", p)
		}
	}
	if err := access.Require("sales.order:delete"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("Require(delete) = %v, want ErrForbidden", err)
	}
	if access.CompanyID() != company.ID() {
		t.Error("company id mismatch")
	}
}

func TestResolveAccessIgnoresRolesNotOnMembership(t *testing.T) {
	user, company, sales, viewer := fixture(t)
	m, _ := entities.NewMembership(user.ID(), company.ID(), []*entities.Role{viewer})
	access, err := ResolveAccess(user, company, m, []*entities.Role{sales, viewer})
	if err != nil {
		t.Fatal(err)
	}
	if access.Can("sales.order:create") {
		t.Error("must not get permission from a role the member does not hold")
	}
}

func TestResolveAccessDeniesInactive(t *testing.T) {
	cases := map[string]struct {
		mutate func(*entities.User, *entities.Company, *entities.Membership)
		want   error
	}{
		"user disabled":        {func(u *entities.User, _ *entities.Company, _ *entities.Membership) { u.Disable() }, domain.ErrUserInactive},
		"company inactive":     {func(_ *entities.User, c *entities.Company, _ *entities.Membership) { c.Deactivate() }, domain.ErrCompanyInactive},
		"membership suspended": {func(_ *entities.User, _ *entities.Company, m *entities.Membership) { m.Suspend() }, domain.ErrMembershipInactive},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			user, company, sales, _ := fixture(t)
			m, _ := entities.NewMembership(user.ID(), company.ID(), []*entities.Role{sales})
			c.mutate(user, company, m)
			if _, err := ResolveAccess(user, company, m, []*entities.Role{sales}); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestMembershipRejectsRoleFromOtherCompany(t *testing.T) {
	user, company, _, _ := fixture(t)
	other, _ := entities.NewCompany("other", "Other", "")
	foreign, _ := entities.NewRole(other.ID(), "sales", "Sales", "", nil)
	if _, err := entities.NewMembership(user.ID(), company.ID(), []*entities.Role{foreign}); !errors.Is(err, domain.ErrRoleCompanyMismatch) {
		t.Fatalf("got %v, want ErrRoleCompanyMismatch", err)
	}
}

func TestOwnerRoleIsImmutable(t *testing.T) {
	_, company, _, _ := fixture(t)
	owner, _ := entities.NewOwnerRole(company.ID())
	if err := owner.SetPermissions(vo.NewPermissionSet()); !errors.Is(err, domain.ErrSystemRoleImmutable) {
		t.Fatalf("got %v", err)
	}
	if err := owner.EnsureDeletable(); !errors.Is(err, domain.ErrSystemRoleImmutable) {
		t.Fatalf("got %v", err)
	}
}
