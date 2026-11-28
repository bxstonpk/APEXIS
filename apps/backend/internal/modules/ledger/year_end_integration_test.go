package ledger_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"apexis/backend/internal/modules/identity"
	"apexis/backend/internal/modules/ledger"
	"apexis/backend/internal/modules/organization"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/reports"
)

// setupYE: เหมือน setup แต่ลงทะเบียน hook สร้างบริษัท (ผังบัญชีมาตรฐาน + ค่าตั้ง) และ hook สร้างปีบัญชี (รายการกลับที่รอ)
func setupYE(t *testing.T) api {
	a := setup(t) // ล้างตาราง / migrate
	_ = a.srv
	dsn := os.Getenv("IDENTITY_TEST_DSN")
	db, err := database.OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
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
	org.OnCompanyCreated(lm.ProvisionCompany)
	org.OnFiscalYearCreated(lm.FiscalYearCreated)
	reg := reports.NewRegistry()
	lm.RegisterReports(reg)
	mux.Handle("GET /api/v1/companies/{companyID}/reports", idm.Authenticate(reg))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api{t: t, srv: srv, db: db}
}

func dbdValue(r resp, key string, col int) string {
	for _, row := range objs(r.data()["rows"]) {
		if row["key"] == key {
			return row["amounts"].([]any)[col].(string)
		}
	}
	return "missing"
}

func TestYearEndCloseAndAdjustments(t *testing.T) {
	a := setupYE(t)
	owner := a.user("ye-owner@example.com")
	staff := a.user("ye-staff@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "ye", "name": "YE", "business_type": "service"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2025, "start_date": "2025-01-01"}), 201, "FY2025")

	acc := func(code string) string {
		t.Helper()
		l := a.expect(a.do("GET", base+"/accounts?q="+code, owner, nil), 200, "account "+code).list()
		for _, x := range l {
			if x["code"] == code {
				return x["id"].(string)
			}
		}
		t.Fatalf("account %s not found", code)
		return ""
	}
	cash, capital, revenue, expense := acc("1101010100"), acc("3100000000"), acc("4101000000"), acc("5313000000")
	accrued, accruedExp, suspense, retained := acc("1103020400"), acc("2102020100"), acc("6400000000"), acc("3302000000")
	gen := ""
	for _, j := range a.expect(a.do("GET", base+"/journals", owner, nil), 200, "journals").list() {
		if j["code"] == "GEN" {
			gen = j["id"].(string)
		}
	}
	entry := func(date string, lines ...obj) obj {
		return obj{"journal_id": gen, "date": date, "description": "t " + date, "status": "posted", "lines": lines}
	}
	adj := func(date, template string, auto *bool, lines ...obj) obj {
		b := obj{"journal_id": gen, "date": date, "description": "adj " + date, "template": template, "adjusting_period": date == "2025-12-31", "lines": lines}
		if auto != nil {
			b["auto_reverse"] = *auto
		}
		return b
	}
	f := false

	// ---------- ปี 2025 ----------
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2025-03-10", ln(cash, "100000", "0"), ln(capital, "0", "100000"))), 201, "capital")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2025-06-15", ln(cash, "50000", "0"), ln(revenue, "0", "50000"))), 201, "revenue")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2025-07-01", ln(expense, "8000", "0"), ln(cash, "0", "8000"))), 201, "expense")

	// รายการปรับปรุงสิ้นปี: รายได้ค้างรับ (งวดปรับปรุง) — ปี 2026 ยังไม่สร้าง = รายการกลับรอผ่าน
	accrual := a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-12-31", "accrued_revenue", nil, ln(accrued, "10000", "0"), ln(revenue, "0", "10000"))), 201, "accrual")
	if accrual.data()["adjusting_period"] != true || accrual.data()["auto_reverse"] != true {
		t.Fatalf("accrual = %v", accrual.data())
	}
	rev := accrual.data()["auto_reversal"].(obj)
	if rev["status"] != "draft" || rev["entry_no"] != nil || rev["date"] != "2026-01-01" {
		t.Fatalf("pending reversal = %v", rev)
	}
	a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-11-15", "accrued_revenue", nil, ln(accrued, "1", "0"), ln(revenue, "0", "1"))), 422, "adjusting not at period end")
	a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-12-31", "bogus", nil, ln(accrued, "1", "0"), ln(revenue, "0", "1"))), 400, "bad template")

	// ปิดงวด 1–12 ไม่ได้ปิดบัญชีเพราะงวดเปิด
	a.expect(a.do("POST", base+"/year-end-closings/2025/close", owner, nil), 422, "close with open periods")
	periods := a.expect(a.do("GET", base+"/fiscal-periods?fiscal_year=2025&sort=start_date&page_size=20", owner, nil), 200, "periods").list()
	for _, p := range periods {
		a.expect(a.do("PATCH", base+"/fiscal-periods/"+p["id"].(string), owner, obj{"status": "closed"}), 200, "close period")
	}
	// งวดปกติปิดแล้ว ลงรายการไม่ได้ — แต่งวดปรับปรุง (วันสุดท้ายของปี) ยังลงได้
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2025-12-20", ln(expense, "1", "0"), ln(cash, "0", "1"))), 422, "closed period")
	bad := entry("2025-12-30", ln(expense, "1", "0"), ln(cash, "0", "1"))
	bad["adjusting_period"] = true
	a.expect(a.do("POST", base+"/journal-entries", owner, bad), 422, "adjusting period wrong date")
	a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-12-31", "accrued_expense", nil, ln(expense, "2000", "0"), ln(accruedExp, "0", "2000"))), 201, "accrued expense in adjusting period")

	// บัญชีพักมียอด -> ปิดไม่ได้
	a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-12-31", "other", &f, ln(cash, "100", "0"), ln(suspense, "0", "100"))), 201, "suspense in")
	a.expect(a.do("POST", base+"/year-end-closings/2025/close", owner, nil), 422, "close with suspense")
	a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-12-31", "other", &f, ln(suspense, "100", "0"), ln(cash, "0", "100"))), 201, "suspense out")

	// ---------- ปีใหม่เริ่มได้ทันทีโดยไม่ต้องปิดปีก่อน ----------
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	got := a.expect(a.do("GET", base+"/journal-entries/"+accrual.id(), owner, nil), 200, "accrual after FY2026").data()["auto_reversal"].(obj)
	if got["status"] != "posted" || got["entry_no"] == nil || got["date"] != "2026-01-01" {
		t.Fatalf("reversal after FY2026 = %v", got)
	}
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-01-15", ln(cash, "10000", "0"), ln(revenue, "0", "10000"))), 201, "invoice 2026 while 2025 unclosed")
	if n := a.expect(a.do("GET", base+"/adjusting-entries?template=accrued_revenue", owner, nil), 200, "list adjustments").total(); n != 1 {
		t.Fatalf("accrued_revenue adjustments = %d", n)
	}

	bs := func(what string) resp {
		return a.expect(a.do("GET", base+"/reports?book=dbd-balance-sheet&end_date=2026-01-31", owner, nil), 200, what)
	}
	is26 := func(what string) resp {
		return a.expect(a.do("GET", base+"/reports?book=dbd-income-statement&end_date=2026-01-31", owner, nil), 200, what)
	}
	before := bs("BS before close")
	if dbdValue(before, "retained_unclosed", 0) != "50000" || dbdValue(before, "total_assets", 0) != dbdValue(before, "total_liabilities_equity", 0) {
		t.Fatalf("BS 2026 before close: unclosed=%s assets=%s le=%s", dbdValue(before, "retained_unclosed", 0), dbdValue(before, "total_assets", 0), dbdValue(before, "total_liabilities_equity", 0))
	}
	isBefore := is26("IS 2026 before")
	// รายได้ปีใหม่ = ใบแจ้งหนี้ 10000 - รายการกลับรายได้ค้างรับ 10000 = 0; ค่าใช้จ่าย = -2000 (กลับค่าใช้จ่ายค้างจ่าย)
	if dbdValue(isBefore, "revenue_services", 0) != "0" || dbdValue(isBefore, "net_profit", 0) != "2000" {
		t.Fatalf("IS 2026 = revenue %s net %s", dbdValue(isBefore, "revenue_services", 0), dbdValue(isBefore, "net_profit", 0))
	}

	// ---------- ปิดบัญชีปี 2025 (มีรายการปี 2026 อยู่แล้ว) ----------
	a.expect(a.do("POST", base+"/year-end-closings/2025/close", staff, nil), 403, "close without permission")
	closed := a.expect(a.do("POST", base+"/year-end-closings/2025/close", owner, nil), 200, "close 2025").data()
	if closed["status"] != "closed" || closed["closing"].(obj)["net_income"] != "50000" || closed["closing"].(obj)["entry_no"] == "" {
		t.Fatalf("closed = %v", closed)
	}
	a.expect(a.do("POST", base+"/year-end-closings/2025/close", owner, nil), 409, "close twice")
	// งบกำไรขาดทุนปี 2025 ยังแสดงผลการดำเนินงาน (รวมงวดปรับปรุง ไม่รวมรายการปิด)
	is25 := a.expect(a.do("GET", base+"/reports?book=income-statement&start_date=2025-01-01&end_date=2025-12-31", owner, nil), 200, "IS 2025").data()
	if is25["net_income"] != "50000" {
		t.Fatalf("IS 2025 net = %v", is25["net_income"])
	}
	tb := a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2025-01-01&end_date=2025-12-31", owner, nil), 200, "TB 2025").data()
	if tb["total_debit"] != tb["total_credit"] {
		t.Fatalf("TB unbalanced %v / %v", tb["total_debit"], tb["total_credit"])
	}
	after := bs("BS after close")
	if dbdValue(after, "retained_unclosed", 0) != "0" || dbdValue(after, "retained_unappropriated", 0) != "50000" ||
		dbdValue(after, "total_assets", 0) != dbdValue(before, "total_assets", 0) || dbdValue(after, "total_liabilities_equity", 0) != dbdValue(after, "total_assets", 0) {
		t.Fatalf("BS 2026 after close: unclosed=%s retained=%s", dbdValue(after, "retained_unclosed", 0), dbdValue(after, "retained_unappropriated", 0))
	}
	if dbdValue(is26("IS 2026 after"), "net_profit", 0) != "2000" {
		t.Fatal("IS 2026 changed by closing 2025")
	}
	bal := a.expect(a.do("GET", base+"/reports?book=balance-sheet&end_date=2026-01-31", owner, nil), 200, "BS").data()
	_ = bal
	_ = retained
	// ปีที่ปิดแล้วลงรายการไม่ได้ (รวมงวดปรับปรุง); ปีใหม่ลงได้ตามปกติ
	a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2025-12-31", "other", &f, ln(cash, "1", "0"), ln(revenue, "0", "1"))), 422, "post into closed year")
	a.expect(a.do("POST", base+"/journal-entries", owner, entry("2026-02-01", ln(expense, "500", "0"), ln(cash, "0", "500"))), 201, "2026 after close")

	// ---------- เปิดปี 2025 ----------
	reopened := a.expect(a.do("POST", base+"/year-end-closings/2025/reopen", owner, nil), 200, "reopen").data()
	if reopened["status"] != "open" || reopened["closing"].(obj)["reversal_no"] == "" {
		t.Fatalf("reopened = %v", reopened)
	}
	if dbdValue(bs("BS after reopen"), "retained_unclosed", 0) != "50000" {
		t.Fatal("reopen did not restore the unclosed line")
	}

	// ---------- ยกเลิกรายการปรับปรุง: รายการกลับที่ผ่านแล้วถูกกลับด้วย / งวดของรายการกลับปิด = 422 ----------
	acc2 := a.expect(a.do("POST", base+"/adjusting-entries", owner, adj("2026-02-28", "accrued_revenue", nil, ln(accrued, "3000", "0"), ln(revenue, "0", "3000"))), 201, "accrual Feb").data()
	if acc2["auto_reversal"].(obj)["status"] != "posted" || acc2["auto_reversal"].(obj)["date"] != "2026-03-01" {
		t.Fatalf("Feb accrual reversal = %v", acc2["auto_reversal"])
	}
	a.expect(a.do("POST", base+"/journal-entries/"+acc2["id"].(string)+"/reversals", owner, obj{"date": "2026-02-28"}), 201, "cancel Feb accrual")
	revAfter := a.expect(a.do("GET", base+"/journal-entries/"+acc2["auto_reversal"].(obj)["id"].(string), owner, nil), 200, "Feb reversal").data()
	if revAfter["status"] != "reversed" {
		t.Fatalf("auto reversal after cancel = %v", revAfter["status"])
	}
	// งวด ม.ค. 2026 ปิด -> ยกเลิกรายการปรับปรุงปี 2025 (รายการกลับลง 1 ม.ค.) ไม่ได้
	p26 := a.expect(a.do("GET", base+"/fiscal-periods?fiscal_year=2026&sort=start_date&page_size=20", owner, nil), 200, "periods 2026").list()
	a.expect(a.do("PATCH", base+"/fiscal-periods/"+p26[0]["id"].(string), owner, obj{"status": "closed"}), 200, "close Jan 2026")
	a.expect(a.do("POST", base+"/journal-entries/"+accrual.id()+"/reversals", owner, obj{"date": "2025-12-31"}), 422, "cancel accrual with closed reversal period")

	// ---------- เปิดปี N ไม่ได้เมื่อปี N+1 ปิดแล้ว ----------
	a.expect(a.do("POST", base+"/year-end-closings/2025/close", owner, nil), 200, "close 2025 again")
	for _, p := range p26[1:] {
		a.expect(a.do("PATCH", base+"/fiscal-periods/"+p["id"].(string), owner, obj{"status": "closed"}), 200, "close 2026 period")
	}
	a.expect(a.do("POST", base+"/year-end-closings/2026/close", owner, nil), 200, "close 2026")
	a.expect(a.do("POST", base+"/year-end-closings/2025/reopen", owner, nil), 422, "reopen with next year closed")

	years := a.expect(a.do("GET", base+"/year-end-closings", owner, nil), 200, "list").list()
	if len(years) != 2 || years[0]["fiscal_year"] != float64(2026) || years[1]["status"] != "closed" || years[1]["deadline"] != "2026-04-30" {
		t.Fatalf("year-end list = %v", years)
	}
}
