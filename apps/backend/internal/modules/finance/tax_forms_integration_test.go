package finance_test

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/platform/integration"
)

// TestTaxForms: ภ.ง.ด.3 / 53 (ใบแนบ, ลำดับที่, ไฟล์ส่งออก, ลำดับที่ใน 50 ทวิ), ภ.พ.30 (ภาษีขาย - ภาษีซื้อจัดซื้อ + เงินสดย่อย
// - ภาษีชำระเกินยกมา), รายงานภาษีขาย / ซื้อ, ทะเบียนการยื่น (1 แบบ / เดือน, ยกเลิก, ยกไป), ภาพรวมรายเดือน, สิทธิ์
func TestTaxForms(t *testing.T) {
	a := setupApp(t, []integration.CompanyCreatedHook{})
	owner := a.user("owner@example.com")
	outsider := a.user("outsider@example.com")
	me := a.expect(a.do("GET", "/api/v1/me", owner, nil), 200, "me").data()["id"].(string)
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "tax", "name": "Tax Co", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	acc := map[string]string{}
	for _, x := range objs(a.expect(a.do("GET", base+"/accounts/tree", owner, nil), 200, "tree").body["data"]) {
		acc[x["code"].(string)] = x["id"].(string)
	}
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BANK", "name": "ธนาคาร", "type": "bank", "bank_name": "KBANK",
		"account_no": "111-1-11111-1", "gl_account_id": acc["1101030100"]}), 201, "bank").id()
	svc3 := a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "SVC", "name": "ค่าบริการ", "rate": "3", "income_type": "3_tres"}), 201, "svc").id()
	rent5 := a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "RENT", "name": "ค่าเช่า", "rate": "5", "income_type": "40_5"}), 400, "bad income type")
	_ = rent5
	rent := a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "RENT", "name": "ค่าเช่า", "rate": "5", "income_type": "other"}), 201, "rent").id()
	dec := func(v any) decimal.Decimal { return decimal.RequireFromString(v.(string)) }
	expense := acc["5103000000"]
	if expense == "" {
		expense = acc["5313000000"]
	}

	pay := func(date, name, taxID, taxType string, lines []obj, net string) obj {
		return a.expect(a.do("POST", base+"/payments", owner, obj{"kind": "other", "date": date,
			"payee": obj{"name": name, "tax_id": taxID, "tax_type": taxType, "address": "1 ถนนสีลม กรุงเทพฯ"},
			"lines": lines, "methods": []obj{{"cash_account_id": bank, "amount": net}}}), 201, "pay "+name).data()
	}
	line := func(amount, wht string) obj {
		l := obj{"account_id": expense, "description": "ค่าบริการ", "amount": amount}
		if wht != "" {
			l["wht_type_id"] = wht
		}
		return l
	}
	// ต.ค.: นิติบุคคล A 10,000 (ภาษี 300)
	pay("2026-10-10", "บริษัท เอ จำกัด", "0105550000001", "juristic", []obj{line("10000", svc3)}, "9700")
	// พ.ย.: บุคคล B 2,000 (60), นิติบุคคล D (3 พ.ย.) 4,000 (120), นิติบุคคล C (7 พ.ย.) 5,000 (150) + ค่าเช่า 1,000 (50) + ไม่หัก 500,
	// นิติบุคคล E (20 พ.ย.) ยกเลิกแล้ว — ไม่นับ
	pay("2026-11-05", "นาย บี", "3100500000004", "individual", []obj{line("2000", svc3)}, "1940")
	pay("2026-11-03", "บริษัท ดี จำกัด", "0105550000004", "juristic", []obj{line("4000", svc3)}, "3880")
	c := pay("2026-11-07", "บริษัท ซี จำกัด", "0105550000003", "juristic", []obj{line("5000", svc3), line("1000", rent), line("500", "")}, "6300")
	e := pay("2026-11-20", "บริษัท อี จำกัด", "0105550000005", "juristic", []obj{line("1000", svc3)}, "970")
	a.expect(a.do("PATCH", base+"/payments/"+e["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel E")

	// ---------- ภ.ง.ด.53 พ.ย. ----------
	a.expect(a.do("GET", base+"/tax-forms/withholding?form=pp30&month=2026-11", owner, nil), 400, "pp30 is not withholding")
	a.expect(a.do("GET", base+"/tax-forms/withholding?form=pnd53&month=2026-13", owner, nil), 400, "bad month")
	a.expect(a.do("GET", base+"/tax-forms/withholding?form=pnd53&month=2026-11", outsider, nil), 403, "outsider")
	f53 := a.expect(a.do("GET", base+"/tax-forms/withholding?form=pnd53&month=2026-11", owner, nil), 200, "pnd53").data()
	sum := f53["summary"].(obj)
	if sum["payees"] != float64(2) || sum["lines"] != float64(3) || !dec(sum["base"]).Equal(decimal.NewFromInt(10000)) || !dec(sum["wht"]).Equal(decimal.NewFromInt(320)) {
		t.Fatalf("pnd53 summary = %v", sum)
	}
	ls := objs(f53["lines"])
	if ls[0]["seq"] != float64(1) || ls[0]["name"] != "บริษัท ดี จำกัด" || ls[1]["seq"] != float64(2) || ls[2]["seq"] != float64(2) ||
		ls[2]["income_type"] != "other" || ls[1]["income_type"] != "3_tres" || ls[0]["condition"] != float64(1) || f53["filing"] != nil {
		t.Fatalf("pnd53 lines = %v", ls)
	}
	f3 := a.expect(a.do("GET", base+"/tax-forms/withholding?form=pnd3&month=2026-11", owner, nil), 200, "pnd3").data()
	if s := f3["summary"].(obj); s["payees"] != float64(1) || !dec(s["wht"]).Equal(decimal.NewFromInt(60)) {
		t.Fatalf("pnd3 = %v", s)
	}
	// ไฟล์ส่งออก
	exp := a.do("GET", base+"/tax-forms/withholding/export?form=pnd53&month=2026-11", owner, nil)
	if exp.status != 200 || !strings.HasPrefix(exp.raw, "1|0105550000004||บริษัท ดี จำกัด|1 ถนนสีลม กรุงเทพฯ|03/11/2569|3_tres|3.00|4000.00|120.00|1\r\n") ||
		strings.Count(exp.raw, "\r\n") != 3 {
		t.Fatalf("export = %d %q", exp.status, exp.raw)
	}
	// ลำดับที่ใน 50 ทวิ ของใบจ่าย C = 2, ใบที่ยกเลิก = 0
	if p := a.expect(a.do("GET", base+"/payments/"+c["id"].(string), owner, nil), 200, "payment C").data(); p["wht_form_seq"] != float64(2) {
		t.Fatalf("payment C seq = %v", p["wht_form_seq"])
	}
	if p := a.expect(a.do("GET", base+"/payments/"+e["id"].(string), owner, nil), 200, "payment E").data(); p["wht_form_seq"] != float64(0) {
		t.Fatalf("cancelled payment seq = %v", p["wht_form_seq"])
	}

	// ---------- ภ.พ.30: ภาษีขาย (ขายบริการในบริษัทซื้อขาย) - ภาษีซื้อเงินสดย่อย ----------
	uom := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "JOB", "name": "งาน"}), 201, "uom").id()
	item := a.expect(a.do("POST", base+"/items", owner, obj{"code": "S1", "name": "ค่าบริการ", "type": "service", "uom_id": uom}), 201, "item").id()
	cust := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C1", "name": "ลูกค้า หนึ่ง", "tax_id": "0105551234567", "tax_type": "juristic",
		"payment_term_days": 30}), 201, "customer").id()
	sell := func(date, price string) {
		a.expect(a.do("POST", base+"/sales-invoices", owner, obj{"customer_id": cust, "date": date,
			"lines": []obj{{"item_id": item, "quantity": "1", "unit_price": price}}}), 201, "sale "+date)
	}
	sell("2026-10-15", "10000")
	sell("2026-11-15", "1000")
	// เงินสดย่อย: ตั้งวงเงิน -> ใบสำคัญจ่ายมี VAT 2,000 + 140 -> เบิกชดเชย (การเงินจ่าย)
	fund := a.expect(a.do("POST", base+"/petty-cash-funds", owner, obj{"code": "PC", "name": "เงินสดย่อย", "custodian_user_id": me, "imprest_amount": "10000"}), 201, "fund").id()
	payReq := func(id, amount, date string) {
		a.expect(a.do("POST", base+"/payments", owner, obj{"kind": "petty_cash", "petty_cash_request_id": id, "date": date,
			"methods": []obj{{"cash_account_id": bank, "amount": amount}}}), 201, "pay request")
	}
	est := a.expect(a.do("POST", base+"/petty-cash-replenishments", owner, obj{"fund_id": fund, "kind": "establish", "date": "2026-11-01"}), 201, "establish").id()
	payReq(est, "10000", "2026-11-01")
	v := a.expect(a.do("POST", base+"/petty-cash-vouchers", owner, obj{"fund_id": fund, "date": "2026-11-10", "vat_rate": "7",
		"payee": obj{"name": "บริษัท เครื่องเขียน จำกัด", "tax_id": "0105555000001", "branch_no": "00000"}, "tax_invoice": obj{"no": "TI-001", "date": "2026-11-10"},
		"lines": []obj{{"account_id": acc["5309050000"], "description": "เครื่องเขียน", "amount": "2000"}}}), 201, "voucher").id()
	a.expect(a.do("POST", base+"/petty-cash-vouchers/"+v+"/approve", owner, nil), 200, "approve")
	rep := a.expect(a.do("POST", base+"/petty-cash-replenishments", owner, obj{"fund_id": fund, "kind": "replenish", "date": "2026-11-12",
		"voucher_ids": []string{v}}), 201, "replenish").id()
	payReq(rep, "2140", "2026-11-12")

	oct := a.expect(a.do("GET", base+"/tax-forms/pp30?month=2026-10", owner, nil), 200, "pp30 oct").data()
	if !dec(oct["sales"].(obj)["tax"]).Equal(decimal.NewFromInt(700)) || !dec(oct["payable"]).Equal(decimal.NewFromInt(700)) || !dec(oct["carry_in"]).IsZero() {
		t.Fatalf("pp30 oct = %v", oct)
	}
	nov := a.expect(a.do("GET", base+"/tax-forms/pp30?month=2026-11", owner, nil), 200, "pp30 nov").data()
	if !dec(nov["sales"].(obj)["tax"]).Equal(decimal.NewFromInt(70)) || !dec(nov["purchases"].(obj)["tax"]).Equal(decimal.NewFromInt(140)) ||
		!dec(nov["overpaid"]).Equal(decimal.NewFromInt(70)) || !dec(nov["payable"]).IsZero() {
		t.Fatalf("pp30 nov = %v", nov)
	}
	// รายงานภาษีซื้อ พ.ย. = ใบสำคัญจ่ายเงินสดย่อย (เลขที่ใบกำกับของผู้ขาย)
	in := a.expect(a.do("GET", base+"/tax-forms/vat-report?kind=input&month=2026-11", owner, nil), 200, "input report").data()
	if rows := objs(in["rows"]); len(rows) != 1 || rows[0]["document_no"] != "TI-001" || rows[0]["seq"] != float64(1) || !dec(in["tax"]).Equal(decimal.NewFromInt(140)) {
		t.Fatalf("input report = %v", in)
	}
	out := a.expect(a.do("GET", base+"/tax-forms/vat-report?kind=output&month=2026-10", owner, nil), 200, "output report").data()
	if rows := objs(out["rows"]); len(rows) != 1 || rows[0]["party_name"] != "ลูกค้า หนึ่ง" || !dec(out["base"]).Equal(decimal.NewFromInt(10000)) {
		t.Fatalf("output report = %v", out)
	}
	a.expect(a.do("GET", base+"/tax-forms/vat-report?kind=x&month=2026-10", owner, nil), 400, "bad kind")

	// ---------- ทะเบียนการยื่น ----------
	filing := func(body obj, status int, what string) resp { return a.expect(a.do("POST", base+"/tax-filings", owner, body), status, what) }
	a.expect(a.do("POST", base+"/tax-filings", outsider, obj{"form": "pnd3", "month": "2026-10", "filed_date": "2026-11-07", "amount": "0"}), 403, "outsider filing")
	a.expect(a.do("GET", base+"/tax-filings", outsider, nil), 403, "outsider list")
	filing(obj{"form": "pnd53", "month": "2026-10", "filed_date": "2026-11-07", "amount": "300", "carry_forward": "10"}, 400, "carry forward on pnd")
	filing(obj{"form": "pp30", "month": "2026-11", "filed_date": "2026-10-31", "amount": "0"}, 400, "filed before month")
	filing(obj{"form": "pnd53", "month": "2026-10", "filed_date": "2026-11-07", "amount": "-1"}, 400, "negative")
	f1 := filing(obj{"form": "pnd53", "month": "2026-10", "filed_date": "2026-11-07", "amount": "300", "reference": "RD-1"}, 201, "file pnd53 oct").data()
	filing(obj{"form": "pnd53", "month": "2026-10", "filed_date": "2026-11-08", "amount": "300"}, 409, "duplicate filing")
	a.expect(a.do("PATCH", base+"/tax-filings/"+f1["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel filing")
	a.expect(a.do("PATCH", base+"/tax-filings/"+f1["id"].(string), owner, obj{"status": "cancelled"}), 422, "cancel twice")
	filing(obj{"form": "pnd53", "month": "2026-10", "filed_date": "2026-11-08", "amount": "300", "surcharge": "4.5"}, 201, "refile")
	filing(obj{"form": "pp30", "month": "2026-10", "filed_date": "2026-11-15", "amount": "700"}, 201, "pp30 oct")
	filing(obj{"form": "pp30", "month": "2026-11", "filed_date": "2026-12-15", "amount": "0", "carry_forward": "70"}, 201, "pp30 nov overpaid")
	dec26 := a.expect(a.do("GET", base+"/tax-forms/pp30?month=2026-12", owner, nil), 200, "pp30 dec").data()
	if !dec(dec26["carry_in"]).Equal(decimal.NewFromInt(70)) || !dec(dec26["overpaid"]).Equal(decimal.NewFromInt(70)) || dec26["prev_filed"] != true {
		t.Fatalf("pp30 dec carry in = %v", dec26)
	}
	if f := a.expect(a.do("GET", base+"/tax-forms/withholding?form=pnd53&month=2026-10", owner, nil), 200, "pnd53 oct").data()["filing"].(obj); f["total"] != "304.5" {
		t.Fatalf("filing on form = %v", f)
	}
	list := objs(a.expect(a.do("GET", base+"/tax-filings?year=2026&status=filed", owner, nil), 200, "list").body["data"])
	if len(list) != 3 || list[0]["form"] != "pnd53" || list[0]["month"] != "2026-10" {
		t.Fatalf("filings = %v", list)
	}

	// ---------- ภาพรวมรายเดือน ----------
	ov := a.expect(a.do("GET", base+"/tax-forms/overview?year=2026", owner, nil), 200, "overview").data()
	months := objs(ov["months"])
	if len(months) != 12 || ov["can_see_filings"] != true {
		t.Fatalf("overview = %v", ov)
	}
	octRow, novRow := months[9], months[10]
	if !dec(octRow["pnd53"].(obj)["amount"]).Equal(decimal.NewFromInt(300)) || octRow["pnd53"].(obj)["filing"] == nil ||
		!dec(octRow["pp30"].(obj)["amount"]).Equal(decimal.NewFromInt(700)) || !dec(novRow["pnd53"].(obj)["amount"]).Equal(decimal.NewFromInt(320)) ||
		novRow["pnd53"].(obj)["payees"] != float64(2) || !dec(novRow["pnd3"].(obj)["amount"]).Equal(decimal.NewFromInt(60)) ||
		!dec(novRow["pp30"].(obj)["amount"]).Equal(decimal.NewFromInt(-70)) {
		t.Fatalf("overview oct / nov = %v / %v", octRow, novRow)
	}
	a.expect(a.do("GET", base+"/tax-forms/overview?year=99", owner, nil), 400, "bad year")
}
