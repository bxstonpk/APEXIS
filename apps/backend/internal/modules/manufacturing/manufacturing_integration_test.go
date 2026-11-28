package manufacturing_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (identity + organization + ledger + inventory + manufacturing)
// รันด้วย: IDENTITY_TEST_DSN="host=... dbname=apexis_test_mfg ..." go test ./internal/modules/manufacturing/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ทุกตารางใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"gorm.io/gorm"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/inventory"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/manufacturing"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/reports"
)

type obj = map[string]any

type api struct {
	t *testing.T
	s *httptest.Server
}

type resp struct {
	status int
	body   obj
}

func (r resp) data() obj  { return r.body["data"].(obj) }
func (r resp) id() string { return r.data()["id"].(string) }

func objs(v any) []obj {
	var out []obj
	for _, x := range v.([]any) {
		out = append(out, x.(obj))
	}
	return out
}

func (a api) do(method, path, token string, body any) resp {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, a.s.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.s.Client().Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out := resp{status: res.StatusCode, body: obj{}}
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
	a.expect(a.do("POST", "/api/v1/users", "", obj{"email": email, "name": email, "password": "secret-pass-1"}), 201, "register "+email)
	r := a.expect(a.do("POST", "/api/v1/auth/tokens", "", obj{"email": email, "password": "secret-pass-1"}), 201, "login "+email)
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
	var tables []string
	if err := db.Raw(`SELECT tablename FROM pg_tables WHERE schemaname = 'public'`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	for _, tbl := range tables {
		if err := db.Exec(`DROP TABLE IF EXISTS "` + tbl + `" CASCADE`).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, migrate := range []func(*gorm.DB) error{identity.Migrate, organization.Migrate, ledger.Migrate, inventory.Migrate, manufacturing.Migrate} {
		if err := migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	idm, err := identity.New(db, identity.Config{JWTSecret: []byte("test-secret-test-secret-test-secret!"), JWTIssuer: "test", BcryptCost: 4})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	idm.RegisterRoutes(mux)
	org := organization.New(db, idm)
	org.RegisterRoutes(mux)
	led := ledger.New(db, idm, org)
	led.RegisterRoutes(mux)
	inv := inventory.New(db, idm, org, led)
	inv.RegisterRoutes(mux)
	mfg := manufacturing.New(db, idm, org, led, inv)
	mfg.RegisterRoutes(mux)
	org.OnCompanyCreated(led.ProvisionCompany)
	org.OnCompanyCreated(inv.ProvisionCompany)
	org.OnCompanyCreated(mfg.ProvisionCompany)
	reg := reports.NewRegistry()
	for _, r := range []interface{ RegisterReports(*reports.Registry) }{led, inv, mfg} {
		r.RegisterReports(reg)
	}
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, s: srv}
}

// company: บริษัทผลิตพร้อมผังบัญชีมาตรฐาน + ปีบัญชี 2026, คลัง, หมวดวัตถุดิบ / สินค้าสำเร็จรูป และสินค้า FG / A / B
type company struct {
	base                string
	wh, fg, a, b        string
	accounts            map[string]string
	rawAcc, fgAcc, wip  string
	conversion, ohdAcct string
}

func (a api) company(owner, code string, perpetual bool) company {
	a.t.Helper()
	id := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": code, "name": code, "business_type": "manufacturing"}), 201, "company "+code).id()
	c := company{base: "/api/v1/companies/" + id, accounts: map[string]string{}}
	a.expect(a.do("POST", c.base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	if perpetual {
		a.expect(a.do("PUT", c.base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 200, "perpetual 2026")
	}
	rows := objs(a.expect(a.do("GET", c.base+"/accounts?page_size=100&q=1105", owner, nil), 200, "accounts 1105").body["data"])
	rows = append(rows, objs(a.expect(a.do("GET", c.base+"/accounts?page_size=100&q=506", owner, nil), 200, "accounts 506").body["data"])...)
	for _, r := range rows {
		c.accounts[r["code"].(string)] = r["id"].(string)
	}
	c.rawAcc, c.fgAcc, c.wip = c.accounts["1105060000"], c.accounts["1105040000"], c.accounts["1105050000"]
	c.conversion, c.ohdAcct = c.accounts["5065000000"], c.accounts["5063060000"]
	pcs := a.expect(a.do("POST", c.base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	raw := a.expect(a.do("POST", c.base+"/item-categories", owner, obj{"code": "RAW", "name": "วัตถุดิบ", "inventory_account_id": c.rawAcc}), 201, "raw cat").id()
	fgCat := a.expect(a.do("POST", c.base+"/item-categories", owner, obj{"code": "FG", "name": "สินค้าสำเร็จรูป", "inventory_account_id": c.fgAcc}), 201, "fg cat").id()
	item := func(code, cat string) string {
		return a.expect(a.do("POST", c.base+"/items", owner, obj{"code": code, "name": "สินค้า " + code, "uom_id": pcs, "category_id": cat}), 201, "item "+code).id()
	}
	c.fg, c.a, c.b = item("FG", fgCat), item("A", raw), item("B", raw)
	c.wh = a.expect(a.do("POST", c.base+"/warehouses", owner, obj{"code": "WH", "name": "คลังหลัก"}), 201, "warehouse").id()
	in := func(it, qty, cost string) obj { return obj{"item_id": it, "quantity": qty, "unit_cost": cost} }
	for _, d := range []obj{
		{"type": "receipt", "date": "2026-01-05", "warehouse_id": c.wh, "status": "posted", "lines": []obj{in(c.a, "10", "10"), in(c.b, "5", "20")}},
		{"type": "receipt", "date": "2026-01-10", "warehouse_id": c.wh, "status": "posted", "lines": []obj{in(c.a, "10", "12")}},
	} {
		a.expect(a.do("POST", c.base+"/stock-documents", owner, d), 201, "stock receipt")
	}
	return c
}

func (a api) tb(owner string, c company) map[string]string {
	a.t.Helper()
	tb := a.expect(a.do("GET", c.base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "trial balance").data()
	out := map[string]string{}
	for _, r := range objs(tb["rows"]) {
		out[r["account"].(obj)["code"].(string)] = r["closing_balance"].(string)
	}
	return out
}

func (a api) stockValue(owner string, c company) string {
	a.t.Helper()
	return a.expect(a.do("GET", c.base+"/reports?book=stock-on-hand&end_date=2026-12-31", owner, nil), 200, "stock on hand").data()["total_value"].(string)
}

func TestManufacturingPerpetual(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")

	// ---------- ประเภทธุรกิจอื่นใช้แอปผลิตไม่ได้ ----------
	trading := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "shop", "name": "Shop", "business_type": "trading"}), 201, "trading").id()
	a.expect(a.do("GET", "/api/v1/companies/"+trading+"/boms", owner, nil), 422, "boms of trading company")
	a.expect(a.do("GET", "/api/v1/companies/"+trading+"/manufacturing-settings", owner, nil), 422, "settings of trading company")

	c := a.company(owner, "mfg", true)
	base := c.base

	// ---------- ค่าตั้งเริ่มต้นจากผังมาตรฐาน + ชนิดเอกสารในหน้าตั้งเลขที่ ----------
	s := a.expect(a.do("GET", base+"/manufacturing-settings", owner, nil), 200, "settings").data()
	if s["wip_account_id"] != c.wip || s["conversion_account_id"] != c.conversion || s["journal"].(obj)["code"] != "GEN" ||
		s["wip_account"].(obj)["code"] != "1105050000" {
		t.Fatalf("provisioned settings = %v", s)
	}
	nums := objs(a.expect(a.do("GET", base+"/document-numberings", owner, nil), 200, "numberings").body["data"])
	idx := slices.IndexFunc(nums, func(n obj) bool { return n["doc_type"] == "manufacturing.production_order" })
	if idx < 0 || nums[idx]["prefix"] != "MO" {
		t.Fatalf("numberings missing production order: %v", nums)
	}

	// ---------- สูตรการผลิต ----------
	bom := func(code string, isDefault bool) obj {
		return obj{"code": code, "name": "สูตร FG", "item_id": c.fg, "output_quantity": "2", "is_default": isDefault,
			"components":       []obj{{"item_id": c.a, "quantity": "4"}, {"item_id": c.b, "quantity": "1"}},
			"conversion_costs": []obj{{"description": "ค่าแรง", "amount": "50"}, {"description": "โสหุ้ย", "account_id": c.ohdAcct, "amount": "10"}}}
	}
	bad := bom("X", false)
	bad["components"] = []obj{{"item_id": c.fg, "quantity": "1"}}
	a.expect(a.do("POST", base+"/boms", owner, bad), 400, "component = product")
	created := a.expect(a.do("POST", base+"/boms", owner, bom("FG-1", true)), 201, "bom").data()
	if created["item"].(obj)["uom"] != "PCS" || objs(created["components"])[0]["item"].(obj)["code"] != "A" ||
		objs(created["conversion_costs"])[1]["account"].(obj)["code"] != "5063060000" || created["created_at"] == nil {
		t.Fatalf("bom = %v", created)
	}
	bomID := created["id"].(string)
	a.expect(a.do("POST", base+"/boms", owner, bom("FG-1", false)), 409, "duplicate code")
	a.expect(a.do("POST", base+"/boms", owner, bom("FG-2", true)), 409, "second default bom")
	if l := a.expect(a.do("GET", base+"/boms?item_id="+c.fg, owner, nil), 200, "list boms").body; l["meta"].(obj)["total"].(float64) != 1 {
		t.Fatalf("boms = %v", l)
	}

	// สิทธิ์: อ่านสูตรได้แต่สร้างไม่ได้
	role := a.expect(a.do("POST", base+"/roles", owner, obj{"code": "viewer", "name": "Viewer", "permissions": []string{"manufacturing.bom:read"}}), 201, "role").id()
	a.expect(a.do("POST", base+"/members", owner, obj{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")
	a.expect(a.do("GET", base+"/boms", staff, nil), 200, "staff reads boms")
	a.expect(a.do("POST", base+"/boms", staff, bom("FG-3", false)), 403, "staff creates bom")
	a.expect(a.do("GET", base+"/production-orders", staff, nil), 403, "staff reads orders")

	// ---------- ใบสั่งผลิต: 4 ชิ้น -> A 8, B 2; ค่าแรงมาตรฐาน (50 + 10) × 4 / 2 = 120 ----------
	orderBody := obj{"date": "2026-03-01", "item_id": c.fg, "bom_id": bomID, "quantity": "4", "warehouse_id": c.wh, "planned_end": "2026-03-10"}
	mo := a.expect(a.do("POST", base+"/production-orders", owner, orderBody), 201, "order").data()
	mats := objs(mo["materials"])
	if mo["document_no"] != "MO2026-000001" || mo["status"] != "draft" || mats[0]["required_quantity"] != "8" || mats[1]["required_quantity"] != "2" ||
		mo["conversion_cost"] != "120" || mo["bom"].(obj)["code"] != "FG-1" || mo["warehouse"].(obj)["code"] != "WH" || mo["item"].(obj)["uom"] != "PCS" {
		t.Fatalf("order = %v", mo)
	}
	moID := mo["id"].(string)
	path := base + "/production-orders/" + moID
	a.expect(a.do("DELETE", base+"/boms/"+bomID, owner, nil), 409, "delete bom in use")
	issue := func(date string, lines ...obj) obj { return obj{"date": date, "lines": lines} }
	line := func(item, qty string) obj { return obj{"item_id": item, "quantity": qty} }
	a.expect(a.do("POST", path+"/issue", owner, issue("2026-03-02", line(c.a, "1"))), 422, "issue draft")
	// แก้ปริมาณวัตถุดิบตอน draft แล้วกลับไปใช้สูตร
	edited := obj{}
	for k, v := range orderBody {
		edited[k] = v
	}
	edited["materials"] = []obj{{"item_id": c.a, "required_quantity": "9"}}
	if m := objs(a.expect(a.do("PUT", path, owner, edited), 200, "edit materials").data()["materials"]); len(m) != 1 || m[0]["required_quantity"] != "9" {
		t.Fatalf("edited materials = %v", m)
	}
	a.expect(a.do("PUT", path, owner, orderBody), 200, "regenerate materials")
	a.expect(a.do("POST", path+"/release", owner, nil), 200, "release")
	a.expect(a.do("PUT", path, owner, orderBody), 422, "edit released")
	a.expect(a.do("POST", path+"/issue", owner, issue("2026-02-28", line(c.a, "1"))), 422, "issue before order date")
	a.expect(a.do("POST", path+"/issue", owner, issue("2026-03-02", line(c.fg, "1"))), 422, "issue item not on order")
	a.expect(a.do("POST", path+"/issue", owner, issue("2026-03-02", line(c.b, "6"))), 422, "issue more than stock")

	// เบิก A 12 (FIFO 10 × 10 + 2 × 12 = 124) + B 2 (40) = 164
	issued := a.expect(a.do("POST", path+"/issue", owner, issue("2026-03-02", line(c.a, "12"), line(c.b, "2"))), 200, "issue").data()
	mv := objs(issued["movements"])
	if issued["costs"].(obj)["materials"] != "164" || len(mv) != 1 || mv[0]["value"] != "164" || mv[0]["entry_no"] == "" || mv[0]["document_no"] == "" {
		t.Fatalf("after issue = %v", issued)
	}
	// คืน A 2 ที่ต้นทุนเฉลี่ย 124 / 12 -> 20.67
	returned := a.expect(a.do("POST", path+"/return", owner, issue("2026-03-03", line(c.a, "2"))), 200, "return").data()
	if m := objs(returned["materials"])[0]; m["net_quantity"] != "10" || m["cost"] != "103.33" {
		t.Fatalf("after return = %v", m)
	}
	a.expect(a.do("POST", path+"/return", owner, issue("2026-03-03", line(c.a, "11"))), 422, "over return")

	wip := a.expect(a.do("GET", base+"/reports?book=wip&end_date=2026-03-02", owner, nil), 200, "wip").data()
	if wip["total"] != "164" || len(objs(wip["rows"])) != 1 {
		t.Fatalf("wip 03-02 = %v", wip)
	}

	// ---------- ปิดงาน 3 ชิ้น: วัตถุดิบ 143.33 + ค่าแรง 60 × 3/2 = 90 -> 233.33 ----------
	a.expect(a.do("POST", path+"/complete", owner, obj{"date": "2026-03-05", "completed_quantity": "0"}), 400, "zero completed")
	done := a.expect(a.do("POST", path+"/complete", owner, obj{"date": "2026-03-05", "completed_quantity": "3"}), 200, "complete").data()
	costs := done["costs"].(obj)
	if done["status"] != "completed" || costs["materials"] != "143.33" || costs["conversion"] != "90" || costs["total"] != "233.33" ||
		costs["unit_cost"] != "77.776667" || done["completed_quantity"] != "3" || done["completed_at"] != "2026-03-05" {
		t.Fatalf("completed = %v", done)
	}
	a.expect(a.do("POST", path+"/issue", owner, issue("2026-03-06", line(c.a, "1"))), 422, "issue completed")
	// GL: WIP ล้างเป็นศูนย์, สินค้าสำเร็จรูป 233.33, วัตถุดิบ −164 + 20.67 (รับเข้าคลังด้วยเอกสารสต็อกไม่ลงบัญชี),
	// ค่าแรงปันเข้างาน 75 (บัญชีในค่าตั้ง) + โสหุ้ย 15 (บัญชีที่ระบุในสูตร)
	tb := a.tb(owner, c)
	for code, want := range map[string]string{"1105050000": "0", "1105040000": "233.33", "1105060000": "-143.33", "5065000000": "-75", "5063060000": "-15"} {
		if tb[code] != want {
			t.Errorf("TB %s = %q, want %s", code, tb[code], want)
		}
	}

	// ---------- รายงาน ----------
	pc := a.expect(a.do("GET", base+"/reports?book=production-cost&start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "production cost").data()
	if rows := objs(pc["rows"]); len(rows) != 1 || rows[0]["total"] != "233.33" || rows[0]["order"].(obj)["document_no"] != "MO2026-000001" ||
		rows[0]["item"].(obj)["code"] != "FG" || pc["totals"].(obj)["total"] != "233.33" {
		t.Fatalf("production cost = %v", pc)
	}
	mu := objs(a.expect(a.do("GET", base+"/reports?book=material-usage&start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "usage").data()["rows"])
	// A: ตามแผน 8 × 3/4 = 6, ใช้จริง 10, เกิน 4 × 10.333 = 41.33
	if len(mu) != 2 || mu[0]["material"].(obj)["code"] != "A" || mu[0]["standard_quantity"] != "6" || mu[0]["actual_quantity"] != "10" ||
		mu[0]["variance_quantity"] != "4" || mu[0]["variance_value"] != "41.33" {
		t.Fatalf("material usage = %v", mu)
	}
	if w := a.expect(a.do("GET", base+"/reports?book=wip&end_date=2026-03-05", owner, nil), 200, "wip after").data(); w["total"] != "0" || len(objs(w["rows"])) != 0 {
		t.Fatalf("wip after completion = %v", w)
	}
	a.expect(a.do("GET", base+"/reports?book=production-cost", owner, nil), 400, "cost without dates")

	// ---------- ยกเลิกใบที่ปิดงานแล้ว: กลับทุกการเคลื่อนไหว ----------
	before := a.stockValue(owner, c)
	a.expect(a.do("POST", path+"/cancel", owner, obj{"reason": " "}), 400, "cancel without reason")
	cancelled := a.expect(a.do("POST", path+"/cancel", owner, obj{"reason": "ผลิตผิดสเปก"}), 200, "cancel completed").data()
	if cancelled["status"] != "cancelled" || cancelled["cancel_reason"] != "ผลิตผิดสเปก" || !objs(cancelled["movements"])[2]["cancelled"].(bool) {
		t.Fatalf("cancelled = %v", cancelled)
	}
	a.expect(a.do("POST", path+"/cancel", owner, obj{"reason": "x"}), 422, "cancel twice")
	if v := a.stockValue(owner, c); v != "320" || before == v { // กลับเป็น A 10×10 + 10×12 + B 5×20
		t.Fatalf("stock after cancel = %s (before %s)", v, before)
	}
	tb = a.tb(owner, c)
	for _, code := range []string{"1105050000", "1105040000", "1105060000", "5065000000", "5063060000"} {
		if tb[code] != "0" && tb[code] != "" {
			t.Errorf("TB after cancel %s = %q", code, tb[code])
		}
	}

	// ---------- ยกเลิกไม่ได้ถ้าสินค้าสำเร็จรูปถูกใช้ไปแล้ว (all-or-nothing) ----------
	mo2 := a.expect(a.do("POST", base+"/production-orders", owner, orderBody), 201, "order 2").data()
	if mo2["document_no"] != "MO2026-000002" {
		t.Fatalf("order 2 no = %v", mo2["document_no"])
	}
	p2 := base + "/production-orders/" + mo2["id"].(string)
	a.expect(a.do("POST", p2+"/release", owner, nil), 200, "release 2")
	a.expect(a.do("POST", p2+"/issue", owner, issue("2026-03-02", line(c.a, "8"), line(c.b, "2"))), 200, "issue 2")
	a.expect(a.do("POST", p2+"/complete", owner, obj{"date": "2026-03-05", "completed_quantity": "4"}), 200, "complete 2")
	a.expect(a.do("POST", base+"/stock-documents", owner, obj{"type": "issue", "date": "2026-03-06", "warehouse_id": c.wh, "status": "posted",
		"lines": []obj{{"item_id": c.fg, "quantity": "1"}}}), 201, "sell 1 FG")
	stock := a.stockValue(owner, c)
	a.expect(a.do("POST", p2+"/cancel", owner, obj{"reason": "x"}), 422, "cancel consumed FG")
	if st := a.expect(a.do("GET", p2, owner, nil), 200, "order 2 after failed cancel").data(); st["status"] != "completed" || a.stockValue(owner, c) != stock {
		t.Fatalf("failed cancel changed state: %v", st["status"])
	}

	// ---------- รูปแบบเลขที่ใบสั่งผลิตตั้งเองได้ ----------
	a.expect(a.do("PUT", base+"/document-numberings/manufacturing.production_order", owner, obj{"prefix": "MP", "template": "{prefix}{yy}{mm}-{seq:4}"}), 200, "numbering")
	mo3 := a.expect(a.do("POST", base+"/production-orders", owner, orderBody), 201, "order 3").data()
	if no := mo3["document_no"].(string); !strings.HasPrefix(no, "MP2603-") {
		t.Fatalf("custom numbering = %s", no)
	}
	if l := a.expect(a.do("GET", base+"/production-orders?status=draft&q=MP", owner, nil), 200, "list orders").body; l["meta"].(obj)["total"].(float64) != 1 {
		t.Fatalf("orders = %v", l)
	}
}

// ระบบสิ้นงวด (ค่าเริ่มต้น): เคลื่อนไหวสต็อก + ต้นทุนจริงครบ แต่ไม่มีรายการบัญชี
func TestManufacturingPeriodic(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	c := a.company(owner, "mfgp", false)
	bom := a.expect(a.do("POST", c.base+"/boms", owner, obj{"code": "B1", "name": "สูตร", "item_id": c.fg, "output_quantity": "1",
		"components": []obj{{"item_id": c.a, "quantity": "2", "scrap_percent": "10"}}, "conversion_costs": []obj{{"description": "ค่าแรง", "amount": "5"}}}), 201, "bom").id()
	mo := a.expect(a.do("POST", c.base+"/production-orders", owner, obj{"date": "2026-04-01", "item_id": c.fg, "bom_id": bom, "quantity": "5", "warehouse_id": c.wh}), 201, "order").data()
	if objs(mo["materials"])[0]["required_quantity"] != "11" { // 2 × 5 × 1.1
		t.Fatalf("required with scrap = %v", mo["materials"])
	}
	path := c.base + "/production-orders/" + mo["id"].(string)
	a.expect(a.do("POST", path+"/release", owner, nil), 200, "release")
	a.expect(a.do("POST", path+"/issue", owner, obj{"date": "2026-04-02", "lines": []obj{{"item_id": c.a, "quantity": "11"}}}), 200, "issue")
	done := a.expect(a.do("POST", path+"/complete", owner, obj{"date": "2026-04-03", "completed_quantity": "5"}), 200, "complete").data()
	// FIFO 10 × 10 + 1 × 12 = 112 + ค่าแรง 25
	if done["costs"].(obj)["total"] != "137" {
		t.Fatalf("periodic total = %v", done["costs"])
	}
	for _, m := range objs(done["movements"]) {
		if m["entry_no"] != "" {
			t.Fatalf("periodic movement has GL entry: %v", m)
		}
	}
	for code, v := range a.tb(owner, c) {
		if v != "0" {
			t.Errorf("periodic TB %s = %s", code, v)
		}
	}
	if v := a.stockValue(owner, c); v != "345" { // 320 − 112 + 137
		t.Fatalf("periodic stock = %s", v)
	}
}
