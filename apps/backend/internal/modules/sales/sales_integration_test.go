package sales_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (ทุก module ที่ขายพึ่งพา)
// รันด้วย IDENTITY_TEST_DSN (ไม่ได้ตั้ง = skip) — test จะ DROP ตารางของทุก module ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"apexis/backend/internal/modules/finance"
	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/inventory"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/modules/purchasing"
	"apexis/backend/internal/modules/sales"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/integration"
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

func setup(t *testing.T) (api, string) { return setupApp(t, false) }

// setupApp: provision = ผูก hook ตั้งค่าเริ่มต้นตอนสร้างบริษัท (ผังบัญชี / ค่าตั้งของทุก module ตามประเภทธุรกิจ) เหมือน main.go
func setupApp(t *testing.T, provision bool) (api, string) {
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
	for _, migrate := range []func() error{
		func() error { return identity.Migrate(db) }, func() error { return organization.Migrate(db) },
		func() error { return ledger.Migrate(db) }, func() error { return inventory.Migrate(db) },
		func() error { return finance.Migrate(db) }, func() error { _, err := purchasing.Migrate(db); return err }, func() error { _, err := sales.Migrate(db); return err },
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
	pur := purchasing.New(db, idm, org, led, inv)
	sal := sales.New(db, idm, org, led, inv)
	lastSales = sal
	sal.RegisterRoutes(mux)
	// ใบเสร็จรับเงินอยู่ที่ finance — ตัดยอดใบแจ้งหนี้ขายผ่าน sales.SettleReceivables
	fin := finance.New(db, idm, org, led, sal, pur)
	fin.RegisterRoutes(mux)
	if provision {
		for _, hook := range []integration.CompanyCreatedHook{led.ProvisionCompany, sal.ProvisionCompany, pur.ProvisionCompany, inv.ProvisionCompany, fin.ProvisionCompany} {
			org.OnCompanyCreated(hook)
		}
	}
	reg := reports.NewRegistry()
	led.RegisterReports(reg)
	inv.RegisterReports(reg)
	sal.RegisterReports(reg)
	fin.RegisterReports(reg)
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	a := api{t: t, srv: srv}
	a.expect(a.do("POST", "/api/v1/users", "", obj{"email": "owner@example.com", "name": "owner", "password": "secret-pass-1"}), 201, "register")
	token := a.expect(a.do("POST", "/api/v1/auth/tokens", "", obj{"email": "owner@example.com", "password": "secret-pass-1"}), 201, "login").data()["access_token"].(string)
	return a, token
}

func TestSalesFlow(t *testing.T) {
	a, owner := setup(t)
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")

	acc := map[string]string{}
	for code, typ := range map[string]string{"1101": "asset", "1102": "asset", "1130": "asset", "1150": "asset", "1170": "asset",
		"2140": "liability", "4100": "revenue", "4110": "revenue", "5100": "expense"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	sj := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "SJ", "name": "สมุดขาย"}), 201, "journal").id()

	// ---------- สินค้า + สต็อกตั้งต้น 10 @ 60 ----------
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	fg := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "FG", "name": "สินค้า", "inventory_account_id": acc["1150"], "revenue_account_id": acc["4100"]}), 201, "category").id()
	itemA := a.expect(a.do("POST", base+"/items", owner, obj{"code": "A", "name": "สินค้า A", "uom_id": pcs, "category_id": fg}), 201, "item A").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ติดตั้ง", "type": "service", "uom_id": pcs, "revenue_account_id": acc["4110"]}), 201, "service").id()
	wh := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH", "name": "คลัง"}), 201, "warehouse").id()
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 200, "perpetual 2026 (default = periodic)")
	a.expect(a.do("POST", base+"/stock-documents", owner, obj{"type": "receipt", "date": "2026-03-01", "warehouse_id": wh, "status": "posted",
		"lines": []obj{{"item_id": itemA, "quantity": "10", "unit_cost": "60"}}}), 201, "opening stock")

	// ---------- ค่าตั้ง / ลูกค้า / ทะเบียนเงิน ----------
	a.expect(a.do("PUT", base+"/sales-settings", owner, obj{"vat_registered": true, "vat_rate": "7", "journal_id": sj, "ar_account_id": acc["1130"],
		"output_vat_account_id": acc["2140"], "cogs_account_id": acc["5100"]}), 200, "settings")
	rj := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "CRJ", "name": "สมุดรายวันรับ"}), 201, "receipt journal").id()
	a.expect(a.do("PUT", base+"/finance-settings", owner, obj{"receipt_journal_id": rj, "wht_received_account_id": acc["1170"]}), 200, "finance settings")
	// ที่อยู่ตามแบบสรรพากร: เก็บแยกช่อง + address ประกอบเป็นข้อความ ("-" = ว่าง, กรุงเทพฯ ใช้ แขวง / เขต)
	custBody := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C", "name": "ลูกค้า C", "tax_id": "0105551234567", "payment_term_days": 30,
		"tax_type": "juristic", "credit_limit": "10000", "address_detail": obj{"house_no": "175", "soi": "-", "street": "สาทรใต้",
			"sub_district": "ทุ่งมหาเมฆ", "district": "สาทร", "province": "กรุงเทพมหานคร", "postal_code": "10120"}}), 201, "customer").data()
	if custBody["address"] != "เลขที่ 175 ถนนสาทรใต้ แขวงทุ่งมหาเมฆ เขตสาทร กรุงเทพมหานคร 10120" ||
		custBody["address_detail"].(obj)["soi"] != "" || custBody["address_detail"].(obj)["street"] != "สาทรใต้" {
		t.Fatalf("customer address = %v / %v", custBody["address"], custBody["address_detail"])
	}
	cust := custBody["id"].(string)
	a.expect(a.do("GET", base+"/customers?q=C", owner, nil), 200, "customer list")
	a.expect(a.do("POST", base+"/customers", owner, obj{"code": "BADADDR", "name": "x", "tax_type": "juristic",
		"address_detail": obj{"postal_code": "123"}}), 400, "bad postal code")
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": acc["1101"]}), 201, "cash").id()
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BANK", "name": "ธนาคาร", "type": "bank", "bank_name": "BBL", "account_no": "111-1-11111-1", "gl_account_id": acc["1102"]}), 201, "bank").id()

	// ---------- ใบเสนอราคา -> ใบสั่งขาย ----------
	qt := a.expect(a.do("POST", base+"/quotations", owner, obj{"customer_id": cust, "date": "2026-03-02", "valid_until": "2026-03-09",
		"lines": []obj{{"item_id": itemA, "quantity": "8", "unit_price": "100"}, {"item_id": svc, "quantity": "1", "unit_price": "1000"}}}), 201, "quotation").data()
	if qt["document_no"] != nil || qt["totals"].(obj)["gross"] != "1926" || objs(qt["lines"])[1]["item"].(obj)["code"] != "SVC" {
		t.Fatalf("quotation = %v", qt)
	}
	qtID := qt["id"].(string)
	a.expect(a.do("POST", base+"/sales-orders", owner, obj{"quotation_id": qtID, "date": "2026-03-05"}), 422, "convert draft quotation")
	if no := a.expect(a.do("PATCH", base+"/quotations/"+qtID, owner, obj{"status": "sent"}), 200, "send quotation").data()["document_no"]; no != "QT2026-000001" {
		t.Fatalf("quotation no = %v", no)
	}
	a.expect(a.do("POST", base+"/sales-orders", owner, obj{"quotation_id": qtID, "date": "2026-03-10"}), 422, "convert expired quotation")
	so := a.expect(a.do("POST", base+"/sales-orders", owner, obj{"quotation_id": qtID, "date": "2026-03-05"}), 201, "order from quotation").data()
	if so["reference"] != "QT2026-000001" || so["quotation_id"] != qtID || len(objs(so["lines"])) != 2 || so["totals"].(obj)["gross"] != "1926" {
		t.Fatalf("order = %v", so)
	}
	a.expect(a.do("POST", base+"/sales-orders", owner, obj{"quotation_id": qtID, "date": "2026-03-05"}), 409, "convert twice")
	soID := so["id"].(string)
	ap := a.expect(a.do("PATCH", base+"/sales-orders/"+soID, owner, obj{"status": "approved"}), 200, "approve").data()
	if ap["document_no"] != "SO2026-000001" || len(ap["warnings"].([]any)) != 0 {
		t.Fatalf("approved = %v", ap)
	}
	// ใบที่ 2 ทำให้ยอดค้างเกินวงเงิน 10,000 -> อนุมัติได้แต่มีคำเตือน
	so2 := a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": cust, "date": "2026-03-05",
		"lines": []obj{{"item_id": itemA, "quantity": "2", "unit_price": "4500"}}}), 201, "order 2").id()
	if w := a.expect(a.do("PATCH", base+"/sales-orders/"+so2, owner, obj{"status": "approved"}), 200, "approve over limit").data()["warnings"].([]any); len(w) != 1 || !strings.Contains(w[0].(string), "credit limit exceeded") {
		t.Fatalf("credit warning = %v", w)
	}
	a.expect(a.do("PATCH", base+"/sales-orders/"+so2, owner, obj{"status": "cancelled"}), 200, "cancel order 2")

	// ---------- ขายสินค้า (ส่งของ + ใบกำกับภาษี) จากใบสั่งขาย: A 3 (ต้นทุน 180) ----------
	ship := func(n int, q string) obj { return obj{"order_line_no": n, "quantity": q} }
	invoice := func(body obj, status int, what string) obj {
		t.Helper()
		body["order_id"], body["date"], body["warehouse_id"] = soID, "2026-03-07", wh
		r := a.expect(a.do("POST", base+"/sales-invoices", owner, body), status, what)
		if status != 201 {
			return nil
		}
		return r.data()
	}
	invoice(obj{"lines": []obj{ship(1, "9")}}, 422, "invoice more than ordered")
	iv1 := invoice(obj{"lines": []obj{ship(1, "3")}}, 201, "goods sale 1")
	if iv1["document_no"] != "IV2026-000001" || iv1["totals"].(obj)["gross"] != "321" || iv1["due_date"] != "2026-04-06" ||
		iv1["kind"] != "invoice" || iv1["vat_status"] != "output" || iv1["stock_document_no"] != "GI2026-000001" || objs(iv1["lines"])[0]["cost"] != "180" {
		t.Fatalf("invoice 1 = %v", iv1)
	}

	// ---------- ขายสินค้า A 5 + บริการ SVC 1 (ต้นทุน 300) — ใบกำกับภาษีทำให้ภาษีของบริการในใบเกิดทันที ----------
	iv2 := invoice(obj{"lines": []obj{ship(1, "5"), ship(2, "1")}}, 201, "goods + service sale")
	if iv2["totals"].(obj)["gross"] != "1605" || iv2["stock_document_no"] != "GI2026-000002" || iv2["accounting_system"] != "perpetual" {
		t.Fatalf("invoice 2 = %v", iv2)
	}
	entryLines := func(id string) map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, l := range objs(a.expect(a.do("GET", base+"/journal-entries/"+id, owner, nil), 200, "entry").data()["lines"]) {
			out[l["account"].(obj)["code"].(string)] = l["debit"].(string) + "/" + l["credit"].(string)
		}
		return out
	}
	want := map[string]string{"1130": "1605/0", "4100": "0/500", "4110": "0/1000", "2140": "0/105", "5100": "300/0", "1150": "0/300"}
	if got := entryLines(iv2["entry_id"].(string)); len(got) != len(want) {
		t.Fatalf("combined invoice entry = %v", got)
	} else {
		for k, v := range want {
			if got[k] != v {
				t.Errorf("combined invoice %s = %s, want %s", k, got[k], v)
			}
		}
	}
	if st := a.expect(a.do("GET", base+"/sales-orders/"+soID, owner, nil), 200, "order").data()["status"]; st != "closed" {
		t.Fatalf("order should auto-close, status = %v", st)
	}
	if v := a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-03-31", owner, nil), 200, "stock").data(); v["total_value"] != "120" {
		t.Fatalf("stock value = %v", v["total_value"])
	}

	// ---------- รับเงิน: IV1 321 (ถูกหัก 9) + IV2 1,000 / เงินสด 312 + ธนาคาร 1,000 ----------
	receipt := func(methods ...obj) obj {
		return obj{"kind": "customer", "customer_id": cust, "date": "2026-03-20", "lines": []obj{
			{"invoice_id": iv1["id"], "amount": "321", "wht": "9"}, {"invoice_id": iv2["id"], "amount": "1000"},
		}, "methods": methods}
	}
	a.expect(a.do("POST", base+"/receipts", owner, receipt(obj{"cash_account_id": cash, "amount": "1321"})), 400, "methods must equal amount - wht")
	rv := a.expect(a.do("POST", base+"/receipts", owner, receipt(obj{"cash_account_id": cash, "amount": "312"}, obj{"cash_account_id": bank, "amount": "1000"})), 201, "receipt").data()
	if rv["document_no"] != "RV2026-000001" || rv["net"] != "1312" || rv["wht"] != "9" || rv["party"].(obj)["code"] != "C" ||
		objs(rv["lines"])[0]["invoice"].(obj)["code"] != iv1["document_no"] {
		t.Fatalf("receipt = %v", rv)
	}
	want = map[string]string{"1101": "312/0", "1102": "1000/0", "1170": "9/0", "1130": "0/1321"}
	if got := entryLines(rv["entry_id"].(string)); len(got) != len(want) || got["1130"] != want["1130"] || got["1170"] != want["1170"] {
		t.Fatalf("receipt entry = %v", got)
	}
	outstanding := func(id any) string {
		return a.expect(a.do("GET", base+"/sales-invoices/"+id.(string), owner, nil), 200, "invoice").data()["outstanding"].(string)
	}
	if outstanding(iv1["id"]) != "0" || outstanding(iv2["id"]) != "605" {
		t.Fatalf("outstanding = %s / %s", outstanding(iv1["id"]), outstanding(iv2["id"]))
	}
	a.expect(a.do("PATCH", base+"/sales-invoices/"+iv1["id"].(string), owner, obj{"status": "cancelled"}), 422, "cancel paid invoice")

	// ---------- รายงาน ----------
	book := func(q string) obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book="+q, owner, nil), 200, q).data()
	}
	if r := book("ar-aging&end_date=2026-03-31"); r["totals"].(obj)["total"] != "605" {
		t.Fatalf("AR aging = %v", r)
	}
	if r := book("ar-aging&end_date=2026-03-15"); r["totals"].(obj)["total"] != "1926" {
		t.Fatalf("AR aging before receipt = %v", r)
	}
	if r := book("output-vat&start_date=2026-03-01&end_date=2026-03-31"); len(objs(r["rows"])) != 2 || r["total_base"] != "1800" || r["total_tax"] != "126" {
		t.Fatalf("output VAT = %v", r)
	}
	if r := book("withholding-tax-received&start_date=2026-03-01&end_date=2026-03-31"); r["total_wht"] != "9" || objs(r["rows"])[0]["party"].(obj)["tax_id"] != "0105551234567" {
		t.Fatalf("WHT received = %v", r)
	}
	if r := book("outstanding-sales-orders"); len(r["rows"].([]any)) != 0 {
		t.Fatalf("outstanding orders = %v", r)
	}
	a.expect(a.do("GET", base+"/reports?book=output-vat&start_date=2026-03-31&end_date=2026-03-01", owner, nil), 400, "bad date range")

	// ---------- ยกเลิกย้อนลำดับ: ใบเสร็จ -> ขายสินค้า (คืนสต็อก, ใบสั่งขายเปิดกลับ) ----------
	a.expect(a.do("PATCH", base+"/receipts/"+rv["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel receipt")
	if r := book("ar-aging&end_date=2026-03-31"); r["totals"].(obj)["total"] != "1926" {
		t.Fatalf("AR aging after cancelling the receipt = %v", r)
	}
	a.expect(a.do("PATCH", base+"/sales-invoices/"+iv2["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel goods sale")
	if v := book("stock-on-hand&end_date=2026-03-31"); v["total_value"] != "420" {
		t.Fatalf("stock after cancel = %v", v["total_value"])
	}
	if st := a.expect(a.do("GET", base+"/sales-orders/"+soID, owner, nil), 200, "order").data()["status"]; st != "approved" {
		t.Fatalf("order should reopen, status = %v", st)
	}
	if r := book("outstanding-sales-orders"); len(objs(r["rows"])) != 2 {
		t.Fatalf("outstanding after cancel = %v", r)
	}
	a.expect(a.do("DELETE", base+"/customers/"+cust, owner, nil), 409, "delete customer in use")
}
