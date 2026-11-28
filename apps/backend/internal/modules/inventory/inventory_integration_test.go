package inventory_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (identity + organization + ledger + inventory)
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/inventory/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ตาราง identity_* / organization_* / ledger_* / inventory_* ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/inventory"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/reports"
)

type obj = map[string]any

type api struct {
	t   *testing.T
	srv *httptest.Server
}

type resp struct {
	status int
	body   obj
}

func (r resp) data() obj  { return r.body["data"].(obj) }
func (r resp) id() string { return r.data()["id"].(string) }
func (r resp) str(k string) string {
	v, _ := r.data()[k].(string)
	return v
}
func (r resp) total() int { return int(r.body["meta"].(obj)["total"].(float64)) }

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
	for _, tbl := range []string{
		"inventory_period_closings", "inventory_accounting_systems", "inventory_settings",
		"ledger_journal_lines", "ledger_journal_entries", "ledger_journal_sequences", "ledger_number_sequences", "ledger_journals", "ledger_accounts",
		"inventory_allocations", "inventory_movements", "inventory_layers", "inventory_sequences", "inventory_number_sequences", "inventory_document_lines",
		"inventory_documents", "inventory_items", "inventory_warehouses", "inventory_item_categories", "inventory_uoms",
		"organization_fiscal_periods", "organization_fiscal_years", "organization_project_sites", "organization_cost_centers",
		"organization_departments", "organization_business_units", "organization_branches", "organization_document_numberings", "organization_companies",
		"identity_membership_roles", "identity_memberships", "identity_role_permissions", "identity_roles", "identity_users",
	} {
		if err := db.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE").Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, migrate := range []func() error{
		func() error { return identity.Migrate(db) },
		func() error { return organization.Migrate(db) },
		func() error { return ledger.Migrate(db) },
		func() error { return inventory.Migrate(db) },
	} {
		if err := migrate(); err != nil {
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
	reg := reports.NewRegistry()
	inv.RegisterReports(reg)
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv}
}

func TestInventoryFlow(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	outsider := a.user("outsider@example.com")

	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	// ทศนิยมกำหนดที่บริษัท: เงิน 2, จำนวนสินค้า 3
	companyBody := obj{"name": "ACME", "base_currency": "THB", "fiscal_year_start_month": 1, "amount_decimals": 2, "quantity_decimals": 3, "status": "active"}
	a.expect(a.do("PUT", base, owner, companyBody), 200, "quantity_decimals = 3")
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "fiscal year")
	branch := a.expect(a.do("POST", base+"/branches", owner, obj{"code": "HQ", "name": "HQ", "tax_branch_no": "00000"}), 201, "branch").id()

	// ---------- master ----------
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "pcs", "name": "ชิ้น"}), 201, "uom pcs")
	if pcs.str("code") != "PCS" || pcs.str("status") != "active" {
		t.Fatalf("uom = %v", pcs.data())
	}
	kg := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "KG", "name": "กิโลกรัม"}), 201, "uom kg").id()
	a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "dup"}), 409, "duplicate uom")
	raw := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "RAW", "name": "วัตถุดิบ"}), 201, "category").id()
	wh1 := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH1", "name": "คลังหลัก", "branch_id": branch}), 201, "WH1")
	if wh1.data()["branch_id"] != branch {
		t.Fatalf("WH1 = %v", wh1.data())
	}
	wh2 := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH2", "name": "คลังสาขา"}), 201, "WH2").id()
	a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH9", "name": "x", "branch_id": "0195f0a0-0000-7000-8000-000000000000"}), 404, "unknown branch")

	item := func(body obj) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/items", owner, body), 201, "item "+body["code"].(string)).id()
	}
	skuA := item(obj{"code": "sku-a", "name": "สินค้า A", "uom_id": pcs.id(), "category_id": raw, "reorder_point": "5"})
	skuK := item(obj{"code": "SKU-K", "name": "สินค้าชั่งน้ำหนัก", "uom_id": kg})
	svc := item(obj{"code": "SVC", "name": "ค่าบริการ", "type": "service", "uom_id": pcs.id()})
	a.expect(a.do("POST", base+"/items", owner, obj{"code": "X", "name": "x", "uom_id": "0195f0a0-0000-7000-8000-000000000000"}), 404, "item with unknown uom")
	if it := a.expect(a.do("GET", base+"/items/"+skuA, owner, nil), 200, "get item").data(); it["code"] != "SKU-A" || it["uom"].(obj)["code"] != "PCS" || it["category"].(obj)["code"] != "RAW" || it["reorder_point"] != "5" {
		t.Fatalf("item = %v", it)
	}
	if r := a.expect(a.do("GET", base+"/items?type=stock&sort=-code", owner, nil), 200, "list stock items"); r.total() != 2 {
		t.Fatalf("stock items = %d", r.total())
	}

	doc := func(typ, date, wh string, status string, lines ...obj) obj {
		return obj{"type": typ, "date": date, "warehouse_id": wh, "status": status, "lines": lines}
	}
	in := func(it, qty, cost string) obj { return obj{"item_id": it, "quantity": qty, "unit_cost": cost} }
	out := func(it, qty string) obj { return obj{"item_id": it, "quantity": qty} }
	post := func(body obj, what string) resp {
		t.Helper()
		return a.expect(a.do("POST", base+"/stock-documents", owner, body), 201, what)
	}

	// ---------- รับเข้า ----------
	r1 := post(doc("receipt", "2026-01-05", wh1.id(), "posted", in(skuA, "10", "10.00")), "receipt 1")
	if r1.str("document_no") != "GR2026-000001" || r1.str("total_value") != "100" || r1.str("status") != "posted" {
		t.Fatalf("receipt 1 = %v", r1.data())
	}
	r2 := post(doc("receipt", "2026-01-10", wh1.id(), "posted", in(skuA, "10", "12")), "receipt 2")
	post(doc("receipt", "2026-01-05", wh1.id(), "posted", in(skuK, "2.5", "40")), "receipt kg (2.5 กก.)")
	a.expect(a.do("POST", base+"/stock-documents", owner, doc("receipt", "2026-01-05", wh1.id(), "", in(skuK, "1.2345", "1"))), 400, "qty 4 decimals (company = 3)")
	a.expect(a.do("POST", base+"/stock-documents", owner, doc("receipt", "2026-01-05", wh1.id(), "", out(skuA, "1"))), 400, "receipt without unit_cost")
	a.expect(a.do("POST", base+"/stock-documents", owner, doc("receipt", "2026-01-05", wh1.id(), "", in(svc, "1", "1"))), 422, "service item in stock document")

	// ---------- เบิก (FIFO, ห้ามติดลบ) ----------
	i1 := post(doc("issue", "2026-01-15", wh1.id(), "posted", out(skuA, "15")), "issue 15")
	if i1.str("document_no") != "GI2026-000001" || objs(i1.data()["lines"])[0]["value"] != "160" { // 10*10 + 5*12
		t.Fatalf("issue = %v", i1.data())
	}
	a.expect(a.do("POST", base+"/stock-documents", owner, doc("issue", "2026-01-16", wh1.id(), "posted", out(skuA, "6"))), 422, "issue more than on hand")
	a.expect(a.do("POST", base+"/stock-documents", owner, doc("issue", "2026-01-07", wh1.id(), "posted", out(skuA, "11"))), 422, "issue using future receipt")

	// ---------- โอน / ปรับปรุง ----------
	toWH2 := doc("transfer", "2026-01-20", wh1.id(), "posted", out(skuA, "3"))
	toWH2["to_warehouse_id"] = wh2
	t1 := post(toWH2, "transfer 3 to WH2")
	if t1.str("document_no") != "TR2026-000001" || t1.str("total_value") != "36" || t1.data()["to_warehouse"].(obj)["code"] != "WH2" {
		t.Fatalf("transfer = %v", t1.data())
	}
	same := doc("transfer", "2026-01-20", wh1.id(), "", out(skuA, "1"))
	same["to_warehouse_id"] = wh1.id()
	a.expect(a.do("POST", base+"/stock-documents", owner, same), 422, "transfer to same warehouse")
	adj := post(doc("adjustment", "2026-01-25", wh2, "posted", out(skuA, "-1")), "adjust -1 in WH2")
	if objs(adj.data()["lines"])[0]["value"] != "12" {
		t.Fatalf("adjustment = %v", adj.data())
	}

	// ---------- รายงาน: ยอดคงเหลือ / stock card ----------
	onHand := func(query string) resp {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=stock-on-hand&"+query, owner, nil), 200, "stock-on-hand "+query)
	}
	soh := onHand("end_date=2026-01-31")
	rows := objs(soh.data()["rows"])
	type row struct{ item, wh, qty, value string }
	var got []row
	for _, r := range rows {
		got = append(got, row{r["item"].(obj)["code"].(string), r["warehouse"].(obj)["code"].(string), r["quantity"].(string), r["value"].(string)})
	}
	want := []row{{"SKU-A", "WH1", "2", "24"}, {"SKU-A", "WH2", "2", "24"}, {"SKU-K", "WH1", "2.5", "100"}}
	if !slices.Equal(got, want) || soh.data()["total_value"] != "148" || soh.data()["book"] != "stock-on-hand" || soh.total() != 3 {
		t.Fatalf("stock on hand = %v total=%v", got, soh.data()["total_value"])
	}
	if r := onHand("end_date=2026-01-12&warehouse_id=" + wh1.id() + "&item_id=" + skuA); objs(r.data()["rows"])[0]["quantity"] != "20" || objs(r.data()["rows"])[0]["value"] != "220" {
		t.Fatalf("on hand 2026-01-12 = %v", r.data())
	}
	if r := onHand("end_date=2026-01-31&page_size=1&page=3"); r.total() != 3 || len(objs(r.data()["rows"])) != 1 || r.data()["total_value"] != "148" {
		t.Fatalf("on hand page 3 = %v", r.body)
	}

	card := func(query string) resp {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=stock-card&item_id="+skuA+"&"+query, owner, nil), 200, "stock-card "+query)
	}
	sc := card("warehouse_id=" + wh1.id() + "&start_date=2026-01-01&end_date=2026-01-31")
	lines := objs(sc.data()["lines"])
	var balances []string
	for _, l := range lines {
		balances = append(balances, l["quantity"].(string)+"/"+l["balance_quantity"].(string)+"/"+l["balance_value"].(string))
	}
	if !slices.Equal(balances, []string{"10/10/100", "10/20/220", "-15/5/60", "-3/2/24"}) || sc.data()["opening_quantity"] != "0" {
		t.Fatalf("stock card = %v", balances)
	}
	if p2 := card("warehouse_id=" + wh1.id() + "&start_date=2026-01-12&end_date=2026-01-31&page_size=1&page=2"); p2.data()["opening_quantity"] != "20" ||
		objs(p2.data()["lines"])[0]["balance_quantity"] != "2" || p2.total() != 2 {
		t.Fatalf("stock card page 2 = %v", p2.body)
	}

	// ---------- ยกเลิก (กลับผลที่วันที่เดิม) ----------
	status := func(id, s string) resp { return a.do("PATCH", base+"/stock-documents/"+id, owner, obj{"status": s}) }
	a.expect(status(r2.id(), "cancelled"), 422, "cancel receipt already used")
	a.expect(status(t1.id(), "cancelled"), 422, "cancel transfer whose stock was adjusted")
	a.expect(status(adj.id(), "cancelled"), 200, "cancel adjustment")
	if c := a.expect(status(t1.id(), "cancelled"), 200, "cancel transfer").data(); c["status"] != "cancelled" || c["cancelled_by"] == nil {
		t.Fatalf("cancelled transfer = %v", c)
	}
	a.expect(status(t1.id(), "cancelled"), 422, "cancel twice")
	if r := onHand("end_date=2026-01-31&item_id=" + skuA); r.total() != 1 || objs(r.data()["rows"])[0]["quantity"] != "5" || objs(r.data()["rows"])[0]["value"] != "60" {
		t.Fatalf("on hand after cancels = %v", r.data())
	}

	// ---------- draft ----------
	draft := post(doc("issue", "2026-01-28", wh1.id(), "", out(skuA, "1")), "draft issue")
	if draft.data()["document_no"] != nil || draft.str("status") != "draft" {
		t.Fatalf("draft = %v", draft.data())
	}
	a.expect(a.do("PUT", base+"/stock-documents/"+draft.id(), owner, doc("issue", "2026-01-28", wh1.id(), "", out(skuA, "2"))), 200, "edit draft")
	a.expect(status(draft.id(), "cancelled"), 422, "cancel draft")
	a.expect(a.do("DELETE", base+"/stock-documents/"+draft.id(), owner, nil), 204, "delete draft")
	a.expect(a.do("PUT", base+"/stock-documents/"+i1.id(), owner, doc("issue", "2026-01-15", wh1.id(), "", out(skuA, "1"))), 422, "edit posted")
	a.expect(a.do("DELETE", base+"/stock-documents/"+i1.id(), owner, nil), 422, "delete posted")
	if r := a.expect(a.do("GET", base+"/stock-documents?type=receipt&status=posted&item_id="+skuA, owner, nil), 200, "list receipts"); r.total() != 2 {
		t.Fatalf("posted receipts of A = %d", r.total())
	}

	// ---------- ลบ / เปลี่ยนชนิดสิ่งที่ถูกใช้ ----------
	a.expect(a.do("PUT", base+"/items/"+skuA, owner, obj{"code": "SKU-A", "name": "A", "type": "service", "uom_id": pcs.id(), "status": "active"}), 422, "change type after movements")
	a.expect(a.do("DELETE", base+"/uoms/"+pcs.id(), owner, nil), 409, "delete uom in use")
	a.expect(a.do("DELETE", base+"/warehouses/"+wh2, owner, nil), 409, "delete warehouse in use")
	a.expect(a.do("DELETE", base+"/items/"+skuA, owner, nil), 409, "delete item in use")
	a.expect(a.do("DELETE", base+"/items/"+svc, owner, nil), 204, "delete unused item")

	// ---------- เบิกพร้อมกัน: ของมี 5 ชิ้น เบิกครั้งละ 1 พร้อมกัน 10 ครั้ง = สำเร็จ 5 ไม่ติดลบ ----------
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		ok, fail int
		nos      []string
	)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a.do("POST", base+"/stock-documents", owner, doc("issue", "2026-01-29", wh1.id(), "posted", out(skuA, "1")))
			mu.Lock()
			defer mu.Unlock()
			switch r.status {
			case 201:
				ok++
				nos = append(nos, r.str("document_no"))
			case 422:
				fail++
			}
		}()
	}
	wg.Wait()
	slices.Sort(nos)
	if ok != 5 || fail != 5 || !slices.Equal(nos, []string{"GI2026-000002", "GI2026-000003", "GI2026-000004", "GI2026-000005", "GI2026-000006"}) {
		t.Fatalf("concurrent issues ok=%d fail=%d nos=%v", ok, fail, nos)
	}
	if r := onHand("end_date=2026-01-31&item_id=" + skuA); r.total() != 0 {
		t.Fatalf("SKU-A should be out of stock: %v", r.data())
	}

	// ---------- สิทธิ์ ----------
	role := a.expect(a.do("POST", base+"/roles", owner, obj{"code": "storekeeper", "name": "Storekeeper", "permissions": []string{"inventory.document:manage", "inventory.document:read"}}), 201, "role").id()
	a.expect(a.do("POST", base+"/members", owner, obj{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")
	a.expect(a.do("POST", base+"/stock-documents", staff, doc("receipt", "2026-01-30", wh1.id(), "", in(skuK, "1", "1"))), 201, "staff creates draft")
	a.expect(a.do("POST", base+"/stock-documents", staff, doc("receipt", "2026-01-30", wh1.id(), "posted", in(skuK, "1", "1"))), 403, "staff posts")
	a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-01-31", staff, nil), 403, "staff reads report")
	a.expect(a.do("GET", base+"/items", outsider, nil), 403, "outsider")
	a.expect(a.do("GET", base+"/items", "", nil), 401, "no token")

	// ---------- งวดปิด / บริษัท inactive ----------
	jan := a.expect(a.do("GET", base+"/fiscal-periods?fiscal_year=2026&date=2026-01-15", owner, nil), 200, "jan period").body["data"].([]any)[0].(obj)["id"].(string)
	a.expect(a.do("PATCH", base+"/fiscal-periods/"+jan, owner, obj{"status": "closed"}), 200, "close Jan")
	a.expect(a.do("POST", base+"/stock-documents", owner, doc("receipt", "2026-01-31", wh1.id(), "posted", in(skuK, "1", "1"))), 422, "post into closed period")
	a.expect(status(i1.id(), "cancelled"), 422, "cancel document in closed period")
	companyBody["status"] = "inactive"
	a.expect(a.do("PUT", base, owner, companyBody), 200, "deactivate company")
	a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "BOX", "name": "กล่อง"}), 422, "write to inactive company")
	onHand("end_date=2026-01-31")

	// ---------- query ผิดรูปแบบ ----------
	for _, bad := range []string{
		"reports?book=stock-card&start_date=2026-01-01&end_date=2026-01-31", "reports?book=stock-on-hand",
		"reports?book=stock-on-hand&end_date=31012026", "reports?book=nope", "reports",
		"stock-documents?type=sale", "items?type=x", "reports?book=stock-on-hand&end_date=2026-01-31&sort=price",
	} {
		a.expect(a.do("GET", base+"/"+bad, owner, nil), 400, "bad query "+bad)
	}
}
