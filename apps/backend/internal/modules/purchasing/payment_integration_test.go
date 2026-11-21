package purchasing_test

import (
	"testing"
)

// เลขที่เอกสารตั้งค่าได้, ทะเบียนเงินสด / ธนาคาร, ใบจ่ายชำระหลายใบแจ้งหนี้ / หลายช่องทาง, ภาษีหัก ณ ที่จ่าย, รายงาน
func TestSupplierPayments(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")

	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	cashGL, bankGL, vatAcc := account("1101", "asset"), account("1102", "asset"), account("1160", "asset")
	apAcc, grirAcc, wht53, wht3 := account("2100", "liability"), account("2110", "liability"), account("2130", "liability"), account("2131", "liability")
	expense := account("5200", "expense")
	header := a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "1000", "name": "h", "type": "asset", "postable": false}), 201, "header").id()

	// ---------- เลขที่เอกสาร: สมุดรายวันตั้งรูปแบบเอง (พ.ศ. + เดือน) ----------
	a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "x", "number_template": "{prefix}{seq}{seq}"}), 400, "journal template with two seq")
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "จ่าย", "number_template": "{prefix}{bb}{mm}-{seq:4}"}), 201, "journal").data()
	if jv["number_template"] != "{prefix}{bb}{mm}-{seq:4}" {
		t.Fatalf("journal = %v", jv)
	}

	// ---------- รูปแบบเลขที่ของเอกสาร (organization) ----------
	nums := objs(a.expect(a.do("GET", base+"/document-numberings", owner, nil), 200, "numberings").body["data"])
	if len(nums) != 8 || nums[0]["doc_type"] != "inventory.adjustment" || nums[0]["custom"] != false {
		t.Fatalf("numberings = %v", nums)
	}
	a.expect(a.do("PUT", base+"/document-numberings/purchasing.payment", owner, obj{"prefix": "PV", "template": "{prefix}{dd}{seq}"}), 400, "unknown variable")
	a.expect(a.do("PUT", base+"/document-numberings/sales.invoice", owner, obj{"prefix": "SI", "template": "{prefix}{seq}"}), 404, "unregistered document type")
	pv := a.expect(a.do("PUT", base+"/document-numberings/purchasing.payment", owner, obj{"prefix": "pv", "template": "{prefix}{yy}{mm}-{seq:5}"}), 200, "payment numbering").data()
	if pv["prefix"] != "PV" || pv["custom"] != true {
		t.Fatalf("payment numbering = %v", pv)
	}

	// ---------- ทะเบียนเงินสด / ธนาคาร ----------
	a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BBL", "name": "x", "type": "bank", "gl_account_id": bankGL}), 400, "bank without account no")
	a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "C2", "name": "x", "type": "cash", "gl_account_id": header}), 422, "header GL account")
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": cashGL}), 201, "cash").id()
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BBL", "name": "กรุงเทพ", "type": "bank", "bank_name": "BBL", "account_no": "123-4-56789-0", "gl_account_id": bankGL}), 201, "bank").id()
	if r := a.expect(a.do("GET", base+"/cash-accounts?type=bank", owner, nil), 200, "list banks"); r.total() != 1 {
		t.Fatalf("banks = %d", r.total())
	}

	// ---------- ภาษีหัก ณ ที่จ่าย / ค่าตั้ง ----------
	a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "X", "name": "x", "rate": "0"}), 400, "zero rate")
	svc3 := a.expect(a.do("POST", base+"/withholding-tax-types", owner, obj{"code": "SVC", "name": "ค่าบริการ", "rate": "3"}), 201, "wht 3%").id()
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "journal_id": jv["id"], "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc, "wht_pnd3_account_id": wht3, "wht_pnd53_account_id": wht53}), 200, "settings")

	// ---------- ใบแจ้งหนี้ 2 ใบ (ค่าบริการ 5,000 + VAT 350 = 5,350 ต่อใบ) ----------
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ค่าบริการ", "type": "service", "uom_id": pcs, "purchase_account_id": expense}), 201, "service").id()
	sup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "V", "name": "V Co.", "tax_id": "0105551234567", "payment_term_days": 0, "vat_registered": true, "tax_type": "juristic"}), 201, "supplier").id()
	a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "Y", "name": "y", "vat_registered": true}), 400, "supplier without tax_type")
	po := a.expect(a.do("POST", base+"/purchase-orders", owner, obj{"supplier_id": sup, "date": "2026-03-01", "lines": []obj{{"item_id": svc, "quantity": "2", "unit_price": "5000"}}}), 201, "PO").id()
	a.expect(a.do("PATCH", base+"/purchase-orders/"+po, owner, obj{"status": "approved"}), 200, "approve")
	a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": po, "date": "2026-03-01", "lines": []obj{{"order_line_no": 1, "quantity": "2"}}}), 201, "receipt")
	invoice := func(no string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"order_id": po, "supplier_invoice_no": no, "date": "2026-03-01",
			"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "invoice "+no).id()
	}
	invA, invB := invoice("A-1"), invoice("B-1")

	// ---------- จ่าย: A เต็ม (หัก 3% ของ 5,000 = 150) + B บางส่วน 2,675 / เงินสด 2,000 + ธนาคาร 5,875 ----------
	payment := func(methods ...obj) obj {
		return obj{"supplier_id": sup, "date": "2026-03-10", "lines": []obj{
			{"invoice_id": invA, "amount": "5350", "wht_type_id": svc3},
			{"invoice_id": invB, "amount": "2675"},
		}, "methods": methods}
	}
	method := func(acc, amount string) obj {
		return obj{"cash_account_id": acc, "amount": amount, "reference": "CHQ-1"}
	}
	a.expect(a.do("POST", base+"/supplier-payments", owner, payment(method(cash, "2000"), method(bank, "6025"))), 400, "methods total != amount - wht")
	over := payment(method(cash, "9000"))
	over["lines"] = []obj{{"invoice_id": invB, "amount": "5350.01"}}
	a.expect(a.do("POST", base+"/supplier-payments", owner, over), 422, "pay more than outstanding")
	p := a.expect(a.do("POST", base+"/supplier-payments", owner, payment(method(cash, "2000"), method(bank, "5875"))), 201, "payment").data()
	lines := objs(p["lines"])
	if p["document_no"] != "PV2603-00001" || p["amount"] != "8025" || p["wht"] != "150" || p["paid"] != "7875" || p["tax_form"] != "pnd53" ||
		p["entry_no"] != "PJ6903-0004" || lines[0]["wht_base"] != "5000" || lines[0]["wht_rate"] != "3" || lines[0]["invoice"].(obj)["supplier_invoice_no"] != "A-1" {
		t.Fatalf("payment = %v", p)
	}
	// รายการที่ 4 ของสมุด PJ ในเดือนนี้ (รับสินค้า + แจ้งหนี้ 2 ใบ ลงก่อน)
	// รายการบัญชี: 1 ผังบัญชี = 1 บรรทัด (เจ้าหนี้ / เงินสด / ธนาคาร / ภาษีหัก ณ ที่จ่าย)
	entry := a.expect(a.do("GET", base+"/journal-entries/"+p["entry_id"].(string), owner, nil), 200, "payment entry").data()
	got := map[string]string{}
	for _, l := range objs(entry["lines"]) {
		got[l["account"].(obj)["code"].(string)] = l["debit"].(string) + "/" + l["credit"].(string)
	}
	want := map[string]string{"2100": "8025/0", "1101": "0/2000", "1102": "0/5875", "2130": "0/150"}
	if len(got) != len(want) || len(objs(entry["lines"])) != 4 {
		t.Fatalf("payment entry lines = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("entry %s = %s, want %s", k, got[k], v)
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
	if r := wht("&form=pnd53"); len(objs(r["rows"])) != 1 || r["total_base"] != "5000" || r["total_wht"] != "150" || objs(r["rows"])[0]["tax_id"] != "0105551234567" {
		t.Fatalf("withholding pnd53 = %v", r)
	}
	if r := wht("&form=pnd3"); len(r["rows"].([]any)) != 0 || r["total_wht"] != "0" {
		t.Fatalf("withholding pnd3 = %v", r)
	}
	a.expect(a.do("GET", base+"/reports?book=withholding-tax&start_date=2026-03-01&end_date=2026-03-31&form=pnd1", owner, nil), 400, "bad form")

	// ---------- ยกเลิกใบจ่าย: ยอดค้างกลับ, ภาษีหัก ณ ที่จ่ายหาย, เลขถัดไปต่อเนื่อง ----------
	a.expect(a.do("PATCH", base+"/supplier-payments/"+p["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel payment")
	a.expect(a.do("PATCH", base+"/supplier-payments/"+p["id"].(string), owner, obj{"status": "cancelled"}), 422, "cancel twice")
	if outstanding(invA) != "5350" || aging("2026-03-31") != "10700" || wht("")["total_wht"] != "0" {
		t.Fatalf("after cancel: A=%s aging=%s", outstanding(invA), aging("2026-03-31"))
	}
	p2 := a.expect(a.do("POST", base+"/supplier-payments", owner, obj{"supplier_id": sup, "date": "2026-03-20",
		"lines": []obj{{"invoice_id": invA, "amount": "5350"}}, "methods": []obj{method(bank, "5350")}}), 201, "payment 2").data()
	if p2["document_no"] != "PV2603-00002" || p2["wht"] != "0" {
		t.Fatalf("payment 2 = %v", p2)
	}
	if r := a.expect(a.do("GET", base+"/supplier-payments?status=posted&supplier_id="+sup, owner, nil), 200, "list payments"); r.total() != 1 {
		t.Fatalf("posted payments = %d", r.total())
	}
}
