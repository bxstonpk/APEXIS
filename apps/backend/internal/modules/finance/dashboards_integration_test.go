package finance_test

import "testing"

// ภาพรวมแอปการเงิน: ยอดคงเหลือจาก ledger (รวมยอดยกมาที่ลงด้วยรายการบัญชี), รับ / จ่ายจริง, โอน, ภาษีหัก ณ ที่จ่าย,
// 12 เดือน, เอกสารล่าสุด — เอกสารที่ยกเลิกไม่นับ
func TestFinanceDashboard(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "dash", "name": "DASH", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	acc := map[string]string{}
	for code, typ := range map[string]string{"1101": "asset", "1102": "asset", "2131": "liability", "3100": "equity", "4900": "revenue", "5300": "expense"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	gen := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "GEN", "name": "ทั่วไป"}), 201, "journal").id()
	// ยอดยกมาเงินสด 10,000 ลงด้วยรายการบัญชี (ไม่ใช่เอกสารการเงิน)
	a.expect(a.do("POST", base+"/journal-entries", owner, obj{"journal_id": gen, "date": "2026-01-01", "description": "ยกมา", "status": "posted",
		"lines": []obj{{"account_id": acc["1101"], "debit": "10000", "credit": "0"}, {"account_id": acc["3100"], "debit": "0", "credit": "10000"}}}), 201, "opening")
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": acc["1101"]}), 201, "cash").id()
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BANK", "name": "ธนาคาร", "type": "bank", "bank_name": "KBANK", "account_no": "222-2-22222-2", "gl_account_id": acc["1102"]}), 201, "bank").id()
	svc3 := a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "SVC", "name": "ค่าบริการ", "rate": "3", "income_type": "3_tres"}), 201, "wht 3%").id()
	a.expect(a.do("PUT", base+"/finance-settings", owner, obj{"receipt_journal_id": gen, "payment_journal_id": gen, "transfer_journal_id": gen,
		"wht_pnd3_account_id": acc["2131"]}), 200, "settings")

	a.expect(a.do("POST", base+"/receipts", owner, obj{"kind": "other", "date": "2026-03-05", "payer": obj{"name": "ลูกค้าเงินสด"},
		"lines": []obj{{"account_id": acc["4900"], "description": "รายได้อื่น", "amount": "1000"}}, "methods": []obj{{"cash_account_id": bank, "amount": "1000"}}}), 201, "receipt")
	a.expect(a.do("POST", base+"/payments", owner, obj{"kind": "other", "date": "2026-03-12",
		"payee":   obj{"name": "นาย ก", "tax_id": "1101700000000", "tax_type": "individual"},
		"lines":   []obj{{"account_id": acc["5300"], "description": "ค่าออกแบบ", "amount": "2500", "wht_type_id": svc3}},
		"methods": []obj{{"cash_account_id": cash, "amount": "2425"}}}), 201, "payment")
	a.expect(a.do("POST", base+"/cash-transfers", owner, obj{"date": "2026-03-20", "from_cash_account_id": cash, "to_cash_account_id": bank, "amount": "500"}), 201, "transfer")
	cancelled := a.expect(a.do("POST", base+"/cash-transfers", owner, obj{"date": "2026-03-21", "from_cash_account_id": cash, "to_cash_account_id": bank, "amount": "99"}), 201, "transfer 2").id()
	a.expect(a.do("PATCH", base+"/cash-transfers/"+cancelled, owner, obj{"status": "cancelled"}), 200, "cancel transfer 2")

	a.expect(a.do("GET", base+"/dashboards/finance?end_date=2026-03-31", owner, nil), 400, "start_date missing")
	d := a.expect(a.do("GET", base+"/dashboards/finance?start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "dashboard").data()

	ca := objs(d["cash_accounts"])
	if len(ca) != 2 || ca[0]["account"].(obj)["code"] != "BANK" || ca[0]["type"] != "bank" || ca[0]["balance"] != "1500" ||
		ca[1]["account"].(obj)["code"] != "CASH" || ca[1]["balance"] != "7075" || d["cash_total"] != "8575" {
		t.Fatalf("cash accounts = %v total %v", ca, d["cash_total"])
	}
	if d["received"] != "1000" || d["paid"] != "2425" || d["net_flow"] != "-1425" || d["transfer_count"] != float64(1) {
		t.Fatalf("flows = %v / %v / %v / %v", d["received"], d["paid"], d["net_flow"], d["transfer_count"])
	}
	if w := d["wht"].(obj); w["deducted"] != "75" || w["suffered"] != "0" {
		t.Fatalf("wht = %v", w)
	}
	m := objs(d["monthly"])
	if len(m) != 12 || m[0]["month"] != "2025-04" || m[11]["month"] != "2026-03" || m[11]["received"] != "1000" || m[11]["paid"] != "2425" || m[10]["paid"] != "0" {
		t.Fatalf("monthly = %v", m)
	}
	recent := objs(d["recent"])
	if len(recent) != 3 || recent[0]["kind"] != "transfer" || recent[0]["party"] != "เงินสด → ธนาคาร" || recent[0]["amount"] != "500" ||
		recent[1]["kind"] != "payment" || recent[1]["party"] != "นาย ก" || recent[1]["amount"] != "2425" || recent[2]["kind"] != "receipt" {
		t.Fatalf("recent = %v", recent)
	}
	// ยอดคงเหลือ ณ end_date: ก่อนรายการเดือนมีนาคม = ยอดยกมาเท่านั้น
	feb := a.expect(a.do("GET", base+"/dashboards/finance?start_date=2026-02-01&end_date=2026-02-28", owner, nil), 200, "february").data()
	if feb["cash_total"] != "10000" || feb["received"] != "0" {
		t.Fatalf("february = %v / %v", feb["cash_total"], feb["received"])
	}
	outsider := a.user("outsider@example.com")
	a.expect(a.do("GET", base+"/dashboards/finance?start_date=2026-03-01&end_date=2026-03-31", outsider, nil), 403, "no permission")
}
