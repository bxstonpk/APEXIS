package ledger_test

import "testing"

// ภาพรวมแอปบัญชี: รายได้ / ค่าใช้จ่าย ช่วงนี้ vs ช่วงก่อน, 12 เดือน, บัญชีพัก, จำนวนรายการ, รายการล่าสุด, งวด
func TestAccountingDashboard(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "dash", "name": "DASH"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	journal := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "JV", "name": "ทั่วไป"}), 201, "journal").id()
	acc := map[string]string{}
	for code, typ := range map[string]string{"1101": "asset", "4100": "revenue", "5100": "expense", "6100": "suspense"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	ln := func(account, debit, credit string) obj {
		return obj{"account_id": acc[account], "debit": debit, "credit": credit}
	}
	entry := func(date, status, desc string, lines ...obj) {
		t.Helper()
		a.expect(a.do("POST", base+"/journal-entries", owner, obj{"journal_id": journal, "date": date, "description": desc, "status": status, "lines": lines}), 201, desc)
	}
	entry("2026-02-10", "posted", "ขายเดือนก่อน", ln("1101", "800", "0"), ln("4100", "0", "800"))
	entry("2026-03-05", "posted", "ขาย", ln("1101", "1000", "0"), ln("4100", "0", "1000"))
	entry("2026-03-06", "posted", "ค่าเช่า", ln("5100", "300", "0"), ln("1101", "0", "300"))
	entry("2026-03-07", "posted", "เงินไม่ทราบที่มา", ln("1101", "50", "0"), ln("6100", "0", "50"))
	entry("2026-03-08", "", "ร่าง", ln("5100", "10", "0"), ln("1101", "0", "10"))

	a.expect(a.do("GET", base+"/dashboards/accounting?start_date=2026-03-01", owner, nil), 400, "end_date missing")
	a.expect(a.do("GET", base+"/dashboards/accounting?start_date=2026-03-31&end_date=2026-03-01", owner, nil), 400, "start after end")
	d := a.expect(a.do("GET", base+"/dashboards/accounting?start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "dashboard").data()

	inc, prev := d["income"].(obj), d["previous"].(obj)
	if inc["revenue"] != "1000" || inc["expense"] != "300" || inc["net_income"] != "700" {
		t.Fatalf("income = %v", inc)
	}
	// ช่วงก่อน = 31 วันก่อน 1 มี.ค. (29 ม.ค. - 28 ก.พ.) ครอบคลุมรายการ 10 ก.พ.
	if prev["revenue"] != "800" || prev["net_income"] != "800" {
		t.Fatalf("previous = %v", prev)
	}
	if d["suspense_balance"] != "-50" {
		t.Fatalf("suspense = %v", d["suspense_balance"])
	}
	m := objs(d["monthly"])
	if len(m) != 12 || m[0]["month"] != "2025-04" || m[11]["month"] != "2026-03" || m[11]["revenue"] != "1000" || m[10]["revenue"] != "800" || m[9]["revenue"] != "0" {
		t.Fatalf("monthly = %v", m)
	}
	if e := d["entries"].(obj); e["draft_count"] != float64(1) || e["posted_count"] != float64(3) {
		t.Fatalf("entries = %v", e)
	}
	recent := objs(d["recent_entries"])
	if len(recent) != 4 || recent[0]["description"] != "เงินไม่ทราบที่มา" || recent[0]["amount"] != "50" || recent[0]["entry_no"] == "" {
		t.Fatalf("recent = %v", recent)
	}
	if p := d["period"].(obj); p["status"] != "open" || p["start_date"] != "2026-03-01" || p["end_date"] != "2026-03-31" || p["fiscal_year"] != float64(2026) {
		t.Fatalf("period = %v", p)
	}
	none := a.expect(a.do("GET", base+"/dashboards/accounting?start_date=2030-01-01&end_date=2030-01-31", owner, nil), 200, "no period").data()
	if none["period"] != nil {
		t.Fatalf("period outside fiscal years = %v", none["period"])
	}
	outsider := a.user("outsider@example.com")
	a.expect(a.do("GET", base+"/dashboards/accounting?start_date=2026-03-01&end_date=2026-03-31", outsider, nil), 403, "no permission")
}
