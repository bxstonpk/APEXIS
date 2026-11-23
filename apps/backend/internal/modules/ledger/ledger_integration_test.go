package ledger_test

// Integration test: HTTP -> application -> gorm -> PostgreSQL จริง (identity + organization + ledger)
// รันด้วย: IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" go test ./internal/modules/ledger/
// (ไม่ได้ตั้ง env = skip) — test จะ DROP ตาราง identity_* / organization_* / ledger_* ใน DB นั้น ห้ามชี้ไป DB จริง

import (
	"gorm.io/gorm"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/reports"
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

type obj = map[string]any

func (r resp) data() obj   { return r.body["data"].(obj) }
func (r resp) id() string  { return r.data()["id"].(string) }
func (r resp) total() int  { return int(r.body["meta"].(obj)["total"].(float64)) }
func (r resp) list() []obj { return objs(r.body["data"]) }
func (r resp) str(k string) string {
	v, _ := r.data()[k].(string)
	return v
}

func objs(v any) []obj {
	var out []obj
	for _, x := range v.([]any) {
		out = append(out, x.(obj))
	}
	return out
}

func field(list []obj, key string) []string {
	var out []string
	for _, o := range list {
		s, _ := o[key].(string)
		out = append(out, s)
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
		"ledger_journal_sequences", "ledger_number_sequences", "ledger_journal_lines", "ledger_journal_entries", "ledger_journals", "ledger_accounts",
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
	lm := ledger.New(db, idm, org)
	lm.RegisterRoutes(mux)
	reg := reports.NewRegistry()
	lm.RegisterReports(reg)
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv, db: db}
}

func ln(account string, debit, credit any) obj {
	return obj{"account_id": account, "debit": debit, "credit": credit}
}

func TestLedgerFlow(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	outsider := a.user("outsider@example.com")

	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME"}), 201, "company").id()
	other := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "beta", "name": "Beta"}), 201, "company B").id()
	base := "/api/v1/companies/" + company
	for _, y := range []obj{{"year": 2025, "start_date": "2025-01-01"}, {"year": 2026, "start_date": "2026-01-01"}} {
		a.expect(a.do("POST", base+"/fiscal-years", owner, y), 201, "fiscal year")
	}
	branch := a.expect(a.do("POST", base+"/branches", owner, obj{"code": "HQ", "name": "HQ", "tax_branch_no": "00000"}), 201, "branch").id()

	// ---------- สมุดรายวัน ----------
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "jv", "name": "สมุดรายวันทั่วไป"}), 201, "journal JV")
	if jv.str("code") != "JV" || jv.str("status") != "active" {
		t.Fatalf("journal = %v", jv.data())
	}
	journal := jv.id()
	a.expect(a.do("POST", base+"/journals", owner, obj{"code": "JV", "name": "dup"}), 409, "duplicate journal")
	a.expect(a.do("POST", base+"/journals", owner, obj{"code": "1X", "name": "bad"}), 400, "bad journal code")

	// ---------- ผังบัญชี ----------
	account := func(body obj) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, body), 201, "account "+body["code"].(string)).id()
	}
	assets := account(obj{"code": "1000", "name": "สินทรัพย์", "type": "asset", "postable": false})
	cash := account(obj{"code": "1101", "name": "เงินสด", "type": "asset", "parent_id": assets})
	bank := account(obj{"code": "1102", "name": "เงินฝากธนาคาร", "type": "asset", "parent_id": assets})
	capital := account(obj{"code": "3101", "name": "ทุน", "type": "equity"})
	revenue := account(obj{"code": "4101", "name": "รายได้จากการขาย", "type": "revenue"})
	rent := account(obj{"code": "5101", "name": "ค่าเช่า", "type": "expense"})

	a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "1101", "name": "dup", "type": "asset"}), 409, "duplicate account code")
	a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "1101-01", "name": "x", "type": "asset", "parent_id": cash}), 422, "parent is postable")
	a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "4999", "name": "x", "type": "revenue", "parent_id": assets}), 422, "parent type mismatch")
	a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "9", "name": "x", "type": "other"}), 400, "bad account type")
	current := account(obj{"code": "1100", "name": "สินทรัพย์หมุนเวียน", "type": "asset", "postable": false, "parent_id": assets})
	a.expect(a.do("PUT", base+"/accounts/"+assets, owner, obj{"code": "1000", "name": "สินทรัพย์", "type": "asset", "postable": false, "status": "active", "parent_id": current}), 422, "account cycle")
	a.expect(a.do("PUT", base+"/accounts/"+assets, owner, obj{"code": "1000", "name": "สินทรัพย์", "type": "asset", "postable": true, "status": "active"}), 422, "header with children -> postable")
	if got := field(a.expect(a.do("GET", base+"/accounts?type=asset&postable=true", owner, nil), 200, "postable assets").list(), "code"); !slices.Equal(got, []string{"1101", "1102"}) {
		t.Fatalf("postable assets = %v", got)
	}
	if p := a.expect(a.do("GET", base+"/accounts/"+cash, owner, nil), 200, "get cash").data()["parent"].(obj); p["code"] != "1000" {
		t.Fatalf("cash parent = %v", p)
	}

	// ---------- รายการปี 2025 (สร้างแล้วผ่านทันที) ----------
	entry := func(date string, status string, lines ...obj) obj {
		t.Helper()
		return obj{"journal_id": journal, "date": date, "description": "test " + date, "status": status, "lines": lines}
	}
	e1 := a.expect(a.do("POST", base+"/journal-entries", owner, entry("2025-06-01", "posted", ln(cash, "100000", "0"), ln(capital, "0", "100000"))), 201, "capital 2025")
	if e1.str("entry_no") != "JV2025-000001" || e1.str("total_debit") != "100000" || e1.str("status") != "posted" {
		t.Fatalf("e1 = %v", e1.data())
	}
	if no := a.expect(a.do("POST", base+"/journal-entries", owner, entry("2025-12-01", "posted", ln(cash, "5000", nil), ln(revenue, nil, "5000"))), 201, "sale 2025").str("entry_no"); no != "JV2025-000002" {
		t.Fatalf("e2 no = %v", no)
	}

	// ---------- draft -> แก้ -> ผ่าน ----------
	draft := a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-01-10", "", ln(rent, "1200", "0"), ln(cash, "0", "1000"))), 201, "unbalanced draft")
	if draft.data()["entry_no"] != nil || draft.str("status") != "draft" {
		t.Fatalf("draft = %v", draft.data())
	}
	rentEntry := draft.id()
	posted := obj{"status": "posted"}
	a.expect(a.do("PATCH", base+"/journal-entries/"+rentEntry, owner, posted), 422, "post unbalanced")
	rentLines := []obj{
		{"account_id": rent, "debit": "1200", "branch_id": branch, "description": "ค่าเช่าเดือนมกราคม"},
		ln(cash, nil, "1200"),
	}
	a.expect(a.do("PUT", base+"/journal-entries/"+rentEntry, owner, obj{"journal_id": journal, "date": "2026-01-10", "lines": []obj{
		{"account_id": rent, "debit": "1200", "branch_id": "0195f0a0-0000-7000-8000-000000000000"}, ln(cash, nil, "1200"),
	}}), 404, "unknown branch")
	a.expect(a.do("PUT", base+"/journal-entries/"+rentEntry, owner, obj{"journal_id": journal, "date": "2026-01-10", "lines": []obj{
		ln(assets, "1200", nil), ln(cash, nil, "1200"),
	}}), 422, "header account in line")
	upd := a.expect(a.do("PUT", base+"/journal-entries/"+rentEntry, owner, obj{"journal_id": journal, "date": "2026-01-10", "description": "ค่าเช่า", "lines": rentLines}), 200, "fix draft").data()
	if l := objs(upd["lines"]); len(l) != 2 || l[0]["branch_id"] != branch || l[0]["account"].(obj)["code"] != "5101" {
		t.Fatalf("draft lines = %v", upd["lines"])
	}
	p := a.expect(a.do("PATCH", base+"/journal-entries/"+rentEntry, owner, posted), 200, "post rent")
	if p.str("entry_no") != "JV2026-000001" || p.data()["fiscal_period_id"] == nil || p.data()["posted_by"] == nil {
		t.Fatalf("posted rent = %v", p.data())
	}
	a.expect(a.do("PATCH", base+"/journal-entries/"+rentEntry, owner, posted), 422, "post twice")
	a.expect(a.do("PATCH", base+"/journal-entries/"+rentEntry, owner, obj{"status": "draft"}), 400, "un-post via PATCH")
	a.expect(a.do("PUT", base+"/journal-entries/"+rentEntry, owner, obj{"journal_id": journal, "date": "2026-01-10", "lines": rentLines}), 422, "edit posted")
	a.expect(a.do("DELETE", base+"/journal-entries/"+rentEntry, owner, nil), 422, "delete posted")

	// จำนวนเงินเป็น number ได้, ทศนิยมเกิน 2 ตำแหน่ง / ทั้งเดบิตและเครดิต / ไม่มีบรรทัด -> 400
	sale := a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-15", "posted", ln(bank, 30000.50, nil), ln(revenue, nil, "30000.5"))), 201, "sale 2026")
	if sale.str("entry_no") != "JV2026-000002" || sale.str("total_credit") != "30000.5" {
		t.Fatalf("sale = %v", sale.data())
	}
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-15", "", ln(bank, "1.234", nil), ln(revenue, nil, "1.234"))), 400, "3 decimals (company amount_decimals = 2)")
	// ทศนิยมไม่ fix ไว้: บริษัทเพิ่มเป็น 3 ตำแหน่งแล้วบันทึกได้ (ลดกลับไม่ได้)
	companyBody := obj{"name": "ACME", "base_currency": "THB", "fiscal_year_start_month": 1, "amount_decimals": 3, "quantity_decimals": 2, "status": "active"}
	a.expect(a.do("PUT", base, owner, companyBody), 200, "company amount_decimals = 3")
	three := a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-15", "", ln(bank, "1.234", nil), ln(revenue, nil, "1.234"))), 201, "3 decimals (company amount_decimals = 3)")
	if l := objs(three.data()["lines"]); l[0]["debit"] != "1.234" {
		t.Fatalf("3-decimal line = %v", l[0])
	}
	a.expect(a.do("DELETE", base+"/journal-entries/"+three.id(), owner, nil), 204, "delete 3-decimal draft")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-15", "", ln(bank, "1.2345", nil), ln(revenue, nil, "1.2345"))), 400, "4 decimals (company amount_decimals = 3)")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-15", "", ln(bank, "1", "1"))), 400, "both sides")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-15", "")), 400, "no lines")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("15022026", "", ln(bank, "1", nil))), 400, "DDMMYYYY date")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2027-01-05", "posted", ln(bank, "1", nil), ln(revenue, nil, "1"))), 422, "no fiscal period")

	// draft ลบได้ (ลบจริง)
	tmp := a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-03-01", "", ln(bank, "1", nil))), 201, "tmp draft").id()
	a.expect(a.do("DELETE", base+"/journal-entries/"+tmp, owner, nil), 204, "delete draft")
	a.expect(a.do("GET", base+"/journal-entries/"+tmp, owner, nil), 404, "deleted draft")

	// บัญชี inactive ผ่านรายการไม่ได้
	a.expect(a.do("PUT", base+"/accounts/"+bank, owner, obj{"code": "1102", "name": "เงินฝากธนาคาร", "type": "asset", "postable": true, "status": "inactive", "parent_id": assets}), 200, "deactivate bank")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-03-01", "posted", ln(bank, "1", nil), ln(revenue, nil, "1"))), 422, "post to inactive account")
	a.expect(a.do("PUT", base+"/accounts/"+bank, owner, obj{"code": "1102", "name": "เงินฝากธนาคาร", "type": "asset", "postable": true, "status": "active", "parent_id": assets}), 200, "reactivate bank")
	a.expect(a.do("PUT", base+"/accounts/"+bank, owner, obj{"code": "1102", "name": "x", "type": "liability", "postable": true, "status": "active"}), 409, "change type of used account")

	// ---------- งวดปิด / กลับรายการ ----------
	jan := a.expect(a.do("GET", base+"/fiscal-periods?fiscal_year=2026&date=2026-01-15", owner, nil), 200, "jan period").list()[0]["id"].(string)
	a.expect(a.do("PATCH", base+"/fiscal-periods/"+jan, owner, obj{"status": "closed"}), 200, "close Jan 2026")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-01-20", "posted", ln(rent, "1", nil), ln(cash, nil, "1"))), 422, "post into closed period")
	a.expect(a.do("POST", base+"/journal-entries/"+rentEntry+"/reversals", owner, obj{"date": "2026-01-31"}), 422, "reverse into closed period")
	rev := a.expect(a.do("POST", base+"/journal-entries/"+rentEntry+"/reversals", owner, obj{"date": "2026-02-28"}), 201, "reverse rent")
	if rev.str("entry_no") != "JV2026-000003" || rev.str("reversal_of_id") != rentEntry || rev.str("reference") != "JV2026-000001" {
		t.Fatalf("reversal = %v", rev.data())
	}
	if rl := objs(rev.data()["lines"]); rl[0]["credit"] != "1200" || rl[1]["debit"] != "1200" {
		t.Fatalf("reversal lines = %v", rl)
	}
	orig := a.expect(a.do("GET", base+"/journal-entries/"+rentEntry, owner, nil), 200, "original").data()
	if orig["status"] != "reversed" || orig["reversed_by_id"] != rev.id() {
		t.Fatalf("original after reversal = %v", orig)
	}
	a.expect(a.do("POST", base+"/journal-entries/"+rentEntry+"/reversals", owner, obj{"date": "2026-02-28"}), 422, "reverse twice")

	// ---------- ค้นหารายการผ่าน URL query ----------
	if r := a.expect(a.do("GET", base+"/journal-entries?status=posted&start_date=2026-01-01&end_date=2026-12-31&sort=entry_no", owner, nil), 200, "posted 2026"); !slices.Equal(field(r.list(), "entry_no"), []string{"JV2026-000002", "JV2026-000003"}) {
		t.Fatalf("posted 2026 = %v", field(r.list(), "entry_no"))
	}
	if r := a.expect(a.do("GET", base+"/journal-entries?account_id="+bank, owner, nil), 200, "by account"); r.total() != 1 {
		t.Fatalf("entries with bank = %d", r.total())
	}
	if r := a.expect(a.do("GET", base+"/journal-entries?q=jv2026&page_size=2", owner, nil), 200, "search no"); r.total() != 3 || len(r.list()) != 2 {
		t.Fatalf("q=jv2026 total=%d", r.total())
	}

	// ---------- รายงาน ----------
	tb := a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "trial balance").data()
	wantTB := map[string][4]string{ // opening, debit, credit, closing (เดบิตเป็นบวก)
		"1101": {"105000", "1200", "1200", "105000"},
		"1102": {"0", "30000.5", "0", "30000.5"},
		"3101": {"-100000", "0", "0", "-100000"},
		"4101": {"0", "0", "30000.5", "-30000.5"}, // รายได้เริ่มนับใหม่ทุกปีบัญชี
		"5101": {"0", "1200", "1200", "0"},
	}
	rows := objs(tb["rows"])
	if len(rows) != len(wantTB) {
		t.Fatalf("trial balance rows = %v", rows)
	}
	for _, r := range rows {
		code := r["account"].(obj)["code"].(string)
		got := [4]string{r["opening_balance"].(string), r["debit"].(string), r["credit"].(string), r["closing_balance"].(string)}
		if got != wantTB[code] {
			t.Errorf("TB %s = %v, want %v", code, got, wantTB[code])
		}
	}
	if tb["book"] != "trial-balance" {
		t.Fatalf("book = %v", tb["book"])
	}
	// หน้าเดียวเปลี่ยนรายงาน: เปลี่ยนแค่ book ค่าอื่นใน URL คงเดิม (parameter ที่ไม่ใช้ถูกเพิกเฉย)
	if b := a.expect(a.do("GET", base+"/reports?book=balance-sheet&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "switch book").data(); b["book"] != "balance-sheet" || b["as_of"] != "2026-12-31" {
		t.Fatalf("switch book = %v", b)
	}
	if tb["total_debit"] != "32400.5" || tb["total_credit"] != "32400.5" || tb["prior_years_earnings"] != "-5000" {
		t.Fatalf("TB totals = %v / %v / %v", tb["total_debit"], tb["total_credit"], tb["prior_years_earnings"])
	}
	a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2025-12-01&end_date=2026-01-31", owner, nil), 422, "TB across fiscal years")

	bs := a.expect(a.do("GET", base+"/reports?book=balance-sheet&end_date=2026-12-31", owner, nil), 200, "balance sheet").data()
	if bs["total_assets"] != "135000.5" || bs["total_liabilities"] != "0" || bs["retained_earnings_prior_years"] != "5000" ||
		bs["current_year_earnings"] != "30000.5" || bs["total_equity"] != "135000.5" || bs["total_liabilities_and_equity"] != "135000.5" {
		t.Fatalf("balance sheet = %v", bs)
	}
	assetLines := objs(bs["assets"])
	if got := field(assetLines, "code"); !slices.Equal(got, []string{"1000", "1101", "1102"}) || assetLines[0]["amount"] != "135000.5" || assetLines[0]["is_header"] != true || assetLines[1]["level"] != float64(1) {
		t.Fatalf("assets = %v", assetLines) // 1100 (ยอดศูนย์) ถูกซ่อน
	}

	is := a.expect(a.do("GET", base+"/reports?book=income-statement&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "income statement").data()
	if is["total_revenue"] != "30000.5" || is["total_expenses"] != "0" || is["net_income"] != "30000.5" || len(objs(is["expenses"])) != 0 {
		t.Fatalf("income statement = %v", is)
	}
	if n := a.expect(a.do("GET", base+"/reports?book=income-statement&start_date=2026-01-01&end_date=2026-01-31", owner, nil), 200, "IS Jan").data()["net_income"]; n != "-1200" {
		t.Fatalf("Jan net income = %v", n)
	}

	gl1 := a.expect(a.do("GET", base+"/reports?book=general-ledger&account_id="+cash+"&start_date=2026-01-01&end_date=2026-12-31&page_size=1", owner, nil), 200, "cash ledger p1")
	gl2 := a.expect(a.do("GET", base+"/reports?book=general-ledger&account_id="+cash+"&start_date=2026-01-01&end_date=2026-12-31&page_size=1&page=2", owner, nil), 200, "cash ledger p2")
	l1, l2 := objs(gl1.data()["lines"]), objs(gl2.data()["lines"])
	if gl1.data()["opening_balance"] != "105000" || gl1.total() != 2 || l1[0]["balance"] != "103800" || l1[0]["entry_no"] != "JV2026-000001" || l2[0]["balance"] != "105000" {
		t.Fatalf("cash ledger = %v / %v", gl1.body, gl2.body)
	}
	if o := a.expect(a.do("GET", base+"/reports?book=general-ledger&account_id="+revenue+"&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "revenue ledger").data()["opening_balance"]; o != "0" {
		t.Fatalf("revenue opening 2026 = %v (must reset at fiscal year start)", o)
	}

	// ---------- ลบ ----------
	a.expect(a.do("DELETE", base+"/accounts/"+cash, owner, nil), 409, "delete used account")
	a.expect(a.do("DELETE", base+"/accounts/"+assets, owner, nil), 409, "delete header with children")
	a.expect(a.do("DELETE", base+"/accounts/"+current, owner, nil), 204, "delete unused account")
	a.expect(a.do("DELETE", base+"/journals/"+journal, owner, nil), 409, "delete used journal")

	// ---------- เลขที่เอกสารเมื่อผ่านรายการพร้อมกัน: ไม่ซ้ำ ไม่ขาดตอน ----------
	const n = 10
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		nos []string
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a.do("POST", base+"/journal-entries", owner, entry("2026-03-01", "posted", ln(cash, "1", nil), ln(capital, nil, "1")))
			mu.Lock()
			defer mu.Unlock()
			if r.status == 201 {
				nos = append(nos, r.str("entry_no"))
			}
		}()
	}
	wg.Wait()
	slices.Sort(nos)
	var want []string
	for i := range n {
		want = append(want, fmt.Sprintf("JV2026-%06d", 4+i))
	}
	if !slices.Equal(nos, want) {
		t.Fatalf("concurrent entry numbers = %v", nos)
	}

	// ---------- สิทธิ์ ----------
	role := a.expect(a.do("POST", base+"/roles", owner, obj{"code": "clerk", "name": "Clerk", "permissions": []string{"ledger.entry:manage", "ledger.entry:read"}}), 201, "clerk role").id()
	a.expect(a.do("POST", base+"/members", owner, obj{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")
	a.expect(a.do("POST", base+"/journal-entries", staff, entry("2026-03-02", "", ln(cash, "1", nil), ln(capital, nil, "1"))), 201, "clerk creates draft")
	a.expect(a.do("POST", base+"/journal-entries", staff, entry("2026-03-02", "posted", ln(cash, "1", nil), ln(capital, nil, "1"))), 403, "clerk posts")
	a.expect(a.do("PATCH", base+"/journal-entries/"+rentEntry, staff, posted), 403, "clerk PATCH posted")
	a.expect(a.do("GET", base+"/reports?book=balance-sheet&end_date=2026-12-31", staff, nil), 403, "clerk reads report")
	a.expect(a.do("GET", base+"/journal-entries", outsider, nil), 403, "outsider")
	a.expect(a.do("GET", base+"/journal-entries", "", nil), 401, "no token")

	// ---------- บริษัท inactive = อ่านอย่างเดียว ----------
	companyBody["status"] = "inactive"
	a.expect(a.do("PUT", base, owner, companyBody), 200, "deactivate company")
	a.expect(a.do("POST", base+"/journals", owner, obj{"code": "RV", "name": "รับ"}), 422, "write to inactive company")
	a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "report of inactive company")
	companyBody["status"] = "active"
	a.expect(a.do("PUT", base, owner, companyBody), 200, "reactivate company")

	// ---------- tenant isolation ----------
	baseB := "/api/v1/companies/" + other
	a.expect(a.do("GET", baseB+"/journal-entries/"+rentEntry, owner, nil), 404, "entry of A via B")
	jB := a.expect(a.do("POST", baseB+"/journals", owner, obj{"code": "JV", "name": "B"}), 201, "journal in B (same code ok)").id()
	a.expect(a.do("POST", baseB+"/journal-entries", owner, obj{"journal_id": jB, "date": "2026-01-01", "lines": []obj{ln(cash, "1", nil)}}), 404, "account of A in B")

	// ---------- query ผิดรูปแบบ ----------
	for _, bad := range []string{
		"reports", "reports?book=balancesheet", "reports?book=trial-balance",
		"reports?book=trial-balance&start_date=2026-12-31&end_date=2026-01-01",
		"reports?book=balance-sheet&end_date=31122026", "reports?book=general-ledger&" + "start_date=2026-01-01&end_date=2026-12-31",
		"reports?book=general-ledger&account_id=x&start_date=2026-01-01&end_date=2026-12-31", "reports?book=general-ledger&account_id=" + cash + "&start_date=2026-01-01",
		"journal-entries?status=void", "accounts?type=other", "journal-entries?sort=total_debit",
	} {
		a.expect(a.do("GET", base+"/"+bad, owner, nil), 400, "bad query "+bad)
	}
}
