package finance_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (ทุก module ที่การเงินพึ่งพา)
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/finance/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ทุกตารางใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/finance"
	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/inventory"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/modules/pettycash"
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
	db  *gorm.DB
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

func setup(t *testing.T) api { return setupApp(t, nil) }

// setupApp: hooks != nil = ลงทะเบียนการตั้งค่าบริษัทใหม่แบบ main.go (ledger -> sales -> purchasing -> inventory -> finance) ตามด้วย hooks
func setupApp(t *testing.T, hooks []integration.CompanyCreatedHook) api {
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
	for _, migrate := range []func(*gorm.DB) error{
		identity.Migrate, organization.Migrate, ledger.Migrate, inventory.Migrate, finance.Migrate, pettycash.Migrate, func(db *gorm.DB) error { _, err := purchasing.Migrate(db); return err }, func(db *gorm.DB) error { _, err := sales.Migrate(db); return err },
	} {
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
	pur := purchasing.New(db, idm, org, led, inv)
	pur.RegisterRoutes(mux)
	sal := sales.New(db, idm, org, led, inv)
	sal.RegisterRoutes(mux)
	pc := pettycash.New(db, idm, org, led)
	pc.RegisterRoutes(mux)
	fin := finance.New(db, idm, org, led, sal, pur, pc)
	fin.RegisterRoutes(mux)
	if hooks != nil {
		for _, h := range append([]integration.CompanyCreatedHook{led.ProvisionCompany, sal.ProvisionCompany, pur.ProvisionCompany, inv.ProvisionCompany, fin.ProvisionCompany, pc.ProvisionCompany}, hooks...) {
			org.OnCompanyCreated(h)
		}
	}
	reg := reports.NewRegistry()
	for _, r := range []interface{ RegisterReports(*reports.Registry) }{led, inv, pur, sal, fin, pc} {
		r.RegisterReports(reg)
	}
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv, db: db}
}

// จ่ายชำระหนี้ผู้ขาย (หลายใบแจ้งหนี้ / หลายช่องทาง / ภาษีหัก ณ ที่จ่าย), จ่ายเงินอื่น, ภ.ง.ด.3 / 53, อายุเจ้าหนี้, ยกเลิก
func TestPayments(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")

	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	cashGL, bankGL, vatAcc := account("1101", "asset"), account("1102", "asset"), account("1160", "asset")
	apAcc, grirAcc, wht53, wht3 := account("2100", "liability"), account("2110", "liability"), account("2130", "liability"), account("2131", "liability")
	expense, fee := account("5200", "expense"), account("5300", "expense")
	header := a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "1000", "name": "h", "type": "asset", "postable": false}), 201, "header").id()

	// ---------- สมุดรายวัน + รูปแบบเลขที่ใบสำคัญจ่าย (เอกสารของการเงิน) ----------
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "จ่าย", "number_template": "{prefix}{bb}{mm}-{seq:4}"}), 201, "journal").data()
	nums := objs(a.expect(a.do("GET", base+"/document-numberings", owner, nil), 200, "numberings").body["data"])
	var types []string
	for _, n := range nums {
		types = append(types, n["doc_type"].(string))
	}
	for _, want := range []string{"finance.receipt", "finance.payment", "finance.transfer"} {
		if !slices.Contains(types, want) {
			t.Fatalf("numberings %v missing %s", types, want)
		}
	}
	if slices.Contains(types, "purchasing.payment") || slices.Contains(types, "sales.receipt") {
		t.Fatalf("payments / receipts are still registered by purchasing / sales: %v", types)
	}
	a.expect(a.do("PUT", base+"/document-numberings/finance.payment", owner, obj{"prefix": "pv", "template": "{prefix}{yy}{mm}-{seq:5}"}), 200, "payment numbering")

	// ---------- ทะเบียนเงินสด / ธนาคาร ----------
	a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BBL", "name": "x", "type": "bank", "gl_account_id": bankGL}), 400, "bank without account no")
	a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "C2", "name": "x", "type": "cash", "gl_account_id": header}), 422, "header GL account")
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": cashGL}), 201, "cash").id()
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BBL", "name": "กรุงเทพ", "type": "bank", "bank_name": "BBL", "account_no": "123-4-56789-0", "gl_account_id": bankGL}), 201, "bank").id()

	// ---------- ภาษีหัก ณ ที่จ่าย / ค่าตั้ง ----------
	a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "X", "name": "x", "rate": "0", "income_type": "3_tres"}), 400, "zero rate")
	a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "X", "name": "x", "rate": "3"}), 400, "income type required")
	a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "X", "name": "x", "rate": "3", "income_type": "40_9"}), 400, "invalid income type")
	svc3 := a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "SVC", "name": "ค่าบริการ", "rate": "3", "income_type": "3_tres"}), 201, "wht 3%").id()
	// PUT ไม่ส่ง income_type = คงค่าเดิม; ส่งค่าใหม่ = เปลี่ยน
	if w := a.expect(a.do("PUT", base+"/withholding-tax-types/"+svc3, owner, obj{"code": "SVC", "name": "ค่าบริการ", "rate": "3", "status": "active"}), 200, "update keeps income type").data(); w["income_type"] != "3_tres" {
		t.Fatalf("wht type after update = %v", w)
	}
	a.expect(a.do("PUT", base+"/withholding-tax-types/"+svc3, owner, obj{"code": "SVC", "name": "ค่าบริการ", "rate": "3", "status": "active", "income_type": "bad"}), 400, "update invalid income type")
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "journal_id": jv["id"], "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc}), 200, "purchasing settings")
	payment := func(lines []obj, methods ...obj) obj {
		return obj{"kind": "supplier", "date": "2026-03-10", "lines": lines, "methods": methods}
	}
	a.expect(a.do("POST", base+"/payments", owner, payment(nil)), 400, "supplier payment without supplier")
	a.expect(a.do("PUT", base+"/finance-settings", owner, obj{"payment_journal_id": header}), 422, "account id as journal")
	a.expect(a.do("PUT", base+"/finance-settings", owner, obj{"payment_journal_id": jv["id"], "wht_pnd3_account_id": wht3, "wht_pnd53_account_id": wht53}), 200, "finance settings")

	// ---------- ใบแจ้งหนี้ 2 ใบ (ค่าบริการ 5,000 + VAT 350 = 5,350 ต่อใบ) ----------
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ค่าบริการ", "type": "service", "uom_id": pcs, "purchase_account_id": expense}), 201, "service").id()
	sup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "V", "name": "V Co.", "tax_id": "0105551234567", "payment_term_days": 0, "vat_registered": true, "tax_type": "juristic"}), 201, "supplier").id()
	po := a.expect(a.do("POST", base+"/purchase-orders", owner, obj{"supplier_id": sup, "date": "2026-03-01", "lines": []obj{{"item_id": svc, "quantity": "2", "unit_price": "5000"}}}), 201, "PO").id()
	a.expect(a.do("PATCH", base+"/purchase-orders/"+po, owner, obj{"status": "approved"}), 200, "approve")
	a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": po, "date": "2026-03-01", "lines": []obj{{"order_line_no": 1, "quantity": "2"}}}), 201, "receipt")
	invoice := func(no string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"order_id": po, "supplier_invoice_no": no, "date": "2026-03-01",
			"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "invoice "+no).id()
	}
	invA, invB := invoice("A-1"), invoice("B-1")
	if open := objs(a.expect(a.do("GET", base+"/payments/open-invoices?supplier_id="+sup, owner, nil), 200, "open invoices").body["data"]); len(open) != 2 || open[0]["outstanding"] != "5350" {
		t.Fatalf("open invoices = %v", open)
	}

	// ---------- จ่าย: A เต็ม (หัก 3% ของ 5,000 = 150) + B บางส่วน 2,675 / เงินสด 2,000 + ธนาคาร 5,875 ----------
	lines := []obj{{"invoice_id": invA, "amount": "5350", "wht_type_id": svc3}, {"invoice_id": invB, "amount": "2675"}}
	method := func(acc, amount string) obj {
		return obj{"cash_account_id": acc, "amount": amount, "reference": "CHQ-1"}
	}
	withSupplier := func(p obj) obj { p["supplier_id"] = sup; return p }
	a.expect(a.do("POST", base+"/payments", owner, withSupplier(payment(lines, method(cash, "2000"), method(bank, "6025")))), 400, "methods total != amount - wht")
	a.expect(a.do("POST", base+"/payments", owner, withSupplier(payment([]obj{{"invoice_id": invB, "amount": "5350.01"}}, method(cash, "5350.01")))), 422, "pay more than outstanding")
	p := a.expect(a.do("POST", base+"/payments", owner, withSupplier(payment(lines, method(cash, "2000"), method(bank, "5875")))), 201, "payment").data()
	pl := objs(p["lines"])
	if p["document_no"] != "PV2603-00001" || p["amount"] != "8025" || p["wht"] != "150" || p["net"] != "7875" || p["party"].(obj)["tax_type"] != "juristic" ||
		p["entry_no"] != "PJ6903-0004" || pl[0]["wht_base"] != "5000" || pl[0]["wht_rate"] != "3" || pl[0]["invoice"].(obj)["name"] != "A-1" ||
		objs(p["methods"])[1]["cash_account"].(obj)["code"] != "BBL" || pl[0]["wht_type"].(obj)["income_type"] != "3_tres" || pl[1]["wht_type"] != nil ||
		p["wht_certificate_no"] != "WT2026-000001" {
		t.Fatalf("payment = %v", p)
	}
	// รายการที่ 4 ของสมุด PJ ในเดือนนี้ (รับสินค้า + แจ้งหนี้ 2 ใบลงก่อน) — 1 ผังบัญชี = 1 บรรทัด
	entry := a.expect(a.do("GET", base+"/journal-entries/"+p["entry_id"].(string), owner, nil), 200, "payment entry").data()
	got := map[string]string{}
	for _, l := range objs(entry["lines"]) {
		got[l["account"].(obj)["code"].(string)] = l["debit"].(string) + "/" + l["credit"].(string)
	}
	for k, v := range map[string]string{"2100": "8025/0", "1101": "0/2000", "1102": "0/5875", "2130": "0/150"} {
		if got[k] != v || len(got) != 4 {
			t.Fatalf("payment entry lines = %v", got)
		}
	}
	outstanding := func(id string) string {
		t.Helper()
		return a.expect(a.do("GET", base+"/supplier-invoices/"+id, owner, nil), 200, "invoice").data()["outstanding"].(string)
	}
	if outstanding(invA) != "0" || outstanding(invB) != "2675" {
		t.Fatalf("outstanding A=%s B=%s", outstanding(invA), outstanding(invB))
	}
	a.expect(a.do("PATCH", base+"/supplier-invoices/"+invB, owner, obj{"status": "cancelled"}), 422, "cancel paid invoice")

	// ---------- จ่ายเงินอื่น: ค่าบริการบุคคลธรรมดา 2,500 หัก 3% (ภ.ง.ด.3) ----------
	other := obj{"kind": "other", "date": "2026-03-12", "payee": obj{"name": "นาย ก"},
		"lines":   []obj{{"account_id": fee, "description": "ค่าออกแบบ", "amount": "2500", "wht_type_id": svc3}},
		"methods": []obj{method(cash, "2425")}}
	a.expect(a.do("POST", base+"/payments", owner, other), 422, "withholding without payee tax id")
	other["payee"] = obj{"name": "นาย ก", "tax_id": "1101700000000", "tax_type": "individual", "address": "กรุงเทพฯ"}
	po2 := a.expect(a.do("POST", base+"/payments", owner, other), 201, "other payment").data()
	if po2["wht"] != "75" || po2["kind"] != "other" || po2["party"].(obj)["id"] != nil || po2["wht_certificate_no"] != "WT2026-000002" {
		t.Fatalf("other payment = %v", po2)
	}

	// ---------- รายงาน ----------
	aging := func(asOf string) string {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=ap-aging&end_date="+asOf, owner, nil), 200, "aging").data()["totals"].(obj)["total"].(string)
	}
	if aging("2026-03-05") != "10700" || aging("2026-03-31") != "2675" {
		t.Fatalf("aging before / after payment = %s / %s", aging("2026-03-05"), aging("2026-03-31"))
	}
	wht := func(query string) obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=withholding-tax&start_date=2026-03-01&end_date=2026-03-31"+query, owner, nil), 200, "wht "+query).data()
	}
	if r := wht("&form=pnd53"); len(objs(r["rows"])) != 1 || r["total_base"] != "5000" || r["total_wht"] != "150" || objs(r["rows"])[0]["party"].(obj)["tax_id"] != "0105551234567" ||
		objs(r["rows"])[0]["certificate_no"] != "WT2026-000001" || objs(r["rows"])[0]["wht_type"].(obj)["income_type"] != "3_tres" {
		t.Fatalf("withholding pnd53 = %v", r)
	}
	if r := wht("&form=pnd3"); len(objs(r["rows"])) != 1 || r["total_wht"] != "75" || objs(r["rows"])[0]["tax_form"] != "pnd3" {
		t.Fatalf("withholding pnd3 = %v", r)
	}
	a.expect(a.do("GET", base+"/reports?book=withholding-tax&start_date=2026-03-01&end_date=2026-03-31&form=pnd1", owner, nil), 400, "bad form")

	// ---------- ยกเลิกใบจ่าย: ยอดค้างกลับ, ภาษีหัก ณ ที่จ่ายหาย, เลขถัดไปต่อเนื่อง ----------
	a.expect(a.do("PATCH", base+"/payments/"+p["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel payment")
	a.expect(a.do("PATCH", base+"/payments/"+p["id"].(string), owner, obj{"status": "cancelled"}), 422, "cancel twice")
	if outstanding(invA) != "5350" || aging("2026-03-31") != "10700" || wht("&form=pnd53")["total_wht"] != "0" {
		t.Fatalf("after cancel: A=%s aging=%s", outstanding(invA), aging("2026-03-31"))
	}
	p2 := a.expect(a.do("POST", base+"/payments", owner, withSupplier(payment([]obj{{"invoice_id": invA, "amount": "5350"}}, method(bank, "5350")))), 201, "payment 2").data()
	if p2["document_no"] != "PV2603-00003" || p2["wht"] != "0" || p2["wht_certificate_no"] != nil {
		t.Fatalf("payment 2 = %v", p2)
	}
	// ใบจ่ายที่ยกเลิกยังคงเลขที่หนังสือรับรองเดิม (เป็นโมฆะพร้อมใบจ่าย) — รายการใบจ่ายแสดงเลขที่ด้วย
	if c := a.expect(a.do("GET", base+"/payments/"+p["id"].(string), owner, nil), 200, "cancelled payment").data(); c["status"] != "cancelled" || c["wht_certificate_no"] != "WT2026-000001" {
		t.Fatalf("cancelled payment = %v", c)
	}
	if rows := objs(a.expect(a.do("GET", base+"/payments?sort=document_no", owner, nil), 200, "payments list").body["data"]); len(rows) != 3 ||
		rows[0]["wht_certificate_no"] != "WT2026-000001" || rows[2]["wht_certificate_no"] != nil {
		t.Fatalf("payments list = %v", rows)
	}

	// ---------- สรุป ภ.พ.30: ภาษีขาย - ภาษีซื้อ ----------
	a.expect(a.do("GET", base+"/reports?book=vat-summary&start_date=2026-03-01", owner, nil), 400, "vat summary without end date")
	a.expect(a.do("GET", base+"/reports?book=vat-summary&start_date=2026-03-31&end_date=2026-03-01", owner, nil), 400, "vat summary reversed range")
	vs := a.expect(a.do("GET", base+"/reports?book=vat-summary&start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "vat summary").data()
	out, in := vs["output"].(obj), vs["input"].(obj)
	dec := func(v any) decimal.Decimal {
		t.Helper()
		d, err := decimal.NewFromString(v.(string))
		if err != nil {
			t.Fatalf("decimal %v: %v", v, err)
		}
		return d
	}
	net := dec(out["tax"]).Sub(dec(in["tax"]))
	if vs["book"] != "vat-summary" || vs["start_date"] != "2026-03-01" || vs["end_date"] != "2026-03-31" || !dec(vs["net_tax"]).Equal(net) ||
		!dec(vs["payable"]).Equal(decimal.Max(net, decimal.Zero)) || !dec(vs["refundable"]).Equal(decimal.Max(net.Neg(), decimal.Zero)) ||
		out["base"] == nil || in["documents"] == nil {
		t.Fatalf("vat summary = %v", vs)
	}
	if r := a.expect(a.do("GET", base+"/payments?status=posted&kind=supplier&party_id="+sup, owner, nil), 200, "list payments"); r.total() != 1 {
		t.Fatalf("posted supplier payments = %d", r.total())
	}
	if r := a.expect(a.do("GET", base+"/payments?cash_account_id="+cash, owner, nil), 200, "payments by cash account"); r.total() != 2 {
		t.Fatalf("payments using cash = %d", r.total())
	}
}

// รับเงินอื่น, โอนเงินระหว่างบัญชี, ค่าตั้งที่ขาด, สิทธิ์เดิมของบทบาทถูกแปลงเป็นของการเงิน
func TestReceiptsTransfersAndPermissionMigration(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	acc := map[string]string{}
	for code, typ := range map[string]string{"1101": "asset", "1102": "asset", "4900": "revenue"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	gen := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "GEN", "name": "ทั่วไป"}), 201, "journal").id()
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": acc["1101"]}), 201, "cash").id()
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BANK", "name": "ธนาคาร", "type": "bank", "bank_name": "KBANK", "account_no": "222-2-22222-2", "gl_account_id": acc["1102"]}), 201, "bank").id()

	receipt := obj{"kind": "other", "date": "2026-04-01", "payer": obj{"name": "ธนาคารกสิกรไทย"},
		"lines": []obj{{"account_id": acc["4900"], "description": "ดอกเบี้ยรับ", "amount": "12.50"}}, "methods": []obj{{"cash_account_id": bank, "amount": "12.5"}}}
	if r := a.expect(a.do("POST", base+"/receipts", owner, receipt), 422, "missing receipt journal"); r.body["error"].(obj)["message"] == "" {
		t.Fatal("settings error should name the missing field")
	}
	a.expect(a.do("PUT", base+"/finance-settings", owner, obj{"receipt_journal_id": gen, "transfer_journal_id": gen}), 200, "settings")
	rv := a.expect(a.do("POST", base+"/receipts", owner, receipt), 201, "other receipt").data()
	if rv["document_no"] != "RV2026-000001" || rv["net"] != "12.5" || rv["kind"] != "other" || rv["party"].(obj)["name"] != "ธนาคารกสิกรไทย" {
		t.Fatalf("receipt = %v", rv)
	}

	// โอนเงินสด 500 เข้าธนาคาร -> Dr ธนาคาร / Cr เงินสด, ยกเลิกแล้วกลับรายการ
	a.expect(a.do("POST", base+"/cash-transfers", owner, obj{"date": "2026-04-02", "from_cash_account_id": cash, "to_cash_account_id": cash, "amount": "500"}), 400, "same account")
	tf := a.expect(a.do("POST", base+"/cash-transfers", owner, obj{"date": "2026-04-02", "from_cash_account_id": cash, "to_cash_account_id": bank, "amount": "500", "reference": "DEP-1"}), 201, "transfer").data()
	if tf["document_no"] != "TF2026-000001" || tf["from"].(obj)["code"] != "CASH" || tf["to"].(obj)["code"] != "BANK" || tf["entry_id"] == nil {
		t.Fatalf("transfer = %v", tf)
	}
	a.expect(a.do("PATCH", base+"/cash-transfers/"+tf["id"].(string), owner, obj{"status": "posted"}), 400, "only cancel via PATCH")
	if c := a.expect(a.do("PATCH", base+"/cash-transfers/"+tf["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel transfer").data(); c["status"] != "cancelled" {
		t.Fatalf("cancelled transfer = %v", c)
	}
	if r := a.expect(a.do("GET", base+"/cash-transfers?status=posted", owner, nil), 200, "transfers"); r.total() != 0 {
		t.Fatalf("posted transfers = %d", r.total())
	}

	// สิทธิ์เดิม: บทบาทที่เคยรับเงินของขาย / จ่ายเงินของจัดซื้อได้ ยังทำต่อได้ที่การเงิน
	var roleID string
	if err := a.db.Raw(`SELECT id FROM identity_roles WHERE company_id = ? LIMIT 1`, company).Scan(&roleID).Error; err != nil || roleID == "" {
		t.Fatalf("role: %v", err)
	}
	for _, p := range []string{"sales.receipt:post", "purchasing.payment:*", "purchasing.settings:read"} {
		a.db.Exec(`INSERT INTO identity_role_permissions (role_id, permission) VALUES (?, ?)`, roleID, p)
	}
	renames := make([]identity.PermissionRename, 0)
	for _, r := range finance.PermissionMigrations() {
		renames = append(renames, identity.PermissionRename{From: r.From, To: r.To, Keep: r.Keep})
	}
	for range 2 { // ทำซ้ำได้
		if err := identity.RenamePermissions(a.db, renames); err != nil {
			t.Fatal(err)
		}
	}
	var perms []string
	a.db.Raw(`SELECT permission FROM identity_role_permissions WHERE role_id = ? ORDER BY permission`, roleID).Scan(&perms)
	for _, want := range []string{"finance.receipt:post", "finance.payment:*", "finance.settings:read", "purchasing.settings:read"} {
		if !slices.Contains(perms, want) {
			t.Fatalf("permissions %v missing %s", perms, want)
		}
	}
	if slices.Contains(perms, "sales.receipt:post") || slices.Contains(perms, "purchasing.payment:*") {
		t.Fatalf("old permissions should be replaced: %v", perms)
	}
}
