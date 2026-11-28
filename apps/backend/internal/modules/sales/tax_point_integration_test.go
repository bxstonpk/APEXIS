package sales_test

import (
	"testing"
)

// TestTaxPointFlow: ขายบริการ (ม.78/1) — ภาษีขายยังไม่ถึงกำหนดจนรับเงินที่ finance / ออกใบกำกับก่อนรับเงิน,
// ใบลดหนี้ก่อน / หลังภาษีถึงกำหนด, มัดจำ (ต้องรับเงินก่อนหัก), ใบวางบิล, รายงานภาษีขาย / ภาษียังไม่ถึงกำหนด / ภ.พ.30
// และประเภทธุรกิจกำหนดเอกสารที่ออกได้ — บริษัทสร้างด้วย hook ตั้งค่าเริ่มต้น (ผังบัญชี / ค่าตั้งตามประเภทธุรกิจ) เหมือน main.go
func TestTaxPointFlow(t *testing.T) {
	a, owner := setupApp(t, true)
	errCode := func(r resp) string {
		e, _ := r.body["error"].(obj)
		code, _ := e["code"].(string)
		return code
	}
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "svc", "name": "Service Co", "business_type": "service"}), 201, "service company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")

	accountID := func(code string) string {
		t.Helper()
		rows := objs(a.expect(a.do("GET", base+"/accounts?q="+code, owner, nil), 200, "account "+code).body["data"])
		for _, r := range rows {
			if r["code"] == code {
				return r["id"].(string)
			}
		}
		t.Fatalf("account %s not in chart", code)
		return ""
	}
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "JOB", "name": "งาน"}), 201, "uom").id()
	s1 := a.expect(a.do("POST", base+"/items", owner, obj{"code": "S1", "name": "ค่าบริการ", "type": "service", "uom_id": pcs}), 201, "service item").id()
	g1 := a.expect(a.do("POST", base+"/items", owner, obj{"code": "G1", "name": "อุปกรณ์", "type": "non_stock", "uom_id": pcs}), 201, "goods item").id()
	cust := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C", "name": "ลูกค้า C", "tax_id": "0105551234567", "tax_type": "juristic", "payment_term_days": 30}), 201, "customer").id()
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": accountID("1101010100")}), 201, "cash").id()

	entryLines := func(id any) map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, l := range objs(a.expect(a.do("GET", base+"/journal-entries/"+id.(string), owner, nil), 200, "entry").data()["lines"]) {
			out[l["account"].(obj)["code"].(string)] = l["debit"].(string) + "/" + l["credit"].(string)
		}
		return out
	}
	sameLines := func(what string, got, want map[string]string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s entry = %v, want %v", what, got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Fatalf("%s %s = %s, want %s (entry %v)", what, k, got[k], v, got)
			}
		}
	}
	sv := func(date, price string, extra obj) obj {
		t.Helper()
		body := obj{"customer_id": cust, "date": date, "lines": []obj{{"item_id": s1, "quantity": "1", "unit_price": price}}}
		for k, v := range extra {
			body[k] = v
		}
		return a.expect(a.do("POST", base+"/service-invoices", owner, body), 201, "service invoice "+date).data()
	}
	getSV := func(id any) obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/service-invoices/"+id.(string), owner, nil), 200, "service invoice").data()
	}
	receipt := func(date string, lines []obj, received string) obj {
		t.Helper()
		return a.expect(a.do("POST", base+"/receipts", owner, obj{"kind": "customer", "customer_id": cust, "date": date, "lines": lines,
			"methods": []obj{{"cash_account_id": cash, "amount": received}}}), 201, "receipt "+date).data()
	}
	book := func(q string) obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book="+q, owner, nil), 200, q).data()
	}

	// ---------- ประเภทธุรกิจ: บริการออกขายสินค้าไม่ได้ ----------
	if r := a.do("POST", base+"/sales-invoices", owner, obj{"customer_id": cust, "date": "2026-03-05",
		"lines": []obj{{"item_id": s1, "quantity": "1", "unit_price": "1"}}}); r.status != 422 || errCode(r) != "document_not_available_for_business_type" {
		t.Fatalf("goods sale in a service company = %d %v", r.status, r.body)
	}
	if r := a.do("POST", base+"/service-invoices", owner, obj{"customer_id": cust, "date": "2026-03-05",
		"lines": []obj{{"item_id": g1, "quantity": "1", "unit_price": "1"}}}); r.status != 422 || errCode(r) != "service_invoice_goods_line" {
		t.Fatalf("goods line on service invoice = %d %v", r.status, r.body)
	}

	// ---------- SV1 1,000 + VAT 70: ภาษียังไม่ถึงกำหนด ----------
	sv1 := sv("2026-03-05", "1000", nil)
	if sv1["document_no"] != "SV2026-000001" || sv1["kind"] != "service_invoice" || sv1["vat_status"] != "undue" || sv1["vat_undue"] != "70" ||
		sv1["totals"].(obj)["gross"] != "1070" || objs(sv1["lines"])[0]["item"].(obj)["type"] != "service" {
		t.Fatalf("SV1 = %v", sv1)
	}
	sameLines("SV1", entryLines(sv1["entry_id"]), map[string]string{"1103010100": "1070/0", "4101000000": "0/1000", "2102020900": "0/70"})

	// ---------- รับเงินครึ่งหนึ่ง (ถูกหัก 15): ภาษีถึงกำหนด 35 ----------
	r1 := receipt("2026-03-10", []obj{{"invoice_id": sv1["id"], "amount": "535", "wht": "15"}}, "520")
	if r1["tax_invoice"] != true || objs(r1["vat_lines"])[0]["vat"] != "35" || objs(r1["vat_lines"])[0]["base"] != "500" {
		t.Fatalf("receipt 1 = %v", r1)
	}
	sameLines("R1", entryLines(r1["entry_id"]), map[string]string{"1101010100": "520/0", "1106030000": "15/0", "1103010100": "0/535",
		"2102020900": "35/0", "2102021100": "0/35"})
	if s := getSV(sv1["id"]); s["vat_status"] != "partial" || s["vat_undue"] != "35" || len(objs(s["vat_recognitions"])) != 1 ||
		objs(s["vat_recognitions"])[0]["kind"] != "receipt" {
		t.Fatalf("SV1 after half = %v", s)
	}
	if r := book("undue-output-vat&end_date=2026-03-31"); len(objs(r["rows"])) != 1 || r["totals"].(obj)["vat_undue"] != "35" {
		t.Fatalf("undue report = %v", r)
	}
	// รับส่วนที่เหลือ -> ภาษีที่เหลือทั้งหมด; ยกเลิกแล้วกลับมาค้าง
	r2 := receipt("2026-03-12", []obj{{"invoice_id": sv1["id"], "amount": "535"}}, "535")
	if objs(r2["vat_lines"])[0]["vat"] != "35" || getSV(sv1["id"])["vat_undue"] != "0" {
		t.Fatalf("receipt 2 = %v", r2)
	}
	if r := book("undue-output-vat&end_date=2026-03-31"); len(objs(r["rows"])) != 0 {
		t.Fatalf("undue after full payment = %v", r)
	}
	a.expect(a.do("PATCH", base+"/receipts/"+r2["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel receipt 2")
	if s := getSV(sv1["id"]); s["vat_undue"] != "35" || s["outstanding"] != "535" {
		t.Fatalf("SV1 after cancelling receipt 2 = %v", s)
	}

	// ---------- ใบลดหนี้ก่อนภาษีถึงกำหนด = ลดภาษียังไม่ถึงกำหนด (ไม่อยู่ในรายงานภาษีขาย) ----------
	cn1 := a.expect(a.do("POST", base+"/sales-credit-notes", owner, obj{"invoice_id": sv1["id"], "date": "2026-03-13", "kind": "allowance",
		"reason": "ลดค่าบริการ", "lines": []obj{{"invoice_line_no": 1, "amount": "100"}}}), 201, "credit note on undue").data()
	if cn1["vat_kind"] != "undue" || cn1["invoice"].(obj)["kind"] != "service_invoice" || getSV(sv1["id"])["vat_undue"] != "28" {
		t.Fatalf("credit note undue = %v", cn1)
	}
	sameLines("CN1", entryLines(cn1["entry_id"]), map[string]string{"4104000000": "100/0", "2102020900": "7/0", "1103010100": "0/107"})

	// ---------- SV2 2,000: ใบกำกับภาษีก่อนรับเงิน -> ภาษี 140 ถึงกำหนดทั้งหมด ----------
	sv2 := sv("2026-03-15", "2000", nil)
	tx := a.expect(a.do("POST", base+"/service-invoices/"+sv2["id"].(string)+"/tax-invoice", owner, obj{"date": "2026-03-16"}), 200, "tax invoice").data()
	if tx["tax_invoice"].(obj)["no"] != "TX2026-000001" || tx["vat_undue"] != "0" || tx["vat_status"] != "output" {
		t.Fatalf("tax invoice = %v", tx)
	}
	a.expect(a.do("POST", base+"/service-invoices/"+sv2["id"].(string)+"/tax-invoice", owner, obj{"date": "2026-03-16"}), 422, "second tax invoice")
	cn2 := a.expect(a.do("POST", base+"/sales-credit-notes", owner, obj{"invoice_id": sv2["id"], "date": "2026-03-17", "kind": "allowance",
		"reason": "ลดราคา", "lines": []obj{{"invoice_line_no": 1, "amount": "100"}}}), 201, "credit note after tax invoice").data()
	if cn2["vat_kind"] != "output" {
		t.Fatalf("credit note output = %v", cn2)
	}
	r3 := receipt("2026-03-18", []obj{{"invoice_id": sv2["id"], "amount": "2033"}}, "2033")
	if r3["tax_invoice"] != false || len(objs(r3["vat_lines"])) != 0 {
		t.Fatalf("receipt after tax invoice recognized vat = %v", r3)
	}
	vat := book("output-vat&start_date=2026-03-01&end_date=2026-03-31")
	kinds := map[string]string{}
	for _, r := range objs(vat["rows"]) {
		kinds[r["kind"].(string)] = r["tax"].(string)
		if r["kind"] == "tax_invoice" && (r["document_id"] != sv2["id"] || r["reference_id"] != sv2["id"]) {
			t.Fatalf("tax invoice row = %v", r)
		}
	}
	if len(kinds) != 3 || kinds["receipt"] != "35" || kinds["tax_invoice"] != "140" || kinds["credit_note"] != "-7" || vat["total_tax"] != "168" || vat["total_base"] != "2400" {
		t.Fatalf("output VAT = %v", vat)
	}

	// ---------- มัดจำ 300 + VAT 21: ภาษีเกิดตามใบ; หักได้เมื่อรับเงินครบที่ finance ----------
	dp := a.expect(a.do("POST", base+"/sales-deposits", owner, obj{"customer_id": cust, "date": "2026-03-20", "description": "มัดจำงาน", "amount": "300",
		"price_includes_vat": false}), 201, "deposit").data()
	if dp["document_no"] != "DP2026-000001" || dp["kind"] != "deposit" || dp["totals"].(obj)["gross"] != "321" || dp["outstanding"] != "321" {
		t.Fatalf("deposit = %v", dp)
	}
	sameLines("DP", entryLines(dp["entry_id"]), map[string]string{"1103010100": "321/0", "2102021000": "0/300", "2102021100": "0/21"})
	depUse := obj{"deposits": []obj{{"deposit_id": dp["id"], "amount": "300"}}}
	if r := a.do("POST", base+"/service-invoices", owner, obj{"customer_id": cust, "date": "2026-03-22", "deposits": depUse["deposits"],
		"lines": []obj{{"item_id": s1, "quantity": "1", "unit_price": "1000"}}}); r.status != 422 || errCode(r) != "deposit_not_paid" {
		t.Fatalf("unpaid deposit applied = %d %v", r.status, r.body)
	}
	open := objs(a.expect(a.do("GET", base+"/receipts/open-invoices?customer_id="+cust, owner, nil), 200, "open invoices").body["data"])
	foundDP := false
	for _, o := range open {
		if o["id"] == dp["id"] && o["kind"] == "deposit" {
			foundDP = true
		}
		if o["id"] == sv1["id"] && (o["kind"] != "service_invoice" || o["vat_undue"] != "28") {
			t.Fatalf("open SV1 = %v", o)
		}
	}
	if !foundDP {
		t.Fatalf("deposit not in open invoices: %v", open)
	}
	receipt("2026-03-21", []obj{{"invoice_id": dp["id"], "amount": "321"}}, "321")
	od := objs(a.expect(a.do("GET", base+"/sales-deposits/open?customer_id="+cust, owner, nil), 200, "open deposits").body["data"])
	if len(od) != 1 || od[0]["remaining_amount"] != "300" || od[0]["vat"] != "21" || od[0]["vat_rate"] != "7" {
		t.Fatalf("open deposits = %v", od)
	}
	sv3 := sv("2026-03-22", "1000", depUse)
	if sv3["deposit_deducted"] != "321" || sv3["outstanding"] != "749" || sv3["vat_undue"] != "49" || len(objs(sv3["deposits_applied"])) != 1 {
		t.Fatalf("SV3 with deposit = %v", sv3)
	}
	sameLines("SV3", entryLines(sv3["entry_id"]), map[string]string{"1103010100": "749/0", "2102021000": "300/0", "4101000000": "0/1000", "2102020900": "0/49"})
	a.expect(a.do("PATCH", base+"/sales-deposits/"+dp["id"].(string), owner, obj{"status": "cancelled"}), 422, "cancel applied deposit")
	if n := len(objs(a.expect(a.do("GET", base+"/sales-deposits/open?customer_id="+cust, owner, nil), 200, "open deposits").body["data"])); n != 0 {
		t.Fatalf("deposit still open after full use: %d", n)
	}

	// ---------- ใบวางบิล SV1 + SV3 -> รับเงินจากใบวางบิล -> ชำระครบ ----------
	a.expect(a.do("POST", base+"/billing-notes", owner, obj{"customer_id": cust, "date": "2026-03-25", "due_date": "2026-04-05",
		"invoice_ids": []string{sv2["id"].(string)}}), 422, "bill a paid invoice")
	bn := a.expect(a.do("POST", base+"/billing-notes", owner, obj{"customer_id": cust, "date": "2026-03-25", "due_date": "2026-04-05",
		"invoice_ids": []string{sv1["id"].(string), sv3["id"].(string)}}), 201, "billing note").data()
	if bn["document_no"] != "BN2026-000001" || bn["status"] != "open" || bn["total"] != "2140" || bn["outstanding"] != "1177" || len(objs(bn["invoices"])) != 2 {
		t.Fatalf("billing note = %v", bn)
	}
	a.expect(a.do("POST", base+"/billing-notes", owner, obj{"customer_id": cust, "date": "2026-03-25", "due_date": "2026-04-05",
		"invoice_ids": []string{sv1["id"].(string)}}), 422, "invoice on two open billing notes")
	billed := objs(a.expect(a.do("GET", base+"/receipts/open-invoices?customer_id="+cust+"&billing_note_id="+bn["id"].(string), owner, nil), 200, "billed").body["data"])
	if len(billed) != 2 {
		t.Fatalf("open invoices of billing note = %v", billed)
	}
	r5 := a.expect(a.do("POST", base+"/receipts", owner, obj{"kind": "customer", "customer_id": cust, "date": "2026-03-28", "billing_note_id": bn["id"],
		"lines":   []obj{{"invoice_id": sv1["id"], "amount": "428"}, {"invoice_id": sv3["id"], "amount": "749"}},
		"methods": []obj{{"cash_account_id": cash, "amount": "1177"}}}), 201, "receipt from billing note").data()
	if len(objs(r5["vat_lines"])) != 2 {
		t.Fatalf("receipt 5 = %v", r5)
	}
	if b := a.expect(a.do("GET", base+"/billing-notes/"+bn["id"].(string), owner, nil), 200, "billing note").data(); b["status"] != "paid" || b["outstanding"] != "0" {
		t.Fatalf("billing note after payment = %v", b)
	}
	if rows := objs(a.expect(a.do("GET", base+"/billing-notes?status=paid", owner, nil), 200, "paid notes").body["data"]); len(rows) != 1 {
		t.Fatalf("paid billing notes = %v", rows)
	}
	vat = book("output-vat&start_date=2026-03-01&end_date=2026-03-31")
	// 168 + มัดจำ 21 + ถึงกำหนดจากใบรับเงิน 5 (SV1 28 + SV3 49)
	if vat["total_tax"] != "266" {
		t.Fatalf("output VAT after billing receipt = %v", vat)
	}
	if vs := book("vat-summary&start_date=2026-03-01&end_date=2026-03-31"); vs["output"].(obj)["tax"] != "266" {
		t.Fatalf("vat summary = %v", vs)
	}

	// ยกเลิกใบรับเงิน -> ยกเลิก SV3 -> มัดจำกลับมาหักได้; ยกเลิกใบวางบิล
	a.expect(a.do("PATCH", base+"/receipts/"+r5["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel receipt 5")
	a.expect(a.do("PATCH", base+"/service-invoices/"+sv3["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel SV3")
	if od := objs(a.expect(a.do("GET", base+"/sales-deposits/open?customer_id="+cust, owner, nil), 200, "open deposits").body["data"]); len(od) != 1 || od[0]["remaining_amount"] != "300" {
		t.Fatalf("deposit after cancelling SV3 = %v", od)
	}
	if b := a.expect(a.do("PATCH", base+"/billing-notes/"+bn["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel billing note").data(); b["status"] != "cancelled" {
		t.Fatalf("billing note cancel = %v", b)
	}
	a.expect(a.do("GET", base+"/sales-invoices/"+sv1["id"].(string), owner, nil), 404, "service invoice via goods endpoint")

	// ---------- ใบสั่งขาย -> ขายบริการ ----------
	so := a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": cust, "date": "2026-03-26",
		"lines": []obj{{"item_id": s1, "quantity": "2", "unit_price": "500"}}}), 201, "order").id()
	a.expect(a.do("PATCH", base+"/sales-orders/"+so, owner, obj{"status": "approved"}), 200, "approve")
	a.expect(a.do("POST", base+"/sales-invoices", owner, obj{"order_id": so, "date": "2026-03-27", "lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 422, "order to goods sale")
	fromSO := a.expect(a.do("POST", base+"/service-invoices", owner, obj{"order_id": so, "date": "2026-03-27",
		"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "order to service invoice").data()
	if fromSO["order"].(obj)["order_no"] != "SO2026-000001" || fromSO["totals"].(obj)["gross"] != "535" || fromSO["vat_status"] != "undue" {
		t.Fatalf("SV from order = %v", fromSO)
	}

	// ---------- สิทธิ์ ----------
	a.expect(a.do("POST", "/api/v1/users", "", obj{"email": "clerk@example.com", "name": "clerk", "password": "secret-pass-1"}), 201, "register clerk")
	clerk := a.expect(a.do("POST", "/api/v1/auth/tokens", "", obj{"email": "clerk@example.com", "password": "secret-pass-1"}), 201, "login clerk").data()["access_token"].(string)
	role := a.expect(a.do("POST", base+"/roles", owner, obj{"code": "viewer", "name": "Viewer", "permissions": []string{"sales.invoice:read"}}), 201, "role").id()
	a.expect(a.do("POST", base+"/members", owner, obj{"email": "clerk@example.com", "role_ids": []string{role}}), 201, "member")
	a.expect(a.do("GET", base+"/service-invoices", clerk, nil), 200, "clerk lists service invoices")
	a.expect(a.do("GET", base+"/billing-notes", clerk, nil), 403, "clerk billing notes")
	a.expect(a.do("POST", base+"/service-invoices/"+sv1["id"].(string)+"/tax-invoice", clerk, obj{"date": "2026-03-30"}), 403, "clerk tax invoice")

	// ---------- ประเภทธุรกิจ: ซื้อขายออกขายบริการไม่ได้ (รวมจากใบสั่งขาย) ----------
	trading := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "trd", "name": "Trading Co", "business_type": "trading"}), 201, "trading company").id()
	tb := "/api/v1/companies/" + trading
	if r := a.do("POST", tb+"/service-invoices", owner, obj{"customer_id": cust, "date": "2026-03-05",
		"lines": []obj{{"item_id": s1, "quantity": "1", "unit_price": "1"}}}); r.status != 422 || errCode(r) != "document_not_available_for_business_type" {
		t.Fatalf("service invoice in a trading company = %d %v", r.status, r.body)
	}
	if r := a.do("POST", tb+"/service-invoices", owner, obj{"order_id": so, "date": "2026-03-05",
		"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}); r.status != 422 || errCode(r) != "document_not_available_for_business_type" {
		t.Fatalf("order to service invoice in a trading company = %d %v", r.status, r.body)
	}
}
