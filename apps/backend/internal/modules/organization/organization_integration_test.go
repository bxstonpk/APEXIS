package organization_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (identity + organization)
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/organization/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ตาราง identity_* / organization_* ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"gorm.io/gorm"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/platform/database"
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
func (r resp) id() string           { return r.data()["id"].(string) }
func (r resp) list() []map[string]any {
	var out []map[string]any
	for _, v := range r.body["data"].([]any) {
		out = append(out, v.(map[string]any))
	}
	return out
}
func (r resp) total() int { return int(r.body["meta"].(map[string]any)["total"].(float64)) }
func (r resp) codes() []string {
	var out []string
	for _, v := range r.list() {
		out = append(out, v["code"].(string))
	}
	return out
}

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

func (a api) user(email string) string {
	a.t.Helper()
	a.expect(a.do("POST", "/api/v1/users", "", map[string]any{"email": email, "name": email, "password": "secret-pass-1"}), 201, "register "+email)
	r := a.expect(a.do("POST", "/api/v1/auth/tokens", "", map[string]any{"email": email, "password": "secret-pass-1"}), 201, "login "+email)
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
	for _, tbl := range []string{
		"organization_fiscal_periods", "organization_fiscal_years", "organization_project_sites", "organization_cost_centers",
		"organization_departments", "organization_business_units", "organization_branches", "organization_document_numberings", "organization_companies",
		"identity_membership_roles", "identity_memberships", "identity_role_permissions", "identity_roles", "identity_users",
		"organization_company_profiles", "identity_companies", // ตารางของ schema เดิม (ก่อนย้ายบริษัทมา organization)
	} {
		if err := db.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := identity.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := organization.Migrate(db); err != nil {
		t.Fatal(err)
	}
	idm, err := identity.New(db, identity.Config{JWTSecret: []byte("test-secret-test-secret-test-secret!"), JWTIssuer: "test", BcryptCost: 4})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	idm.RegisterRoutes(mux)
	organization.New(db, idm).RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv, db: db}
}

func TestOrganizationFlow(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	outsider := a.user("outsider@example.com")

	// ---------- company: สร้างแล้วผู้สร้างเป็น owner (identity อยู่ใน transaction เดียวกัน) ----------
	ca := a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "acme", "name": "ACME", "tax_id": "0105556000000"}), 201, "company A")
	if d := ca.data(); d["code"] != "ACME" || d["status"] != "active" || d["base_currency"] != "THB" ||
		d["fiscal_year_start_month"] != float64(1) || d["legal_name_th"] != "ACME" ||
		d["amount_decimals"] != float64(2) || d["quantity_decimals"] != float64(2) {
		t.Fatalf("company A defaults = %v", d)
	}
	compA := ca.id()
	compB := a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "beta", "name": "Beta"}), 201, "company B").id()
	a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "ACME", "name": "dup"}), 409, "duplicate company code")
	a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"name": "no code"}), 400, "company without code")
	a.expect(a.do("POST", "/api/v1/companies", "", map[string]any{"code": "X", "name": "x"}), 401, "create company without token")

	// identity ล้มเหลว (ผู้ใช้ถูกปิด) -> บริษัทที่ insert ไปแล้วต้องถูก rollback ด้วย (transaction เดียวกันข้าม module)
	disabled := a.user("disabled@example.com")
	if err := a.db.Exec("UPDATE identity_users SET status = 'disabled' WHERE email = ?", "disabled@example.com").Error; err != nil {
		t.Fatal(err)
	}
	a.expect(a.do("POST", "/api/v1/companies", disabled, map[string]any{"code": "GHOST", "name": "ghost"}), 403, "disabled user creates company")
	var ghosts int64
	if err := a.db.Table("organization_companies").Where("code = ?", "GHOST").Count(&ghosts).Error; err != nil || ghosts != 0 {
		t.Fatalf("company must be rolled back: count=%d err=%v", ghosts, err)
	}
	base := "/api/v1/companies/" + compA

	company := map[string]any{
		"code": "CHANGED", "name": "ACME", "legal_name_th": "บริษัท แอคมี จำกัด", "tax_id": "0105556000000",
		"base_currency": "thb", "fiscal_year_start_month": 10, "status": "active",
		"amount_decimals": 2, "quantity_decimals": 3,
		"address": map[string]any{"line": "99 ถ.สุขุมวิท", "province": "กรุงเทพมหานคร", "postal_code": "10110"},
	}
	c := a.expect(a.do("PUT", base, owner, company), 200, "update company").data()
	if c["code"] != "ACME" || c["base_currency"] != "THB" || c["fiscal_year_start_month"] != float64(10) ||
		c["address"].(map[string]any)["country"] != "TH" {
		t.Fatalf("company = %v", c)
	}
	if n := a.expect(a.do("GET", base, owner, nil), 200, "get company").data()["legal_name_th"]; n != "บริษัท แอคมี จำกัด" {
		t.Fatalf("legal name = %v", n)
	}
	company["fiscal_year_start_month"] = 13
	a.expect(a.do("PUT", base, owner, company), 400, "invalid month")
	company["fiscal_year_start_month"] = 10
	delete(company, "status")
	a.expect(a.do("PUT", base, owner, company), 400, "PUT company without status")
	company["status"] = "active"
	company["quantity_decimals"] = 2
	a.expect(a.do("PUT", base, owner, company), 422, "decrease quantity decimals")
	company["quantity_decimals"] = 7
	a.expect(a.do("PUT", base, owner, company), 400, "7 decimals")
	delete(company, "quantity_decimals")
	a.expect(a.do("PUT", base, owner, company), 400, "PUT company without quantity_decimals")
	company["quantity_decimals"] = 3

	// บริษัทของฉัน: id จาก identity + กรอง / เรียง / แบ่งหน้าใน organization
	if got := a.expect(a.do("GET", "/api/v1/me/companies?sort=-code", owner, nil), 200, "my companies").codes(); !slices.Equal(got, []string{"BETA", "ACME"}) {
		t.Fatalf("my companies = %v", got)
	}
	if got := a.expect(a.do("GET", "/api/v1/me/companies?q=แอคมี", owner, nil), 200, "my companies q").codes(); !slices.Equal(got, []string{"ACME"}) {
		t.Fatalf("my companies q = %v", got)
	}
	if r := a.expect(a.do("GET", "/api/v1/me/companies", outsider, nil), 200, "outsider companies"); r.total() != 0 || len(r.list()) != 0 {
		t.Fatalf("outsider companies = %v", r.body)
	}
	a.expect(a.do("GET", "/api/v1/me/companies?sort=tax_id", owner, nil), 400, "bad company sort")
	if got := a.expect(a.do("GET", "/api/v1/me/companies/"+compA, owner, nil), 200, "my company").data(); got["code"] != "ACME" || got["id"] != compA {
		t.Fatalf("my company = %v", got)
	}
	a.expect(a.do("GET", "/api/v1/me/companies/"+compA, outsider, nil), 404, "outsider my company")
	a.expect(a.do("GET", "/api/v1/me/companies/not-a-uuid", owner, nil), 400, "my company bad id")

	// ---------- branches ----------
	hq := a.expect(a.do("POST", base+"/branches", owner, map[string]any{"code": "hq", "name": "สำนักงานใหญ่", "tax_branch_no": "00000"}), 201, "create HQ")
	if hq.data()["is_head_office"] != true || hq.data()["code"] != "HQ" || hq.data()["status"] != "active" {
		t.Fatalf("hq = %v", hq.data())
	}
	b1 := a.expect(a.do("POST", base+"/branches", owner, map[string]any{"code": "B1", "name": "สาขาบางนา", "tax_branch_no": "00001"}), 201, "create B1").id()
	a.expect(a.do("POST", base+"/branches", owner, map[string]any{"code": "B1", "name": "dup", "tax_branch_no": "00009"}), 409, "duplicate branch code")
	a.expect(a.do("POST", base+"/branches", owner, map[string]any{"code": "HQ2", "name": "second HQ", "tax_branch_no": "00000"}), 409, "second head office")
	a.expect(a.do("POST", base+"/branches", owner, map[string]any{"code": "B9", "name": "x", "tax_branch_no": "1"}), 400, "bad tax branch no")
	a.expect(a.do("DELETE", base+"/branches/"+hq.id(), owner, nil), 422, "delete HQ")
	a.expect(a.do("PUT", base+"/branches/"+hq.id(), owner, map[string]any{"code": "HQ", "name": "HQ", "tax_branch_no": "00000", "status": "inactive"}), 422, "deactivate HQ")
	a.expect(a.do("PUT", base+"/branches/"+hq.id(), owner, map[string]any{"code": "HQ", "name": "HQ"}), 400, "PUT without status/tax no")
	if got := a.expect(a.do("GET", base+"/branches?sort=-code", owner, nil), 200, "list branches").codes(); !slices.Equal(got, []string{"HQ", "B1"}) {
		t.Fatalf("branches sort=-code = %v", got)
	}

	// ---------- business units (ลำดับชั้น) ----------
	buA := a.expect(a.do("POST", base+"/business-units", owner, map[string]any{"code": "RETAIL", "name": "ค้าปลีก"}), 201, "BU A").id()
	buB := a.expect(a.do("POST", base+"/business-units", owner, map[string]any{"code": "RETAIL-N", "name": "ค้าปลีกภาคเหนือ", "parent_id": buA}), 201, "BU B")
	if buB.data()["parent"].(map[string]any)["code"] != "RETAIL" {
		t.Fatalf("BU B parent = %v", buB.data()["parent"])
	}
	a.expect(a.do("PUT", base+"/business-units/"+buA, owner, map[string]any{"code": "RETAIL", "name": "ค้าปลีก", "parent_id": buB.id(), "status": "active"}), 422, "BU cycle")
	a.expect(a.do("POST", base+"/business-units", owner, map[string]any{"code": "X", "name": "x", "parent_id": "0195f0a0-0000-7000-8000-000000000000"}), 404, "unknown parent")

	// ---------- departments ----------
	d1 := a.expect(a.do("POST", base+"/departments", owner, map[string]any{"code": "SALES", "name": "ฝ่ายขาย", "business_unit_id": buA, "branch_id": b1}), 201, "dept 1")
	if d1.data()["branch"].(map[string]any)["name"] != "สาขาบางนา" || d1.data()["business_unit"].(map[string]any)["code"] != "RETAIL" {
		t.Fatalf("dept refs = %v", d1.data())
	}
	a.expect(a.do("POST", base+"/departments", owner, map[string]any{"code": "SALES-B2B", "name": "ขายองค์กร", "parent_id": d1.id()}), 201, "dept 2")
	if got := a.expect(a.do("GET", base+"/departments?parent_id="+d1.id(), owner, nil), 200, "children").codes(); !slices.Equal(got, []string{"SALES-B2B"}) {
		t.Fatalf("children = %v", got)
	}
	if r := a.expect(a.do("GET", base+"/departments?business_unit_id="+buA+"&q=ขาย", owner, nil), 200, "dept by BU"); r.total() != 1 {
		t.Fatalf("dept by BU = %v", r.codes())
	}
	a.expect(a.do("DELETE", base+"/departments/"+d1.id(), owner, nil), 409, "delete dept with child")
	a.expect(a.do("DELETE", base+"/branches/"+b1, owner, nil), 409, "delete branch used by dept")
	a.expect(a.do("DELETE", base+"/business-units/"+buA, owner, nil), 409, "delete BU with child")

	// ---------- cost centers (ช่วงวันที่) ----------
	cc1 := a.expect(a.do("POST", base+"/cost-centers", owner, map[string]any{"code": "CC-H1", "name": "ครึ่งปีแรก", "department_id": d1.id(), "valid_from": "2026-01-01", "valid_to": "2026-06-30"}), 201, "cc1")
	if cc1.data()["valid_to"] != "2026-06-30" || cc1.data()["department"].(map[string]any)["code"] != "SALES" {
		t.Fatalf("cc1 = %v", cc1.data())
	}
	a.expect(a.do("POST", base+"/cost-centers", owner, map[string]any{"code": "CC-H2", "name": "ครึ่งปีหลัง", "valid_from": "2026-07-01"}), 201, "cc2")
	a.expect(a.do("POST", base+"/cost-centers", owner, map[string]any{"code": "CC-X", "name": "x", "valid_from": "2026-07-01", "valid_to": "2026-01-01"}), 400, "cc bad range")
	a.expect(a.do("POST", base+"/cost-centers", owner, map[string]any{"code": "CC-Y", "name": "y", "valid_from": "01072026"}), 400, "cc DDMMYYYY")
	if got := a.expect(a.do("GET", base+"/cost-centers?active_on=2026-12-19", owner, nil), 200, "cc active_on").codes(); !slices.Equal(got, []string{"CC-H2"}) {
		t.Fatalf("active_on = %v", got)
	}
	a.expect(a.do("GET", base+"/cost-centers?active_on=19122026", owner, nil), 400, "active_on DDMMYYYY")

	// ---------- project sites (ช่วงวันที่ซ้อนทับ) ----------
	a.expect(a.do("POST", base+"/project-sites", owner, map[string]any{"code": "P1", "name": "ไซต์ A", "branch_id": b1, "cost_center_id": cc1.id(), "start_date": "2026-01-01", "end_date": "2026-03-31", "status": "closed"}), 201, "P1")
	p2 := a.expect(a.do("POST", base+"/project-sites", owner, map[string]any{"code": "P2", "name": "ไซต์ B", "start_date": "2026-12-01", "status": "active"}), 201, "P2").id()
	a.expect(a.do("POST", base+"/project-sites", owner, map[string]any{"code": "P3", "name": "x", "start_date": "2026-01-01", "status": "closed"}), 422, "closed without end date")
	if got := a.expect(a.do("GET", base+"/project-sites?start_date=2026-12-19&end_date=2026-12-31", owner, nil), 200, "sites in Dec").codes(); !slices.Equal(got, []string{"P2"}) {
		t.Fatalf("sites overlap = %v", got)
	}
	a.expect(a.do("GET", base+"/project-sites?start_date=2026-12-31&end_date=2026-12-01", owner, nil), 400, "reversed range")
	a.expect(a.do("DELETE", base+"/cost-centers/"+cc1.id(), owner, nil), 409, "delete cc used by site")
	// soft delete แล้วใช้รหัสเดิมซ้ำได้
	a.expect(a.do("DELETE", base+"/project-sites/"+p2, owner, nil), 204, "delete P2")
	a.expect(a.do("GET", base+"/project-sites/"+p2, owner, nil), 404, "deleted P2 hidden")
	a.expect(a.do("POST", base+"/project-sites", owner, map[string]any{"code": "P2", "name": "ไซต์ B ใหม่", "start_date": "2027-01-01"}), 201, "reuse code P2")

	// ---------- fiscal years / periods ----------
	fy := a.expect(a.do("POST", base+"/fiscal-years", owner, map[string]any{"year": 2026, "start_date": "2026-01-01"}), 201, "fiscal 2026").data()
	periods := fy["periods"].([]any)
	if len(periods) != 12 || fy["end_date"] != "2026-12-31" || periods[1].(map[string]any)["end_date"] != "2026-02-28" {
		t.Fatalf("fiscal year = %v", fy)
	}
	a.expect(a.do("POST", base+"/fiscal-years", owner, map[string]any{"year": 2027, "start_date": "2026-06-01"}), 409, "overlapping year")
	a.expect(a.do("POST", base+"/fiscal-years", owner, map[string]any{"year": 2026, "start_date": "2027-01-01"}), 409, "duplicate year")
	a.expect(a.do("POST", base+"/fiscal-years", owner, map[string]any{"year": 2027, "start_date": "2027-01-15"}), 400, "mid-month start")
	dec := a.expect(a.do("GET", base+"/fiscal-periods?fiscal_year=2026&date=2026-12-19", owner, nil), 200, "period of date").list()
	if len(dec) != 1 || dec[0]["period_no"] != float64(12) {
		t.Fatalf("period for 2026-12-19 = %v", dec)
	}
	jan := periods[0].(map[string]any)["id"].(string)
	feb := periods[1].(map[string]any)["id"].(string)
	a.expect(a.do("PATCH", base+"/fiscal-periods/"+feb, owner, map[string]any{"status": "closed"}), 422, "close Feb before Jan")
	if s := a.expect(a.do("PATCH", base+"/fiscal-periods/"+jan, owner, map[string]any{"status": "closed"}), 200, "close Jan").data()["status"]; s != "closed" {
		t.Fatalf("jan status = %v", s)
	}
	a.expect(a.do("PATCH", base+"/fiscal-periods/"+feb, owner, map[string]any{"status": "closed"}), 200, "close Feb")
	a.expect(a.do("PATCH", base+"/fiscal-periods/"+jan, owner, map[string]any{"status": "open"}), 422, "reopen Jan while Feb closed")
	if r := a.expect(a.do("GET", base+"/fiscal-periods?status=closed", owner, nil), 200, "closed periods"); r.total() != 2 {
		t.Fatalf("closed periods = %d", r.total())
	}

	// ---------- สิทธิ์ ----------
	role := a.expect(a.do("POST", base+"/roles", owner, map[string]any{"code": "viewer", "name": "Viewer", "permissions": []string{"organization.branch:read"}}), 201, "viewer role").id()
	a.expect(a.do("POST", base+"/members", owner, map[string]any{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")
	a.expect(a.do("GET", base+"/branches", staff, nil), 200, "staff read branches")
	a.expect(a.do("POST", base+"/branches", staff, map[string]any{"code": "B2", "name": "x", "tax_branch_no": "00002"}), 403, "staff create branch")
	a.expect(a.do("GET", base+"/departments", staff, nil), 403, "staff read departments")
	a.expect(a.do("PUT", base, staff, company), 403, "staff update company")
	if got := a.expect(a.do("GET", "/api/v1/me/companies", staff, nil), 200, "staff companies").codes(); !slices.Equal(got, []string{"ACME"}) {
		t.Fatalf("staff companies = %v", got)
	}
	a.expect(a.do("GET", base, outsider, nil), 403, "outsider read company")
	a.expect(a.do("GET", base+"/branches", outsider, nil), 403, "outsider")
	a.expect(a.do("GET", base+"/branches", "", nil), 401, "no token")

	// ---------- tenant isolation ----------
	baseB := "/api/v1/companies/" + compB
	a.expect(a.do("GET", baseB+"/branches/"+b1, owner, nil), 404, "branch of A via B")
	a.expect(a.do("POST", baseB+"/departments", owner, map[string]any{"code": "D", "name": "d", "branch_id": b1}), 404, "reference branch of A from B")
	if r := a.expect(a.do("GET", baseB+"/branches", owner, nil), 200, "B branches"); r.total() != 0 {
		t.Fatalf("company B sees %d branches", r.total())
	}

	// ---------- บริษัท inactive = อ่านอย่างเดียว ----------
	companyB := map[string]any{"name": "Beta", "base_currency": "THB", "fiscal_year_start_month": 1, "amount_decimals": 2, "quantity_decimals": 2, "status": "inactive"}
	a.expect(a.do("PUT", baseB, owner, companyB), 200, "deactivate B")
	a.expect(a.do("POST", baseB+"/branches", owner, map[string]any{"code": "HQ", "name": "HQ", "tax_branch_no": "00000"}), 422, "write to inactive company")
	a.expect(a.do("POST", baseB+"/fiscal-years", owner, map[string]any{"year": 2026, "start_date": "2026-01-01"}), 422, "fiscal year in inactive company")
	a.expect(a.do("GET", baseB+"/branches", owner, nil), 200, "read inactive company")
	if got := a.expect(a.do("GET", "/api/v1/me/companies?status=inactive", owner, nil), 200, "inactive companies").codes(); !slices.Equal(got, []string{"BETA"}) {
		t.Fatalf("inactive companies = %v", got)
	}
	companyB["status"] = "active"
	a.expect(a.do("PUT", baseB, owner, companyB), 200, "reactivate B")
	a.expect(a.do("POST", baseB+"/branches", owner, map[string]any{"code": "HQ", "name": "HQ", "tax_branch_no": "00000"}), 201, "write after reactivate")

	// ---------- query ผิดรูปแบบ ----------
	for _, bad := range []string{"branches?page=0", "branches?sort=password", "branches?status=deleted", "departments?branch_id=x", "fiscal-periods?fiscal_year=abc"} {
		a.expect(a.do("GET", fmt.Sprintf("%s/%s", base, bad), owner, nil), 400, "bad query "+bad)
	}
}
