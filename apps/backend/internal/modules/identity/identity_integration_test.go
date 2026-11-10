package identity_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/identity/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ตาราง identity_* ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"gorm.io/gorm"

	"account/backend/internal/modules/identity"
	"account/backend/internal/platform/database"
)

type api struct {
	t   *testing.T
	srv *httptest.Server
	db  *gorm.DB
}

type resp struct {
	status int
	body   map[string]any
}

func (r resp) data() map[string]any { return r.body["data"].(map[string]any) }
func (r resp) list() []any          { return r.body["data"].([]any) }

func (a api) do(method, path, token string, body any) resp {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, a.srv.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.srv.Client().Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{status: res.StatusCode, body: map[string]any{}}
	_ = json.NewDecoder(res.Body).Decode(&out.body)
	return out
}

func (a api) expect(r resp, status int, what string) resp {
	a.t.Helper()
	if r.status != status {
		a.t.Fatalf("%s: status = %d, want %d, body = %v", what, r.status, status, r.body)
	}
	return r
}

func (a api) register(email, name string) {
	a.t.Helper()
	a.expect(a.do("POST", "/api/v1/auth/register", "", map[string]any{"email": email, "name": name, "password": "secret-pass-1"}), 201, "register "+email)
}

func (a api) login(email string) string {
	a.t.Helper()
	r := a.expect(a.do("POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": "secret-pass-1"}), 200, "login "+email)
	return r.data()["access_token"].(string)
}

func setup(t *testing.T) api {
	dsn := os.Getenv("IDENTITY_TEST_DSN")
	if dsn == "" {
		t.Skip("IDENTITY_TEST_DSN not set")
	}
	db, err := database.OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []string{"identity_membership_roles", "identity_memberships", "identity_role_permissions", "identity_roles", "identity_companies", "identity_users"} {
		if err := db.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := identity.Migrate(db); err != nil {
		t.Fatal(err)
	}
	mod, err := identity.New(db, identity.Config{JWTSecret: []byte("test-secret-test-secret-test-secret!"), JWTIssuer: "test", BcryptCost: 4})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mod.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv, db: db}
}

func TestMultiCompanyAccessFlow(t *testing.T) {
	a := setup(t)

	a.register("owner@example.com", "Owner")
	a.register("staff@example.com", "Staff")
	a.register("hr@example.com", "HR Admin")
	a.expect(a.do("POST", "/api/v1/auth/register", "", map[string]any{"email": "OWNER@example.com", "name": "Dup", "password": "secret-pass-1"}), 409, "duplicate email")
	a.expect(a.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "owner@example.com", "password": "wrong-password"}), 401, "wrong password")
	a.expect(a.do("GET", "/api/v1/me", "", nil), 401, "no token")

	owner := a.login("owner@example.com")
	staff := a.login("staff@example.com")
	hr := a.login("hr@example.com")

	// owner สร้าง 2 บริษัท
	compA := a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "acme", "name": "ACME Co., Ltd.", "tax_id": "0105556000000"}), 201, "create A").data()["id"].(string)
	compB := a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "BETA", "name": "Beta Co., Ltd."}), 201, "create B").data()["id"].(string)
	a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "ACME", "name": "Dup"}), 409, "duplicate company code")

	if n := len(a.expect(a.do("GET", "/api/v1/me/companies", owner, nil), 200, "owner companies").list()); n != 2 {
		t.Fatalf("owner should see 2 companies, got %d", n)
	}
	if n := len(a.expect(a.do("GET", "/api/v1/me/companies", staff, nil), 200, "staff companies").list()); n != 0 {
		t.Fatalf("staff should see 0 companies, got %d", n)
	}

	base := "/api/v1/companies/" + compA
	salesRole := a.expect(a.do("POST", base+"/roles", owner, map[string]any{
		"code": "sales", "name": "Sales", "permissions": []string{"sales.order:*", "identity.company:read"},
	}), 201, "create sales role").data()["id"].(string)
	hrRole := a.expect(a.do("POST", base+"/roles", owner, map[string]any{
		"code": "hr-admin", "name": "HR Admin",
		"permissions": []string{"identity.member:read", "identity.member:manage", "identity.role:read", "sales.order:*"},
	}), 201, "create hr role").data()["id"].(string)
	a.expect(a.do("POST", base+"/roles", owner, map[string]any{"code": "bad", "name": "Bad", "permissions": []string{"not a perm"}}), 400, "invalid permission")

	// เพิ่มพนักงาน
	a.expect(a.do("POST", base+"/members", owner, map[string]any{"email": "staff@example.com", "role_ids": []string{salesRole}}), 201, "add staff")
	a.expect(a.do("POST", base+"/members", owner, map[string]any{"email": "hr@example.com", "role_ids": []string{hrRole}}), 201, "add hr")
	a.expect(a.do("POST", base+"/members", owner, map[string]any{"email": "staff@example.com", "role_ids": []string{salesRole}}), 409, "add staff twice")

	// role ของบริษัท A ใช้ในบริษัท B ไม่ได้ (tenant isolation)
	a.expect(a.do("POST", "/api/v1/companies/"+compB+"/members", owner, map[string]any{"email": "staff@example.com", "role_ids": []string{salesRole}}), 404, "role from other company")

	// staff เห็นสิทธิ์ของตัวเองใน A, เข้า B ไม่ได้, ดูรายชื่อพนักงานไม่ได้
	acc := a.expect(a.do("GET", base+"/access", staff, nil), 200, "staff access").data()
	perms := toStrings(acc["permissions"].([]any))
	if !slices.Equal(perms, []string{"identity.company:read", "sales.order:*"}) {
		t.Fatalf("staff permissions = %v", perms)
	}
	a.expect(a.do("GET", base, staff, nil), 200, "staff read company A")
	a.expect(a.do("GET", "/api/v1/companies/"+compB, staff, nil), 403, "staff read company B")
	a.expect(a.do("GET", base+"/members", staff, nil), 403, "staff list members")

	// HR: จัดการพนักงานได้ แต่ให้สิทธิ์เกินตัวเองไม่ได้ และแตะ owner ไม่ได้
	if n := len(a.expect(a.do("GET", base+"/members", hr, nil), 200, "hr list members").list()); n != 3 {
		t.Fatalf("members = %d, want 3", n)
	}
	a.expect(a.do("POST", base+"/roles", hr, map[string]any{"code": "x", "name": "X", "permissions": []string{"*"}}), 403, "hr create role (no role:manage)")
	ownerID := a.expect(a.do("GET", "/api/v1/me", owner, nil), 200, "owner me").data()["id"].(string)
	staffID := a.expect(a.do("GET", "/api/v1/me", staff, nil), 200, "staff me").data()["id"].(string)
	hrID := a.expect(a.do("GET", "/api/v1/me", hr, nil), 200, "hr me").data()["id"].(string)
	a.expect(a.do("POST", base+"/members/"+ownerID+"/suspend", hr, nil), 403, "hr suspend owner")
	a.expect(a.do("POST", base+"/members/"+hrID+"/suspend", hr, nil), 422, "hr suspend self")
	// staff ถือ identity.company:read ซึ่ง hr ไม่มี -> hr จัดการ staff ไม่ได้
	a.expect(a.do("PUT", base+"/members/"+staffID+"/roles", hr, map[string]any{"role_ids": []string{hrRole}}), 403, "hr manage member with more perms")
	a.expect(a.do("PUT", base+"/roles/"+hrRole, owner, map[string]any{
		"name":        "HR Admin",
		"permissions": []string{"identity.company:read", "identity.member:read", "identity.member:manage", "identity.role:read", "sales.order:*"},
	}), 200, "owner widen hr role")
	a.expect(a.do("PUT", base+"/members/"+staffID+"/roles", hr, map[string]any{"role_ids": []string{hrRole}}), 200, "hr promote staff to hr")

	// ระงับ staff แล้วเข้าไม่ได้ทันที (token เดิมยังใช้ได้ แต่สิทธิ์ resolve จาก DB)
	a.expect(a.do("POST", base+"/members/"+staffID+"/suspend", hr, nil), 204, "suspend staff")
	a.expect(a.do("GET", base+"/access", staff, nil), 403, "suspended staff access")
	a.expect(a.do("POST", base+"/members/"+staffID+"/activate", hr, nil), 204, "activate staff")
	a.expect(a.do("GET", base+"/access", staff, nil), 200, "reactivated staff access")

	// role
	a.expect(a.do("DELETE", base+"/roles/"+hrRole, owner, nil), 409, "delete role in use")
	roles := a.expect(a.do("GET", base+"/roles", owner, nil), 200, "list roles").list()
	ownerRole := roles[0].(map[string]any)
	if ownerRole["code"] != "owner" || ownerRole["is_system"] != true {
		t.Fatalf("first role should be system owner, got %v", ownerRole)
	}
	a.expect(a.do("PUT", base+"/roles/"+ownerRole["id"].(string), owner, map[string]any{"name": "Boss", "permissions": []string{}}), 422, "edit owner role")
	upd := a.expect(a.do("PUT", base+"/roles/"+salesRole, owner, map[string]any{"name": "Sales Team", "permissions": []string{"sales.order:read"}}), 200, "update sales role").data()
	if got := toStrings(upd["permissions"].([]any)); !slices.Equal(got, []string{"sales.order:read"}) {
		t.Fatalf("updated permissions = %v", got)
	}
	a.expect(a.do("DELETE", base+"/roles/"+salesRole, owner, nil), 204, "delete unused sales role")

	// บริษัท — timestamp ถูกจัดการโดย gorm (domain ไม่รู้จัก created_at/updated_at)
	before := a.companyTimestamps(compA)
	time.Sleep(20 * time.Millisecond)
	a.expect(a.do("PUT", base, owner, map[string]any{"name": "ACME Thailand", "tax_id": ""}), 200, "owner update company")
	after := a.companyTimestamps(compA)
	if before.CreatedAt.IsZero() || !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at should be set once and never change: before=%v after=%v", before.CreatedAt, after.CreatedAt)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("updated_at should move forward on update: before=%v after=%v", before.UpdatedAt, after.UpdatedAt)
	}
	a.expect(a.do("PUT", base, hr, map[string]any{"name": "Hacked"}), 403, "hr update company")
	a.expect(a.do("GET", "/api/v1/companies/not-a-uuid", owner, nil), 400, "bad company id")
}

type timestamps struct{ CreatedAt, UpdatedAt time.Time }

func (a api) companyTimestamps(id string) timestamps {
	a.t.Helper()
	var ts timestamps
	if err := a.db.Raw("SELECT created_at, updated_at FROM identity_companies WHERE id = ?", id).Scan(&ts).Error; err != nil {
		a.t.Fatal(err)
	}
	return ts
}

func toStrings(in []any) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = v.(string)
	}
	return out
}
