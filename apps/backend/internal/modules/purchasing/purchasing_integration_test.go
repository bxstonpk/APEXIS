package purchasing_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (identity + organization + ledger + inventory + purchasing)
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/purchasing/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ตารางของทุก module ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/inventory"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/modules/purchasing"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/reports"
)

type obj = map[string]any

type api struct {
	t   *testing.T
	srv *httptest.Server
	pur *purchasing.Module
}

type resp struct {
	status int
	body   obj
}

func (r resp) data() obj  { return r.body["data"].(obj) }
func (r resp) id() string { return r.data()["id"].(string) }
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
		"purchasing_credit_note_lines", "purchasing_credit_notes", "purchasing_invoice_settlements",
		"purchasing_invoice_lines", "purchasing_invoices", "purchasing_receipt_lines", "purchasing_receipts",
		"purchasing_order_lines", "purchasing_orders", "purchasing_supplier_branches", "purchasing_suppliers", "purchasing_settings", "purchasing_sequences", "purchasing_number_sequences",
		"inventory_period_closings", "inventory_accounting_systems", "inventory_settings",
		"inventory_allocations", "inventory_movements", "inventory_layers", "inventory_sequences", "inventory_number_sequences", "inventory_document_lines",
		"inventory_documents", "inventory_items", "inventory_warehouses", "inventory_item_categories", "inventory_uoms",
		"ledger_journal_lines", "ledger_journal_entries", "ledger_journal_sequences", "ledger_number_sequences", "ledger_journals", "ledger_accounts",
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
		func() error { _, err := purchasing.Migrate(db); return err },
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
	pur.RegisterRoutes(mux)
	reg := reports.NewRegistry()
	led.RegisterReports(reg)
	inv.RegisterReports(reg)
	pur.RegisterReports(reg)
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv, pur: pur}
}

func TestPurchasingFlow(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2027, "start_date": "2027-01-01"}), 201, "FY2027")

	// ---------- ผังบัญชี / สมุดรายวัน ----------
	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	invAcc, vatAcc := account("1150", "asset"), account("1160", "asset")
	apAcc, grirAcc := account("2100", "liability"), account("2110", "liability")
	purchaseAcc, varianceAcc := account("5110", "expense"), account("5120", "expense")
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "สมุดรายวันซื้อ"}), 201, "journal").id()

	// ---------- สินค้า (inventory) ----------
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	raw := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "RAW", "name": "วัตถุดิบ", "inventory_account_id": invAcc, "purchase_account_id": purchaseAcc}), 201, "category").id()
	itemA := a.expect(a.do("POST", base+"/items", owner, obj{"code": "A", "name": "สินค้า A", "uom_id": pcs, "category_id": raw}), 201, "item A").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ค่าขนส่ง", "type": "service", "uom_id": pcs, "purchase_account_id": purchaseAcc}), 201, "service").id()
	wh := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH", "name": "คลัง"}), 201, "warehouse").id()
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 200, "perpetual 2026 (default = periodic)")

	// ---------- ค่าตั้งจัดซื้อ ----------
	if s := a.expect(a.do("GET", base+"/purchasing-settings", owner, nil), 200, "default settings").data(); s["vat_rate"] != "7" {
		t.Fatalf("default vat rate = %v", s["vat_rate"])
	}
	settings := obj{"vat_rate": "7", "journal_id": jv, "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc, "price_variance_account_id": varianceAcc}
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "107"}), 400, "vat rate > 100")
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "ap_account_id": "0195f0a0-0000-7000-8000-000000000000"}), 422, "unknown AP account")
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, settings), 200, "settings")

	// ---------- ผู้ขาย ----------
	// เลขผู้เสียภาษี 1 เลข = ผู้ขาย 1 รหัส
	supplier := func(code, taxID string, vat bool) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": code, "name": code, "tax_id": taxID, "payment_term_days": 30, "vat_registered": vat, "tax_type": "juristic"}), 201, "supplier "+code).id()
	}
	vatSup, plainSup := supplier("V", "0105551234567", true), supplier("N", "0105557654321", false)
	a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "X", "name": "x", "tax_id": "123", "vat_registered": true, "tax_type": "juristic"}), 400, "bad tax id")
	a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "X", "name": "x"}), 400, "vat_registered missing")
	sup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "ADDR", "name": "ผู้ขายที่อยู่", "vat_registered": true, "tax_type": "juristic",
		"address_detail": obj{"house_no": "72/7", "moo": "3", "sub_district": "ท่าทราย", "district": "เมืองสมุทรสาคร", "province": "สมุทรสาคร", "postal_code": "74000"}}), 201, "supplier address").data()
	if sup["address"] != "เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000" {
		t.Fatalf("supplier address = %v", sup["address"])
	}
	got := a.expect(a.do("GET", base+"/suppliers/"+sup["id"].(string), owner, nil), 200, "supplier get").data()
	if got["address_detail"].(obj)["moo"] != "3" {
		t.Fatalf("supplier address_detail = %v", got["address_detail"])
	}

	// ---------- ใบสั่งซื้อ: VAT ตามผู้ขาย ณ ตอนสร้าง ----------
	line := func(item, qty, price string) obj { return obj{"item_id": item, "quantity": qty, "unit_price": price} }
	order := func(sup, date string, inclusive bool, lines ...obj) obj {
		return obj{"supplier_id": sup, "date": date, "price_includes_vat": inclusive, "lines": lines}
	}
	a.expect(a.do("POST", base+"/purchase-orders", owner, order(plainSup, "2026-02-01", true, line(itemA, "1", "1"))), 422, "VAT-inclusive price for non-VAT supplier")
	po := a.expect(a.do("POST", base+"/purchase-orders", owner, order(vatSup, "2026-02-01", false, line(itemA, "10", "100"), line(svc, "1", "500"))), 201, "PO draft").data()
	if po["order_no"] != nil || po["status"] != "draft" || po["vat_rate"] != "7" || po["totals"].(obj)["gross"] != "1605" ||
		objs(po["lines"])[0]["item"].(obj)["code"] != "A" {
		t.Fatalf("PO = %v", po)
	}
	poID := po["id"].(string)
	receipt := func(date string, lines ...obj) obj {
		return obj{"order_id": poID, "date": date, "warehouse_id": wh, "lines": lines}
	}
	qty := func(n int, q string) obj { return obj{"order_line_no": n, "quantity": q} }
	a.expect(a.do("POST", base+"/goods-receipts", owner, receipt("2026-02-10", qty(1, "1"))), 422, "receive draft PO")

	// สิทธิ์: สร้าง / แก้ได้ แต่อนุมัติไม่ได้
	role := a.expect(a.do("POST", base+"/roles", owner, obj{"code": "buyer", "name": "Buyer", "permissions": []string{"purchasing.order:manage", "purchasing.order:read"}}), 201, "role").id()
	a.expect(a.do("POST", base+"/members", owner, obj{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")
	a.expect(a.do("PATCH", base+"/purchase-orders/"+poID, staff, obj{"status": "approved"}), 403, "buyer approves")

	approved := a.expect(a.do("PATCH", base+"/purchase-orders/"+poID, owner, obj{"status": "approved"}), 200, "approve").data()
	if approved["order_no"] != "PO2026-000001" || approved["status"] != "approved" {
		t.Fatalf("approved PO = %v", approved)
	}
	a.expect(a.do("PUT", base+"/purchase-orders/"+poID, owner, order(vatSup, "2026-02-01", false, line(itemA, "1", "1"))), 422, "edit approved PO")

	// ผู้ขายยกเลิกจด VAT -> ใบสั่งซื้อเดิมยังคิด VAT 7% (เอกสารที่ตามมาใช้ค่าในใบสั่งซื้อ)
	a.expect(a.do("PUT", base+"/suppliers/"+vatSup, owner, obj{"code": "V", "name": "V", "tax_id": "0105551234567", "payment_term_days": 30, "vat_registered": false, "tax_type": "juristic", "status": "active"}), 200, "supplier deregisters VAT")

	// ---------- รับสินค้า (perpetual): Dr สินค้าคงเหลือ 400 + ซื้อ 500 / Cr GR/IR 900 ----------
	a.expect(a.do("POST", base+"/goods-receipts", owner, receipt("2026-02-10", qty(1, "11"))), 422, "over receipt")
	a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": poID, "date": "2026-02-10", "lines": []obj{qty(1, "1")}}), 422, "stock item without warehouse")
	gr := a.expect(a.do("POST", base+"/goods-receipts", owner, receipt("2026-02-10", qty(1, "4"), qty(2, "1"))), 201, "receipt").data()
	grLines := objs(gr["lines"])
	if gr["document_no"] != "RC2026-000001" || gr["total_value"] != "900" || gr["stock_document_no"] != "GR2026-000001" ||
		gr["accounting_system"] != "perpetual" || gr["entry_no"] == "" || grLines[0]["account_id"] != invAcc || grLines[1]["account_id"] != purchaseAcc {
		t.Fatalf("receipt = %v", gr)
	}
	onHand := a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-02-28", owner, nil), 200, "stock on hand").data()
	if onHand["total_value"] != "400" {
		t.Fatalf("stock value after receipt = %v", onHand["total_value"])
	}

	// ---------- ใบแจ้งหนี้: ราคา A 110 (รับที่ 100) -> ผลต่าง 40; VAT 65.80; เจ้าหนี้ 1005.80 ----------
	invoice := func(no string, lines ...obj) obj {
		return obj{"order_id": poID, "supplier_invoice_no": no, "date": "2026-02-15", "lines": lines}
	}
	a.expect(a.do("POST", base+"/supplier-invoices", owner, invoice("INV-1", qty(1, "5"))), 422, "invoice more than received")
	pi := a.expect(a.do("POST", base+"/supplier-invoices", owner, invoice("INV-1", obj{"order_line_no": 1, "quantity": "4", "unit_price": "110"}, qty(2, "1"))), 201, "invoice").data()
	piLines := objs(pi["lines"])
	if pi["document_no"] != "PI2026-000001" || pi["due_date"] != "2026-03-17" || pi["totals"].(obj)["net"] != "940" ||
		pi["totals"].(obj)["vat"] != "65.8" || pi["totals"].(obj)["gross"] != "1005.8" ||
		piLines[0]["price_variance"] != "40" || piLines[0]["price_variance_account_id"] != varianceAcc || piLines[1]["price_variance"] != "0" {
		t.Fatalf("invoice = %v", pi)
	}
	a.expect(a.do("POST", base+"/supplier-invoices", owner, invoice("INV-1", qty(1, "1"))), 422, "nothing left to invoice")

	// GL ณ สิ้น ก.พ.: GR/IR ล้างหมด, สินค้าคงเหลือ 400, ซื้อ 500, ผลต่าง 40, ภาษีซื้อ 65.8, เจ้าหนี้ -1005.8
	tb := a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "trial balance").data()
	closing := map[string]string{}
	for _, r := range objs(tb["rows"]) {
		closing[r["account"].(obj)["code"].(string)] = r["closing_balance"].(string)
	}
	for code, want := range map[string]string{"1150": "400", "1160": "65.8", "2100": "-1005.8", "2110": "0", "5110": "500", "5120": "40"} {
		if closing[code] != want {
			t.Errorf("TB %s = %q, want %s (all = %v)", code, closing[code], want, closing)
		}
	}

	// ---------- รายงาน ----------
	out := a.expect(a.do("GET", base+"/reports?book=outstanding-purchase-orders", owner, nil), 200, "outstanding").data()
	rows := objs(out["rows"])
	if len(rows) != 1 || rows[0]["to_receive"] != "6" || rows[0]["to_invoice"] != "0" || rows[0]["item"].(obj)["code"] != "A" || rows[0]["order"].(obj)["order_no"] != "PO2026-000001" {
		t.Fatalf("outstanding = %v", rows)
	}
	aging := func(asOf string) obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=ap-aging&end_date="+asOf, owner, nil), 200, "aging "+asOf).data()
	}
	if ag := aging("2026-02-28"); ag["totals"].(obj)["not_due"] != "1005.8" || ag["totals"].(obj)["total"] != "1005.8" {
		t.Fatalf("aging Feb = %v", ag)
	}
	if ag := aging("2026-04-20"); objs(ag["rows"])[0]["days_31_60"] != "1005.8" || ag["totals"].(obj)["not_due"] != "0" { // เลยกำหนด 34 วัน
		t.Fatalf("aging Apr = %v", ag)
	}
	a.expect(a.do("GET", base+"/reports?book=ap-aging", owner, nil), 400, "aging without end_date")

	// ---------- ยกเลิก: ใบรับที่แจ้งหนี้แล้วยกเลิกไม่ได้ -> ยกเลิกใบแจ้งหนี้ก่อน ----------
	cancel := func(path string, status int, what string) {
		t.Helper()
		a.expect(a.do("PATCH", base+path, owner, obj{"status": "cancelled"}), status, what)
	}
	cancel("/goods-receipts/"+gr["id"].(string), 422, "cancel invoiced receipt")
	cancel("/supplier-invoices/"+pi["id"].(string), 200, "cancel invoice")
	cancel("/supplier-invoices/"+pi["id"].(string), 422, "cancel invoice twice")
	cancel("/goods-receipts/"+gr["id"].(string), 200, "cancel receipt")
	if v := a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-02-28", owner, nil), 200, "stock after cancel").data(); v["total_value"] != "0" {
		t.Fatalf("stock value after cancel = %v", v["total_value"])
	}
	tb = a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "TB after cancel").data()
	for _, r := range objs(tb["rows"]) {
		if r["closing_balance"] != "0" {
			t.Errorf("after cancels %v closing = %v", r["account"].(obj)["code"], r["closing_balance"])
		}
	}
	// เลขใบแจ้งหนี้เดิมใช้ใหม่ได้หลังยกเลิก
	a.expect(a.do("POST", base+"/goods-receipts", owner, receipt("2026-02-11", qty(1, "2"))), 201, "receive again")
	a.expect(a.do("POST", base+"/supplier-invoices", owner, invoice("INV-1", qty(1, "2"))), 201, "reuse cancelled invoice number")
	a.expect(a.do("POST", base+"/supplier-invoices", owner, invoice("INV-1", qty(1, "0"))), 400, "zero quantity")

	// ปิดใบสั่งซื้อ: หยุดรับ
	if c := a.expect(a.do("PATCH", base+"/purchase-orders/"+poID, owner, obj{"status": "closed"}), 200, "close PO").data(); c["status"] != "closed" {
		t.Fatalf("closed PO = %v", c)
	}
	a.expect(a.do("POST", base+"/goods-receipts", owner, receipt("2026-02-12", qty(1, "1"))), 422, "receive closed PO")
	a.expect(a.do("PATCH", base+"/purchase-orders/"+poID, owner, obj{"status": "cancelled"}), 422, "cancel PO with activity")

	// ---------- periodic (ปี 2027): ผู้ขายไม่จด VAT, รับสินค้า -> Dr ซื้อ / Cr GR/IR, แจ้งหนี้ VAT 0 ----------
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2027", owner, obj{"system": "periodic"}), 200, "periodic 2027")
	po2 := a.expect(a.do("POST", base+"/purchase-orders", owner, order(plainSup, "2027-01-05", false, line(itemA, "2", "50"))), 201, "PO2").data()
	if po2["vat_rate"] != "0" || po2["totals"].(obj)["gross"] != "100" {
		t.Fatalf("PO2 = %v", po2)
	}
	poID = po2["id"].(string)
	if no := a.expect(a.do("PATCH", base+"/purchase-orders/"+poID, owner, obj{"status": "approved"}), 200, "approve PO2").data()["order_no"]; no != "PO2027-000001" {
		t.Fatalf("PO2 no = %v", no)
	}
	gr2 := a.expect(a.do("POST", base+"/goods-receipts", owner, receipt("2027-01-10", qty(1, "2"))), 201, "receipt periodic").data()
	if gr2["accounting_system"] != "periodic" || objs(gr2["lines"])[0]["account_id"] != purchaseAcc || gr2["stock_document_no"] != "GR2027-000001" {
		t.Fatalf("periodic receipt = %v", gr2)
	}
	pi2 := a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"order_id": poID, "supplier_invoice_no": "N-1", "date": "2027-01-12", "lines": []obj{qty(1, "2")}}), 201, "invoice no VAT").data()
	if pi2["totals"].(obj)["vat"] != "0" || pi2["totals"].(obj)["gross"] != "100" {
		t.Fatalf("invoice without VAT = %v", pi2)
	}
	if st := a.expect(a.do("GET", base+"/purchase-orders/"+poID, owner, nil), 200, "PO2 after invoice").data()["status"]; st != "closed" {
		t.Fatalf("PO2 should auto-close, status = %v", st)
	}
	if r := a.expect(a.do("GET", base+"/supplier-invoices?supplier_id="+plainSup+"&status=posted", owner, nil), 200, "list invoices"); r.total() != 1 {
		t.Fatalf("invoices of N = %d", r.total())
	}
	a.expect(a.do("GET", base+"/purchase-orders?status=open", owner, nil), 400, "bad status filter")
	a.expect(a.do("DELETE", base+"/suppliers/"+plainSup, owner, nil), 409, "delete supplier in use")
}
