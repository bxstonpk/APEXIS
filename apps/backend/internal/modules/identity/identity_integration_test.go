package identity_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/identity/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ตาราง identity_* ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/platform/database"
)

type api struct {
	t   *testing.T
	srv *httptest.Server
	db  *gorm.DB
	mod *identity.Module
}

type resp struct {
	status int
	body   map[string]any
}

func (r resp) data() map[string]any { return r.body["data"].(map[string]any) }
func (r resp) list() []any          { return r.body["data"].([]any) }
func (r resp) total() int           { return int(r.body["meta"].(map[string]any)["total"].(float64)) }

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
	defer func() { _ = res.Body.Close() }()
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
	a.expect(a.do("POST", "/api/v1/users", "", map[string]any{"email": email, "name": name, "password": "secret-pass-1"}), 201, "register "+email)
}

func (a api) login(email string) string {
	a.t.Helper()
	r := a.expect(a.do("POST", "/api/v1/auth/tokens", "", map[string]any{"email": email, "password": "secret-pass-1"}), 201, "login "+email)
	return r.data()["access_token"].(string)
}

// createCompany: ตัวบริษัทอยู่ที่ organization module — test ของ identity จำลองการสร้างบริษัท
// ด้วย id ใหม่ แล้วเรียก ProvisionCompanyOwner แบบเดียวกับที่ organization เรียก
func (a api) createCompany(token string) string {
	a.t.Helper()
	me := a.expect(a.do("GET", "/api/v1/me", token, nil), 200, "me").data()["id"].(string)
	id := uuid.New()
	ok, err := a.mod.ProvisionCompanyOwner(context.Background(), id, uuid.MustParse(me))
	if err != nil || !ok {
		a.t.Fatalf("provision owner: ok=%v err=%v", ok, err)
	}
	return id.String()
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
	for _, tbl := range []string{"identity_membership_roles", "identity_memberships", "identity_role_permissions", "identity_roles", "identity_users"} {
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
	return api{t: t, srv: srv, db: db, mod: mod}
}

func TestMultiCompanyAccessFlow(t *testing.T) {
	a := setup(t)

	a.register("owner@example.com", "Owner")
	a.register("staff@example.com", "Staff")
	a.register("hr@example.com", "HR Admin")
	a.expect(a.do("POST", "/api/v1/users", "", map[string]any{"email": "OWNER@example.com", "name": "Dup", "password": "secret-pass-1"}), 409, "duplicate email")
	a.expect(a.do("POST", "/api/v1/auth/tokens", "", map[string]any{"email": "owner@example.com", "password": "wrong-password"}), 401, "wrong password")
	a.expect(a.do("GET", "/api/v1/me", "", nil), 401, "no token")

	owner := a.login("owner@example.com")
	staff := a.login("staff@example.com")
	hr := a.login("hr@example.com")

	// owner สร้าง 2 บริษัท (ได้ role owner + เป็นสมาชิก)
	compA := a.createCompany(owner)
	compB := a.createCompany(owner)
	if ids, err := a.mod.MemberCompanyIDs(context.Background(), uuid.MustParse(a.expect(a.do("GET", "/api/v1/me", owner, nil), 200, "me").data()["id"].(string))); err != nil || len(ids) != 2 {
		t.Fatalf("owner companies = %v %v", ids, err)
	}
	a.expect(a.do("GET", "/api/v1/companies/"+compA+"/access", staff, nil), 403, "non-member access")

	base := "/api/v1/companies/" + compA
	salesRole := a.expect(a.do("POST", base+"/roles", owner, map[string]any{
		"code": "sales", "name": "Sales", "permissions": []string{"sales.order:*", "organization.company:read"},
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
	if !slices.Equal(perms, []string{"organization.company:read", "sales.order:*"}) {
		t.Fatalf("staff permissions = %v", perms)
	}
	a.expect(a.do("GET", "/api/v1/companies/"+compB+"/access", staff, nil), 403, "staff access company B")
	a.expect(a.do("GET", base+"/members", staff, nil), 403, "staff list members")

	// HR: จัดการพนักงานได้ แต่ให้สิทธิ์เกินตัวเองไม่ได้ และแตะ owner ไม่ได้
	if n := len(a.expect(a.do("GET", base+"/members", hr, nil), 200, "hr list members").list()); n != 3 {
		t.Fatalf("members = %d, want 3", n)
	}
	a.expect(a.do("POST", base+"/roles", hr, map[string]any{"code": "x", "name": "X", "permissions": []string{"*"}}), 403, "hr create role (no role:manage)")
	suspended := map[string]any{"status": "suspended"}
	ownerID := a.expect(a.do("GET", "/api/v1/me", owner, nil), 200, "owner me").data()["id"].(string)
	staffID := a.expect(a.do("GET", "/api/v1/me", staff, nil), 200, "staff me").data()["id"].(string)
	hrID := a.expect(a.do("GET", "/api/v1/me", hr, nil), 200, "hr me").data()["id"].(string)
	a.expect(a.do("PATCH", base+"/members/"+ownerID, hr, suspended), 403, "hr suspend owner")
	a.expect(a.do("PATCH", base+"/members/"+hrID, hr, suspended), 422, "hr suspend self")
	// staff ถือ organization.company:read ซึ่ง hr ไม่มี -> hr จัดการ staff ไม่ได้
	a.expect(a.do("PUT", base+"/members/"+staffID+"/roles", hr, map[string]any{"role_ids": []string{hrRole}}), 403, "hr manage member with more perms")
	a.expect(a.do("PUT", base+"/roles/"+hrRole, owner, map[string]any{
		"name":        "HR Admin",
		"permissions": []string{"organization.company:read", "identity.member:read", "identity.member:manage", "identity.role:read", "sales.order:*"},
	}), 200, "owner widen hr role")
	a.expect(a.do("PUT", base+"/members/"+staffID+"/roles", hr, map[string]any{"role_ids": []string{hrRole}}), 200, "hr promote staff to hr")

	// ระงับ staff แล้วเข้าไม่ได้ทันที (token เดิมยังใช้ได้ แต่สิทธิ์ resolve จาก DB)
	a.expect(a.do("PATCH", base+"/members/"+staffID, hr, map[string]any{"status": "deleted"}), 400, "invalid member status")
	a.expect(a.do("PATCH", base+"/members/"+staffID, hr, suspended), 200, "suspend staff")
	a.expect(a.do("GET", base+"/access", staff, nil), 403, "suspended staff access")
	a.expect(a.do("PATCH", base+"/members/"+staffID, hr, map[string]any{"status": "active"}), 200, "activate staff")
	a.expect(a.do("GET", base+"/access", staff, nil), 200, "reactivated staff access")

	// role
	a.expect(a.do("DELETE", base+"/roles/"+hrRole, owner, nil), 409, "delete role in use")
	roles := a.expect(a.do("GET", base+"/roles", owner, nil), 200, "list roles").list()
	ownerRole := roles[0].(map[string]any)
	if ownerRole["code"] != "owner" || ownerRole["is_system"] != true {
		t.Fatalf("first role should be system owner, got %v", ownerRole)
	}
	a.expect(a.do("PUT", base+"/roles/"+ownerRole["id"].(string), owner, map[string]any{"name": "Boss", "permissions": []string{}}), 422, "edit owner role")
	// timestamp ถูกจัดการโดย gorm (domain ไม่รู้จัก created_at/updated_at)
	before := a.roleTimestamps(salesRole)
	time.Sleep(20 * time.Millisecond)
	upd := a.expect(a.do("PUT", base+"/roles/"+salesRole, owner, map[string]any{"name": "Sales Team", "permissions": []string{"sales.order:read"}}), 200, "update sales role").data()
	after := a.roleTimestamps(salesRole)
	if before.CreatedAt.IsZero() || !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at should be set once and never change: before=%v after=%v", before.CreatedAt, after.CreatedAt)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("updated_at should move forward on update: before=%v after=%v", before.UpdatedAt, after.UpdatedAt)
	}
	if got := toStrings(upd["permissions"].([]any)); !slices.Equal(got, []string{"sales.order:read"}) {
		t.Fatalf("updated permissions = %v", got)
	}
	a.expect(a.do("DELETE", base+"/roles/"+salesRole, owner, nil), 204, "delete unused sales role")
	a.expect(a.do("GET", "/api/v1/companies/not-a-uuid/access", owner, nil), 400, "bad company id")
}

type timestamps struct{ CreatedAt, UpdatedAt time.Time }

func (a api) roleTimestamps(id string) timestamps {
	a.t.Helper()
	var ts timestamps
	if err := a.db.Raw("SELECT created_at, updated_at FROM identity_roles WHERE id = ?", id).Scan(&ts).Error; err != nil {
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

// TestListSearchViaURLQuery: การค้นหา/กรอง/เรียง/แบ่งหน้าผ่าน URL query
// frontend ส่ง query ของหน้าเดิมมาตรงๆ ต้องได้ผลเหมือนเดิมทุกครั้ง (กด Back แล้วได้หน้าการค้นหาเดิม)
func TestListSearchViaURLQuery(t *testing.T) {
	a := setup(t)
	a.register("owner@example.com", "Owner")
	for _, n := range []string{"Somchai", "Somsak", "Malee", "Niran_x"} {
		a.register(strings.ToLower(n)+"@example.com", n)
	}
	owner := a.login("owner@example.com")
	compA := a.createCompany(owner)
	base := "/api/v1/companies/" + compA
	sales := a.expect(a.do("POST", base+"/roles", owner, map[string]any{"code": "sales", "name": "Sales", "permissions": []string{"sales.order:*"}}), 201, "role").data()["id"].(string)
	for _, n := range []string{"somchai", "somsak", "malee", "niran_x"} {
		a.expect(a.do("POST", base+"/members", owner, map[string]any{"email": n + "@example.com", "role_ids": []string{sales}}), 201, "add "+n)
	}
	maleeID := ""
	for _, m := range a.expect(a.do("GET", base+"/members?q=malee", owner, nil), 200, "find malee").list() {
		maleeID = m.(map[string]any)["user"].(map[string]any)["id"].(string)
	}
	a.expect(a.do("PATCH", base+"/members/"+maleeID, owner, map[string]any{"status": "suspended"}), 200, "suspend malee")

	names := func(r resp) []string {
		var out []string
		for _, m := range r.list() {
			out = append(out, m.(map[string]any)["user"].(map[string]any)["name"].(string))
		}
		return out
	}

	// ค่าเริ่มต้น: เรียงตามชื่อ, หน้า 1, 20 ต่อหน้า
	r := a.expect(a.do("GET", base+"/members", owner, nil), 200, "list members")
	if got := names(r); !slices.Equal(got, []string{"Malee", "Niran_x", "Owner", "Somchai", "Somsak"}) || r.total() != 5 {
		t.Fatalf("default list = %v total=%d", got, r.total())
	}
	meta := r.body["meta"].(map[string]any)
	if meta["page"].(float64) != 1 || meta["page_size"].(float64) != 20 {
		t.Fatalf("meta = %v", meta)
	}

	// ค้นหาข้อความ (ไม่สนตัวพิมพ์) + เรียงมากไปน้อย
	if got := names(a.expect(a.do("GET", base+"/members?q=SOM&sort=-name", owner, nil), 200, "search som")); !slices.Equal(got, []string{"Somsak", "Somchai"}) {
		t.Fatalf("q=SOM = %v", got)
	}
	// "_" ต้องถูกค้นแบบตรงตัว ไม่ใช่ wildcard
	if got := names(a.expect(a.do("GET", base+"/members?q=n_x", owner, nil), 200, "search underscore")); !slices.Equal(got, []string{"Niran_x"}) {
		t.Fatalf("q=n_x = %v", got)
	}
	// filter หลายค่า + filter ตาม role
	if got := names(a.expect(a.do("GET", base+"/members?status=suspended", owner, nil), 200, "status")); !slices.Equal(got, []string{"Malee"}) {
		t.Fatalf("status=suspended = %v", got)
	}
	if r := a.expect(a.do("GET", base+"/members?status=active,suspended&role_id="+sales, owner, nil), 200, "role filter"); r.total() != 4 {
		t.Fatalf("role_id filter total = %d, want 4", r.total())
	}

	// แบ่งหน้าใน DB: total คงที่, หน้าไม่ซ้ำกัน, หน้าเกินข้อมูลยังได้ total ถูก
	p1 := a.expect(a.do("GET", base+"/members?page_size=2&page=1", owner, nil), 200, "page 1")
	p3 := a.expect(a.do("GET", base+"/members?page_size=2&page=3", owner, nil), 200, "page 3")
	p9 := a.expect(a.do("GET", base+"/members?page_size=2&page=9", owner, nil), 200, "page 9")
	if !slices.Equal(names(p1), []string{"Malee", "Niran_x"}) || !slices.Equal(names(p3), []string{"Somsak"}) {
		t.Fatalf("pages = %v / %v", names(p1), names(p3))
	}
	if len(p9.list()) != 0 || p9.total() != 5 || p1.total() != 5 {
		t.Fatalf("page 9 = %v total=%d", p9.list(), p9.total())
	}

	// query parameter ที่ผิด -> 400, ที่ไม่รู้จัก -> เพิกเฉย
	for _, bad := range []string{"page=0", "page_size=101", "sort=password_hash", "status=deleted", "role_id=x"} {
		a.expect(a.do("GET", base+"/members?"+bad, owner, nil), 400, "bad query "+bad)
	}
	a.expect(a.do("GET", base+"/members?utm_source=mail", owner, nil), 200, "unknown param ignored")

	// roles ใช้รูปแบบเดียวกัน (me/companies อยู่ที่ organization module)
	if r := a.expect(a.do("GET", base+"/roles?is_system=false", owner, nil), 200, "roles"); r.total() != 1 {
		t.Fatalf("non-system roles = %d", r.total())
	}
	if r := a.expect(a.do("GET", base+"/roles?q=own", owner, nil), 200, "roles q"); r.total() != 1 || r.list()[0].(map[string]any)["code"] != "owner" {
		t.Fatalf("roles q=own = %v", r.list())
	}
}
